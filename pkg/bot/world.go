package bot

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sync"

	pk "github.com/deware-pk/go-mcbots/pkg/protocol/net/packet"
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

// BlockType classifies a block for pathfinding purposes.
type BlockType int

const (
	BlockAir       BlockType = iota // passable, non-solid
	BlockSolid                      // non-passable, can stand on
	BlockWater                      // passable, swimmable
	BlockClimbable                  // ladder, vine — climbable
	BlockDangerous                  // lava, fire, cactus — avoid
)

// Known block state ID ranges for Minecraft 1.21.x (protocol 774).
// These are approximate and may need adjustment for different sub-versions.
// Air variants
var airStates = map[uint32]bool{
	0:     true, // air
	12516: true, // cave_air
	12517: true, // void_air
}

// ClassifyBlock maps a block state ID to a BlockType.
func ClassifyBlock(stateID uint32) BlockType {
	if airStates[stateID] {
		return BlockAir
	}
	// Water: states 113-128 (water[level=0..15])
	if stateID >= 113 && stateID <= 128 {
		return BlockWater
	}
	// Lava: states 129-144 (lava[level=0..15])
	if stateID >= 129 && stateID <= 144 {
		return BlockDangerous
	}
	// Ladder: states 5765-5772 (4 facing * 2 waterlogged)
	if stateID >= 5765 && stateID <= 5772 {
		return BlockClimbable
	}
	// Vine: states 6849-6880 (32 states from boolean properties)
	if stateID >= 6849 && stateID <= 6880 {
		return BlockClimbable
	}
	// Fire: states 2600-2631
	if stateID >= 2600 && stateID <= 2631 {
		return BlockDangerous
	}
	// Cactus: states 5765 is already taken by ladder, cactus ~5654-5669
	if stateID >= 5654 && stateID <= 5669 {
		return BlockDangerous
	}
	return BlockSolid
}

func (w *World) IsPassable(x, y, z int) bool {
	bt := ClassifyBlock(w.GetBlock(x, y, z))
	return bt == BlockAir || bt == BlockWater
}

func (w *World) IsWater(x, y, z int) bool {
	return ClassifyBlock(w.GetBlock(x, y, z)) == BlockWater
}

func (w *World) IsClimbable(x, y, z int) bool {
	return ClassifyBlock(w.GetBlock(x, y, z)) == BlockClimbable
}

func (w *World) IsDangerous(x, y, z int) bool {
	return ClassifyBlock(w.GetBlock(x, y, z)) == BlockDangerous
}

// CanStandAt returns true if a player can stand at block position (x,y,z):
// solid or climbable block below, and 2 passable blocks at feet (y) and head (y+1).
func (w *World) CanStandAt(x, y, z int) bool {
	below := ClassifyBlock(w.GetBlock(x, y-1, z))
	feet := ClassifyBlock(w.GetBlock(x, y, z))
	head := ClassifyBlock(w.GetBlock(x, y+1, z))

	solidBelow := below == BlockSolid || below == BlockClimbable
	feetClear := feet == BlockAir || feet == BlockWater || feet == BlockClimbable
	headClear := head == BlockAir || head == BlockWater

	return solidBelow && feetClear && headClear
}

// CanStandInWater returns true if the position is water with passable head space.
func (w *World) CanStandInWater(x, y, z int) bool {
	feet := ClassifyBlock(w.GetBlock(x, y, z))
	head := ClassifyBlock(w.GetBlock(x, y+1, z))
	return feet == BlockWater && (head == BlockAir || head == BlockWater)
}

// IsSafeToFall checks if falling from (x,startY,z) will land safely within maxDrop blocks.
// Returns the landing Y or -1 if unsafe.
func (w *World) IsSafeToFall(x, startY, z, maxDrop int) int {
	for dy := 1; dy <= maxDrop; dy++ {
		checkY := startY - dy
		bt := ClassifyBlock(w.GetBlock(x, checkY, z))
		if bt == BlockSolid {
			landY := checkY + 1
			// Check feet and head are clear at landing
			feetClear := ClassifyBlock(w.GetBlock(x, landY, z)) == BlockAir || ClassifyBlock(w.GetBlock(x, landY, z)) == BlockWater
			headClear := ClassifyBlock(w.GetBlock(x, landY+1, z)) == BlockAir || ClassifyBlock(w.GetBlock(x, landY+1, z)) == BlockWater
			if feetClear && headClear {
				return landY
			}
			return -1
		}
		if bt == BlockDangerous {
			return -1
		}
		if bt == BlockWater {
			// Water breaks the fall
			return checkY
		}
	}
	return -1 // too far to fall
}

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

