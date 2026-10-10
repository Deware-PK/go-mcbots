package pathfinder

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
)

// Pathfinder manages A* pathfinding and path-following for a bot.
type Pathfinder struct {
	mu       sync.Mutex
	bot      BotController
	world    WorldView
	follower *Follower
	opts     Options
	// gen is bumped by every GoTo and Stop; an A* result is only used if
	// gen has not changed since its GoTo started.
	gen uint64

	onGoalReached func()
	onPathFailed  func(reason string)
}

// New creates a Pathfinder for the given bot and world.
func New(bot BotController, world WorldView) *Pathfinder {
	return &Pathfinder{
		bot:   bot,
		world: world,
		opts:  DefaultOptions(),
	}
}

// SetCallbacks sets the event callbacks for path completion/failure.
func (p *Pathfinder) SetCallbacks(onReached func(), onFailed func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onGoalReached = onReached
	p.onPathFailed = onFailed
}

// SetOptions sets the A* options.
func (p *Pathfinder) SetOptions(opts Options) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.opts = opts
}

// maxReplans bounds how many times GoTo re-plans after following a partial
// path (goal too far for one search, or unreachable).
const maxReplans = 10

// maxStuckReplans bounds re-planning after the bot got stuck on a path.
const maxStuckReplans = 2

// GoTo computes a path to the target and begins following it.
// The A* computation runs in a goroutine to avoid blocking the physics loop.
//
// If the goal is too far for one search, the bot follows a partial path to
// the closest point found and re-plans from there. If the goal is
// unreachable, the bot walks to the closest reachable point and OnPathFailed
// reports "goal unreachable".
func (p *Pathfinder) GoTo(x, y, z float64, sprint bool) error {
	p.mu.Lock()
	p.gen++
	gen := p.gen
	// Stop any current navigation
	if p.follower != nil && p.follower.IsActive() {
		p.follower.Stop(p.bot)
	}
	p.follower = nil
	p.mu.Unlock()

	p.plan(gen, x, y, z, sprint, 0)
	return nil
}

// plan runs one A* search toward (x, y, z) and installs a follower for the
// result. replans counts previous partial paths for this GoTo.
func (p *Pathfinder) plan(gen uint64, x, y, z float64, sprint bool, replans int) {
	p.mu.Lock()
	if p.gen != gen {
		p.mu.Unlock()
		return
	}
	bot := p.bot
	world := p.world
	opts := p.opts
	onReached := p.onGoalReached
	onFailed := p.onPathFailed
	p.mu.Unlock()

	fail := func(reason string) {
		log.Printf("[Pathfinder] %s", reason)
		if onFailed != nil {
			onFailed(reason)
		}
	}

	bx, by, bz := bot.GetPosition()
	start := Vec3{
		X: int(math.Floor(bx)),
		Y: int(math.Floor(by)),
		Z: int(math.Floor(bz)),
	}
	goal := Vec3{
		X: int(math.Floor(x)),
		Y: int(math.Floor(y)),
		Z: int(math.Floor(z)),
	}

	// Feet rounding (82.999 -> 82) can put the start inside the floor.
	if footingAt(world, start, opts).kind == footNone {
		if footingAt(world, start.Add(0, 1, 0), opts).kind != footNone {
			start.Y++
		}
	}
	// The goal Y is often typed by hand (F3 coordinates, the block looked
	// at, a slab or carpet): snap to the nearest valid Y nearby.
	if footingAt(world, goal, opts).kind == footNone {
		for _, dy := range []int{1, -1, 2, -2, -3} {
			if footingAt(world, goal.Add(0, dy, 0), opts).kind != footNone {
				goal.Y += dy
				break
			}
		}
	}

	if start.Equals(goal) {
		if onReached != nil {
			onReached()
		}
		return
	}

	go func() {
		log.Printf("[Pathfinder] Computing path from %s to %s (attempt %d)", start, goal, replans+1)

		path, err := FindPath(start, goal, world, opts)
		if !p.isCurrent(gen) {
			return // cancelled by Stop() or superseded by a newer GoTo
		}

		switch {
		case err == nil:
			log.Printf("[Pathfinder] Path found: %d nodes", len(path))
			p.follow(gen, path, sprint, onReached, p.retryWhenStuck(gen, x, y, z, sprint, replans, onFailed))

		case (errors.Is(err, ErrNoPath) || errors.Is(err, ErrMaxIterations)) &&
			len(path) > 1 && replans < maxReplans:
			end := path[len(path)-1].Pos
			reason := "too far for one search"
			if errors.Is(err, ErrNoPath) {
				reason = "goal unreachable"
			}
			log.Printf("[Pathfinder] %s; following partial path (%d nodes) to %s, %.1f blocks from goal",
				reason, len(path), end, end.DistanceTo(goal))
			// When the partial path ends, re-plan from there. If the goal
			// is unreachable the next search finds no closer node and fails.
			p.follow(gen, path, sprint, func() {
				p.plan(gen, x, y, z, sprint, replans+1)
			}, p.retryWhenStuck(gen, x, y, z, sprint, replans, onFailed))

		case errors.Is(err, ErrNoPath) || errors.Is(err, ErrMaxIterations):
			if replans > 0 {
				fail(fmt.Sprintf("%v: stopped at the closest reachable point, %.1f blocks away",
					ErrUnreachable, start.DistanceTo(goal)))
			} else {
				fail(fmt.Sprintf("%v: no walkable route from %s to %s", ErrUnreachable, start, goal))
			}

		default:
			fail(fmt.Sprintf("pathfinding failed: %v", err))
		}
	}()
}

// retryWhenStuck wraps onFailed: when the follower gets stuck, plan again
// from where the bot is (the world or the bot's position may differ from
// what the path assumed) before giving up.
func (p *Pathfinder) retryWhenStuck(gen uint64, x, y, z float64, sprint bool, replans int, onFailed func(string)) func(string) {
	return func(reason string) {
		if strings.HasPrefix(reason, "stuck") && replans < maxStuckReplans && p.isCurrent(gen) {
			log.Printf("[Pathfinder] %s; re-planning", reason)
			p.plan(gen, x, y, z, sprint, replans+1)
			return
		}
		if onFailed != nil {
			onFailed(reason)
		}
	}
}

// follow installs a follower for path if gen is still current.
func (p *Pathfinder) follow(gen uint64, path []Node, sprint bool, onReached func(), onFailed func(string)) {
	follower := NewFollower(path, sprint, onReached, onFailed)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gen != gen {
		return
	}
	p.follower = follower
}

// Stop cancels the current navigation and clears bot controls.
func (p *Pathfinder) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gen++

	if p.follower != nil {
		p.follower.Stop(p.bot)
		p.follower = nil
	}
}

func (p *Pathfinder) isCurrent(gen uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gen == gen
}

// IsNavigating returns true if the bot is currently following a path.
func (p *Pathfinder) IsNavigating() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.follower != nil && p.follower.IsActive()
}

// GetProgress returns the current waypoint index and total path length.
// Returns (0, 0) if not navigating.
func (p *Pathfinder) GetProgress() (current, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.follower == nil {
		return 0, 0
	}
	return p.follower.GetProgress()
}

// Tick should be called every physics tick. It advances the follower.
func (p *Pathfinder) Tick() {
	p.mu.Lock()
	f := p.follower
	p.mu.Unlock()

	if f == nil {
		return
	}
	f.Tick(p.bot)
}
