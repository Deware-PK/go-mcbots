package bot

import (
	"math"
	"testing"
)

func shapeTop(boxes []aabb) float64 {
	top := 0.0
	for _, b := range boxes {
		top = math.Max(top, b.maxY)
	}
	return top
}

// State IDs from Mojang's 26.2 reports/blocks.json.
func TestBlockShapes262(t *testing.T) {
	ver, err := ResolveVersion("26.2")
	if err != nil {
		t.Fatal(err)
	}
	s := shapesFor(ver.BlockShapes)
	if s == nil || len(s.perState) != len(ver.BlockClasses) {
		t.Fatalf("shape table does not cover every state")
	}
	tests := []struct {
		name  string
		state uint32
		boxes int
		top   float64
	}{
		{"air", 0, 0, 0},
		{"stone", 1, 1, 1},
		{"water", 86, 0, 0},
		{"oak_slab bottom", 13333, 1, 0.5},
		{"oak_slab top", 13331, 1, 1},
		{"oak_stairs east bottom straight", 3978, 2, 1},
		{"oak_fence post", 6996, 1, 1.5},
		{"ladder facing north", 5720, 1, 1},
		{"white_carpet", 12896, 1, 1.0 / 16},
		{"dirt_path", 14815, 1, 15.0 / 16},
		{"sulfur_slab bottom (new in 26.2)", 24696, 1, 0.5},
	}
	for _, tt := range tests {
		boxes := s.boxes(tt.state)
		if len(boxes) != tt.boxes || shapeTop(boxes) != tt.top {
			t.Errorf("%s (%d): %d boxes, top %v; want %d, %v: %v",
				tt.name, tt.state, len(boxes), shapeTop(boxes), tt.boxes, tt.top, boxes)
		}
	}

	// The ladder facing north hangs on the south side of its block.
	if b := s.boxes(5720)[0]; b.minZ < 0.8 || b.maxZ != 1 {
		t.Errorf("ladder box %+v, want a thin box at z=0.8125..1", b)
	}
	// Unknown states are full cubes.
	if got := s.boxes(1 << 20); len(got) != 1 || got[0] != fullCube[0] {
		t.Errorf("unknown state: %v", got)
	}
}

// State IDs from PrismarineJS/minecraft-data pc/1.21.11 blocks.json.
func TestBlockShapes12111(t *testing.T) {
	ver, err := ResolveVersion("1.21.11")
	if err != nil {
		t.Fatal(err)
	}
	s := shapesFor(ver.BlockShapes)
	if s == nil || len(s.perState) != len(ver.BlockClasses) {
		t.Fatalf("shape table does not cover every state")
	}
	// oak_slab: minStateId 13128 = type=top,waterlogged=true; +3 = bottom,false.
	if top := shapeTop(s.boxes(13128 + 3)); top != 0.5 {
		t.Errorf("oak_slab bottom top = %v", top)
	}
	if top := shapeTop(s.boxes(1)); top != 1 {
		t.Errorf("stone top = %v", top)
	}
}
