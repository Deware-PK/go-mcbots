package pathfinder

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// box is a collision box in block-local coordinates.
type box struct{ minX, minY, minZ, maxX, maxY, maxZ float64 }

type fakeBlock struct {
	boxes  []box
	water  bool
	climb  bool
	danger bool
}

var (
	fullBlock  = fakeBlock{boxes: []box{{0, 0, 0, 1, 1, 1}}}
	waterBlock = fakeBlock{water: true}
	slabBlock  = fakeBlock{boxes: []box{{0, 0, 0, 1, 0.5, 1}}}
	fenceBlock = fakeBlock{boxes: []box{{0.375, 0, 0.375, 0.625, 1.5, 0.625}}}
	// ladder on the +X face of its block
	ladderBlock = fakeBlock{boxes: []box{{0.8125, 0, 0, 1, 1, 1}}, climb: true}
	lavaBlock   = fakeBlock{danger: true}
)

// fakeWorld is a map of blocks; everything else is air. Chunks are loaded
// unless listed in unloaded (by block X/Z).
type fakeWorld struct {
	blocks   map[Vec3]fakeBlock
	unloaded map[[2]int]bool
	// gate, if set, blocks HasChunk until it is closed.
	gate chan struct{}
}

func newFakeWorld() *fakeWorld {
	return &fakeWorld{blocks: map[Vec3]fakeBlock{}, unloaded: map[[2]int]bool{}}
}

func (w *fakeWorld) fillWith(b fakeBlock, x0, y0, z0, x1, y1, z1 int) *fakeWorld {
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for z := z0; z <= z1; z++ {
				w.blocks[Vec3{x, y, z}] = b
			}
		}
	}
	return w
}

func (w *fakeWorld) fill(x0, y0, z0, x1, y1, z1 int) *fakeWorld {
	return w.fillWith(fullBlock, x0, y0, z0, x1, y1, z1)
}

func (w *fakeWorld) HasChunk(x, z int) bool {
	if w.gate != nil {
		<-w.gate
	}
	return !w.unloaded[[2]int{x, z}]
}

func (w *fakeWorld) IsWater(x, y, z int) bool     { return w.blocks[Vec3{x, y, z}].water }
func (w *fakeWorld) IsClimbable(x, y, z int) bool { return w.blocks[Vec3{x, y, z}].climb }
func (w *fakeWorld) IsDangerous(x, y, z int) bool { return w.blocks[Vec3{x, y, z}].danger }

// topInColumn is the highest box top of block p inside the centered
// player column (x+0.2..x+0.8), relative to the block.
func (w *fakeWorld) topInColumn(p Vec3) (float64, bool) {
	top, ok := 0.0, false
	for _, b := range w.blocks[p].boxes {
		if b.maxX > 0.2 && b.minX < 0.8 && b.maxZ > 0.2 && b.minZ < 0.8 && (!ok || b.maxY > top) {
			top, ok = b.maxY, true
		}
	}
	return top, ok
}

func (w *fakeWorld) StandHeight(x, y, z int) (float64, bool) {
	if w.unloaded[[2]int{x, z}] {
		return 0, false
	}
	var feet float64
	if t, ok := w.topInColumn(Vec3{x, y, z}); ok {
		if t >= 1 {
			return 0, false
		}
		feet = float64(y) + t
	} else {
		t, ok := w.topInColumn(Vec3{x, y - 1, z})
		if !ok || t < 1 {
			return 0, false
		}
		feet = float64(y-1) + t
	}
	if !w.ColumnFree(x, z, feet, feet+1.8) {
		return 0, false
	}
	return feet, true
}

