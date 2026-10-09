package bot

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

// writeLpVec3 is a port of net.minecraft.network.LpVec3.write (26.2), used to
// produce test input for the decoder.
func writeLpVec3(v LpVec3) []byte {
	m := math.Max(math.Abs(v.X), math.Max(math.Abs(v.Y), math.Abs(v.Z)))
	if m < 3.051944088384301e-5 {
		return []byte{0}
	}
	scale := uint64(math.Ceil(m))
	cont := scale&3 != scale
	flags := scale
	if cont {
		flags = scale&3 | 4
	}
	pack := func(d float64) uint64 { return uint64(math.Round((d*0.5 + 0.5) * lpMaxQuantized)) }
	s := float64(scale)
	packed := flags | pack(v.X/s)<<3 | pack(v.Y/s)<<18 | pack(v.Z/s)<<33

	out := []byte{byte(packed), byte(packed >> 8)}
	out = binary.BigEndian.AppendUint32(out, uint32(packed>>16))
	if cont {
		var buf bytes.Buffer
		pk.VarInt(int32(scale >> 2)).WriteTo(&buf)
		out = append(out, buf.Bytes()...)
	}
	return out
}

func readLp(t *testing.T, data []byte) LpVec3 {
	t.Helper()
	var v LpVec3
	n, err := v.ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if int(n) != len(data) {
		t.Fatalf("read %d bytes, want %d", n, len(data))
	}
	return v
}

func TestLpVec3Golden(t *testing.T) {
	// (0, 0.4, 0): scale 1, x=z=16383, y=22936.
	v := readLp(t, []byte{0xf9, 0xff, 0x7f, 0xff, 0x66, 0x61})
	if v.X != 0 || v.Z != 0 || math.Abs(v.Y-0.4) > 1e-4 {
		t.Fatalf("got %+v, want (0, 0.4, 0)", v)
	}
	if v := readLp(t, []byte{0}); v != (LpVec3{}) {
		t.Fatalf("zero: got %+v", v)
	}
}

func TestLpVec3RoundTrip(t *testing.T) {
	for _, want := range []LpVec3{
		{0.1, 0.36, -0.2},   // typical melee knockback
		{-0.95, 0.4, 0.3},   // scale 1
		{2.5, -1.2, 0},      // scale 3, no continuation
		{10, 0.5, -7.25},    // scale 10: continuation VarInt
		{-300, 120.5, 0.01}, // large plugin velocity
	} {
		got := readLp(t, writeLpVec3(want))
		scale := math.Ceil(math.Max(math.Abs(want.X), math.Max(math.Abs(want.Y), math.Abs(want.Z))))
		tol := scale / lpMaxQuantized * 1.01 // one quantization step
		if math.Abs(got.X-want.X) > tol || math.Abs(got.Y-want.Y) > tol || math.Abs(got.Z-want.Z) > tol {
			t.Errorf("round trip %+v -> %+v (tol %g)", want, got, tol)
		}
	}
}

func motionPacket(b *Bot, id int32, v LpVec3) pk.Packet {
	data := append(pk.Marshal(pk.VarInt(b.version.IDs.CB_SetEntityMotion), pk.VarInt(id)).Data, writeLpVec3(v)...)
	return pk.Packet{ID: b.version.IDs.CB_SetEntityMotion, Data: data}
}

