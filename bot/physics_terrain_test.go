package bot

import "testing"

// Walking into a bottom slab steps up onto it without jumping.
func TestPhysicsStepUpSlab(t *testing.T) {
	w := fakeWorld{}.floor(63, -5, 10, -5, 5)
	for x := 2; x <= 10; x++ {
		for z := -5; z <= 5; z++ {
			w.set(x, 64, z, bottomSlab)
		}
	}
	b := run(w, body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}, ControlState{Forward: true}, yawPlusX, 30)
	if !approx(b.Y, 64.5) || !b.OnGround {
		t.Fatalf("got y=%v onGround=%v, want standing on the slab at 64.5", b.Y, b.OnGround)
	}
	if b.X < 3 {
		t.Fatalf("stopped at the slab: x=%v", b.X)
	}
}

// A staircase is walked up step by step, one block up per block forward.
func TestPhysicsWalkUpStairs(t *testing.T) {
	w := fakeWorld{}.floor(63, -5, 12, -5, 5)
	// Stairs at x=2 (y 64), x=3 (y 65), x=4 (y 66), then a landing.
	for i := 0; i < 3; i++ {
		for z := -5; z <= 5; z++ {
			w.set(2+i, 64+i, z, stairsPlusX())
			for y := 64; y < 64+i; y++ {
				w.set(2+i, y, z, solid)
			}
		}
	}
	for x := 5; x <= 12; x++ {
		for z := -5; z <= 5; z++ {
			for y := 64; y <= 66; y++ {
				w.set(x, y, z, solid)
			}
		}
	}
	b := run(w, body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}, ControlState{Forward: true}, yawPlusX, 60)
	if !approx(b.Y, 67) || !b.OnGround || b.X < 6 {
		t.Fatalf("got x=%v y=%v onGround=%v, want on the landing at y=67", b.X, b.Y, b.OnGround)
	}
}

// A full block is too high to step onto, and a fence too high to jump.
func TestPhysicsNoStepUpBlockOrFence(t *testing.T) {
	w := fakeWorld{}.floor(63, -5, 10, -5, 5)
	for z := -5; z <= 5; z++ {
		w.set(2, 64, z, solid)
	}
	b := run(w, body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}, ControlState{Forward: true}, yawPlusX, 30)
	if !approx(b.Y, 64) || b.X > 2-PlayerWidth/2+1e-6 {
		t.Fatalf("walked onto a full block: x=%v y=%v", b.X, b.Y)
	}

	w = fakeWorld{}.floor(63, -5, 10, -5, 5)
	for z := -5; z <= 5; z++ {
		// A fence line: posts with full-width bars (like connected fences).
		w.set(2, 64, z, fakeBlock{boxes: []aabb{{0.375, 0, 0, 0.625, 1.5, 1}}})
	}
	b = run(w, body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}, ControlState{Forward: true, Jump: true}, yawPlusX, 60)
	if b.X > 2.375-PlayerWidth/2+1e-6 {
		t.Fatalf("jumped over a fence: x=%v y=%v", b.X, b.Y)
	}
}

func waterPool(x0, x1, z0, z1, floorY, surfaceY int) fakeWorld {
	const bank = 10
	w := fakeWorld{}.floor(floorY, x0-bank, x1+bank, z0-bank, z1+bank)
	for x := x0; x <= x1; x++ {
		for z := z0; z <= z1; z++ {
			for y := floorY; y <= surfaceY; y++ {
				w.set(x, y, z, water)
			}
		}
	}
	// Shore: solid up to the surface level around the pool.
	for x := x0 - bank; x <= x1+bank; x++ {
		for z := z0 - bank; z <= z1+bank; z++ {
			if x >= x0 && x <= x1 && z >= z0 && z <= z1 {
				continue
			}
			for y := floorY; y <= surfaceY; y++ {
				w.set(x, y, z, solid)
			}
		}
	}
	return w
}

