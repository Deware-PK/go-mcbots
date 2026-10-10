package pathfinder

import "math"

// Movement limits, matching the bot's vanilla physics.
const (
	stepHeight   = 0.6  // walked up without jumping (slabs, stairs, carpets)
	jumpHeight   = 1.2  // highest ledge reachable by jumping (vanilla apex ~1.25)
	playerHeight = 1.8  // player box height
	jumpClear    = 1.25 // extra headroom needed above the feet to jump
	heightEps    = 1e-6
)

// footKind says how the bot is held at a node.
type footKind int

const (
	footNone   footKind = iota
	footGround          // standing on a block
	footWater           // swimming
	footLadder          // holding on to a ladder or vine
)

// footing describes a node the bot can occupy: what holds it and the
// height of its feet.
type footing struct {
	kind footKind
	feet float64
	deep bool // footWater: the head is under water too (diving)
}

// swimCost is the cost of swimming into a water node. Diving (head under
// water) is much more expensive than swimming at the surface: it is slow,
// the bot can drown, and players swim on top.
func swimCost(t footing) float64 {
	if t.deep {
		return 6.0
	}
	return 2.0
}

// footingAt returns how the bot can be at block position p (feet inside
// block p), or footNone if it cannot be there.
func footingAt(w WorldView, p Vec3, opts Options) footing {
	if w.IsDangerous(p.X, p.Y, p.Z) || w.IsDangerous(p.X, p.Y+1, p.Z) {
		return footing{}
	}
	// Walking on the bottom with the head under water is diving, not
	// walking: with swimming allowed such nodes count as (deep) water, so
	// the search prefers to swim at the surface.
	headUnderwater := opts.AllowWater && w.IsWater(p.X, p.Y+1, p.Z)
	if feet, ok := w.StandHeight(p.X, p.Y, p.Z); ok && !headUnderwater {
		if w.IsDangerous(p.X, p.Y-1, p.Z) {
			return footing{}
		}
		return footing{kind: footGround, feet: feet}
	}
	y := float64(p.Y)
	if opts.AllowLadder && w.IsClimbable(p.X, p.Y, p.Z) && w.ColumnFree(p.X, p.Z, y, y+playerHeight) {
		return footing{kind: footLadder, feet: y}
	}
	// Swimming: water at the feet and room for the body. The bot floats at
	// the surface, so the water must be at the feet or the head must be in
	// water or air.
	if opts.AllowWater && w.IsWater(p.X, p.Y, p.Z) && w.ColumnFree(p.X, p.Z, y, y+playerHeight) {
		return footing{kind: footWater, feet: y, deep: headUnderwater}
	}
	return footing{}
}

