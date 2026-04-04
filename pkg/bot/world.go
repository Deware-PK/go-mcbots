package bot

import (
	"bytes"
	"fmt"
	"io"
	pk "go-mcbots/pkg/protocol/net/packet"
	"sync"
)

type chunkPos struct {
	X, Z int32
}

type ChunkSection struct {
	BlockCount   int16
	BitsPerEntry byte
	Palette      []uint32
	Data         []int64
}

type ChunkColumn struct {
	X, Z     int32
	Sections []ChunkSection
	MinY     int
}

type World struct {
	mu     sync.RWMutex
	chunks map[chunkPos]*ChunkColumn
	MinY   int
	Height int
}

func newWorld() *World {
	return &World{
		chunks: make(map[chunkPos]*ChunkColumn),
		MinY:   -64,
		Height: 384,
	}
}

func (w *World) GetBlock(x, y, z int) uint32 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	chunkX := int32(x >> 4)
	chunkZ := int32(z >> 4)
	pos := chunkPos{chunkX, chunkZ}

	col, ok := w.chunks[pos]
	if !ok {
		return 0
	}

	sectionIndex := (y - col.MinY) >> 4
	if sectionIndex < 0 || sectionIndex >= len(col.Sections) {
		return 0
	}

	section := &col.Sections[sectionIndex]
	if section.BitsPerEntry == 0 {
		if len(section.Palette) > 0 {
			return section.Palette[0]
		}
		return 0
	}

	localX := x & 0xF
	localY := y & 0xF
	localZ := z & 0xF
	blockIndex := (localY << 8) | (localZ << 4) | localX

	return getFromPalettedContainer(section, blockIndex)
}

func (w *World) HasChunk(x, z int) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.chunks[chunkPos{int32(x >> 4), int32(z >> 4)}]
	return ok
}

func (w *World) IsBlockSolid(x, y, z int) bool {
	blockState := w.GetBlock(x, y, z)
	return blockState != 0
}

func (w *World) IsBlockSolidOrUnloaded(x, y, z int) bool {
	if !w.HasChunk(x, z) {
		return true // assume solid when chunk not loaded (prevents falling through void)
	}
	return w.GetBlock(x, y, z) != 0
}

func (w *World) SetChunk(col *ChunkColumn) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.chunks[chunkPos{col.X, col.Z}] = col
}

func (w *World) UnloadChunk(x, z int32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.chunks, chunkPos{x, z})
}

func getFromPalettedContainer(section *ChunkSection, blockIndex int) uint32 {
	bpe := int(section.BitsPerEntry)
	if bpe == 0 {
		if len(section.Palette) > 0 {
			return section.Palette[0]
		}
		return 0
	}

	blocksPerLong := 64 / bpe
	longIndex := blockIndex / blocksPerLong
	bitOffset := (blockIndex % blocksPerLong) * bpe

	if longIndex >= len(section.Data) {
		return 0
	}

	value := uint32((section.Data[longIndex] >> bitOffset) & ((1 << bpe) - 1))

	if section.Palette != nil && int(value) < len(section.Palette) {
		return section.Palette[value]
	}
	return value
}

func (b *Bot) handleChunkData(p pk.Packet) error {
	r := bytes.NewReader(p.Data)

	var chunkX, chunkZ pk.Int
	if _, err := chunkX.ReadFrom(r); err != nil {
		return fmt.Errorf("chunk X: %w", err)
	}
	if _, err := chunkZ.ReadFrom(r); err != nil {
		return fmt.Errorf("chunk Z: %w", err)
	}

	// Heightmap is NBT compound in network format — skip it
	var heightmaps map[string]interface{}
	nbtField := pk.NBTField{V: &heightmaps, AllowUnknownFields: true}
	if _, err := nbtField.ReadFrom(r); err != nil {
		return nil
	}

	var dataSize pk.VarInt
	if _, err := dataSize.ReadFrom(r); err != nil {
		return fmt.Errorf("chunk data size: %w", err)
	}

	chunkDataBytes := make([]byte, int(dataSize))
	if _, err := io.ReadFull(r, chunkDataBytes); err != nil {
		return fmt.Errorf("chunk data read: %w", err)
	}

	col := &ChunkColumn{
		X:    int32(chunkX),
		Z:    int32(chunkZ),
		MinY: b.world.MinY,
	}

	numSections := b.world.Height / 16
	col.Sections = make([]ChunkSection, numSections)

	sectionReader := bytes.NewReader(chunkDataBytes)
	for i := 0; i < numSections; i++ {
		section, err := parseChunkSection(sectionReader)
		if err != nil {
			break
		}
		col.Sections[i] = section
	}

	b.world.SetChunk(col)
	return nil
}

