package pathfinder

import (
	"container/heap"
	"errors"
	"math"
)

var (
	ErrNoPath        = errors.New("no path found")
	ErrTooFar        = errors.New("path exceeds maximum length")
	ErrMaxIterations = errors.New("exceeded maximum iterations")
	ErrUnloaded      = errors.New("start chunk not loaded")
	ErrUnreachable   = errors.New("goal unreachable")
)

// WorldView provides read-only access to the world for the pathfinder.
type WorldView interface {
	HasChunk(x, z int) bool
	IsWater(x, y, z int) bool
	IsClimbable(x, y, z int) bool
	IsDangerous(x, y, z int) bool
	// StandHeight returns the feet height when standing centered in block
	// column (x, z) with the feet in block y, and whether the player fits.
	StandHeight(x, y, z int) (float64, bool)
	// ColumnFree reports whether the centered player column of (x, z) is
	// clear of collisions between heights y0 and y1.
	ColumnFree(x, z int, y0, y1 float64) bool
}

// FindPath computes an A* path from start to goal.
//
// On success it returns the nodes from start to goal (inclusive) and a nil
// error. If the goal cannot be reached (ErrNoPath) or the search gives up
// (ErrMaxIterations), it ALSO returns a partial path from start to the
// explored node closest to the goal, so callers can move closer and re-plan.
// The partial path is nil when no node is closer than the start.
//
// A goal in an unloaded chunk is allowed (the search heads toward it); only an
// unloaded start chunk returns ErrUnloaded.
func FindPath(start, goal Vec3, world WorldView, opts Options) ([]Node, error) {
	if opts.MaxIterations == 0 {
		opts = DefaultOptions()
	}
	if !world.HasChunk(start.X, start.Z) {
		return nil, ErrUnloaded
	}

	startNode := &Node{
		Pos: start,
		G:   0,
		H:   heuristic(start, goal),
	}
	startNode.F = startNode.G + startNode.H

	openSet := &nodeHeap{}
	heap.Init(openSet)
	heap.Push(openSet, startNode)

	closedSet := make(map[Vec3]bool)
	gScores := map[Vec3]float64{start: 0}

	iterations := 0
	best := startNode // explored node closest to the goal

	for openSet.Len() > 0 {
		iterations++
		if iterations > opts.MaxIterations {
			return partialPath(startNode, best), ErrMaxIterations
		}

		current := heap.Pop(openSet).(*Node)
		// A position can be pushed several times with decreasing cost;
		// only the first (cheapest) pop is expanded.
		if closedSet[current.Pos] {
			continue
		}

		if current.Pos.Equals(goal) {
			return reconstructPath(current), nil
		}

		closedSet[current.Pos] = true
		if current.H < best.H || (current.H == best.H && current.G < best.G) {
			best = current
		}

		neighbors := getNeighbors(current, world, opts)
		for _, neighbor := range neighbors {
			if closedSet[neighbor.Pos] {
				continue
			}

			tentativeG := current.G + neighbor.G // neighbor.G holds the edge cost initially

			existing, exists := gScores[neighbor.Pos]
			if exists && tentativeG >= existing {
				continue
			}

			gScores[neighbor.Pos] = tentativeG

			node := &Node{
				Pos:    neighbor.Pos,
				G:      tentativeG,
				H:      heuristic(neighbor.Pos, goal),
				Parent: current,
				Move:   neighbor.Move,
				Depth:  current.Depth + 1,
			}
			node.F = node.G + node.H

			// Path length in nodes, including the start, is Depth+1.
			if opts.MaxPathLength > 0 && node.Depth+1 > opts.MaxPathLength {
				continue
			}

			heap.Push(openSet, node)
		}
	}

	return partialPath(startNode, best), ErrNoPath
}

// partialPath returns the path to best, or nil if best is the start.
func partialPath(start, best *Node) []Node {
	if best == start {
		return nil
	}
	return reconstructPath(best)
}

// heuristic uses octile distance in 3D for A*.
func heuristic(a, goal Vec3) float64 {
	dx := math.Abs(float64(a.X - goal.X))
	dy := math.Abs(float64(a.Y - goal.Y))
	dz := math.Abs(float64(a.Z - goal.Z))
	// Octile distance on XZ plane + vertical cost
	maxXZ := math.Max(dx, dz)
	minXZ := math.Min(dx, dz)
	return (maxXZ - minXZ) + minXZ*1.41 + dy*1.5
}

func reconstructPath(node *Node) []Node {
	var path []Node
	for n := node; n != nil; n = n.Parent {
		path = append(path, *n)
	}
	// Reverse
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// nodeHeap implements heap.Interface for A* open set (min-heap on F cost).
type nodeHeap []*Node

func (h nodeHeap) Len() int           { return len(h) }
func (h nodeHeap) Less(i, j int) bool { return h[i].F < h[j].F }
func (h nodeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *nodeHeap) Push(x any) {
	*h = append(*h, x.(*Node))
}

func (h *nodeHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}
