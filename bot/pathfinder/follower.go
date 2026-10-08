package pathfinder

import (
	"math"
	"sync"
)

const (
	// WaypointReachThreshold is how close the bot needs to be to a waypoint (in blocks).
	WaypointReachThreshold = 0.35
	// StuckTickThreshold is how many ticks with no progress before we consider the bot stuck.
	StuckTickThreshold = 60 // ~3 seconds at 50ms ticks
	// StuckDistThreshold is the minimum distance the bot must move per stuck check window.
	StuckDistThreshold = 0.1
)

// BotController is the interface the follower uses to control the bot.
type BotController interface {
	GetPosition() (x, y, z float64)
	SetControlState(control string, state bool)
	ClearControlStates()
	LookAt(x, y, z float64) error
	IsOnGround() bool
}

// Follower executes a computed path by steering the bot each physics tick.
type Follower struct {
	mu           sync.Mutex
	path         []Node
	currentIndex int
	active       bool
	sprint       bool
	stuckTicks   int
	lastX, lastZ float64

	onGoalReached func()
	onPathFailed  func(reason string)
}

// NewFollower creates a path follower with the given path and callbacks.
func NewFollower(path []Node, sprint bool, onReached func(), onFailed func(string)) *Follower {
	return &Follower{
		path:          path,
		currentIndex:  1, // skip the starting node (we're already there)
		active:        true,
		sprint:        sprint,
		onGoalReached: onReached,
		onPathFailed:  onFailed,
	}
}

// IsActive returns whether the follower is still navigating.
func (f *Follower) IsActive() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

// GetProgress returns the current waypoint index and total path length.
func (f *Follower) GetProgress() (current, total int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.currentIndex, len(f.path)
}

// Stop cancels the follower and clears the bot's controls.
func (f *Follower) Stop(bot BotController) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active = false
	bot.ClearControlStates()
}

// Tick is called every physics tick to advance the bot along the path.
func (f *Follower) Tick(bot BotController) {
	f.mu.Lock()
	if !f.active {
		f.mu.Unlock()
		return
	}

	if f.currentIndex >= len(f.path) {
		f.active = false
		f.mu.Unlock()
		bot.ClearControlStates()
		if f.onGoalReached != nil {
			f.onGoalReached()
		}
		return
	}

	target := f.path[f.currentIndex]
	f.mu.Unlock()

	bx, by, bz := bot.GetPosition()

	// Target center of the block
	tx := float64(target.Pos.X) + 0.5
	ty := float64(target.Pos.Y)
	tz := float64(target.Pos.Z) + 0.5

	dx := tx - bx
	dz := tz - bz
	horizontalDist := math.Sqrt(dx*dx + dz*dz)
	verticalDist := math.Abs(ty - by)

	// Check if we've reached the current waypoint
	reached := horizontalDist < WaypointReachThreshold && verticalDist < 1.5
	if reached {
		f.mu.Lock()
		f.currentIndex++
		f.stuckTicks = 0

		if f.currentIndex >= len(f.path) {
			f.active = false
			f.mu.Unlock()
			bot.ClearControlStates()
			if f.onGoalReached != nil {
				f.onGoalReached()
			}
			return
		}
		target = f.path[f.currentIndex]
		f.mu.Unlock()

		tx = float64(target.Pos.X) + 0.5
		ty = float64(target.Pos.Y)
		tz = float64(target.Pos.Z) + 0.5
		dx = tx - bx
		dz = tz - bz
		horizontalDist = math.Sqrt(dx*dx + dz*dz)
	}

	// Stuck detection
	f.mu.Lock()
	distMoved := math.Sqrt((bx-f.lastX)*(bx-f.lastX) + (bz-f.lastZ)*(bz-f.lastZ))
	if distMoved < StuckDistThreshold {
		f.stuckTicks++
	} else {
		f.stuckTicks = 0
	}
	f.lastX = bx
	f.lastZ = bz
	stuck := f.stuckTicks > StuckTickThreshold
	f.mu.Unlock()

	if stuck {
		f.mu.Lock()
		f.active = false
		f.mu.Unlock()
		bot.ClearControlStates()
		if f.onPathFailed != nil {
			f.onPathFailed("stuck: no progress for too long")
		}
		return
	}

	// Steer the bot based on movement type
	f.steer(bot, target, tx, ty, tz, horizontalDist)
}

func (f *Follower) steer(bot BotController, target Node, tx, ty, tz, horizontalDist float64) {
	bx, by, bz := bot.GetPosition()

	// Look at the target block center (at eye height of target)
	bot.LookAt(tx, ty+1.0, tz)

	// Compute desired control states, then apply once to avoid toggling
	wantForward := false
	wantSprint := false
	wantJump := false
	wantSneak := false

	switch target.Move {
	case MoveWalk, MoveDiagonal:
		wantForward = true
		if f.sprint || horizontalDist > 2.0 {
			wantSprint = true
		}

	case MoveJump:
		wantForward = true
		if bot.IsOnGround() && ty > by+0.5 {
			wantJump = true
		}

	case MoveDrop:
		wantForward = true
		// Just walk off the edge, gravity handles the rest

	case MoveSprintJump:
		wantForward = true
		wantSprint = true
		// Jump when on ground and we need to cross a gap
		if bot.IsOnGround() {
			// Check if we need to jump (when close to the edge)
			f.mu.Lock()
			prevIdx := f.currentIndex - 1
			f.mu.Unlock()
			if prevIdx >= 0 && prevIdx < len(f.path) {
				prev := f.path[prevIdx]
				distFromPrev := math.Sqrt(
					math.Pow(bx-float64(prev.Pos.X)-0.5, 2) +
						math.Pow(bz-float64(prev.Pos.Z)-0.5, 2))
				if distFromPrev > 0.4 {
					wantJump = true
				}
			}
		}

	case MoveLadderUp:
		wantForward = true
		if ty > by+0.5 {
			wantJump = true
		}

	case MoveLadderDown:
		wantForward = true
		wantSneak = true

	case MoveSwim:
		wantForward = true
		_ = bx
		_ = bz
		if ty > by+0.3 {
			wantJump = true
		}
		if f.sprint || horizontalDist > 2.0 {
			wantSprint = true
		}
	}

	// Apply final desired states — only triggers server commands on actual changes
	bot.SetControlState("forward", wantForward)
	bot.SetControlState("sprint", wantSprint)
	bot.SetControlState("jump", wantJump)
	bot.SetControlState("sneak", wantSneak)
}
