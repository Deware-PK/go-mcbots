package bot

import (
	"math"
	"testing"
	"time"
)

// Block state IDs of 26.2 (Mojang reports/blocks.json).
const (
	s262Stone       = 1
	s262Water       = 86
	s262OakStairsE  = 3978 // facing=east, half=bottom, straight: walk up going +X
	s262OakSlabLow  = 13333
	s262OakFenceEW  = 6979
	s262LadderNorth = 5720 // hangs on the south (+Z) side of its block
)

// simWorld is a bot on 26.2 with an empty (all air) loaded area.
func simBot(t *testing.T) *Bot {
	t.Helper()
	ver, err := ResolveVersion("26.2")
	if err != nil {
		t.Fatal(err)
	}
	b := New("Sim", ver)
	for cx := int32(-3); cx <= 3; cx++ {
		for cz := int32(-3); cz <= 3; cz++ {
			b.world.SetChunk(&ChunkColumn{X: cx, Z: cz, MinY: -64, Sections: make([]ChunkSection, 24)})
		}
	}
	b.awaitingSpawn.Store(false)
	return b
}

func (b *Bot) fillSim(state uint32, x0, y0, z0, x1, y1, z1 int) {
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for z := z0; z <= z1; z++ {
				b.world.SetBlock(x, y, z, state)
			}
		}
	}
}

// navigate runs GoTo and the physics loop synchronously until the goal is
// reached, the path fails, or maxTicks pass.
func navigate(t *testing.T, b *Bot, start [3]float64, goal [3]float64, maxTicks int) (x, y, z float64) {
	t.Helper()
	b.state.SetPosition(start[0], start[1], start[2])
	b.state.SetOnGround(true)
	done := make(chan string, 4)
	b.Events.OnGoalReached = func() { done <- "" }
	b.Events.OnPathFailed = func(r string) { done <- r }
	if err := b.GoTo(goal[0], goal[1], goal[2], true); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !b.IsNavigating() {
		select {
		case r := <-done:
			if r != "" {
				t.Fatalf("path failed before moving: %s", r)
			}
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("no path computed")
		}
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < maxTicks; i++ {
		b.physics.tick()
		select {
		case r := <-done:
			x, y, z = b.GetPosition()
			if r != "" {
				t.Fatalf("path failed at %.2f %.2f %.2f after %d ticks: %s", x, y, z, i, r)
			}
			if !b.IsNavigating() {
				// Re-planning (partial paths) installs a new follower async.
				time.Sleep(20 * time.Millisecond)
				if !b.IsNavigating() {
					return x, y, z
				}
			}
		default:
		}
	}
	x, y, z = b.GetPosition()
	t.Fatalf("goal not reached in %d ticks; at %.2f %.2f %.2f", maxTicks, x, y, z)
	return
}

func assertNear(t *testing.T, x, y, z float64, goal [3]float64) {
	t.Helper()
	if math.Abs(x-goal[0]) > 0.6 || math.Abs(z-goal[2]) > 0.6 || math.Abs(y-goal[1]) > 0.6 {
		t.Fatalf("ended at %.2f %.2f %.2f, want near %v", x, y, z, goal)
	}
}

// Up a staircase of real oak stairs onto a platform.
func TestNavigateStairs(t *testing.T) {
	b := simBot(t)
	b.fillSim(s262Stone, -10, 63, -10, 20, 63, 10)
	for i := 0; i < 4; i++ {
		b.fillSim(s262OakStairsE, 2+i, 64+i, -1, 2+i, 64+i, 1)
		if i > 0 {
			b.fillSim(s262Stone, 2+i, 64, -1, 2+i, 63+i, 1)
		}
	}
	b.fillSim(s262Stone, 6, 64, -3, 12, 67, 3) // platform, top at 68
	goal := [3]float64{9.5, 68, 0.5}
	x, y, z := navigate(t, b, [3]float64{-3.5, 64, 0.5}, goal, 400)
	assertNear(t, x, y, z, goal)
}

// Over bottom slabs (no jumping needed) and around a fence line.
func TestNavigateSlabsAndFence(t *testing.T) {
	b := simBot(t)
	b.fillSim(s262Stone, -10, 63, -10, 20, 63, 10)
	b.fillSim(s262OakSlabLow, 2, 64, -10, 3, 64, 10)
	b.fillSim(s262OakFenceEW, 7, 64, -4, 7, 64, 10) // gap only at z < -4
	goal := [3]float64{12.5, 64, 2.5}
	x, y, z := navigate(t, b, [3]float64{-3.5, 64, 2.5}, goal, 800)
	assertNear(t, x, y, z, goal)
}

// Swim across a lake and climb out on the far bank.
func TestNavigateSwimAcross(t *testing.T) {
	b := simBot(t)
	b.fillSim(s262Stone, -15, 55, -10, 30, 55, 10)
	b.fillSim(s262Stone, -15, 56, -10, 2, 62, 10) // near bank, ground 63
	b.fillSim(s262Water, 3, 56, -10, 14, 62, 10)  // lake, surface block 62
	b.fillSim(s262Stone, 15, 56, -10, 30, 62, 10) // far bank, ground 63
	goal := [3]float64{19.5, 63, 0.5}
	x, y, z := navigate(t, b, [3]float64{-2.5, 63, 0.5}, goal, 1200)
	assertNear(t, x, y, z, goal)
}

// Climb a ladder up a wall and walk onto the top.
func TestNavigateLadder(t *testing.T) {
	b := simBot(t)
	b.fillSim(s262Stone, -10, 63, -10, 10, 63, 20)
	b.fillSim(s262Stone, -10, 64, 1, 10, 71, 12)   // wall face at z=1, top at 72
	b.fillSim(s262LadderNorth, 0, 64, 0, 0, 71, 0) // ladder on the face
	goal := [3]float64{0.5, 72, 5.5}
	x, y, z := navigate(t, b, [3]float64{-4.5, 64, -4.5}, goal, 800)
	assertNear(t, x, y, z, goal)

	// And back down.
	back := [3]float64{-4.5, 64, -4.5}
	x, y, z = navigate(t, b, goal, back, 800)
	assertNear(t, x, y, z, back)
}
