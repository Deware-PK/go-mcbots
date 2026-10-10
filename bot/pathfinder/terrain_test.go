package pathfinder

import (
	"errors"
	"testing"
)

func countMoves(path []Node, m MoveType) int {
	n := 0
	for _, p := range path {
		if p.Move == m {
			n++
		}
	}
	return n
}

// Slabs are walked up without jumping: half a block per step.
func TestFindPathSlabStaircase(t *testing.T) {
	w := flat()
	// x=2: slab at 64; x=3: full block at 64; x=4: block + slab; x>=5: 2 blocks.
	w.fillWith(slabBlock, 2, 64, -10, 2, 64, 10)
	w.fill(3, 64, -10, 10, 64, 10)
	w.fillWith(slabBlock, 4, 65, -10, 4, 65, 10)
	w.fill(5, 65, -10, 10, 65, 10)
	path := mustFind(t, w, Vec3{0, 64, 0}, Vec3{7, 66, 0})
	if hasMove(path, MoveJump) {
		t.Errorf("jumped on a slab staircase: %v", path)
	}
}

// A fence (1.5 high) is not jumped over: the path goes around.
func TestFindPathFenceDetour(t *testing.T) {
	w := flat().fillWith(fenceBlock, 3, 64, -4, 3, 64, 4)
	path := mustFind(t, w, Vec3{0, 64, 0}, Vec3{6, 64, 0})
	for _, n := range path {
		if n.Pos.X == 3 && n.Pos.Z >= -4 && n.Pos.Z <= 4 {
			t.Fatalf("path crosses the fence at %v", n.Pos)
		}
	}
}

// Without water moves the far bank is unreachable; with them the bot swims
// across and climbs out (the bank is one block above the water level).
func TestFindPathSwimAcross(t *testing.T) {
	w := newFakeWorld().fill(-10, 58, -10, 20, 58, 10)
	w.fill(-10, 59, -10, 2, 63, 10)                // near bank, ground at 64
	w.fillWith(waterBlock, 3, 59, -10, 10, 63, 10) // lake, surface block y=63
	w.fill(11, 59, -10, 20, 63, 10)                // far bank, ground at 64

	opts := DefaultOptions()
	opts.AllowWater = false
	if _, err := FindPath(Vec3{0, 64, 0}, Vec3{14, 64, 0}, w, opts); !errors.Is(err, ErrNoPath) {
		t.Fatalf("without water: err = %v, want ErrNoPath", err)
	}

	path := mustFind(t, w, Vec3{0, 64, 0}, Vec3{14, 64, 0})
	if countMoves(path, MoveSwim) < 5 {
		t.Errorf("path does not swim: %v", path)
	}
	for _, n := range path {
		if w.IsWater(n.Pos.X, n.Pos.Y, n.Pos.Z) && n.Pos.Y < 63 {
			t.Errorf("path dives to %v; surface swimming is enough", n.Pos)
		}
	}
}

// A ladder on a wall is the only way up.
func TestFindPathLadder(t *testing.T) {
	w := flat()
	w.fill(1, 64, -10, 10, 69, 10)              // 6-block-high plateau from x=1
	w.fillWith(ladderBlock, 0, 64, 0, 0, 69, 0) // ladder on its face at x=0, z=0
	path := mustFind(t, w, Vec3{-4, 64, 0}, Vec3{4, 70, 0})
	if countMoves(path, MoveLadderUp) < 4 {
		t.Errorf("path does not climb the ladder: %v", path)
	}

	opts := DefaultOptions()
	opts.AllowLadder = false
	if _, err := FindPath(Vec3{-4, 64, 0}, Vec3{4, 70, 0}, w, opts); !errors.Is(err, ErrNoPath) {
		t.Errorf("without ladders: err = %v, want ErrNoPath", err)
	}

	// And back down.
	down := mustFind(t, w, Vec3{4, 70, 0}, Vec3{-4, 64, 0})
	if countMoves(down, MoveLadderDown) < 3 {
		t.Errorf("path does not climb down the ladder: %v", down)
	}
}

// Lava is walked around, never through.
func TestFindPathAvoidsLava(t *testing.T) {
	w := flat()
	for z := -3; z <= 3; z++ {
		delete(w.blocks, Vec3{3, 63, z})
		w.blocks[Vec3{3, 63, z}] = lavaBlock
	}
	path := mustFind(t, w, Vec3{0, 64, 0}, Vec3{6, 64, 0})
	for _, n := range path {
		if n.Pos.X == 3 && n.Pos.Z >= -3 && n.Pos.Z <= 3 {
			t.Fatalf("path goes over lava at %v", n.Pos)
		}
	}
}

// Dropping into deep water is safe from heights that would hurt on land.
func TestFindPathDropIntoWater(t *testing.T) {
	w := newFakeWorld().fill(-5, 50, -5, 10, 50, 5)
	w.fill(-5, 51, -5, 2, 62, 5)                 // cliff, ground at 63
	w.fillWith(waterBlock, 3, 51, -5, 10, 55, 5) // pool 7 blocks below
	path := mustFind(t, w, Vec3{0, 63, 0}, Vec3{6, 55, 0})
	if !hasMove(path, MoveDrop) {
		t.Errorf("no drop into the water: %v", path)
	}
}

// A shallow lake (2 deep) whose bottom could be walked: the path swims at
// the surface instead of diving along the bottom.
func TestFindPathSwimsInsteadOfDiving(t *testing.T) {
	w := newFakeWorld().fill(-10, 60, -10, 20, 60, 10)
	w.fill(-10, 61, -10, 2, 62, 10)                // near bank, ground at 63
	w.fillWith(waterBlock, 3, 61, -10, 10, 62, 10) // lake: water 61..62, bottom at 61
	w.fill(11, 61, -10, 20, 62, 10)                // far bank, ground at 63
	path := mustFind(t, w, Vec3{0, 63, 0}, Vec3{14, 63, 0})
	for _, n := range path {
		if w.IsWater(n.Pos.X, n.Pos.Y+1, n.Pos.Z) {
			t.Fatalf("path dives at %v (head under water): %v", n.Pos, path)
		}
	}
	if countMoves(path, MoveSwim) < 5 {
		t.Errorf("path does not swim: %v", path)
	}
}
