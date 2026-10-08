package bot

import (
	"math"
	"testing"
)

// fakeWorld is a set of solid blocks; everything else is air.
type fakeWorld map[[3]int]bool

func (w fakeWorld) IsBlockSolidOrUnloaded(x, y, z int) bool { return w[[3]int{x, y, z}] }

// floor makes a solid layer at y over [x0,x1]x[z0,z1].
func (w fakeWorld) floor(y, x0, x1, z0, z1 int) fakeWorld {
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			w[[3]int{x, y, z}] = true
		}
	}
	return w
}

const (
	yawPlusX = -90 // facing +X
	yawPlusZ = 0   // facing +Z
)

func run(w collider, b body, ctrl ControlState, yaw float32, ticks int) body {
	for i := 0; i < ticks; i++ {
		b = stepPhysics(w, b, ctrl, yaw)
	}
	return b
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestPhysicsFlatGround(t *testing.T) {
	w := fakeWorld{}.floor(63, -20, 20, -20, 20)
	start := body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}

	idle := run(w, start, ControlState{}, yawPlusZ, 40)
	if !approx(idle.Y, 64) || !idle.OnGround || !approx(idle.X, 0.5) || !approx(idle.Z, 0.5) {
		t.Fatalf("idle: got %+v, want to stay at (0.5, 64, 0.5) on ground", idle)
	}

	walked := run(w, start, ControlState{Forward: true}, yawPlusZ, 20)
	if !approx(walked.Y, 64) || !walked.OnGround {
		t.Fatalf("walk: got y=%v onGround=%v, want y=64 on ground", walked.Y, walked.OnGround)
	}
	if walked.Z < 1.5 {
		t.Fatalf("walk: moved only to z=%v in 20 ticks", walked.Z)
	}
}

func TestPhysicsWallSlide(t *testing.T) {
	w := fakeWorld{}.floor(63, -20, 20, -20, 20)
	// Wall at x=2, two blocks high, along the whole floor.
	for z := -20; z <= 20; z++ {
		w[[3]int{2, 64, z}] = true
		w[[3]int{2, 65, z}] = true
	}
	// Facing +X+Z diagonally (yaw -45) into the wall.
	b := run(w, body{X: 1.5, Y: 64, Z: 0.5, OnGround: true}, ControlState{Forward: true}, -45, 40)

	if b.X > 2-PlayerWidth/2+1e-6 {
		t.Fatalf("went into the wall: x=%v", b.X)
	}
	if !approx(b.X, 2-PlayerWidth/2) {
		t.Fatalf("not pressed against the wall: x=%v", b.X)
	}
	if b.Z < 1.5 {
		t.Fatalf("did not slide along the wall: z=%v", b.Z)
	}
}

func TestPhysicsThreeBlockDrop(t *testing.T) {
	w := fakeWorld{}.floor(63, -5, 5, -5, 5)
	b := run(w, body{X: 0.5, Y: 67, Z: 0.5}, ControlState{}, yawPlusZ, 40)
	if !approx(b.Y, 64) || !b.OnGround {
		t.Fatalf("got y=%v onGround=%v, want landed at 64", b.Y, b.OnGround)
	}
}

func TestPhysicsLongFallNoTunneling(t *testing.T) {
	// Single 1-block-thick floor with nothing below it.
	w := fakeWorld{}.floor(10, -2, 2, -2, 2)
	b := body{X: 0.5, Y: 41, Z: 0.5}

	maxSpeed := 0.0
	for i := 0; i < 200 && !b.OnGround; i++ {
		maxSpeed = math.Max(maxSpeed, -b.VY)
		b = stepPhysics(w, b, ControlState{}, yawPlusZ)
		if b.Y < 11-1e-6 {
			t.Fatalf("tick %d: fell through the floor, y=%v", i, b.Y)
		}
	}
	if !b.OnGround || !approx(b.Y, 11) {
		t.Fatalf("got y=%v onGround=%v, want landed at 11", b.Y, b.OnGround)
	}
	if maxSpeed <= 1 {
		t.Fatalf("test is too weak: max fall speed %v blocks/tick never exceeded the floor thickness", maxSpeed)
	}
}

func TestPhysicsLandOnFloorEdge(t *testing.T) {
	// Center is over air, but the 0.6-wide box overlaps the floor block at x=0.
	w := fakeWorld{}.floor(63, 0, 0, 0, 0)
	b := run(w, body{X: 1.2, Y: 66, Z: 0.5}, ControlState{}, yawPlusZ, 40)
	if !approx(b.Y, 64) || !b.OnGround {
		t.Fatalf("got y=%v onGround=%v, want to land on the block edge at 64", b.Y, b.OnGround)
	}
}

func TestPhysicsJumpUpOneBlock(t *testing.T) {
	w := fakeWorld{}.floor(63, -5, 10, -5, 5)
	w.floor(64, 2, 10, -5, 5) // 1-block step starting at x=2

	b := body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}
	b = run(w, b, ControlState{Forward: true, Jump: true}, yawPlusX, 30)
	b = run(w, b, ControlState{}, yawPlusX, 20) // settle

	if !approx(b.Y, 65) || !b.OnGround {
		t.Fatalf("got y=%v onGround=%v, want standing on the step at 65", b.Y, b.OnGround)
	}
	if b.X < 2.3 {
		t.Fatalf("did not get onto the step: x=%v", b.X)
	}
}

func TestPhysicsCeilingStopsJump(t *testing.T) {
	w := fakeWorld{}.floor(63, -2, 2, -2, 2)
	w.floor(66, -2, 2, -2, 2) // ceiling: only 2 blocks of headroom (player is 1.8)

	b := stepPhysics(w, body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}, ControlState{Jump: true}, yawPlusZ)
	if b.Y+PlayerHeight > 66+1e-6 {
		t.Fatalf("head went into the ceiling: top=%v", b.Y+PlayerHeight)
	}
	if b.VY > 0 {
		t.Fatalf("upward velocity kept after hitting the ceiling: vy=%v", b.VY)
	}
	b = run(w, b, ControlState{}, yawPlusZ, 20)
	if !approx(b.Y, 64) || !b.OnGround {
		t.Fatalf("got y=%v onGround=%v, want back on the floor", b.Y, b.OnGround)
	}
}

func TestPhysicsAirMovement(t *testing.T) {
	// No floor: horizontal velocity follows v = (v + 0.02*input) * 0.91.
	b := body{X: 0.5, Y: 100, Z: 0.5}
	b = stepPhysics(fakeWorld{}, b, ControlState{Forward: true}, yawPlusZ)
	if want := 0.02 * 0.91; !approx(b.VZ, want) {
		t.Fatalf("vz=%v, want %v", b.VZ, want)
	}
	b = stepPhysics(fakeWorld{}, b, ControlState{Forward: true}, yawPlusZ)
	if want := (0.02*0.91 + 0.02) * 0.91; !approx(b.VZ, want) {
		t.Fatalf("vz=%v, want %v", b.VZ, want)
	}
}
