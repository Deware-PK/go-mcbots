package bot

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sync"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
	v774 "github.com/deware-pk/go-mcbots/internal/protocol/versions/v774"
)

type chunkPos struct {
	X, Z int32
}

type ChunkSection struct {
	BlockCount   int16
	BitsPerEntry byte
	Palette      []uint32
	Data         []int64

	// Blocks holds all 4096 state IDs once the section has been changed by a
	// block update (index = y<<8 | z<<4 | x). nil means read from the
	// paletted data above.
	Blocks []uint32
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

	// classes is the version's block state classification table
	// (Version.BlockClasses).
	classes string
}

func newWorld(classes string) *World {
	if classes == "" {
		classes = v774.BlockClasses
	}
	return &World{
		chunks:  make(map[chunkPos]*ChunkColumn),
		MinY:    -64,
		Height:  384,
		classes: classes,
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
	blockIndex := sectionIndexOf(x, y, z)
	if section.Blocks != nil {
		return section.Blocks[blockIndex]
	}
	return getFromPalettedContainer(section, blockIndex)
}

// sectionIndexOf returns the index of block (x, y, z) inside its section.
func sectionIndexOf(x, y, z int) int {
	return (y&0xF)<<8 | (z&0xF)<<4 | x&0xF
}

// SetBlock changes one block state, e.g. from a Block Update packet.
// Changes in chunks that are not loaded are ignored.
func (w *World) SetBlock(x, y, z int, state uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.setBlockLocked(x, y, z, state)
}

func (w *World) setBlockLocked(x, y, z int, state uint32) {
	col, ok := w.chunks[chunkPos{int32(x >> 4), int32(z >> 4)}]
	if !ok {
		return
	}
	sectionIndex := (y - col.MinY) >> 4
	if sectionIndex < 0 || sectionIndex >= len(col.Sections) {
		return
	}
	section := &col.Sections[sectionIndex]
	if section.Blocks == nil {
		// First change to this section: expand the paletted data so
		// writes never need a palette resize.
		section.Blocks = make([]uint32, blockEntries)
		for i := range section.Blocks {
			section.Blocks[i] = getFromPalettedContainer(section, i)
		}
		section.Palette, section.Data = nil, nil
	}
	section.Blocks[sectionIndexOf(x, y, z)] = state
}

func (w *World) HasChunk(x, z int) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.chunks[chunkPos{int32(x >> 4), int32(z >> 4)}]
	return ok
}

// IsBlockSolid reports whether the block at (x,y,z) has collision.
// Unloaded chunks read as air. Uses ClassifyBlock, like the pathfinder.
func (w *World) IsBlockSolid(x, y, z int) bool {
	return w.classify(w.GetBlock(x, y, z)) == BlockSolid
}

// IsBlockSolidOrUnloaded is IsBlockSolid, but treats unloaded chunks as solid
// so physics does not fall through the world before chunks arrive.
func (w *World) IsBlockSolidOrUnloaded(x, y, z int) bool {
	if !w.HasChunk(x, z) {
		return true
	}
	return w.IsBlockSolid(x, y, z)
}

// BlockType classifies a block for physics and pathfinding.
type BlockType int

const (
	BlockAir       BlockType = iota // passable, non-solid (air, plants, torches, ...)
	BlockSolid                      // has collision, can stand on
	BlockWater                      // passable, swimmable
	BlockClimbable                  // ladder, vine — climbable
	BlockDangerous                  // lava, fire, cobweb, ... — passable but avoid
)

// ClassifyBlock maps a 1.21.11 block state ID to a BlockType. Block state IDs
// differ between versions: use ClassifyBlockFor, or the World methods, which
// use the connected version's table.
func ClassifyBlock(stateID uint32) BlockType {
	return classify(v774.BlockClasses, stateID)
}

// ClassifyBlockFor maps a block state ID of version v to a BlockType.
func ClassifyBlockFor(v Version, stateID uint32) BlockType {
	return classify(v.BlockClasses, stateID)
}

// classify looks stateID up in a generated table (see internal/tools/genblocks).
// Unknown IDs count as solid.
func classify(table string, stateID uint32) BlockType {
	if int(stateID) >= len(table) {
		return BlockSolid
	}
	return BlockType(table[stateID] - '0')
}

// classify uses the world's version table.
func (w *World) classify(stateID uint32) BlockType {
	return classify(w.classes, stateID)
}

func (w *World) IsPassable(x, y, z int) bool {
	bt := w.classify(w.GetBlock(x, y, z))
	return bt == BlockAir || bt == BlockWater
}

func (w *World) IsWater(x, y, z int) bool {
	return w.classify(w.GetBlock(x, y, z)) == BlockWater
}

func (w *World) IsClimbable(x, y, z int) bool {
	return w.classify(w.GetBlock(x, y, z)) == BlockClimbable
}

func (w *World) IsDangerous(x, y, z int) bool {
	return w.classify(w.GetBlock(x, y, z)) == BlockDangerous
}

// CanStandAt returns true if a player can stand at block position (x,y,z):
// solid or climbable block below, and 2 passable blocks at feet (y) and head (y+1).
func (w *World) CanStandAt(x, y, z int) bool {
	below := w.classify(w.GetBlock(x, y-1, z))
	feet := w.classify(w.GetBlock(x, y, z))
	head := w.classify(w.GetBlock(x, y+1, z))

	solidBelow := below == BlockSolid || below == BlockClimbable
	feetClear := feet == BlockAir || feet == BlockWater || feet == BlockClimbable
	headClear := head == BlockAir || head == BlockWater

	return solidBelow && feetClear && headClear
}

