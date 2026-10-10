package bot

import "math"

// Collision and footing queries on the world, used by physics (collider)
// and the pathfinder.

// appendBoxes implements collider.
func (w *World) appendBoxes(dst []aabb, x, y, z int) []aabb {
	var local []aabb
	if !w.HasChunk(x, z) {
		local = fullCube
	} else {
		local = w.shapes.boxes(w.GetBlock(x, y, z))
	}
	fx, fy, fz := float64(x), float64(y), float64(z)
	for _, b := range local {
		dst = append(dst, b.offset(fx, fy, fz))
	}
	return dst
}

// waterHeight implements collider. Water levels are not tracked, so every
// water block counts as a source (8/9 high, full if water is above it).
func (w *World) waterHeight(x, y, z int) float64 {
	if !w.IsWater(x, y, z) {
		return 0
	}
	if w.IsWater(x, y+1, z) {
		return 1
	}
	return sourceWaterHeight
}

// climbable implements collider.
func (w *World) climbable(x, y, z int) bool {
	return w.IsClimbable(x, y, z)
}

// The pathfinder assumes the bot is centered in its block column: the
// player box (0.6 wide) then covers x+0.2..x+0.8, z+0.2..z+0.8.
const columnInset = (1 - PlayerWidth) / 2

func columnBox(x, z int, y0, y1 float64) aabb {
	fx, fz := float64(x), float64(z)
	return aabb{fx + columnInset, y0, fz + columnInset, fx + 1 - columnInset, y1, fz + 1 - columnInset}
}

// topInColumn returns the highest collision box top of block (x, y, z)
// within the centered player column, relative to y.
func (w *World) topInColumn(x, y, z int) (float64, bool) {
	col := columnBox(x, z, math.Inf(-1), math.Inf(1))
	top, found := 0.0, false
	for _, b := range w.appendBoxes(nil, x, y, z) {
		if b.maxX > col.minX && b.minX < col.maxX && b.maxZ > col.minZ && b.minZ < col.maxZ {
			if !found || b.maxY > top {
				top, found = b.maxY, true
			}
		}
	}
	return top - float64(y), found
}

// StandHeight returns where the feet rest when standing centered in block
// column (x, z) with the feet inside block y, and whether the player fits
// there. Slabs, carpets, snow layers, paths and the like hold the feet
// inside their own block (y + height); full blocks, stairs and top slabs
// put them at the bottom of the block above; fences and walls (1.5 high)
// half a block higher.
func (w *World) StandHeight(x, y, z int) (float64, bool) {
	if !w.HasChunk(x, z) {
		return 0, false
	}
	var feet float64
	if t, ok := w.topInColumn(x, y, z); ok {
		if t >= 1 {
			return 0, false // the block fills the feet space
		}
		feet = float64(y) + t
	} else {
		t, ok := w.topInColumn(x, y-1, z)
		if !ok || t < 1 {
			return 0, false // nothing to stand on (or the floor is in block y-1)
		}
		feet = float64(y-1) + t
		if feet >= float64(y+1) {
			return 0, false
		}
	}
	if !w.ColumnFree(x, z, feet, feet+PlayerHeight) {
		return 0, false
	}
	return feet, true
}

// ColumnFree reports whether the centered player column of (x, z) has no
// collision between heights y0 and y1, and its chunk is loaded.
func (w *World) ColumnFree(x, z int, y0, y1 float64) bool {
	if !w.HasChunk(x, z) {
		return false
	}
	col := columnBox(x, z, y0, y1).deflate(collisionEpsilon)
	var boxes []aabb
	for y := int(math.Floor(y0)) - 1; y <= int(math.Floor(y1)); y++ {
		boxes = w.appendBoxes(boxes[:0], x, y, z)
		for _, b := range boxes {
			if b.intersects(col) {
				return false
			}
		}
	}
	return true
}
