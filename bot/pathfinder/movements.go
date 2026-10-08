package pathfinder

// getNeighbors returns all reachable neighbor nodes from the current position.
// Each returned node's G field holds the EDGE COST (not cumulative).
func getNeighbors(current *Node, world WorldView, opts Options) []Node {
	var neighbors []Node
	pos := current.Pos

	// Cardinal directions
	cardinals := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	// Diagonal directions
	diagonals := [4][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

	for _, dir := range cardinals {
		dx, dz := dir[0], dir[1]

		// --- Walk (flat) ---
		dest := pos.Add(dx, 0, dz)
		if world.CanStandAt(dest.X, dest.Y, dest.Z) {
			neighbors = append(neighbors, Node{Pos: dest, G: 1.0, Move: MoveWalk})
		}

		// --- Jump up (1 block) ---
		// Need 3 air blocks above current pos (feet, head, above head),
		// and destination 1 block higher must be standable.
		up := pos.Add(dx, 1, dz)
		if world.IsPassable(pos.X, pos.Y+2, pos.Z) && // above head at current
			world.CanStandAt(up.X, up.Y, up.Z) &&
			world.IsPassable(up.X, up.Y+1, up.Z) { // head clear at destination+1
			neighbors = append(neighbors, Node{Pos: up, G: 2.0, Move: MoveJump})
		}

		// --- Drop down (1-3 blocks) ---
		if world.IsPassable(dest.X, dest.Y, dest.Z) && world.IsPassable(dest.X, dest.Y+1, dest.Z) {
			landY := world.IsSafeToFall(dest.X, dest.Y, dest.Z, opts.MaxFallDistance)
			if landY >= 0 {
				drop := Vec3{dest.X, landY, dest.Z}
				fallDist := pos.Y - landY
				cost := 1.0 + float64(fallDist)*0.5
				neighbors = append(neighbors, Node{Pos: drop, G: cost, Move: MoveDrop})
			}
		}

		// --- Sprint-jump (2-block gap) ---
		if opts.Sprint {
			// Jump across a 2-block horizontal gap
			far := pos.Add(dx*2, 0, dz*2)
			mid := pos.Add(dx, 0, dz)
			// Need: air at mid (feet+head), air above current (3 blocks), standable at far
			if world.IsPassable(mid.X, mid.Y, mid.Z) &&
				world.IsPassable(mid.X, mid.Y+1, mid.Z) &&
				world.IsPassable(pos.X, pos.Y+2, pos.Z) &&
				world.CanStandAt(far.X, far.Y, far.Z) {
				neighbors = append(neighbors, Node{Pos: far, G: 2.5, Move: MoveSprintJump})
			}
			// Sprint-jump across gap and 1 block up
			farUp := pos.Add(dx*2, 1, dz*2)
			if world.IsPassable(mid.X, mid.Y, mid.Z) &&
				world.IsPassable(mid.X, mid.Y+1, mid.Z) &&
				world.IsPassable(mid.X, mid.Y+2, mid.Z) &&
				world.IsPassable(pos.X, pos.Y+2, pos.Z) &&
				world.CanStandAt(farUp.X, farUp.Y, farUp.Z) {
				neighbors = append(neighbors, Node{Pos: farUp, G: 3.5, Move: MoveSprintJump})
			}
		}

		// --- Ladder up ---
		if opts.AllowLadder {
			above := pos.Add(0, 1, 0)
			if world.IsClimbable(pos.X, pos.Y, pos.Z) || world.IsClimbable(above.X, above.Y, above.Z) {
				climbDest := pos.Add(0, 1, 0)
				if world.IsPassable(climbDest.X, climbDest.Y+1, climbDest.Z) {
					neighbors = append(neighbors, Node{Pos: climbDest, G: 1.5, Move: MoveLadderUp})
				}
			}
			// Also check climbing from adjacent ladder
			adjLadder := pos.Add(dx, 0, dz)
			if world.IsClimbable(adjLadder.X, adjLadder.Y, adjLadder.Z) {
				climbDest := adjLadder.Add(0, 1, 0)
				if world.IsPassable(climbDest.X, climbDest.Y, climbDest.Z) &&
					world.IsPassable(climbDest.X, climbDest.Y+1, climbDest.Z) {
					neighbors = append(neighbors, Node{Pos: climbDest, G: 2.0, Move: MoveLadderUp})
				}
			}
		}

		// --- Ladder down ---
		if opts.AllowLadder {
			below := pos.Add(0, -1, 0)
			if world.IsClimbable(pos.X, pos.Y, pos.Z) || world.IsClimbable(below.X, below.Y, below.Z) {
				if world.IsPassable(below.X, below.Y, below.Z) ||
					world.IsClimbable(below.X, below.Y, below.Z) {
					neighbors = append(neighbors, Node{Pos: below, G: 1.5, Move: MoveLadderDown})
				}
			}
		}

		// --- Swim ---
		if opts.AllowWater {
			// Swim horizontally
			if world.CanStandInWater(dest.X, dest.Y, dest.Z) {
				neighbors = append(neighbors, Node{Pos: dest, G: 2.0, Move: MoveSwim})
			}
			// Swim up
			swimUp := pos.Add(dx, 1, dz)
			if world.IsWater(swimUp.X, swimUp.Y, swimUp.Z) &&
				(world.IsPassable(swimUp.X, swimUp.Y+1, swimUp.Z) || world.IsWater(swimUp.X, swimUp.Y+1, swimUp.Z)) {
				neighbors = append(neighbors, Node{Pos: swimUp, G: 2.5, Move: MoveSwim})
			}
			// Swim down
			swimDown := pos.Add(dx, -1, dz)
			if world.IsWater(swimDown.X, swimDown.Y, swimDown.Z) {
				neighbors = append(neighbors, Node{Pos: swimDown, G: 2.0, Move: MoveSwim})
			}
			// Swim straight up (no horizontal movement)
			straightUp := pos.Add(0, 1, 0)
			if world.IsWater(pos.X, pos.Y, pos.Z) &&
				(world.IsWater(straightUp.X, straightUp.Y, straightUp.Z) ||
					world.IsPassable(straightUp.X, straightUp.Y, straightUp.Z)) {
				neighbors = append(neighbors, Node{Pos: straightUp, G: 2.0, Move: MoveSwim})
			}
			// Swim straight down
			straightDown := pos.Add(0, -1, 0)
			if world.IsWater(pos.X, pos.Y, pos.Z) && world.IsWater(straightDown.X, straightDown.Y, straightDown.Z) {
				neighbors = append(neighbors, Node{Pos: straightDown, G: 1.5, Move: MoveSwim})
			}
		}
	}

	// Diagonal movement
	for _, dir := range diagonals {
		dx, dz := dir[0], dir[1]
		dest := pos.Add(dx, 0, dz)

		// Both adjacent cardinal directions must be passable (no corner cutting)
		adj1 := pos.Add(dx, 0, 0)
		adj2 := pos.Add(0, 0, dz)

		if world.CanStandAt(dest.X, dest.Y, dest.Z) &&
			world.IsPassable(adj1.X, adj1.Y, adj1.Z) && world.IsPassable(adj1.X, adj1.Y+1, adj1.Z) &&
			world.IsPassable(adj2.X, adj2.Y, adj2.Z) && world.IsPassable(adj2.X, adj2.Y+1, adj2.Z) {
			neighbors = append(neighbors, Node{Pos: dest, G: 1.41, Move: MoveDiagonal})
		}
	}

	return neighbors
}
