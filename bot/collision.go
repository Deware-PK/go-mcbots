package bot

import "math"

// Vanilla player movement constants (LivingEntity.travel / Player).
const (
	jumpVelocity     = 0.42
	sprintJumpBoost  = 0.2  // horizontal boost in facing direction when jumping while sprinting
	baseMoveSpeed    = 0.1  // player movement_speed attribute
	sprintMultiplier = 1.3  // sprinting adds +30% movement speed
	sneakMultiplier  = 0.3  // sneaking_speed attribute (scales the input vector)
	inputScale       = 0.98 // client multiplies forward/strafe input by 0.98
	airAccel         = 0.02 // horizontal acceleration while airborne
	airAccelSprint   = 0.026
	airFriction      = 0.91 // horizontal velocity multiplier while airborne
	defaultSlip      = 0.6  // block slipperiness of almost every block
	groundAccelConst = 0.16277136
	minVelocity      = 0.003 // vanilla zeroes tiny velocities

	collisionEpsilon = 1e-7
)

// collider is the part of the world the physics step needs.
type collider interface {
	IsBlockSolidOrUnloaded(x, y, z int) bool
}

// body is the simulated player: feet position, velocity and ground contact.
type body struct {
	X, Y, Z    float64
	VX, VY, VZ float64
	OnGround   bool
}

type aabb struct {
	minX, minY, minZ float64
	maxX, maxY, maxZ float64
}

func playerBox(x, y, z float64) aabb {
	h := PlayerWidth / 2
	return aabb{x - h, y, z - h, x + h, y + PlayerHeight, z + h}
}

func (a aabb) offset(dx, dy, dz float64) aabb {
	return aabb{a.minX + dx, a.minY + dy, a.minZ + dz, a.maxX + dx, a.maxY + dy, a.maxZ + dz}
}

// stepPhysics advances the player by one tick (50 ms).
//
// Order follows vanilla: apply input and jump, move with collisions, then
// apply gravity and drag to the velocity used next tick.
func stepPhysics(w collider, b body, ctrl ControlState, yaw float32) body {
	moveX, moveZ := inputVector(ctrl, yaw)

	// Vanilla zeroes very small velocities at the start of the tick.
	if math.Abs(b.VX) < minVelocity {
		b.VX = 0
	}
	if math.Abs(b.VZ) < minVelocity {
		b.VZ = 0
	}

	wasOnGround := b.OnGround

	// Jump: vertical impulse, plus a forward boost when sprinting.
	if wasOnGround && ctrl.Jump {
		b.VY = jumpVelocity
		if ctrl.Sprint {
			yawRad := float64(yaw) * math.Pi / 180.0
			b.VX -= math.Sin(yawRad) * sprintJumpBoost
			b.VZ += math.Cos(yawRad) * sprintJumpBoost
		}
	}

	// Friction for this tick depends on whether we started it on the ground.
	friction := airFriction
	var accel float64
	if wasOnGround {
		friction = defaultSlip * airFriction // 0.546
		speed := baseMoveSpeed
		if ctrl.Sprint {
			speed *= sprintMultiplier
		}
		accel = speed * (groundAccelConst / (friction * friction * friction))
	} else {
		accel = airAccel
		if ctrl.Sprint {
			accel = airAccelSprint
		}
	}
	b.VX += moveX * accel
	b.VZ += moveZ * accel

	dx, dy, dz := moveWithCollisions(w, playerBox(b.X, b.Y, b.Z), b.VX, b.VY, b.VZ)
	b.X += dx
	b.Y += dy
	b.Z += dz

	verticalHit := dy != b.VY
	b.OnGround = verticalHit && b.VY < 0
	if verticalHit {
		b.VY = 0 // landed, or head hit a ceiling
	}
	if dx != b.VX {
		b.VX = 0
	}
	if dz != b.VZ {
		b.VZ = 0
	}

	b.VY = (b.VY - Gravity) * 0.98
	if b.VY < TerminalVelocity {
		b.VY = TerminalVelocity
	}
	b.VX *= friction
	b.VZ *= friction
	return b
}

