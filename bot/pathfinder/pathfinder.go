package pathfinder

import (
	"fmt"
	"log"
	"math"
	"sync"
)

// Pathfinder manages A* pathfinding and path-following for a bot.
type Pathfinder struct {
	mu       sync.Mutex
	bot      BotController
	world    WorldView
	follower *Follower
	opts     Options

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

// GoTo computes a path to the target and begins following it.
// The A* computation runs in a goroutine to avoid blocking the physics loop.
func (p *Pathfinder) GoTo(x, y, z float64, sprint bool) error {
	p.mu.Lock()
	// Stop any current navigation
	if p.follower != nil && p.follower.IsActive() {
		p.follower.Stop(p.bot)
	}
	p.follower = nil

	bot := p.bot
	world := p.world
	opts := p.opts
	onReached := p.onGoalReached
	onFailed := p.onPathFailed
	p.mu.Unlock()

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

	// Auto-correct Y if feet are inside ground (float rounding: 82.999 → 82)
	if !world.CanStandAt(start.X, start.Y, start.Z) {
		if world.CanStandAt(start.X, start.Y+1, start.Z) {
			start.Y++
		}
	}
	if !world.CanStandAt(goal.X, goal.Y, goal.Z) {
		if world.CanStandAt(goal.X, goal.Y+1, goal.Z) {
			goal.Y++
		} else if world.CanStandAt(goal.X, goal.Y-1, goal.Z) {
			goal.Y--
		}
	}

	if start.Equals(goal) {
		if onReached != nil {
			onReached()
		}
		return nil
	}

	go func() {
		log.Printf("[Pathfinder] Computing path from %s to %s (start chunk loaded: %v, goal chunk loaded: %v)",
			start, goal, world.HasChunk(start.X, start.Z), world.HasChunk(goal.X, goal.Z))

		path, err := FindPath(start, goal, world, opts)
		if err != nil {
			log.Printf("[Pathfinder] Path computation failed: %v", err)
			if onFailed != nil {
				onFailed(fmt.Sprintf("pathfinding failed: %v", err))
			}
			return
		}

		log.Printf("[Pathfinder] Path found: %d nodes", len(path))

		follower := NewFollower(path, sprint, onReached, onFailed)

		p.mu.Lock()
		p.follower = follower
		p.mu.Unlock()
	}()

	return nil
}

// Stop cancels the current navigation and clears bot controls.
func (p *Pathfinder) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.follower != nil {
		p.follower.Stop(p.bot)
		p.follower = nil
	}
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
