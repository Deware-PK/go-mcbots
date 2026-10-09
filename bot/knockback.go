package bot

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

// LpVec3 is Mojang's low-precision Vec3 (net.minecraft.network.LpVec3),
// used for entity velocity since 1.21.9.
//
// Wire format: one byte; 0 means the zero vector. Otherwise two bytes and a
// big-endian uint32 form a 48-bit little-endian value:
//
//	bits 0-1   scale (low two bits)
//	bit  2     continuation: a VarInt follows with the rest of the scale (<< 2)
//	bits 3-17  x, bits 18-32 y, bits 33-47 z (15 bits each)
//
// Each component decodes to (min(q, 32766) * 2 / 32766 - 1) * scale.
type LpVec3 struct{ X, Y, Z float64 }

const (
	lpDataMask     = 32767
	lpMaxQuantized = 32766.0
)

func lpUnpack(v uint64) float64 {
	return math.Min(float64(v&lpDataMask), lpMaxQuantized)*2.0/lpMaxQuantized - 1.0
}

func (v *LpVec3) ReadFrom(r io.Reader) (int64, error) {
	var head [2]byte
	if _, err := io.ReadFull(r, head[:1]); err != nil {
		return 0, err
	}
	if head[0] == 0 {
		*v = LpVec3{}
		return 1, nil
	}
	var rest [5]byte
	if _, err := io.ReadFull(r, rest[:]); err != nil {
		return 1, err
	}
	head[1] = rest[0]
	hi := uint64(binary.BigEndian.Uint32(rest[1:]))
	packed := hi<<16 | uint64(head[1])<<8 | uint64(head[0])
	n := int64(6)

	scale := uint64(head[0] & 3)
	if head[0]&4 != 0 {
		var more pk.VarInt
		m, err := more.ReadFrom(r)
		n += m
		if err != nil {
			return n, err
		}
		scale |= uint64(uint32(more)) << 2
	}
	s := float64(scale)
	*v = LpVec3{
		X: lpUnpack(packed>>3) * s,
		Y: lpUnpack(packed>>18) * s,
		Z: lpUnpack(packed>>33) * s,
	}
	return n, nil
}

// handleSetEntityMotion handles Set Entity Velocity. For our own player it
// replaces the velocity (vanilla LocalPlayer.lerpMotion), which is how the
// server delivers knockback from hits, explosions it simulates, wind charges
// and plugins calling setVelocity.
func (b *Bot) handleSetEntityMotion(p pk.Packet) error {
	var id pk.VarInt
	var vel LpVec3
	if err := p.Scan(&id, &vel); err != nil {
		return fmt.Errorf("set_entity_motion: %w", err)
	}
	if int32(id) != b.state.EntityID {
		return nil
	}
	b.physics.queueMotion(vel.X, vel.Y, vel.Z, false)
	b.Events.emit("knockback", vel.X, vel.Y, vel.Z)
	return nil
}

// handleExplode handles Explosion. Only the fields up to the player
// knockback are read; the rest (particles, sound) are cosmetic. Vanilla adds
// the knockback to the current velocity.
func (b *Bot) handleExplode(p pk.Packet) error {
	var cx, cy, cz pk.Double
	var radius pk.Float
	var blockCount pk.Int
	var hasKnockback pk.Boolean
	var kx, ky, kz pk.Double
	if err := p.Scan(&cx, &cy, &cz, &radius, &blockCount, &hasKnockback,
		pk.Opt{Has: &hasKnockback, Field: pk.Tuple{&kx, &ky, &kz}}); err != nil {
		return fmt.Errorf("explode: %w", err)
	}
	b.Events.emit("explosion", float64(cx), float64(cy), float64(cz), float32(radius))
	if hasKnockback {
		b.physics.queueMotion(float64(kx), float64(ky), float64(kz), true)
		b.Events.emit("knockback", float64(kx), float64(ky), float64(kz))
	}
	return nil
}