func (w *World) ChunkCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.chunks)
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

	// Heightmaps — Prefixed Array of {VarInt type, Prefixed Array of Long}
	// As of 1.21.5+ (protocol 774), heightmaps are no longer NBT.
	var hmCount pk.VarInt
	if _, err := hmCount.ReadFrom(r); err != nil {
		return fmt.Errorf("chunk (%d,%d) heightmap count: %w", chunkX, chunkZ, err)
	}
	for i := 0; i < int(hmCount); i++ {
		var hmType pk.VarInt
		if _, err := hmType.ReadFrom(r); err != nil {
			return fmt.Errorf("chunk (%d,%d) heightmap[%d] type: %w", chunkX, chunkZ, i, err)
		}
		var longCount pk.VarInt
		if _, err := longCount.ReadFrom(r); err != nil {
			return fmt.Errorf("chunk (%d,%d) heightmap[%d] long count: %w", chunkX, chunkZ, i, err)
		}
		// Skip longCount × 8 bytes of packed long data
		skip := int64(longCount) * 8
		if _, err := r.Seek(skip, 1); err != nil {
			return fmt.Errorf("chunk (%d,%d) heightmap[%d] skip: %w", chunkX, chunkZ, i, err)
		}
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
	count := b.world.ChunkCount()
	if count <= 5 || count%100 == 0 {
		log.Printf("[World] Stored chunk (%d,%d) — total: %d", col.X, col.Z, count)
	}
	return nil
}

// dataArrayLongCount calculates the number of longs in a data array.
// As of 1.21.5 (protocol 774), this is no longer sent; it must be computed.
func dataArrayLongCount(bpe int, numEntries int) int {
	if bpe == 0 {
		return 0
	}
	entriesPerLong := 64 / bpe
	return (numEntries + entriesPerLong - 1) / entriesPerLong
}

func readLongs(r *bytes.Reader, count int) ([]int64, error) {
	longs := make([]int64, count)
	for i := range longs {
		var v pk.Long
		if _, err := v.ReadFrom(r); err != nil {
			return nil, err
		}
		longs[i] = int64(v)
	}
	return longs, nil
}

func parsePalettedContainer(r *bytes.Reader, numEntries int) (bpe byte, palette []uint32, data []int64, err error) {
	bpe, err = r.ReadByte()
	if err != nil {
		return
	}

	if bpe == 0 {
		// Single valued — one VarInt palette entry, no data array
		var singleValue pk.VarInt
		if _, err = singleValue.ReadFrom(r); err != nil {
			return
		}
		palette = []uint32{uint32(singleValue)}
		return
	}

	if int(bpe) <= 8 {
		// Indirect palette — VarInt count + VarInt[] entries
		var paletteLen pk.VarInt
		if _, err = paletteLen.ReadFrom(r); err != nil {
			return
		}
		palette = make([]uint32, int(paletteLen))
		for i := range palette {
			var v pk.VarInt
			if _, err = v.ReadFrom(r); err != nil {
				return
			}
			palette[i] = uint32(v)
		}
	}
	// Direct palette (bpe > 8): no palette to read

	// Data array — length calculated, not sent (1.21.5+)
	longCount := dataArrayLongCount(int(bpe), numEntries)
	data, err = readLongs(r, longCount)
	return
}

const (
	blockEntries = 16 * 16 * 16 // 4096
	biomeEntries = 4 * 4 * 4    // 64
)

func parseChunkSection(r *bytes.Reader) (ChunkSection, error) {
	var section ChunkSection

	var blockCount pk.Short
	if _, err := blockCount.ReadFrom(r); err != nil {
		return section, err
	}
	section.BlockCount = int16(blockCount)

	bpe, palette, data, err := parsePalettedContainer(r, blockEntries)
	if err != nil {
		return section, err
	}
	section.BitsPerEntry = bpe
	section.Palette = palette
	section.Data = data

	// Biome paletted container — skip it (we don't use biomes)
	_, _, _, err = parsePalettedContainer(r, biomeEntries)
	if err != nil {
		return section, err
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