// getNeighbors returns all reachable neighbor nodes from the current position.
// Each returned node's G field holds the EDGE COST (not cumulative).
func getNeighbors(current *Node, world WorldView, opts Options) []Node {
	var neighbors []Node
	pos := current.Pos
	from := footingAt(world, pos, opts)
	if from.kind == footNone {
		// The start node can be odd (mid-air after a knockback, inside a
		// block after rounding): let the search leave it by walking.
		from = footing{kind: footGround, feet: float64(pos.Y)}
	}
	add := func(p Vec3, cost float64, move MoveType) {
		neighbors = append(neighbors, Node{Pos: p, G: cost, Move: move})
	}

	// --- Straight up / down: ladders and swimming ---
	for _, dy := range []int{1, -1} {
		dest := pos.Add(0, dy, 0)
		to := footingAt(world, dest, opts)
		switch {
		case to.kind == footNone:
			continue
		case from.kind == footLadder || to.kind == footLadder:
			// Climb the ladder, or step off its bottom onto the ground.
			if dy > 0 && to.kind == footGround && from.kind != footLadder {
				continue
			}
			if dy > 0 && !world.ColumnFree(pos.X, pos.Z, from.feet, to.feet+playerHeight) {
				continue
			}
			if dy > 0 {
				add(dest, 1.5, MoveLadderUp)
			} else {
				add(dest, 1.5, MoveLadderDown)
			}
		case from.kind == footWater && (to.kind == footWater || dy < 0):
			add(dest, swimCost(to), MoveSwim)
		case to.kind == footWater && dy > 0:
			add(dest, swimCost(to), MoveSwim)
		}
	}

	cardinals := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for _, dir := range cardinals {
		dx, dz := dir[0], dir[1]

		// --- Same level, one up, one down ---
		for _, dy := range []int{0, 1, -1} {
			dest := pos.Add(dx, dy, dz)
			to := footingAt(world, dest, opts)
			if to.kind == footNone {
				continue
			}
			if move, cost, ok := transition(world, pos, from, dest, to); ok {
				add(dest, cost, move)
			}
		}

		// --- Longer drops (2+ blocks) ---
		addDrops(world, pos, from, dx, dz, opts, add)

		// --- Sprint-jump over a 1-block gap ---
		if opts.Sprint && from.kind == footGround {
			addGapJumps(world, pos, from, dx, dz, opts, add)
		}
	}

	// --- Diagonals on (nearly) flat ground ---
	if from.kind == footGround {
		diagonals := [4][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
		for _, dir := range diagonals {
			dx, dz := dir[0], dir[1]
			dest := pos.Add(dx, 0, dz)
			to := footingAt(world, dest, opts)
			if to.kind != footGround || math.Abs(to.feet-from.feet) > stepHeight+heightEps {
				continue
			}
			// No corner cutting: both side columns must be clear.
			lo := math.Max(from.feet, to.feet)
			if !world.ColumnFree(pos.X+dx, pos.Z, lo, lo+playerHeight) ||
				!world.ColumnFree(pos.X, pos.Z+dz, lo, lo+playerHeight) {
				continue
			}
			add(dest, 1.41, MoveDiagonal)
		}
	}

	return neighbors
}

// transition decides how to move between horizontally adjacent nodes.
func transition(w WorldView, from Vec3, f footing, to Vec3, t footing) (MoveType, float64, bool) {
	rise := t.feet - f.feet
	top := math.Max(f.feet, t.feet) + playerHeight

	switch {
	case t.kind == footLadder:
		// Walk (or swim) into a ladder column at about the same height, or
		// step down onto the top of a ladder going down a shaft.
		if rise > stepHeight+heightEps || rise < -jumpHeight || f.kind == footLadder {
			return 0, 0, false
		}
		if !w.ColumnFree(to.X, to.Z, t.feet, top) || !w.ColumnFree(from.X, from.Z, f.feet, top) {
			return 0, 0, false
		}
		if rise < -stepHeight {
			return MoveDrop, 1.5, true
		}
		return MoveWalk, 1.2, true

	case t.kind == footWater:
		if rise > heightEps { // swim up and over: only from water
			if f.kind != footWater || rise > 1+heightEps {
				return 0, 0, false
			}
		}
		if !w.ColumnFree(to.X, to.Z, t.feet, top) || !w.ColumnFree(from.X, from.Z, f.feet, top) {
			return 0, 0, false
		}
		return MoveSwim, swimCost(t), true
	}

	// To the ground.
	switch {
	case rise > jumpHeight:
		return 0, 0, false

	case rise > stepHeight+heightEps:
		// Jump onto a ledge (from the ground), climb out of water onto it,
		// or step off the top of a ladder.
		if f.kind == footWater && rise > 1.0+heightEps {
			return 0, 0, false
		}
		if !w.ColumnFree(from.X, from.Z, f.feet, t.feet+playerHeight) {
			return 0, 0, false
		}
		cost := 2.0
		if f.kind == footWater {
			cost = 3.0
		}
		return MoveJump, cost, true

	case rise >= -stepHeight-heightEps:
		// Walk; stepping up or down a little is handled by physics.
		if f.kind == footLadder && rise < -heightEps {
			return 0, 0, false // let the ladder-down move handle this
		}
		if !w.ColumnFree(to.X, to.Z, t.feet, top) || !w.ColumnFree(from.X, from.Z, f.feet, top) {
			return 0, 0, false
		}
		if f.kind == footWater {
			return MoveJump, 2.5, true // climb out of the water
		}
		return MoveWalk, 1.0, true

	default:
		// Drop down to a lower floor (up to ~1.5 blocks here; longer
		// drops come from addDrops).
		if f.kind == footLadder {
			return 0, 0, false
		}
		if !w.ColumnFree(to.X, to.Z, t.feet, f.feet+playerHeight) {
			return 0, 0, false
		}
		return MoveDrop, 1.0 - rise*0.5, true
	}
}

// addDrops adds falls of two or more blocks down from pos toward (dx, dz):
// onto ground within MaxFallDistance, or into water from any height that
// fits the search.
func addDrops(w WorldView, pos Vec3, from footing, dx, dz int, opts Options, add func(Vec3, float64, MoveType)) {
	if from.kind != footGround {
		return
	}
	col := pos.Add(dx, 0, dz)
	// Step out over the edge at the current height.
	if !w.ColumnFree(col.X, col.Z, from.feet, from.feet+playerHeight) {
		return
	}
	maxScan := opts.MaxFallDistance + 1
	if opts.AllowWater {
		maxScan = max(maxScan, 24)
	}
	for k := 2; k <= maxScan; k++ {
		p := Vec3{col.X, pos.Y - k, col.Z}
		to := footingAt(w, p, opts)
		if to.kind == footGround {
			fall := from.feet - to.feet
			if fall <= float64(opts.MaxFallDistance)+heightEps && w.ColumnFree(p.X, p.Z, to.feet, from.feet) {
				add(p, 1.0+fall*0.5, MoveDrop)
			}
			return
		}
		if to.kind == footWater {
			if w.ColumnFree(p.X, p.Z, to.feet, from.feet) {
				add(p, 2.0+float64(k)*0.3, MoveDrop)
			}
			return
		}
		if w.IsDangerous(p.X, p.Y, p.Z) || !w.ColumnFree(p.X, p.Z, float64(p.Y), float64(p.Y+1)) {
			return // hit something we cannot stand on
		}
	}
}

// addGapJumps adds sprint-jumps across a one-block gap (to the same level
// or one up).
func addGapJumps(w WorldView, pos Vec3, from footing, dx, dz int, opts Options, add func(Vec3, float64, MoveType)) {
	mid := pos.Add(dx, 0, dz)
	if _, ok := w.StandHeight(mid.X, mid.Y, mid.Z); ok {
		return // no gap
	}
	if footingAt(w, mid, opts).kind != footNone {
		return // a ladder or water, not a gap
	}
	apex := from.feet + jumpClear + playerHeight
	if !w.ColumnFree(pos.X, pos.Z, from.feet, apex) || !w.ColumnFree(mid.X, mid.Z, from.feet, apex) {
		return
	}
	for _, dy := range []int{0, 1} {
		far := pos.Add(dx*2, dy, dz*2)
		to := footingAt(w, far, opts)
		if to.kind != footGround {
			continue
		}
		rise := to.feet - from.feet
		if rise > 1.0+heightEps || rise < -stepHeight-heightEps {
			continue
		}
		if !w.ColumnFree(far.X, far.Z, to.feet, apex) {
			continue
		}
		cost := 2.5
		if dy > 0 {
			cost = 3.5
		}
		add(far, cost, MoveSprintJump)
	}
}
