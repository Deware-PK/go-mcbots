package pathfinder

import (
	"errors"
	"testing"
	"time"
)

// fakeWorld is a set of solid blocks; everything else is air. Chunks are
// loaded unless listed in unloaded (by block X/Z).
type fakeWorld struct {
	solid    map[Vec3]bool
	unloaded map[[2]int]bool
	// gate, if set, blocks HasChunk until it is closed.
	gate chan struct{}
}

func newFakeWorld() *fakeWorld {
	return &fakeWorld{solid: map[Vec3]bool{}, unloaded: map[[2]int]bool{}}
}

func (w *fakeWorld) fill(x0, y0, z0, x1, y1, z1 int) *fakeWorld {
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for z := z0; z <= z1; z++ {
				w.solid[Vec3{x, y, z}] = true
			}
		}
	}
	return w
}

func (w *fakeWorld) GetBlock(x, y, z int) uint32 {
	if w.solid[Vec3{x, y, z}] {
		return 1
	}
	return 0
}

func (w *fakeWorld) HasChunk(x, z int) bool {
	if w.gate != nil {
		<-w.gate
	}
	return !w.unloaded[[2]int{x, z}]
}

func (w *fakeWorld) IsBlockSolid(x, y, z int) bool    { return w.solid[Vec3{x, y, z}] }
func (w *fakeWorld) IsPassable(x, y, z int) bool      { return !w.solid[Vec3{x, y, z}] }
func (w *fakeWorld) IsWater(x, y, z int) bool         { return false }
func (w *fakeWorld) IsClimbable(x, y, z int) bool     { return false }
func (w *fakeWorld) IsDangerous(x, y, z int) bool     { return false }
func (w *fakeWorld) CanStandInWater(x, y, z int) bool { return false }

func (w *fakeWorld) CanStandAt(x, y, z int) bool {
	return w.IsBlockSolid(x, y-1, z) && w.IsPassable(x, y, z) && w.IsPassable(x, y+1, z)
}

func (w *fakeWorld) IsSafeToFall(x, startY, z, maxDrop int) int {
	for dy := 1; dy <= maxDrop; dy++ {
		if w.IsBlockSolid(x, startY-dy, z) {
			land := startY - dy + 1
			if w.IsPassable(x, land, z) && w.IsPassable(x, land+1, z) {
				return land
			}
			return -1
		}
	}
	return -1
}

// flat returns a world with a floor at y=63 over x,z in [-10, 10].
func flat() *fakeWorld { return newFakeWorld().fill(-10, 63, -10, 10, 63, 10) }

func mustFind(t *testing.T, w WorldView, start, goal Vec3) []Node {
	t.Helper()
	path, err := FindPath(start, goal, w, DefaultOptions())
	if err != nil {
		t.Fatalf("FindPath(%v -> %v): %v", start, goal, err)
	}
	if !path[0].Pos.Equals(start) || !path[len(path)-1].Pos.Equals(goal) {
		t.Fatalf("path runs %v -> %v, want %v -> %v", path[0].Pos, path[len(path)-1].Pos, start, goal)
	}
	for i, n := range path {
		if n.Depth != i {
			t.Fatalf("node %d has Depth %d", i, n.Depth)
		}
		if !w.CanStandAt(n.Pos.X, n.Pos.Y, n.Pos.Z) {
			t.Fatalf("node %d at %v is not standable", i, n.Pos)
		}
	}
	return path
}

func hasMove(path []Node, m MoveType) bool {
	for _, n := range path {
		if n.Move == m {
			return true
		}
	}
	return false
}

func TestFindPathFlat(t *testing.T) {
	path := mustFind(t, flat(), Vec3{0, 64, 0}, Vec3{5, 64, 3})
	// 3 diagonal + 2 straight moves is optimal.
	if len(path) != 6 {
		t.Errorf("path has %d nodes, want 6: %v", len(path), path)
	}
	for _, n := range path {
		if n.Pos.Y != 64 {
			t.Errorf("left the floor at %v", n.Pos)
		}
	}
}

func TestFindPathWallDetour(t *testing.T) {
	// Wall at x=3 for z in [-3, 3], 2 blocks high: too high to jump.
	w := flat().fill(3, 64, -3, 3, 65, 3)
	path := mustFind(t, w, Vec3{0, 64, 0}, Vec3{6, 64, 0})
	for _, n := range path {
		if n.Pos.X == 3 && n.Pos.Z >= -3 && n.Pos.Z <= 3 {
			t.Fatalf("path goes through the wall at %v", n.Pos)
		}
	}
}

