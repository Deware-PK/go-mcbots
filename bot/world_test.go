package bot

import "testing"

// State IDs for 1.21.11, from PrismarineJS/minecraft-data pc/1.21.11 blocks.json.
func TestClassifyBlock(t *testing.T) {
	tests := []struct {
		name  string
		state uint32
		want  BlockType
	}{
		{"air", 0, BlockAir},
		{"stone", 1, BlockSolid},
		{"water level=0", 86, BlockWater},
		{"water level=15", 101, BlockWater},
		{"lava", 102, BlockDangerous},
		{"cobweb", 2047, BlockDangerous},
		{"short_grass", 2048, BlockAir},
		{"dandelion", 2121, BlockAir},
		{"torch", 3169, BlockAir},
		{"oak_sign", 5134, BlockAir},
		{"ladder", 5518, BlockClimbable},
		{"cactus (has collision)", 6728, BlockSolid},
		{"vine", 8157, BlockClimbable},
		{"oak_slab", 13128, BlockSolid},
		{"void_air", 15090, BlockAir},
		{"cave_air", 15091, BlockAir},
		{"bubble_column", 15092, BlockWater},
		{"out of range", 1 << 20, BlockSolid},
	}
	for _, tt := range tests {
		if got := ClassifyBlock(tt.state); got != tt.want {
			t.Errorf("ClassifyBlock(%d) [%s] = %d, want %d", tt.state, tt.name, got, tt.want)
		}
	}
}