func parseChunkSection(r *bytes.Reader) (ChunkSection, error) {
	var section ChunkSection

	var blockCount pk.Short
	if _, err := blockCount.ReadFrom(r); err != nil {
		return section, err
	}
	section.BlockCount = int16(blockCount)

	bpe, err := r.ReadByte()
	if err != nil {
		return section, err
	}
	section.BitsPerEntry = bpe

	if bpe == 0 {
		var singleValue pk.VarInt
		if _, err := singleValue.ReadFrom(r); err != nil {
			return section, err
		}
		section.Palette = []uint32{uint32(singleValue)}

		var dataLen pk.VarInt
		if _, err := dataLen.ReadFrom(r); err != nil {
			return section, err
		}
		section.Data = make([]int64, int(dataLen))
		for i := range section.Data {
			var v pk.Long
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
			section.Data[i] = int64(v)
		}
	} else if bpe <= 8 {
		var paletteLen pk.VarInt
		if _, err := paletteLen.ReadFrom(r); err != nil {
			return section, err
		}
		section.Palette = make([]uint32, int(paletteLen))
		for i := range section.Palette {
			var v pk.VarInt
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
			section.Palette[i] = uint32(v)
		}

		var dataLen pk.VarInt
		if _, err := dataLen.ReadFrom(r); err != nil {
			return section, err
		}
		section.Data = make([]int64, int(dataLen))
		for i := range section.Data {
			var v pk.Long
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
			section.Data[i] = int64(v)
		}
	} else {
		var dataLen pk.VarInt
		if _, err := dataLen.ReadFrom(r); err != nil {
			return section, err
		}
		section.Data = make([]int64, int(dataLen))
		for i := range section.Data {
			var v pk.Long
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
			section.Data[i] = int64(v)
		}
	}

	biomeBpe, err := r.ReadByte()
	if err != nil {
		return section, err
	}
	if biomeBpe == 0 {
		var singleBiome pk.VarInt
		if _, err := singleBiome.ReadFrom(r); err != nil {
			return section, err
		}
		var biomeDataLen pk.VarInt
		if _, err := biomeDataLen.ReadFrom(r); err != nil {
			return section, err
		}
		for i := 0; i < int(biomeDataLen); i++ {
			var v pk.Long
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
		}
	} else if biomeBpe <= 3 {
		var biomePaletteLen pk.VarInt
		if _, err := biomePaletteLen.ReadFrom(r); err != nil {
			return section, err
		}
		for i := 0; i < int(biomePaletteLen); i++ {
			var v pk.VarInt
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
		}
		var biomeDataLen pk.VarInt
		if _, err := biomeDataLen.ReadFrom(r); err != nil {
			return section, err
		}
		for i := 0; i < int(biomeDataLen); i++ {
			var v pk.Long
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
		}
	} else {
		var biomeDataLen pk.VarInt
		if _, err := biomeDataLen.ReadFrom(r); err != nil {
			return section, err
		}
		for i := 0; i < int(biomeDataLen); i++ {
			var v pk.Long
			if _, err := v.ReadFrom(r); err != nil {
				return section, err
			}
		}
	}

	return section, nil
}

func (b *Bot) handleUnloadChunk(p pk.Packet) error {
	var chunkZ, chunkX pk.Int
	if err := p.Scan(&chunkZ, &chunkX); err != nil {
		return nil
	}
	b.world.UnloadChunk(int32(chunkX), int32(chunkZ))
	return nil
}
