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
	// WaypointTimeoutTicks is how long one waypoint may take before the bot
	// counts as stuck even if it keeps moving (bobbing in water, sliding
	// along a wall).
	WaypointTimeoutTicks = 100 // 5 seconds
)

// BotController is the interface the follower uses to control the bot.
type BotController interface {
	GetPosition() (x, y, z float64)
	SetControlState(control string, state bool)
	ClearControlStates()
	LookAt(x, y, z float64) error
	IsOnGround() bool
	IsInWater() bool
	IsOnClimbable() bool
	IsCollidedHorizontally() bool
}

// Follower executes a computed path by steering the bot each physics tick.
type Follower struct {
	mu            sync.Mutex
	path          []Node
	currentIndex  int
	active        bool
	sprint        bool
	stuckTicks    int
	waypointTicks int // ticks spent on the current waypoint
	blockedTicks  int // consecutive ticks pressed against something
	// Position at the last progress check (3D: climbing a ladder is progress).
	lastX, lastY, lastZ float64

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

// reached reports whether the bot at (bx, by, bz) has arrived at node n.
// Node Y is the block the feet are in; the feet may rest higher in it
// (slab, stairs top) and float above it in water.
func reached(n Node, bx, by, bz float64) bool {
	tx := float64(n.Pos.X) + 0.5
	ty := float64(n.Pos.Y)
	tz := float64(n.Pos.Z) + 0.5
	h := math.Hypot(tx-bx, tz-bz)

	switch n.Move {
	case MoveLadderUp:
		return h < 0.5 && by >= ty-0.1 && by < ty+1
	case MoveLadderDown:
		return h < 0.5 && by < ty+0.6 && by > ty-1
	case MoveSwim:
		return h < 0.5 && by > ty-0.8 && by < ty+1.2
	default:
		return h < WaypointReachThreshold && by > ty-0.8 && by < ty+1.3
	}
}

// Tick is called every physics tick to advance the bot along the path.
func (f *Follower) Tick(bot BotController) {
	f.mu.Lock()
	if !f.active {
		f.mu.Unlock()
		return
	}

	bx, by, bz := bot.GetPosition()
	// Skip every waypoint already reached (several can be passed in one
	// tick, e.g. straight up a ladder).
	for f.currentIndex < len(f.path) && reached(f.path[f.currentIndex], bx, by, bz) {
		f.currentIndex++
		f.stuckTicks = 0
		f.waypointTicks = 0
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
	var prev Node
	if f.currentIndex > 0 {
		prev = f.path[f.currentIndex-1]
	}

	// Stuck detection: no movement of StuckDistThreshold (in 3D: climbing
	// a ladder is progress) for StuckTickThreshold ticks.
	if math.Sqrt((bx-f.lastX)*(bx-f.lastX)+(by-f.lastY)*(by-f.lastY)+(bz-f.lastZ)*(bz-f.lastZ)) > StuckDistThreshold {
		f.stuckTicks = 0
		f.lastX, f.lastY, f.lastZ = bx, by, bz
	} else {
		f.stuckTicks++
	}
	f.waypointTicks++
	stuck := f.stuckTicks > StuckTickThreshold || f.waypointTicks > WaypointTimeoutTicks
	if stuck {
		f.active = false
	}
	if bot.IsCollidedHorizontally() {
		f.blockedTicks++
	} else {
		f.blockedTicks = 0
	}
	blockedTicks := f.blockedTicks
	f.mu.Unlock()

	if stuck {
		bot.ClearControlStates()
		if f.onPathFailed != nil {
			f.onPathFailed("stuck: no progress for too long")
		}
		return
	}

	f.steer(bot, prev, target, bx, by, bz, blockedTicks)
}

func (f *Follower) steer(bot BotController, prev, target Node, bx, by, bz float64, blockedTicks int) {
	tx := float64(target.Pos.X) + 0.5
	ty := float64(target.Pos.Y)
	tz := float64(target.Pos.Z) + 0.5
	horizontalDist := math.Hypot(tx-bx, tz-bz)

	inWater := bot.IsInWater()
	onGround := bot.IsOnGround()
	blocked := blockedTicks > 0

	// Face the target. Straight up or down (ladders, swimming) keep the
	// current yaw unless the bot drifted off the column center.
	if horizontalDist > 0.15 {
		bot.LookAt(tx, ty+1.0, tz)
	}

	forward := horizontalDist > 0.15
	sprint := false
	jump := false
	sneak := false

	switch target.Move {
	case MoveWalk, MoveDiagonal:
		sprint = f.sprint || horizontalDist > 2.0
		// An obstacle the step-up could not handle (an edge, a raised
		// block the path walks around closely): hop.
		jump = blockedTicks >= 3 && onGround

	case MoveJump:
		// Jump when pressed against the ledge, or right away from water
		// and ladders (they climb while jump is held).
		jump = inWater || bot.IsOnClimbable() || (onGround && (blocked || ty > by+0.5 && horizontalDist < 1.0))

	case MoveDrop:
		// Walk off the edge; gravity does the rest.

	case MoveSprintJump:
		sprint = true
		// Jump at the edge of the take-off block.
		px := float64(prev.Pos.X) + 0.5
		pz := float64(prev.Pos.Z) + 0.5
		if onGround && math.Hypot(bx-px, bz-pz) > 0.4 {
			jump = true
		}

	case MoveLadderUp:
		// Holding jump climbs a ladder or vine (and swims up in water).
		jump = true
		forward = horizontalDist > 0.2

	case MoveLadderDown:
		// Let go: the bot slides down at the ladder's speed.
		forward = horizontalDist > 0.2

	case MoveSwim:
		// Stay afloat unless the target is in a lower block (diving); the
		// feet bob within the surface block, so compare whole blocks.
		switch {
		case target.Pos.Y < int(math.Floor(by)):
			sneak = inWater
		default:
			jump = true
		}
		forward = horizontalDist > 0.2
	}
	if inWater {
		sprint = false // surface swimming does not sprint
	}

	// Apply final desired states — only triggers server commands on actual changes
	bot.SetControlState("forward", forward)
	bot.SetControlState("sprint", sprint)
	bot.SetControlState("jump", jump)
	bot.SetControlState("sneak", sneak)
}
