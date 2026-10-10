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
	jumpDelayTicks   = 10    // ticks between jumps while jump is held

	// StepHeight is how high the player walks up without jumping
	// (slabs, stairs, carpets): the step_height attribute.
	StepHeight = 0.6

	// Water (LivingEntity.travelInFluid, Entity.updateFluidHeightAndDoFluidPushing).
	waterSlowdown       = 0.8   // horizontal drag in water
	waterSlowdownSprint = 0.9   // ...while sprint-swimming
	waterAccel          = 0.02  // input acceleration in water
	waterVerticalDrag   = 0.8   // vertical drag in water
	waterGravity        = 0.005 // gravity / 16
	waterSwimUp         = 0.04  // jump held in water (jumpInLiquid)
	waterSinkDown       = 0.04  // sneak held in water (goDownInWater)
	waterJumpThreshold  = 0.4   // standing in water shallower than this jumps normally
	waterClimbOutBoost  = 0.3   // vertical speed when swimming against a ledge
	sourceWaterHeight   = 8.0 / 9.0

	// Ladders and vines (LivingEntity.handleOnClimbable).
	climbMaxHorizontal = 0.15
	climbMaxFall       = 0.15
	climbUpSpeed       = 0.2

	collisionEpsilon = 1e-7
)

// collider is the part of the world the physics step needs.
type collider interface {
	// appendBoxes appends the collision boxes of block (x, y, z) in world
	// coordinates. Blocks in unloaded chunks are full cubes, so the bot
	// does not fall out of the world before its chunks arrive.
	appendBoxes(dst []aabb, x, y, z int) []aabb
	// waterHeight is the water surface height inside block (x, y, z),
	// 0 if there is no water.
	waterHeight(x, y, z int) float64
	// climbable reports ladders, vines and other climbable blocks.
	climbable(x, y, z int) bool
}

// body is the simulated player: feet position, velocity and contact state.
type body struct {
	X, Y, Z    float64
	VX, VY, VZ float64
	OnGround   bool

	// Results of the last tick, used by the next one and by the pathfinder.
	HorizontalCollision bool
	InWater             bool
	OnClimbable         bool
	JumpDelay           int
}

type aabb struct {
	minX, minY, minZ float64
	maxX, maxY, maxZ float64
}

// The player box exactly as the server builds it: vanilla stores entity
// dimensions as float32 (0.6f, 1.8f), so the half width is 0.30000001192.
// Using 0.3 leaves the bot ~1e-8 inside a wall it walks into, and Paper
// then rejects the move ("clipped into block") and teleports the bot back.
var (
	playerHalfWidth = float64(float32(PlayerWidth) / 2)
	playerBoxHeight = float64(float32(PlayerHeight))
)

func playerBox(x, y, z float64) aabb {
	h := playerHalfWidth
	return aabb{x - h, y, z - h, x + h, y + playerBoxHeight, z + h}
}

func (a aabb) offset(dx, dy, dz float64) aabb {
	return aabb{a.minX + dx, a.minY + dy, a.minZ + dz, a.maxX + dx, a.maxY + dy, a.maxZ + dz}
}

// expandTowards grows the box in the direction of (dx, dy, dz).
func (a aabb) expandTowards(dx, dy, dz float64) aabb {
	if dx < 0 {
		a.minX += dx
	} else {
		a.maxX += dx
	}
	if dy < 0 {
		a.minY += dy
	} else {
		a.maxY += dy
	}
	if dz < 0 {
		a.minZ += dz
	} else {
		a.maxZ += dz
	}
	return a
}

func (a aabb) deflate(d float64) aabb {
	return aabb{a.minX + d, a.minY + d, a.minZ + d, a.maxX - d, a.maxY - d, a.maxZ - d}
}

func (a aabb) intersects(b aabb) bool {
	return a.minX < b.maxX && a.maxX > b.minX &&
		a.minY < b.maxY && a.maxY > b.minY &&
		a.minZ < b.maxZ && a.maxZ > b.minZ
}

