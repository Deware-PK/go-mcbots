package bot

import (
	"fmt"
	pk "go-mcbots/pkg/protocol/net/packet"
)

func (b *Bot) login(addr string) error {

	if err := b.sendHandshake(addr); err != nil {
		return err
	}

	if err := b.sendLoginStart(); err != nil {
		return err
	}

	return b.waitLoginSuccess()
}

func (b *Bot) sendHandshake(addr string) error {
	return b.conn.WritePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_Handshake),
		pk.VarInt(b.version.ProtocolNumber),
		pk.String(addr),
		pk.UnsignedShort(25565),
		pk.VarInt(2),
	))
}

func (b *Bot) sendLoginStart() error {
	uuid := offlineUUID(b.Name)
	return b.conn.WritePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_LoginStart),
		pk.String(b.Name),
		pk.UUID(uuid),
	))
}

func (b *Bot) waitLoginSuccess() error {
	for {
		var p pk.Packet
		if err := b.conn.ReadPacket(&p); err != nil {
			fmt.Printf("readpacket error: %v\n", err)
			return err
		}

		switch p.ID {

		case b.version.IDs.CB_SetCompression:
			var threshold pk.VarInt
			if err := p.Scan(&threshold); err != nil {
				return err
			}
			b.conn.SetThreshold(int(threshold))

		case b.version.IDs.CB_LoginSuccess:
			if err := b.conn.WritePacket(pk.Marshal(
				pk.VarInt(b.version.IDs.SB_LoginAck),
			)); err != nil {
				return err
			}

			return b.handleConfiguration()

		case b.version.IDs.CB_Disconnect:
			var reason pk.String
			p.Scan(&reason)

		default:
			fmt.Printf("→ unhandled packet 0x%02X\n", p.ID)
		}
	}
}

func (b *Bot) handleConfiguration() error {
	for {
		var p pk.Packet
		if err := b.conn.ReadPacket(&p); err != nil {
			fmt.Printf("config ReadPacket error: %v\n", err)
			return err
		}

		switch p.ID {

		case b.version.IDs.CB_KnownPacks:
			if err := b.conn.WritePacket(pk.Marshal(
				pk.VarInt(b.version.IDs.SB_KnownPacks),
				pk.VarInt(1),
				pk.String("minecraft"),
				pk.String("core"),
				pk.String(b.version.MCVersion),
			)); err != nil {
				fmt.Printf("  KnownPacks write err: %v\n", err)
				return err
			}

		case b.version.IDs.CB_RegistryData:
			// intentionally ignored

		case b.version.IDs.CB_PluginRequest:
			// intentionally ignored
		case b.version.IDs.CB_FinishConfig:
			if err := b.conn.WritePacket(pk.Marshal(
				pk.VarInt(b.version.IDs.SB_FinishConfig),
			)); err != nil {
				return err
			}
			return b.HandleGame()

		case b.version.IDs.CB_FeatureFlags:
			// Ignored

		case b.version.IDs.CB_Disconnect:
			var reason pk.String
			p.Scan(&reason)
			return fmt.Errorf("disconnected during config: %s", reason)

		default:
			fmt.Printf("→ unhandled 0x%02X data=%X\n", p.ID, p.Data[:min(len(p.Data), 16)])
		}
	}
}