// CanStandInWater returns true if the position is water with passable head space.
func (w *World) CanStandInWater(x, y, z int) bool {
	feet := w.classify(w.GetBlock(x, y, z))
	head := w.classify(w.GetBlock(x, y+1, z))
	return feet == BlockWater && (head == BlockAir || head == BlockWater)
}

// IsSafeToFall checks if falling from (x,startY,z) will land safely within maxDrop blocks.
// Returns the landing Y or -1 if unsafe.
func (w *World) IsSafeToFall(x, startY, z, maxDrop int) int {
	for dy := 1; dy <= maxDrop; dy++ {
		checkY := startY - dy
		bt := w.classify(w.GetBlock(x, checkY, z))
		if bt == BlockSolid {
			landY := checkY + 1
			// Check feet and head are clear at landing
			feetClear := w.classify(w.GetBlock(x, landY, z)) == BlockAir || w.classify(w.GetBlock(x, landY, z)) == BlockWater
			headClear := w.classify(w.GetBlock(x, landY+1, z)) == BlockAir || w.classify(w.GetBlock(x, landY+1, z)) == BlockWater
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

// reset drops all chunks and sets the dimension height range.
func (w *World) reset(minY, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.chunks = make(map[chunkPos]*ChunkColumn)
	w.MinY, w.Height = minY, height
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
		section, err := parseChunkSection(sectionReader, b.version.SectionFluidCount)
		if err != nil {
			log.Printf("[World] chunk (%d,%d) section %d/%d: %v", col.X, col.Z, i, numSections, err)
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

// parsePalettedContainer reads a paletted container. maxIndirect is the
// largest bits-per-entry that still uses an indirect palette: 8 for block
// states, 3 for biomes. Above it the container uses the global palette.
func parsePalettedContainer(r *bytes.Reader, numEntries, maxIndirect int) (bpe byte, palette []uint32, data []int64, err error) {
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

	if int(bpe) <= maxIndirect {
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

	blockMaxIndirect = 8
	biomeMaxIndirect = 3
)

// parseChunkSection reads one chunk section. hasFluidCount is
// Version.SectionFluidCount (26.1+ sends a fluid count after the block count).
func parseChunkSection(r *bytes.Reader, hasFluidCount bool) (ChunkSection, error) {
	var section ChunkSection

	var blockCount pk.Short
	if _, err := blockCount.ReadFrom(r); err != nil {
		return section, err
	}
	section.BlockCount = int16(blockCount)

	if hasFluidCount {
		var fluidCount pk.Short
		if _, err := fluidCount.ReadFrom(r); err != nil {
			return section, err
		}
	}

	bpe, palette, data, err := parsePalettedContainer(r, blockEntries, blockMaxIndirect)
	if err != nil {
		return section, err
	}
	section.BitsPerEntry = bpe
	section.Palette = palette
	section.Data = data

	// Biome paletted container — skip it (we don't use biomes)
	_, _, _, err = parsePalettedContainer(r, biomeEntries, biomeMaxIndirect)
	if err != nil {
		return section, err
	}

	return section, nil
}

// handleBlockUpdate applies a clientbound Block Update: Position, VarInt state.
func (b *Bot) handleBlockUpdate(p pk.Packet) error {
	var (
		pos   pk.Position
		state pk.VarInt
	)
	if err := p.Scan(&pos, &state); err != nil {
		return fmt.Errorf("block update: %w", err)
	}
	b.world.SetBlock(pos.X, pos.Y, pos.Z, uint32(state))
	return nil
}

// handleSectionBlocksUpdate applies a clientbound Update Section Blocks
// (multi block change): Long section position (x 22 bits, z 22 bits, y 20
// bits), then a VarInt-prefixed array of VarLong state<<12 | x<<8 | z<<4 | y.
func (b *Bot) handleSectionBlocksUpdate(p pk.Packet) error {
	r := bytes.NewReader(p.Data)
	var (
		sectionPos pk.Long
		count      pk.VarInt
	)
	if _, err := sectionPos.ReadFrom(r); err != nil {
		return fmt.Errorf("section blocks update: %w", err)
	}
	if _, err := count.ReadFrom(r); err != nil {
		return fmt.Errorf("section blocks update count: %w", err)
	}
	sp := int64(sectionPos)
	sx, sy, sz := int(sp>>42), int(sp<<44>>44), int(sp<<22>>42)

	b.world.mu.Lock()
	defer b.world.mu.Unlock()
	for i := 0; i < int(count); i++ {
		var entry pk.VarLong
		if _, err := entry.ReadFrom(r); err != nil {
			return fmt.Errorf("section blocks update entry %d: %w", i, err)
		}
		e := int64(entry)
		local := int(e & 0xFFF)
		x := sx<<4 | local>>8&0xF
		z := sz<<4 | local>>4&0xF
		y := sy<<4 | local&0xF
		b.world.setBlockLocked(x, y, z, uint32(e>>12))
	}
	return nil
}

func (b *Bot) handleUnloadChunk(p pk.Packet) error {
	var chunkZ, chunkX pk.Int
	if err := p.Scan(&chunkZ, &chunkX); err != nil {
		return nil
	}
	b.world.UnloadChunk(int32(chunkX), int32(chunkZ))
	return nil
}