// Holding jump keeps the bot afloat at the surface; without it, it sinks.
func TestPhysicsSwimFloat(t *testing.T) {
	w := waterPool(0, 10, 0, 10, 55, 62) // water y 55..62, surface at 62.89
	b := run(w, body{X: 5.5, Y: 60, Z: 5.5}, ControlState{Jump: true}, yawPlusX, 100)
	for i := 0; i < 40; i++ { // bobbing at the surface
		b = stepPhysics(w, b, ControlState{Jump: true}, yawPlusX)
		if b.Y < 62 || b.Y > 63.3 {
			t.Fatalf("floating: y=%v, want feet bobbing at the surface (62..63.3)", b.Y)
		}
	}
	sunk := run(w, b, ControlState{}, yawPlusX, 200)
	if sunk.Y > 59 { // ~0.5 m/s
		t.Fatalf("did not sink without jump: y=%v", sunk.Y)
	}
}

// Swimming forward moves at a slow, steady pace (~2 m/s for vanilla).
func TestPhysicsSwimForward(t *testing.T) {
	w := waterPool(0, 40, 0, 4, 55, 62)
	b := run(w, body{X: 1.5, Y: 62, Z: 2.5}, ControlState{Jump: true}, yawPlusX, 40)
	x0 := b.X
	b = run(w, b, ControlState{Forward: true, Jump: true}, yawPlusX, 40)
	speed := (b.X - x0) / 2
	if speed < 1.5 || speed > 2.6 {
		t.Fatalf("swim speed %.2f m/s, want ~2", speed)
	}
}

// Swimming into the bank climbs out onto it.
func TestPhysicsClimbOutOfWater(t *testing.T) {
	w := waterPool(0, 5, 0, 5, 55, 62) // bank top at 63 for x > 5
	b := run(w, body{X: 3.5, Y: 62.3, Z: 2.5}, ControlState{Jump: true}, yawPlusX, 20)
	b = run(w, b, ControlState{Forward: true, Jump: true}, yawPlusX, 30)
	b = run(w, b, ControlState{}, yawPlusX, 20)
	if !approx(b.Y, 63) || !b.OnGround || b.X < 6.3 {
		t.Fatalf("got x=%v y=%v onGround=%v inWater=%v, want on the bank at y=63", b.X, b.Y, b.OnGround, b.InWater)
	}
}

func ladderTower(height int) fakeWorld {
	w := fakeWorld{}.floor(63, -5, 10, -5, 5)
	// A tower from x=1 to 10, ladder in x=0 on its face.
	for y := 64; y < 64+height; y++ {
		for x := 1; x <= 10; x++ {
			for z := -5; z <= 5; z++ {
				w.set(x, y, z, solid)
			}
		}
		w.set(0, y, 0, ladderPlusX())
	}
	return w
}

// Holding jump (or walking into the ladder) climbs it at ~2.35 m/s, then
// the bot steps off onto the top of the wall.
func TestPhysicsClimbLadder(t *testing.T) {
	w := ladderTower(6) // wall top at 70
	b := body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}
	b = run(w, b, ControlState{Jump: true}, yawPlusX, 20)
	if !b.OnClimbable || b.Y < 65.5 {
		t.Fatalf("after 1 s: y=%v climbable=%v, want climbing", b.Y, b.OnClimbable)
	}
	if b.Y > 67.5 {
		t.Fatalf("climbed too fast: y=%v after 1 s", b.Y)
	}
	b = run(w, b, ControlState{Forward: true, Jump: true}, yawPlusX, 50)
	b = run(w, b, ControlState{}, yawPlusX, 20)
	if !approx(b.Y, 70) || !b.OnGround || b.X < 1.2 {
		t.Fatalf("got x=%v y=%v onGround=%v, want on top of the wall at y=70", b.X, b.Y, b.OnGround)
	}
}

// Letting go on a ladder slides down slowly (0.15 blocks/tick), and
// sneaking holds the position.
func TestPhysicsLadderDescend(t *testing.T) {
	w := ladderTower(8)
	b := body{X: 0.5, Y: 70, Z: 0.5}
	held := run(w, b, ControlState{Sneak: true}, yawPlusX, 20)
	if held.Y < 69.5 {
		t.Fatalf("sneaking on a ladder slid to y=%v", held.Y)
	}
	slid := run(w, b, ControlState{}, yawPlusX, 20)
	if slid.Y > 67.5 || slid.Y < 66.5 {
		t.Fatalf("slid to y=%v in 1 s, want ~67 (0.15 b/t)", slid.Y)
	}
}
