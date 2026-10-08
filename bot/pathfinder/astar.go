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
	ErrUnloaded      = errors.New("start or goal in unloaded chunk")
)

// WorldView provides read-only access to the world for the pathfinder.
type WorldView interface {
	GetBlock(x, y, z int) uint32
	HasChunk(x, z int) bool
	IsBlockSolid(x, y, z int) bool
	IsPassable(x, y, z int) bool
	IsWater(x, y, z int) bool
	IsClimbable(x, y, z int) bool
	IsDangerous(x, y, z int) bool
	CanStandAt(x, y, z int) bool
	CanStandInWater(x, y, z int) bool
	IsSafeToFall(x, startY, z, maxDrop int) int
}

// FindPath computes an A* path from start to goal.
// Returns a slice of Nodes from start to goal (inclusive).
func FindPath(start, goal Vec3, world WorldView, opts Options) ([]Node, error) {
	if opts.MaxIterations == 0 {
		opts = DefaultOptions()
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

	for openSet.Len() > 0 {
		iterations++
		if iterations > opts.MaxIterations {
			return nil, ErrMaxIterations
		}

		current := heap.Pop(openSet).(*Node)

		if current.Pos.Equals(goal) {
			return reconstructPath(current), nil
		}

		closedSet[current.Pos] = true

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
			}
			node.F = node.G + node.H

			if opts.MaxPathLength > 0 {
				pathLen := countPathLength(node)
				if pathLen > opts.MaxPathLength {
					continue
				}
			}

			heap.Push(openSet, node)
		}
	}

	return nil, ErrNoPath
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

func countPathLength(node *Node) int {
	count := 0
	for n := node; n != nil; n = n.Parent {
		count++
	}
	return count
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