// stepPhysics advances the player by one tick (50 ms), following vanilla
// LivingEntity.aiStep/travel: input and jumping, movement with collisions
// (including stepping up 0.6 blocks), then gravity and drag for the next
// tick. Water and climbable blocks use their own movement rules.
func stepPhysics(w collider, b body, ctrl ControlState, yaw float32) body {
	moveX, moveZ := inputVector(ctrl, yaw)

	// Vanilla zeroes very small velocities at the start of the tick.
	if math.Abs(b.VX) < minVelocity {
		b.VX = 0
	}
	if math.Abs(b.VY) < minVelocity {
		b.VY = 0
	}
	if math.Abs(b.VZ) < minVelocity {
		b.VZ = 0
	}

	// Fluid state at the start of the tick (Entity.baseTick).
	inWater, waterDepth := waterState(w, playerBox(b.X, b.Y, b.Z))
	b.InWater = inWater

	// Jumping and swimming up/down (LivingEntity.aiStep).
	if b.JumpDelay > 0 {
		b.JumpDelay--
	}
	if ctrl.Jump {
		switch {
		case inWater && !(b.OnGround && waterDepth <= waterJumpThreshold):
			b.VY += waterSwimUp // swim up
		case b.OnGround && b.JumpDelay == 0:
			b.VY = math.Max(jumpVelocity, b.VY)
			if ctrl.Sprint {
				yawRad := float64(yaw) * math.Pi / 180.0
				b.VX -= math.Sin(yawRad) * sprintJumpBoost
				b.VZ += math.Cos(yawRad) * sprintJumpBoost
			}
			b.JumpDelay = jumpDelayTicks
		}
	} else {
		b.JumpDelay = 0
	}
	if inWater && ctrl.Sneak {
		b.VY -= waterSinkDown
	}

	if inWater {
		return travelInWater(w, b, ctrl, moveX, moveZ)
	}
	return travelInAir(w, b, ctrl, moveX, moveZ)
}

// travelInAir is vanilla LivingEntity.travelInAir: walking, falling, and
// climbing ladders.
func travelInAir(w collider, b body, ctrl ControlState, moveX, moveZ float64) body {
	wasOnGround := b.OnGround

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

	if onClimbable(w, b) {
		b.VX = clamp(b.VX, -climbMaxHorizontal, climbMaxHorizontal)
		b.VZ = clamp(b.VZ, -climbMaxHorizontal, climbMaxHorizontal)
		b.VY = math.Max(b.VY, -climbMaxFall)
		if b.VY < 0 && ctrl.Sneak {
			b.VY = 0 // sneaking holds the player on the ladder
		}
	}

	b = move(w, b)

	b.OnClimbable = onClimbable(w, b)
	if (b.HorizontalCollision || ctrl.Jump) && b.OnClimbable {
		b.VY = climbUpSpeed
	}

	b.VY = (b.VY - Gravity) * 0.98
	if b.VY < TerminalVelocity {
		b.VY = TerminalVelocity
	}
	b.VX *= friction
	b.VZ *= friction
	return b
}

// travelInWater is vanilla LivingEntity.travelInFluid for water.
func travelInWater(w collider, b body, ctrl ControlState, moveX, moveZ float64) body {
	startY := b.Y
	falling := b.VY <= 0
	drag := waterSlowdown
	if ctrl.Sprint {
		drag = waterSlowdownSprint
	}
	b.VX += moveX * waterAccel
	b.VZ += moveZ * waterAccel

	b = move(w, b)

	b.OnClimbable = onClimbable(w, b)
	if b.HorizontalCollision && b.OnClimbable {
		b.VY = climbUpSpeed
	}
	b.VX *= drag
	b.VY *= waterVerticalDrag
	b.VZ *= drag

	// Entity.getFluidFallingAdjustedMovement.
	if !ctrl.Sprint {
		if falling && math.Abs(b.VY-0.005) >= 0.003 && math.Abs(b.VY-Gravity/16) < 0.003 {
			b.VY = -0.003
		} else {
			b.VY -= waterGravity
		}
	}

	// Swimming against a ledge: jump out if the space 0.6 above is free.
	if b.HorizontalCollision {
		up := b.VY + 0.6 - b.Y + startY
		if isFree(w, playerBox(b.X, b.Y, b.Z).offset(b.VX, up, b.VZ)) {
			b.VY = waterClimbOutBoost
		}
	}
	return b
}

// move is vanilla Entity.move for the player: collide the velocity with the
// world (stepping up to StepHeight when on the ground), move, then zero the
// velocity on each axis that hit something.
func move(w collider, b body) body {
	box := playerBox(b.X, b.Y, b.Z)
	dx, dy, dz := collide(w, box, b.VX, b.VY, b.VZ, b.OnGround)

	collX := dx != b.VX
	collY := dy != b.VY
	collZ := dz != b.VZ

	b.X += dx
	b.Y += dy
	b.Z += dz
	b.HorizontalCollision = collX || collZ
	b.OnGround = collY && b.VY < 0
	if collX {
		b.VX = 0
	}
	if collY {
		b.VY = 0 // landed, or head hit a ceiling
	}
	if collZ {
		b.VZ = 0
	}
	return b
}

// collide is vanilla Entity.collide: clip the movement against the world,
// and if a horizontal move is blocked while on the ground, try stepping up
// onto the obstacle (up to StepHeight) and keep whichever goes farther.
func collide(w collider, box aabb, dx, dy, dz float64, onGround bool) (float64, float64, float64) {
	blocks := boxesIn(w, nil, box.expandTowards(dx, dy, dz))
	vx, vy, vz := collideWithBoxes(blocks, box, dx, dy, dz)

	collX, collY, collZ := vx != dx, vy != dy, vz != dz
	landed := collY && dy < 0
	if !(landed || onGround) || !(collX || collZ) {
		return vx, vy, vz
	}

	base := box
	if landed {
		base = box.offset(0, vy, 0)
	}
	region := base.expandTowards(dx, StepHeight, dz)
	if !landed {
		region = region.expandTowards(0, -1e-5, 0)
	}
	stepBlocks := boxesIn(w, nil, region)
	for _, h := range stepUpHeights(base, stepBlocks, float32(vy)) {
		sx, sy, sz := collideWithBoxes(stepBlocks, base, dx, float64(h), dz)
		if sx*sx+sz*sz > vx*vx+vz*vz {
			return sx, sy + (base.minY - box.minY), sz
		}
	}
	return vx, vy, vz
}

