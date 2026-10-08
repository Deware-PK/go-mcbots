package bot

import (
	"testing"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

const stone = 1 // same ID in 1.21.11 and 26.2

// airWorld returns a 26.2 world with chunks around the origin loaded and
// filled with air (single-value sections, as servers send them).
func airWorld(t *testing.T) (*Bot, *World) {
	t.Helper()
	b := newTestBot(t)
	w := b.world
	for cx := int32(-2); cx <= 1; cx++ {
		for cz := int32(-2); cz <= 1; cz++ {
			col := &ChunkColumn{X: cx, Z: cz, MinY: w.MinY, Sections: make([]ChunkSection, w.Height/16)}
			for i := range col.Sections {
				col.Sections[i] = ChunkSection{Palette: []uint32{0}}
			}
			w.SetChunk(col)
		}
	}
	return b, w
}

func TestSetBlockExpandsSection(t *testing.T) {
	_, w := airWorld(t)
	w.SetBlock(3, 70, -5, stone)
	if got := w.GetBlock(3, 70, -5); got != stone {
		t.Fatalf("GetBlock after SetBlock = %d, want %d", got, stone)
	}
	if got := w.GetBlock(4, 70, -5); got != 0 {
		t.Fatalf("neighbour = %d, want air", got)
	}
	w.SetBlock(3, 70, -5, 0)
	if got := w.GetBlock(3, 70, -5); got != 0 {
		t.Fatalf("after removing: %d, want air", got)
	}
	// Unloaded chunk: ignored, no panic.
	w.SetBlock(1000, 70, 1000, stone)
}

func TestHandleBlockUpdate(t *testing.T) {
	b, w := airWorld(t)
	p := pk.Marshal(pk.VarInt(b.version.IDs.CB_BlockUpdate),
		pk.Position{X: -7, Y: -60, Z: 12}, pk.VarInt(stone))
	if err := b.handleBlockUpdate(p); err != nil {
		t.Fatal(err)
	}
	if got := w.GetBlock(-7, -60, 12); got != stone {
		t.Fatalf("block = %d, want stone", got)
	}
}

func TestHandleSectionBlocksUpdate(t *testing.T) {
	b, w := airWorld(t)
	// Section (-1, -4, 0) holds x in [-16,-1], y in [-64,-49], z in [0,15].
	sx, sy, sz := int64(-1), int64(-4), int64(0)
	sectionPos := (sx&0x3FFFFF)<<42 | (sz&0x3FFFFF)<<20 | sy&0xFFFFF
	entry := func(state, lx, ly, lz int64) pk.VarLong {
		return pk.VarLong(state<<12 | lx<<8 | lz<<4 | ly)
	}
	p := pk.Marshal(pk.VarInt(b.version.IDs.CB_SectionBlocksUpdate),
		pk.Long(sectionPos), pk.VarInt(2),
		entry(stone, 15, 0, 3), // (-1, -64, 3)
		entry(stone, 0, 15, 9), // (-16, -49, 9)
	)
	if err := b.handleSectionBlocksUpdate(p); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][3]int{{-1, -64, 3}, {-16, -49, 9}} {
		if got := w.GetBlock(c[0], c[1], c[2]); got != stone {
			t.Errorf("block %v = %d, want stone", c, got)
		}
	}
	if got := w.GetBlock(-2, -64, 3); got != 0 {
		t.Errorf("untouched block = %d, want air", got)
	}
}

// The reported bug: a bot standing on a floating block must fall when the
// block is broken.
func TestBotFallsWhenBlockBelowIsBroken(t *testing.T) {
	b, w := airWorld(t)
	for x := -3; x <= 3; x++ {
		for z := -3; z <= 3; z++ {
			w.SetBlock(x, 63, z, stone) // ground
		}
	}
	w.SetBlock(0, 70, 0, stone) // floating block

	body0 := body{X: 0.5, Y: 71, Z: 0.5, OnGround: true}
	stay := run(w, body0, ControlState{}, 0, 20)
	if stay.Y != 71 || !stay.OnGround {
		t.Fatalf("on the block: y=%v onGround=%v, want to stay at 71", stay.Y, stay.OnGround)
	}

	// Someone breaks the block: the server sends a Block Update with air.
	p := pk.Marshal(pk.VarInt(b.version.IDs.CB_BlockUpdate), pk.Position{X: 0, Y: 70, Z: 0}, pk.VarInt(0))
	if err := b.handleBlockUpdate(p); err != nil {
		t.Fatal(err)
	}
	fell := run(w, stay, ControlState{}, 0, 60)
	if fell.Y != 64 || !fell.OnGround {
		t.Fatalf("after breaking: y=%v onGround=%v, want to land on the ground at 64", fell.Y, fell.OnGround)
	}
}