func (w *fakeWorld) ColumnFree(x, z int, y0, y1 float64) bool {
	if w.unloaded[[2]int{x, z}] {
		return false
	}
	const e = 1e-7
	for y := int(math.Floor(y0)) - 1; y <= int(math.Floor(y1)); y++ {
		for _, b := range w.blocks[Vec3{x, y, z}].boxes {
			if b.maxX > 0.2 && b.minX < 0.8 && b.maxZ > 0.2 && b.minZ < 0.8 &&
				float64(y)+b.maxY > y0+e && float64(y)+b.minY < y1-e {
				return false
			}
		}
	}
	return true
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
		if footingAt(w, n.Pos, DefaultOptions()).kind == footNone {
			t.Fatalf("node %d at %v is not a valid position", i, n.Pos)
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
	delete(w.blocks, Vec3{5, 64, 0}) // ...with a 2-high hole inside
	delete(w.blocks, Vec3{5, 65, 0})

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
	w.unloaded[[2]int{0, 0}] = true
	if _, err := FindPath(Vec3{0, 64, 0}, Vec3{5, 64, 0}, w, DefaultOptions()); !errors.Is(err, ErrUnloaded) {
		t.Fatalf("unloaded start: err = %v, want ErrUnloaded", err)
	}

	// An unloaded goal chunk is fine: the search heads toward it and
	// returns a partial path to re-plan from once the chunk arrives.
	w = flat()
	w.unloaded[[2]int{5, 0}] = true
	path, err := FindPath(Vec3{0, 64, 0}, Vec3{5, 64, 0}, w, DefaultOptions())
	if !errors.Is(err, ErrNoPath) || len(path) < 2 || path[len(path)-1].Pos.DistanceTo(Vec3{5, 64, 0}) > 1.5 {
		t.Fatalf("unloaded goal: err = %v, path %v; want a partial path next to the goal", err, path)
	}
}

// A goal on top of a 2-block pillar can't be reached (no climbing), but the
// search returns a partial path that ends right next to the pillar.
func TestFindPathPartialToUnreachableGoal(t *testing.T) {
	w := newFakeWorld().fill(-15, 63, -15, 15, 63, 15).fill(5, 64, 5, 5, 65, 5)
	goal := Vec3{5, 66, 5}
	path, err := FindPath(Vec3{-10, 64, -10}, goal, w, DefaultOptions())
	if !errors.Is(err, ErrNoPath) && !errors.Is(err, ErrMaxIterations) {
		t.Fatalf("err = %v, want ErrNoPath or ErrMaxIterations", err)
	}
	if len(path) < 2 {
		t.Fatalf("no partial path returned")
	}
	end := path[len(path)-1].Pos
	if d := end.DistanceTo(goal); d > 2.5 {
		t.Fatalf("partial path ends at %v, %.1f blocks from goal; want next to the pillar", end, d)
	}
}

// A search cut short by MaxIterations still makes progress toward the goal.
func TestFindPathPartialOnMaxIterations(t *testing.T) {
	w := newFakeWorld().fill(-5, 63, -5, 200, 63, 5)
	opts := DefaultOptions()
	opts.MaxIterations = 50
	start, goal := Vec3{0, 64, 0}, Vec3{190, 64, 0}
	path, err := FindPath(start, goal, w, opts)
	if !errors.Is(err, ErrMaxIterations) {
		t.Fatalf("err = %v, want ErrMaxIterations", err)
	}
	if len(path) < 2 || path[len(path)-1].Pos.DistanceTo(goal) >= start.DistanceTo(goal) {
		t.Fatalf("partial path %v makes no progress", path)
	}
}

type fakeBot struct{ x, y, z float64 }

func (b *fakeBot) GetPosition() (x, y, z float64) { return b.x, b.y, b.z }
func (b *fakeBot) SetControlState(string, bool)   {}
func (b *fakeBot) ClearControlStates()            {}
func (b *fakeBot) LookAt(x, y, z float64) error   { return nil }
func (b *fakeBot) IsOnGround() bool               { return true }
func (b *fakeBot) IsInWater() bool                { return false }
func (b *fakeBot) IsOnClimbable() bool            { return false }
func (b *fakeBot) IsCollidedHorizontally() bool   { return false }

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

func TestGoToUnreachableReportsFailure(t *testing.T) {
	w := newFakeWorld().fill(-15, 63, -15, 15, 63, 15).fill(5, 64, 5, 5, 65, 5)
	bot := &fakeBot{-9.5, 64, -9.5}
	p := New(bot, w)
	failed := make(chan string, 1)
	reached := make(chan struct{}, 1)
	p.SetCallbacks(func() { reached <- struct{}{} }, func(r string) { failed <- r })

	if err := p.GoTo(5.5, 66, 5.5, false); err != nil {
		t.Fatal(err)
	}
	// Drive the follower: jump the fake bot along each waypoint.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		f := p.follower
		p.mu.Unlock()
		if f != nil && f.IsActive() {
			cur, _ := f.GetProgress()
			if cur < len(f.path) {
				n := f.path[cur].Pos
				bot.x, bot.y, bot.z = float64(n.X)+0.5, float64(n.Y), float64(n.Z)+0.5
			}
			p.Tick()
		}
		select {
		case r := <-failed:
			if !strings.Contains(r, ErrUnreachable.Error()) || !strings.Contains(r, "closest reachable point") {
				t.Fatalf("failure reason = %q", r)
			}
			return
		case <-reached:
			t.Fatal("reported goal reached for an unreachable goal")
		default:
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no failure reported")
}