// inputVector returns the horizontal input in world space, as vanilla builds
// it: each pressed key contributes 0.98 (x0.3 while sneaking), and the result
// is normalized only when its length exceeds 1 (e.g. forward + strafe).
func inputVector(ctrl ControlState, yaw float32) (moveX, moveZ float64) {
	var forward, strafe float64
	if ctrl.Forward {
		forward++
	}
	if ctrl.Back {
		forward--
	}
	if ctrl.Left {
		strafe++
	}
	if ctrl.Right {
		strafe--
	}
	forward *= inputScale
	strafe *= inputScale
	if ctrl.Sneak {
		forward *= sneakMultiplier
		strafe *= sneakMultiplier
	}
	if l := math.Hypot(forward, strafe); l > 1 {
		forward /= l
		strafe /= l
	}

	yawRad := float64(yaw) * math.Pi / 180.0
	sin, cos := math.Sin(yawRad), math.Cos(yawRad)
	moveX = strafe*cos - forward*sin
	moveZ = forward*cos + strafe*sin
	return moveX, moveZ
}

// moveWithCollisions clips the movement (dx, dy, dz) of box against solid
// blocks and returns the allowed movement.
//
// Every block in the region swept by the move is considered, so a fast fall
// stops at the first floor instead of tunneling through it. Y is resolved
// first, then X and Z separately (larger first, like vanilla), so a blocked
// axis does not cancel movement on the other one (sliding along walls).
func moveWithCollisions(w collider, box aabb, dx, dy, dz float64) (float64, float64, float64) {
	blocks := solidBlocksIn(w, box, dx, dy, dz)

	dy = clipAxis(blocks, box, dy, 1)
	box = box.offset(0, dy, 0)

	if math.Abs(dx) >= math.Abs(dz) {
		dx = clipAxis(blocks, box, dx, 0)
		box = box.offset(dx, 0, 0)
		dz = clipAxis(blocks, box, dz, 2)
	} else {
		dz = clipAxis(blocks, box, dz, 2)
		box = box.offset(0, 0, dz)
		dx = clipAxis(blocks, box, dx, 0)
	}
	return dx, dy, dz
}

// solidBlocksIn returns the unit boxes of solid blocks touching box expanded by the move.
func solidBlocksIn(w collider, box aabb, dx, dy, dz float64) []aabb {
	x0, x1 := math.Floor(math.Min(box.minX, box.minX+dx)), math.Floor(math.Max(box.maxX, box.maxX+dx))
	y0, y1 := math.Floor(math.Min(box.minY, box.minY+dy)), math.Floor(math.Max(box.maxY, box.maxY+dy))
	z0, z1 := math.Floor(math.Min(box.minZ, box.minZ+dz)), math.Floor(math.Max(box.maxZ, box.maxZ+dz))

	var blocks []aabb
	for x := int(x0); x <= int(x1); x++ {
		for y := int(y0); y <= int(y1); y++ {
			for z := int(z0); z <= int(z1); z++ {
				if w.IsBlockSolidOrUnloaded(x, y, z) {
					fx, fy, fz := float64(x), float64(y), float64(z)
					blocks = append(blocks, aabb{fx, fy, fz, fx + 1, fy + 1, fz + 1})
				}
			}
		}
	}
	return blocks
}

// clipAxis limits movement d of box along axis (0=X, 1=Y, 2=Z) so it stops at
// the first block in the way. Blocks the box already overlaps are ignored.
func clipAxis(blocks []aabb, box aabb, d float64, axis int) float64 {
	if d == 0 {
		return 0
	}
	bMin, bMax := box.axis(axis)
	for _, blk := range blocks {
		if !box.overlapsOther(blk, axis) {
			continue
		}
		kMin, kMax := blk.axis(axis)
		if d > 0 && bMax <= kMin+collisionEpsilon {
			d = math.Min(d, kMin-bMax)
		} else if d < 0 && bMin >= kMax-collisionEpsilon {
			d = math.Max(d, kMax-bMin)
		}
	}
	return d
}

func (a aabb) axis(axis int) (lo, hi float64) {
	switch axis {
	case 0:
		return a.minX, a.maxX
	case 1:
		return a.minY, a.maxY
	default:
		return a.minZ, a.maxZ
	}
}

// overlapsOther reports whether a and b overlap on the two axes other than axis.
func (a aabb) overlapsOther(b aabb, axis int) bool {
	ox := a.maxX > b.minX+collisionEpsilon && a.minX < b.maxX-collisionEpsilon
	oy := a.maxY > b.minY+collisionEpsilon && a.minY < b.maxY-collisionEpsilon
	oz := a.maxZ > b.minZ+collisionEpsilon && a.minZ < b.maxZ-collisionEpsilon
	switch axis {
	case 0:
		return oy && oz
	case 1:
		return ox && oz
	default:
		return ox && oy
	}
}
