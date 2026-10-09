package bot

import (
	"fmt"

	"github.com/deware-pk/go-mcbots/internal/protocol/chat"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

func (b *Bot) HandleGame() error {
	defer func() {
		if b.onClose != nil {
			b.onClose()
		}
	}()

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
			// First position after join or respawn: we are in the world.
			if b.awaitingSpawn.CompareAndSwap(true, false) {
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

		case b.version.IDs.CB_BlockUpdate:
			if err := b.handleBlockUpdate(p); err != nil {
				fmt.Printf("[World] %v\n", err)
			}

		case b.version.IDs.CB_SectionBlocksUpdate:
			if err := b.handleSectionBlocksUpdate(p); err != nil {
				fmt.Printf("[World] %v\n", err)
			}

		case b.version.IDs.CB_SetEntityMotion:
			if err := b.handleSetEntityMotion(p); err != nil {
				fmt.Printf("[Physics] %v\n", err)
			}

		case b.version.IDs.CB_Explode:
			if err := b.handleExplode(p); err != nil {
				fmt.Printf("[Physics] %v\n", err)
			}

		case b.version.IDs.CB_UnloadChunk:
			if err := b.handleUnloadChunk(p); err != nil {
				// non-fatal
			}

		case b.version.IDs.CB_Respawn:
			if err := b.handleRespawn(p); err != nil {
				return fmt.Errorf("respawn error: %w", err)
			}

		case b.version.IDs.CB_CombatDeath:
			// Death screen - death already handled via health=0

		case b.version.IDs.CB_SetDefaultSpawnPosition:
			if err := b.handleSpawnPosition(p); err != nil {
				// non-fatal
			}

		case b.version.IDs.CB_Disconnect_Play:
			reason := readDisconnectReason(p)
			b.Events.emit("disconnect", reason)
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

// readDisconnectReason decodes the NBT text component of a configuration- or
// play-state Disconnect packet (1.20.3+).
func readDisconnectReason(p pk.Packet) string {
	var reason chat.Message
	if err := p.Scan(&reason); err != nil {
		return "(unreadable disconnect reason)"
	}
	return reason.ClearString()
}
