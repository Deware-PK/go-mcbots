package pathfinder

import (
	"fmt"
	"math"
)

// Vec3 represents a block position in the world.
type Vec3 struct {
	X, Y, Z int
}

func (v Vec3) String() string {
	return fmt.Sprintf("(%d, %d, %d)", v.X, v.Y, v.Z)
}

func (v Vec3) Equals(other Vec3) bool {
	return v.X == other.X && v.Y == other.Y && v.Z == other.Z
}

func (v Vec3) Add(dx, dy, dz int) Vec3 {
	return Vec3{v.X + dx, v.Y + dy, v.Z + dz}
}

func (v Vec3) DistanceTo(other Vec3) float64 {
	dx := float64(v.X - other.X)
	dy := float64(v.Y - other.Y)
	dz := float64(v.Z - other.Z)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// MoveType describes how the bot moves between two path nodes.
type MoveType int

const (
	MoveWalk       MoveType = iota // flat ground movement
	MoveDiagonal                   // diagonal ground movement
	MoveJump                       // 1-block jump up
	MoveDrop                       // 1-3 block drop down
	MoveSprintJump                 // sprint-jump across 2-block gap
	MoveLadderUp                   // climb ladder/vine up
	MoveLadderDown                 // climb ladder/vine down
	MoveSwim                       // swim in water
)

func (m MoveType) String() string {
	switch m {
	case MoveWalk:
		return "walk"
	case MoveDiagonal:
		return "diagonal"
	case MoveJump:
		return "jump"
	case MoveDrop:
		return "drop"
	case MoveSprintJump:
		return "sprint_jump"
	case MoveLadderUp:
		return "ladder_up"
	case MoveLadderDown:
		return "ladder_down"
	case MoveSwim:
		return "swim"
	default:
		return "unknown"
	}
}

// Node is a single step in a path computed by the A* algorithm.
type Node struct {
	Pos    Vec3
	G      float64  // cost from start to this node
	H      float64  // heuristic cost from this node to goal
	F      float64  // G + H
	Parent *Node
	Move   MoveType // how we got here from parent
}

// Options configures the A* pathfinder.
type Options struct {
	MaxIterations   int     // max A* iterations before giving up (default 5000)
	MaxFallDistance  int     // max safe fall distance in blocks (default 3)
	AllowWater      bool    // allow swimming through water
	AllowLadder     bool    // allow climbing ladders/vines
	Sprint          bool    // allow sprint-jumping across gaps
	MaxPathLength   int     // max path length (default 200)
}

func DefaultOptions() Options {
	return Options{
		MaxIterations:  5000,
		MaxFallDistance: 3,
		AllowWater:     true,
		AllowLadder:    true,
		Sprint:         true,
		MaxPathLength:  200,
	}
}