// stepUpHeights is vanilla Entity.collectCandidateStepUpHeights: the heights
// of box tops and bottoms within StepHeight above the box, ascending.
func stepUpHeights(box aabb, blocks []aabb, skip float32) []float32 {
	var hs []float32
	add := func(y float64) {
		f := float32(y - box.minY)
		if f < 0 || f == skip || f > StepHeight {
			return
		}
		for _, h := range hs {
			if h == f {
				return
			}
		}
		hs = append(hs, f)
	}
	for _, b := range blocks {
		add(b.minY)
		add(b.maxY)
	}
	// Insertion sort: there are only a handful.
	for i := 1; i < len(hs); i++ {
		for j := i; j > 0 && hs[j] < hs[j-1]; j-- {
			hs[j], hs[j-1] = hs[j-1], hs[j]
		}
	}
	return hs
}

// collideWithBoxes clips the movement (dx, dy, dz) of box against blocks:
// Y first, then the larger horizontal axis, then the other one, so a
// blocked axis does not cancel movement on the others (sliding along walls).
func collideWithBoxes(blocks []aabb, box aabb, dx, dy, dz float64) (float64, float64, float64) {
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

// boxesIn appends the collision boxes that touch region.
func boxesIn(w collider, dst []aabb, region aabb) []aabb {
	x0, x1 := int(math.Floor(region.minX-collisionEpsilon)), int(math.Floor(region.maxX+collisionEpsilon))
	// Fences and walls are 1.5 blocks tall: look one block lower.
	y0, y1 := int(math.Floor(region.minY-collisionEpsilon))-1, int(math.Floor(region.maxY+collisionEpsilon))
	z0, z1 := int(math.Floor(region.minZ-collisionEpsilon)), int(math.Floor(region.maxZ+collisionEpsilon))
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for z := z0; z <= z1; z++ {
				n := len(dst)
				dst = w.appendBoxes(dst, x, y, z)
				// Keep only the boxes that touch the region.
				kept := dst[:n]
				for _, b := range dst[n:] {
					if b.touches(region) {
						kept = append(kept, b)
					}
				}
				dst = kept
			}
		}
	}
	return dst
}

// touches is intersects including shared faces.
func (a aabb) touches(b aabb) bool {
	const e = collisionEpsilon
	return a.minX <= b.maxX+e && a.maxX >= b.minX-e &&
		a.minY <= b.maxY+e && a.maxY >= b.minY-e &&
		a.minZ <= b.maxZ+e && a.maxZ >= b.minZ-e
}

// isFree reports whether box touches no collision box and no water
// (vanilla Entity.isFree).
func isFree(w collider, box aabb) bool {
	for _, b := range boxesIn(w, nil, box) {
		if b.intersects(box) {
			return false
		}
	}
	for x := int(math.Floor(box.minX)); x < int(math.Ceil(box.maxX)); x++ {
		for y := int(math.Floor(box.minY)); y < int(math.Ceil(box.maxY)); y++ {
			for z := int(math.Floor(box.minZ)); z < int(math.Ceil(box.maxZ)); z++ {
				if w.waterHeight(x, y, z) > 0 {
					return false
				}
			}
		}
	}
	return true
}

// waterState is vanilla Entity.updateFluidHeightAndDoFluidPushing for water
// (without currents): whether the player box touches water, and how deep
// the water is above the feet.
func waterState(w collider, box aabb) (inWater bool, depth float64) {
	box = box.deflate(0.001)
	for x := int(math.Floor(box.minX)); x < int(math.Ceil(box.maxX)); x++ {
		for y := int(math.Floor(box.minY)); y < int(math.Ceil(box.maxY)); y++ {
			for z := int(math.Floor(box.minZ)); z < int(math.Ceil(box.maxZ)); z++ {
				h := w.waterHeight(x, y, z)
				if h <= 0 {
					continue
				}
				top := float64(y) + h
				if top >= box.minY {
					inWater = true
					depth = math.Max(depth, top-box.minY)
				}
			}
		}
	}
	return inWater, depth
}

// onClimbable reports whether the block at the player's feet is a ladder,
// vine or other climbable block.
func onClimbable(w collider, b body) bool {
	return w.climbable(int(math.Floor(b.X)), int(math.Floor(b.Y)), int(math.Floor(b.Z)))
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
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
