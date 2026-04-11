package bot

import (
	"fmt"

	pk "github.com/deware-pk/go-mcbots/pkg/protocol/net/packet"
)

func (b *Bot) HandleGame() error {
	defer func() {
		if b.onClose != nil {
			b.onClose()
		}
	}()

	spawned := false

	for {
		var p pk.Packet
		if err := b.conn.ReadPacket(&p); err != nil {
			b.Events.emit("disconnect", err.Error())
			return err
		}

		switch p.ID {

		case b.version.IDs.CB_Login:
			if err := b.handleLogin(p); err != nil {
				return fmt.Errorf("login error: %w", err)
			}

		case b.version.IDs.CB_KeepAlive:
			if err := b.handleKeepAlive(p); err != nil {
				return fmt.Errorf("keepalive error: %w", err)
			}

		case b.version.IDs.CB_SyncPosition:
			if err := b.handleSyncPosition(p); err != nil {
				return fmt.Errorf("sync position error: %w", err)
			}
			if !spawned {
				spawned = true
				b.sendPlayerLoaded()
				b.physics.Start()
				b.Events.emit("spawn")
			}

		case b.version.IDs.CB_UpdateHealth:
			if err := b.handleUpdateHealth(p); err != nil {
				return fmt.Errorf("health error: %w", err)
			}

		case b.version.IDs.CB_PlayerChat:
			if err := b.handlePlayerChat(p); err != nil {
				return fmt.Errorf("player chat error: %w", err)
			}

		case b.version.IDs.CB_SystemChat:
			if err := b.handleSystemChat(p); err != nil {
				return fmt.Errorf("system chat error: %w", err)
			}

		case b.version.IDs.CB_ChunkBatchStart:
			// batch started, nothing to do

		case b.version.IDs.CB_ChunkBatchFinished:
			b.handleChunkBatchFinished(p)

		case b.version.IDs.CB_ChunkData:
			if err := b.handleChunkData(p); err != nil {
				fmt.Printf("[World] Chunk parse error: %v\n", err)
			}

		case b.version.IDs.CB_UnloadChunk:
			if err := b.handleUnloadChunk(p); err != nil {
				// non-fatal
			}

		case b.version.IDs.CB_Respawn:
			b.handleRespawn(p)

		case b.version.IDs.CB_CombatDeath:
			// Death screen - death already handled via health=0

		case b.version.IDs.CB_SetDefaultSpawnPosition:
			if err := b.handleSpawnPosition(p); err != nil {
				// non-fatal
			}

		case b.version.IDs.CB_Disconnect_Play:
			var reason pk.String
			p.Scan(&reason)
			b.Events.emit("disconnect", string(reason))
			return fmt.Errorf("disconnected: %s", reason)

		default:
			// ignore unhandled packets
		}
	}
}

func (b *Bot) sendPlayerLoaded() {
	b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_PlayerLoaded),
	))
}

func (b *Bot) handleChunkBatchFinished(p pk.Packet) {
	b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_ChunkBatchReceived),
		pk.Float(20.0),
	))
}
