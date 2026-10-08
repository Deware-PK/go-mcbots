package bot

import "math"

const (
	jumpVelocity = 0.42
	airAccel     = 0.02 // horizontal acceleration per tick while airborne
	airFriction  = 0.91 // horizontal velocity multiplier per tick while airborne

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

	speed := WalkSpeed
	if ctrl.Sprint {
		speed = SprintSpeed
	}
	if ctrl.Sneak {
		speed = SneakSpeed
	}

	wasOnGround := b.OnGround
	if wasOnGround {
		b.VX = moveX * speed * (1 - Drag)
		b.VZ = moveZ * speed * (1 - Drag)
		if ctrl.Jump {
			b.VY = jumpVelocity
		}
	} else {
		b.VX += moveX * airAccel
		b.VZ += moveZ * airAccel
	}

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
	if !wasOnGround {
		b.VX *= airFriction
		b.VZ *= airFriction
	}
	return b
}

// inputVector returns the normalized horizontal movement direction.
func inputVector(ctrl ControlState, yaw float32) (moveX, moveZ float64) {
	yawRad := float64(yaw) * math.Pi / 180.0
	sin, cos := math.Sin(yawRad), math.Cos(yawRad)
	if ctrl.Forward {
		moveX -= sin
		moveZ += cos
	}
	if ctrl.Back {
		moveX += sin
		moveZ -= cos
	}
	if ctrl.Left {
		moveX += cos
		moveZ += sin
	}
	if ctrl.Right {
		moveX -= cos
		moveZ -= sin
	}
	if l := math.Hypot(moveX, moveZ); l > 0 {
		moveX /= l
		moveZ /= l
	}
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