func TestFindPathJumpUp(t *testing.T) {
	w := flat().fill(3, 64, -10, 10, 64, 10) // 1-block step up at x=3
	path := mustFind(t, w, Vec3{0, 64, 0}, Vec3{5, 65, 0})
	if !hasMove(path, MoveJump) {
		t.Errorf("no jump in path: %v", path)
	}
}

func TestFindPathDrop(t *testing.T) {
	w := newFakeWorld().
		fill(-5, 63, -5, 2, 63, 5). // upper floor, x <= 2
		fill(3, 60, -5, 8, 60, 5)   // lower floor 3 blocks down, x >= 3
	path := mustFind(t, w, Vec3{0, 64, 0}, Vec3{5, 61, 0})
	if !hasMove(path, MoveDrop) {
		t.Errorf("no drop in path: %v", path)
	}

	// 4 blocks is more than MaxFallDistance (3).
	deep := newFakeWorld().
		fill(-5, 63, -5, 2, 63, 5).
		fill(3, 59, -5, 8, 59, 5)
	if _, err := FindPath(Vec3{0, 64, 0}, Vec3{5, 60, 0}, deep, DefaultOptions()); !errors.Is(err, ErrNoPath) {
		t.Errorf("4-block drop: err = %v, want ErrNoPath", err)
	}
}

func TestFindPathNoPath(t *testing.T) {
	// Goal enclosed by 3-high walls and a roof.
	w := flat().
		fill(4, 64, -1, 6, 66, 1). // solid box...
		fill(4, 67, -1, 6, 67, 1)
	delete(w.solid, Vec3{5, 64, 0}) // ...with a 2-high hole inside
	delete(w.solid, Vec3{5, 65, 0})

	_, err := FindPath(Vec3{0, 64, 0}, Vec3{5, 64, 0}, w, DefaultOptions())
	if !errors.Is(err, ErrNoPath) {
		t.Fatalf("err = %v, want ErrNoPath", err)
	}
}

func TestFindPathMaxPathLength(t *testing.T) {
	opts := DefaultOptions()
	opts.MaxPathLength = 5
	_, err := FindPath(Vec3{-8, 64, 0}, Vec3{8, 64, 0}, flat(), opts)
	if !errors.Is(err, ErrNoPath) {
		t.Fatalf("err = %v, want ErrNoPath for a 17-node path with MaxPathLength 5", err)
	}
}

func TestFindPathUnloaded(t *testing.T) {
	w := flat()
	w.unloaded[[2]int{5, 0}] = true
	_, err := FindPath(Vec3{0, 64, 0}, Vec3{5, 64, 0}, w, DefaultOptions())
	if !errors.Is(err, ErrUnloaded) {
		t.Fatalf("err = %v, want ErrUnloaded", err)
	}
}

type fakeBot struct{ x, y, z float64 }

func (b *fakeBot) GetPosition() (x, y, z float64) { return b.x, b.y, b.z }
func (b *fakeBot) SetControlState(string, bool)   {}
func (b *fakeBot) ClearControlStates()            {}
func (b *fakeBot) LookAt(x, y, z float64) error   { return nil }
func (b *fakeBot) IsOnGround() bool               { return true }

func TestGoToStopDiscardsPendingSearch(t *testing.T) {
	w := flat()
	w.gate = make(chan struct{})
	p := New(&fakeBot{0.5, 64, 0.5}, w)
	failed := make(chan string, 1)
	p.SetCallbacks(nil, func(reason string) { failed <- reason })

	// GoTo's own standability checks don't call HasChunk, so this returns
	// while the search goroutine is blocked in FindPath.
	if err := p.GoTo(5.5, 64, 0.5, false); err != nil {
		t.Fatal(err)
	}
	p.Stop()
	close(w.gate)

	select {
	case r := <-failed:
		t.Fatalf("stale search reported failure: %s", r)
	case <-time.After(200 * time.Millisecond):
	}
	if p.IsNavigating() {
		t.Fatal("stale search installed a follower after Stop()")
	}
}

func TestGoToInstallsFollower(t *testing.T) {
	p := New(&fakeBot{0.5, 64, 0.5}, flat())
	if err := p.GoTo(5.5, 64, 0.5, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for !p.IsNavigating() {
		if time.Now().After(deadline) {
			t.Fatal("follower never installed")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
