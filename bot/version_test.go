package bot

import (
	"bytes"
	"testing"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

func TestResolveVersion(t *testing.T) {
	for _, tc := range []struct {
		in    string
		proto int
	}{
		{"26.2", 776},
		{"776", 776},
		{"1.21.11", 774},
		{"774", 774},
		{LatestVersion, 776},
	} {
		v, err := ResolveVersion(tc.in)
		if err != nil || v.ProtocolNumber != tc.proto {
			t.Errorf("ResolveVersion(%q) = %d, %v; want %d", tc.in, v.ProtocolNumber, err, tc.proto)
		}
	}
	for _, bad := range []string{"1.20.4", "999", "", "26"} {
		if _, err := ResolveVersion(bad); err == nil {
			t.Errorf("ResolveVersion(%q) succeeded, want error", bad)
		}
	}
	if got := SupportedVersions(); len(got) != 2 || got[0] != "1.21.11" || got[1] != "26.2" {
		t.Errorf("SupportedVersions() = %v", got)
	}
}

// Packet IDs that moved between 1.21.11 and 26.2 (from Mojang's packets.json).
func TestPacketIDs262(t *testing.T) {
	v, _ := ResolveVersion("26.2")
	ids := v.IDs
	for name, got := range map[string][2]int32{
		"SB_Chat":         {ids.SB_Chat, 0x09},
		"SB_PlayerInput":  {ids.SB_PlayerInput, 0x2B},
		"CB_Login":        {ids.CB_Login, 0x31},
		"CB_PlayerChat":   {ids.CB_PlayerChat, 0x41},
		"CB_SystemChat":   {ids.CB_SystemChat, 0x79},
		"CB_ChunkData":    {ids.CB_ChunkData, 0x2D},
		"CB_Disconnect":   {ids.CB_Disconnect, 0x00},
		"CB_LoginSuccess": {ids.CB_LoginSuccess, 0x02},
	} {
		if got[0] != got[1] {
			t.Errorf("%s = 0x%02X, want 0x%02X", name, got[0], got[1])
		}
	}
}

// Block state IDs from Mojang's 26.2 reports/blocks.json.
func TestClassifyBlock262(t *testing.T) {
	v, _ := ResolveVersion("26.2")
	for _, tc := range []struct {
		name  string
		state uint32
		want  BlockType
	}{
		{"air", 0, BlockAir},
		{"stone", 1, BlockSolid},
		{"water", 86, BlockWater},
		{"lava", 102, BlockDangerous},
		{"cobweb", 2247, BlockDangerous},
		{"short_grass", 2248, BlockAir},
		{"ladder", 5719, BlockClimbable},
		{"oak_slab", 13330, BlockSolid},
		{"cave_air", 15293, BlockAir},
		{"sulfur (new in 26.2)", 24687, BlockSolid},
		{"sulfur_spike (new in 26.2)", 30229, BlockSolid},
		{"out of range", 1 << 20, BlockSolid},
	} {
		if got := ClassifyBlockFor(v, tc.state); got != tc.want {
			t.Errorf("%s (%d) = %d, want %d", tc.name, tc.state, got, tc.want)
		}
	}
}

// section builds one chunk section: blocks single-valued (stone), biomes
// either single-valued or direct (global palette, 7 bits) to exercise the
// biome threshold.
func section(fluidCount, directBiomes bool) []byte {
	var buf bytes.Buffer
	pk.Short(4096).WriteTo(&buf) // non-empty block count
	if fluidCount {
		pk.Short(0).WriteTo(&buf)
	}
	buf.WriteByte(0)           // blocks: 0 bits = single value
	pk.VarInt(1).WriteTo(&buf) // stone
	if directBiomes {
		buf.WriteByte(7) // > 3 bits: global palette, no palette list
		for i := 0; i < dataArrayLongCount(7, biomeEntries); i++ {
			pk.Long(0).WriteTo(&buf)
		}
	} else {
		buf.WriteByte(0)
		pk.VarInt(0).WriteTo(&buf)
	}
	return buf.Bytes()
}

func TestParseChunkSectionVersions(t *testing.T) {
	for _, tc := range []struct {
		name         string
		fluidCount   bool
		directBiomes bool
	}{
		{"1.21.11", false, false},
		{"26.2 (fluid count)", true, false},
		{"26.2 direct biomes", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Two sections back to back: the second only parses if the
			// first consumed exactly its own bytes.
			data := append(section(tc.fluidCount, tc.directBiomes), section(tc.fluidCount, tc.directBiomes)...)
			r := bytes.NewReader(data)
			for i := 0; i < 2; i++ {
				s, err := parseChunkSection(r, tc.fluidCount)
				if err != nil {
					t.Fatalf("section %d: %v", i, err)
				}
				if s.BitsPerEntry != 0 || len(s.Palette) != 1 || s.Palette[0] != 1 {
					t.Fatalf("section %d: got bpe=%d palette=%v, want single stone", i, s.BitsPerEntry, s.Palette)
				}
			}
			if r.Len() != 0 {
				t.Fatalf("%d bytes left over", r.Len())
			}
		})
	}
}

func TestDisconnectReason(t *testing.T) {
	p := pk.Marshal(pk.VarInt(0x20), pk.NBT(map[string]any{
		"translate": "multiplayer.disconnect.kicked",
	}))
	if got := readDisconnectReason(p); got != "Kicked by an operator" {
		t.Fatalf("reason = %q", got)
	}
	p = pk.Marshal(pk.VarInt(0x20), pk.NBT("Server closed"))
	if got := readDisconnectReason(p); got != "Server closed" {
		t.Fatalf("reason = %q", got)
	}
}
