package bot

import (
	"testing"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

func dimensionRegistryPacket() []byte {
	tall := struct {
		MinY   int32  `nbt:"min_y"`
		Height int32  `nbt:"height"`
		Other  string `nbt:"infiniburn"`
	}{-128, 512, "#minecraft:infiniburn_overworld"}

	return pk.Marshal(0,
		pk.String("minecraft:dimension_type"),
		pk.VarInt(3),
		pk.String("minecraft:overworld"), pk.Boolean(false), // vanilla: data omitted (known pack)
		pk.String("test:tall"), pk.Boolean(true), pk.NBT(tall),
		pk.String("test:mystery"), pk.Boolean(false),
	).Data
}

func TestParseRegistryData(t *testing.T) {
	id, dims, err := parseRegistryData(dimensionRegistryPacket())
	if err != nil {
		t.Fatal(err)
	}
	if id != "minecraft:dimension_type" {
		t.Fatalf("id = %q", id)
	}
	want := []dimensionType{
		{"minecraft:overworld", -64, 384, true},
		{"test:tall", -128, 512, true},
		{"test:mystery", 0, 0, false},
	}
	if len(dims) != len(want) {
		t.Fatalf("got %d entries, want %d", len(dims), len(want))
	}
	for i := range want {
		if dims[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, dims[i], want[i])
		}
	}

	other := pk.Marshal(0, pk.String("minecraft:chat_type"), pk.VarInt(0)).Data
	if id, dims, err := parseRegistryData(other); err != nil || id != "minecraft:chat_type" || dims != nil {
		t.Fatalf("other registry: id=%q dims=%v err=%v", id, dims, err)
	}
}

func newTestBot(t *testing.T) *Bot {
	t.Helper()
	ver, err := ResolveVersion("1.21.11")
	if err != nil {
		t.Fatal(err)
	}
	b := New("test", ver)
	_, b.dimTypes, err = parseRegistryData(dimensionRegistryPacket())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLoginSetsDimension(t *testing.T) {
	b := newTestBot(t)
	login := pk.Marshal(b.version.IDs.CB_Login,
		pk.Int(42),        // entity ID
		pk.Boolean(false), // hardcore
		pk.VarInt(2),      // world names
		pk.String("minecraft:overworld"), pk.String("test:tall"),
		pk.VarInt(20), pk.VarInt(10), pk.VarInt(10), // max players, view, simulation distance
		pk.Boolean(false), pk.Boolean(true), pk.Boolean(false),
		pk.VarInt(1), // SpawnInfo.dimension = test:tall
		pk.String("test:tall"),
	)
	if err := b.handleLogin(login); err != nil {
		t.Fatal(err)
	}
	if b.state.EntityID != 42 {
		t.Errorf("entity ID = %d", b.state.EntityID)
	}
	if b.world.MinY != -128 || b.world.Height != 512 {
		t.Errorf("world min_y/height = %d/%d, want -128/512", b.world.MinY, b.world.Height)
	}
}

func TestRespawnResetsAndWaitsForPositionSync(t *testing.T) {
	b := newTestBot(t)
	b.awaitingSpawn.Store(false) // already in the world
	b.world.SetChunk(&ChunkColumn{X: 0, Z: 0, MinY: -64})
	b.state.SetVelocity(1, 2, 3)
	b.state.SetAlive(false)

	respawn := pk.Marshal(b.version.IDs.CB_Respawn, pk.VarInt(0), pk.String("minecraft:overworld"))
	if err := b.handleRespawn(respawn); err != nil {
		t.Fatal(err)
	}
	if !b.awaitingSpawn.Load() {
		t.Error("not awaiting spawn after respawn")
	}
	if !b.state.IsAlive() || b.state.IsOnGround() {
		t.Errorf("alive=%v onGround=%v, want alive and not on ground", b.state.IsAlive(), b.state.IsOnGround())
	}
	if vx, vy, vz := b.state.GetVelocity(); vx != 0 || vy != 0 || vz != 0 {
		t.Errorf("velocity not reset: %v %v %v", vx, vy, vz)
	}
	if b.world.ChunkCount() != 0 {
		t.Errorf("chunks not cleared: %d", b.world.ChunkCount())
	}
	if b.world.MinY != -64 || b.world.Height != 384 {
		t.Errorf("world min_y/height = %d/%d, want -64/384", b.world.MinY, b.world.Height)
	}
}

func TestHealthUpdateDoesNotEmitSpawn(t *testing.T) {
	b := newTestBot(t)
	spawns := 0
	b.Events.OnSpawn = func() { spawns++ }
	b.state.SetAlive(false)

	health := pk.Marshal(b.version.IDs.CB_UpdateHealth, pk.Float(20), pk.VarInt(20), pk.Float(5))
	if err := b.handleUpdateHealth(health); err != nil {
		t.Fatal(err)
	}
	if spawns != 0 {
		t.Errorf("OnSpawn fired %d times from a health update", spawns)
	}
}
