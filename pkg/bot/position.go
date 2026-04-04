package bot

import (
	"fmt"
	"math"
	pk "go-mcbots/pkg/protocol/net/packet"
)

func (b *Bot) handleSyncPosition(p pk.Packet) error {
	var teleportID pk.VarInt
	var x, y, z pk.Double
	var deltaX, deltaY, deltaZ pk.Double
	var yaw, pitch pk.Float
	var flags pk.Int

	if err := p.Scan(&teleportID, &x, &y, &z, &deltaX, &deltaY, &deltaZ, &yaw, &pitch, &flags); err != nil {
		return fmt.Errorf("failed to parse sync position: %w", err)
	}

	curX, curY, curZ := b.state.GetPosition()
	curYaw, curPitch := b.state.GetRotation()

	newX, newY, newZ := float64(x), float64(y), float64(z)
	newYaw, newPitch := float32(yaw), float32(pitch)

	if flags&0x01 != 0 {
		newX += curX
	}
	if flags&0x02 != 0 {
		newY += curY
	}
	if flags&0x04 != 0 {
		newZ += curZ
	}
	if flags&0x08 != 0 {
		newYaw += curYaw
	}
	if flags&0x10 != 0 {
		newPitch += curPitch
	}

	b.state.SetPosition(newX, newY, newZ)
	b.state.SetRotation(newYaw, newPitch)
	b.state.SetOnGround(true)
	b.state.SetVelocity(float64(deltaX), float64(deltaY), float64(deltaZ))

	if err := b.acceptTeleport(int32(teleportID)); err != nil {
		return fmt.Errorf("failed to accept teleport: %w", err)
	}

	if err := b.sendPositionAndRotation(); err != nil {
		return fmt.Errorf("failed to send position after sync: %w", err)
	}

	b.Events.emit("position_update", newX, newY, newZ)
	return nil
}

func (b *Bot) acceptTeleport(teleportID int32) error {
	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_AcceptTeleport),
		pk.VarInt(teleportID),
	))
}

func (b *Bot) handleLogin(p pk.Packet) error {
	var entityID pk.Int
	if err := p.Scan(&entityID); err != nil {
		return fmt.Errorf("failed to parse login: %w", err)
	}
	b.state.mu.Lock()
	b.state.EntityID = int32(entityID)
	b.state.mu.Unlock()
	return nil
}

func (b *Bot) handleUpdateHealth(p pk.Packet) error {
	var health pk.Float
	var food pk.VarInt
	var saturation pk.Float

	if err := p.Scan(&health, &food, &saturation); err != nil {
		return fmt.Errorf("failed to parse health: %w", err)
	}

	b.state.SetHealth(float32(health), float32(food), float32(saturation))

	if float32(health) <= 0 {
		if b.state.IsAlive() {
			b.state.SetAlive(false)
			b.physics.Stop()
			b.Events.emit("death")
		}
	} else if !b.state.IsAlive() {
		b.state.SetAlive(true)
		b.Events.emit("spawn")
	}

	b.Events.emit("health_update", float32(health), float32(food))
	return nil
}

func (b *Bot) handleRespawn(p pk.Packet) {
	b.state.SetAlive(true)
	b.state.SetVelocity(0, 0, 0)
	b.state.SetOnGround(true)
	b.physics.Start()
}

func (b *Bot) handleSpawnPosition(p pk.Packet) error {
	return nil
}

func (b *Bot) sendPositionAndRotation() error {
	x, y, z := b.state.GetPosition()
	yaw, pitch := b.state.GetRotation()
	onGround := b.state.IsOnGround()

	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PlayerPositionRotation),
		pk.Double(x),
		pk.Double(y),
		pk.Double(z),
		pk.Float(yaw),
		pk.Float(pitch),
		pk.Byte(boolToByte(onGround)),
	))
}

func (b *Bot) sendPosition() error {
	x, y, z := b.state.GetPosition()
	onGround := b.state.IsOnGround()

	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PlayerPosition),
		pk.Double(x),
		pk.Double(y),
		pk.Double(z),
		pk.Byte(boolToByte(onGround)),
	))
}

func (b *Bot) sendRotation() error {
	yaw, pitch := b.state.GetRotation()
	onGround := b.state.IsOnGround()

	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PlayerRotation),
		pk.Float(yaw),
		pk.Float(pitch),
		pk.Byte(boolToByte(onGround)),
	))
}

func (b *Bot) sendOnGround() error {
	onGround := b.state.IsOnGround()
	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PlayerOnGround),
		pk.Byte(boolToByte(onGround)),
	))
}

func (b *Bot) Look(yaw, pitch float32) error {
	b.state.SetRotation(yaw, pitch)
	return nil
}

func (b *Bot) LookAt(x, y, z float64) error {
	px, py, pz := b.state.GetPosition()
	eyeY := py + 1.62

	dx := x - px
	dy := y - eyeY
	dz := z - pz

	dist := math.Sqrt(dx*dx + dz*dz)
	yaw := float32(-math.Atan2(dx, dz) * 180.0 / math.Pi)
	pitch := float32(-math.Atan2(dy, dist) * 180.0 / math.Pi)

	b.state.SetRotation(yaw, pitch)
	return nil
}

func boolToByte(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
