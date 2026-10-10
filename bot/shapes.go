package bot

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

// blockShapes holds the collision boxes of every block state of a version
// (decoded from Version.BlockShapes, see internal/tools/genblocks).
type blockShapes struct {
	shapes   [][]aabb // boxes in block-local coordinates (0..1, fences up to 1.5)
	perState []uint16 // shape index per state ID
}

var fullCube = []aabb{{0, 0, 0, 1, 1, 1}}

// boxes returns the local collision boxes of a block state. Unknown state
// IDs are full cubes, like classify treats them as solid.
func (s *blockShapes) boxes(state uint32) []aabb {
	if s == nil || int(state) >= len(s.perState) {
		return fullCube
	}
	return s.shapes[s.perState[state]]
}

var shapeCache sync.Map // encoded string -> *blockShapes

// shapesFor decodes (once per version) the shape table. An empty or
// invalid table yields nil, which makes every block a full cube.
func shapesFor(encoded string) *blockShapes {
	if encoded == "" {
		return nil
	}
	if s, ok := shapeCache.Load(encoded); ok {
		return s.(*blockShapes)
	}
	s, err := decodeShapes(encoded)
	if err != nil {
		panic(fmt.Sprintf("go-mcbots: corrupt block shape table: %v", err))
	}
	actual, _ := shapeCache.LoadOrStore(encoded, s)
	return actual.(*blockShapes)
}

func decodeShapes(encoded string) (*blockShapes, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(raw)
	le := binary.LittleEndian

	var nShapes uint16
	if err := binary.Read(r, le, &nShapes); err != nil {
		return nil, err
	}
	s := &blockShapes{shapes: make([][]aabb, nShapes)}
	for i := range s.shapes {
		n, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		boxes := make([]aabb, n)
		for j := range boxes {
			var q [6]int8
			if err := binary.Read(r, le, &q); err != nil {
				return nil, err
			}
			boxes[j] = aabb{
				float64(q[0]) / 64, float64(q[1]) / 64, float64(q[2]) / 64,
				float64(q[3]) / 64, float64(q[4]) / 64, float64(q[5]) / 64,
			}
		}
		s.shapes[i] = boxes
	}

	var nStates, nRuns uint32
	if err := binary.Read(r, le, &nStates); err != nil {
		return nil, err
	}
	if err := binary.Read(r, le, &nRuns); err != nil {
		return nil, err
	}
	s.perState = make([]uint16, 0, nStates)
	for i := uint32(0); i < nRuns; i++ {
		var run struct{ N, Shape uint16 }
		if err := binary.Read(r, le, &run); err != nil {
			return nil, err
		}
		if int(run.Shape) >= len(s.shapes) {
			return nil, fmt.Errorf("run %d: shape %d out of range", i, run.Shape)
		}
		for j := uint16(0); j < run.N; j++ {
			s.perState = append(s.perState, run.Shape)
		}
	}
	if len(s.perState) != int(nStates) {
		return nil, fmt.Errorf("%d states decoded, header says %d", len(s.perState), nStates)
	}
	return s, nil
}