func explodePacket(b *Bot, kb *[3]float64) pk.Packet {
	fields := []pk.FieldEncoder{
		pk.Double(1), pk.Double(64), pk.Double(-3), // center
		pk.Float(4),           // radius
		pk.Int(12),            // block count
		pk.Boolean(kb != nil), // knockback present
	}
	if kb != nil {
		fields = append(fields, pk.Double(kb[0]), pk.Double(kb[1]), pk.Double(kb[2]))
	}
	// Particle + sound + block particles follow; the handler must not need them.
	fields = append(fields, pk.VarInt(0), pk.VarInt(0))
	return pk.Marshal(pk.VarInt(b.version.IDs.CB_Explode), fields...)
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func TestSetEntityMotionReplacesVelocity(t *testing.T) {
	b := newTestBot(t)
	b.state.EntityID = 42
	b.state.SetVelocity(0.3, -0.08, 0.3)

	// Another entity's velocity is ignored.
	if err := b.handleSetEntityMotion(motionPacket(b, 7, LpVec3{1, 1, 1})); err != nil {
		t.Fatal(err)
	}
	if vx, vy, vz := b.physics.applyMotion(b.state.GetVelocity()); vx != 0.3 || vy != -0.08 || vz != 0.3 {
		t.Fatalf("other entity changed our velocity: %v %v %v", vx, vy, vz)
	}

	if err := b.handleSetEntityMotion(motionPacket(b, 42, LpVec3{-0.4, 0.36, 0.1})); err != nil {
		t.Fatal(err)
	}
	vx, vy, vz := b.physics.applyMotion(b.state.GetVelocity())
	if !near(vx, -0.4) || !near(vy, 0.36) || !near(vz, 0.1) {
		t.Fatalf("velocity = %v %v %v, want -0.4 0.36 0.1", vx, vy, vz)
	}
	// Consumed: the next tick sees no pending motion.
	if vx, _, _ := b.physics.applyMotion(0, 0, 0); vx != 0 {
		t.Fatalf("motion applied twice")
	}
}

func TestExplodeAddsKnockback(t *testing.T) {
	b := newTestBot(t)
	b.state.EntityID = 42

	// No knockback (e.g. creative, or out of range): velocity untouched.
	if err := b.handleExplode(explodePacket(b, nil)); err != nil {
		t.Fatal(err)
	}
	if vx, vy, vz := b.physics.applyMotion(0.2, 0, 0); vx != 0.2 || vy != 0 || vz != 0 {
		t.Fatalf("no-knockback explosion changed velocity")
	}

	// Set then add, in arrival order: (0.1,0,0) then +(1,0.5,-1).
	b.handleSetEntityMotion(motionPacket(b, 42, LpVec3{0.1, 0, 0}))
	if err := b.handleExplode(explodePacket(b, &[3]float64{1, 0.5, -1})); err != nil {
		t.Fatal(err)
	}
	vx, vy, vz := b.physics.applyMotion(5, 5, 5)
	if !near(vx, 1.1) || !near(vy, 0.5) || !near(vz, -1) {
		t.Fatalf("velocity = %v %v %v, want 1.1 0.5 -1", vx, vy, vz)
	}

	// Add then set: the set wins.
	b.handleExplode(explodePacket(b, &[3]float64{1, 1, 1}))
	b.handleSetEntityMotion(motionPacket(b, 42, LpVec3{0, 0.2, 0}))
	vx, vy, vz = b.physics.applyMotion(5, 5, 5)
	if !near(vx, 0) || !near(vy, 0.2) || !near(vz, 0) {
		t.Fatalf("velocity = %v %v %v, want 0 0.2 0", vx, vy, vz)
	}
}

func TestTeleportDropsPendingKnockback(t *testing.T) {
	b := newTestBot(t)
	b.state.EntityID = 42
	b.handleSetEntityMotion(motionPacket(b, 42, LpVec3{1, 1, 1}))
	b.physics.clearMotion() // what handleSyncPosition does
	if vx, vy, vz := b.physics.applyMotion(0, 0, 0); vx != 0 || vy != 0 || vz != 0 {
		t.Fatalf("pending motion survived teleport")
	}
}

// A melee hit while standing still pushes the bot back and up, and it lands
// again a few ticks later.
func TestKnockbackMovesBody(t *testing.T) {
	w := fakeWorld{}.floor(63, -20, 20, -20, 20)
	bd := body{X: 0.5, Y: 64, Z: 0.5, OnGround: true}
	bd.VX, bd.VY, bd.VZ = 0.4, 0.36, 0

	maxY := bd.Y
	landed := -1
	for tick := 0; tick < 40; tick++ {
		bd = stepPhysics(w, bd, ControlState{}, 0)
		maxY = math.Max(maxY, bd.Y)
		if bd.OnGround && tick > 0 {
			landed = tick
			break
		}
	}
	if maxY < 64.3 {
		t.Errorf("peak height %.3f: knockback did not lift the bot", maxY)
	}
	if bd.X < 1.0 {
		t.Errorf("x = %.3f: knockback did not push the bot", bd.X)
	}
	if landed < 0 {
		t.Fatalf("never landed")
	}
	if bd.Y != 64 {
		t.Errorf("landed at y=%v, want 64", bd.Y)
	}
}
