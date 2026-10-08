package bot

import (
	"bytes"
	"fmt"
	"log"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

// dimensionType is one entry of the minecraft:dimension_type registry.
type dimensionType struct {
	Name   string
	MinY   int
	Height int
	Known  bool // false if neither the server nor vanillaDimensionTypes gave us min_y/height
}

// vanillaDimensionTypes holds min_y/height of the built-in dimension types.
// The server omits their NBT because the bot declares the minecraft:core
// known pack. Source: PrismarineJS/minecraft-data pc/1.21.11 loginPacket.json.
var vanillaDimensionTypes = map[string][2]int{
	"minecraft:overworld":       {-64, 384},
	"minecraft:overworld_caves": {-64, 384},
	"minecraft:the_end":         {0, 256},
	"minecraft:the_nether":      {0, 256},
}

// parseRegistryData returns the registry ID and, for minecraft:dimension_type,
// its entries in registry order (the index used by Login/Respawn).
func parseRegistryData(data []byte) (string, []dimensionType, error) {
	r := bytes.NewReader(data)
	var id pk.String
	if _, err := id.ReadFrom(r); err != nil {
		return "", nil, fmt.Errorf("registry id: %w", err)
	}
	if id != "minecraft:dimension_type" {
		return string(id), nil, nil
	}

	var count pk.VarInt
	if _, err := count.ReadFrom(r); err != nil {
		return string(id), nil, fmt.Errorf("entry count: %w", err)
	}
	dims := make([]dimensionType, 0, int(count))
	for i := 0; i < int(count); i++ {
		var key pk.String
		var hasData pk.Boolean
		if _, err := key.ReadFrom(r); err != nil {
			return string(id), nil, fmt.Errorf("entry %d key: %w", i, err)
		}
		if _, err := hasData.ReadFrom(r); err != nil {
			return string(id), nil, fmt.Errorf("entry %d has-data: %w", i, err)
		}
		dim := dimensionType{Name: string(key)}
		if hasData {
			var v struct {
				MinY   int32 `nbt:"min_y"`
				Height int32 `nbt:"height"`
			}
			if _, err := (pk.NBTField{V: &v, AllowUnknownFields: true}).ReadFrom(r); err != nil {
				return string(id), nil, fmt.Errorf("entry %d (%s) nbt: %w", i, key, err)
			}
			dim.MinY, dim.Height, dim.Known = int(v.MinY), int(v.Height), v.Height > 0
		} else if mh, ok := vanillaDimensionTypes[dim.Name]; ok {
			dim.MinY, dim.Height, dim.Known = mh[0], mh[1], true
		}
		dims = append(dims, dim)
	}
	return string(id), dims, nil
}

// applyDimension switches the world to the dimension type at index and
// drops all loaded chunks.
func (b *Bot) applyDimension(index int) {
	minY, height := b.world.MinY, b.world.Height
	if index >= 0 && index < len(b.dimTypes) && b.dimTypes[index].Known {
		minY, height = b.dimTypes[index].MinY, b.dimTypes[index].Height
	} else {
		log.Printf("[World] unknown dimension type %d; keeping min_y=%d height=%d", index, minY, height)
	}
	b.world.reset(minY, height)
}

// readSpawnInfoDimension reads the leading "dimension type" VarInt of SpawnInfo.
func readSpawnInfoDimension(r *bytes.Reader) (int, error) {
	var dim pk.VarInt
	if _, err := dim.ReadFrom(r); err != nil {
		return 0, err
	}
	return int(dim), nil
}
