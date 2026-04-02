package bot

import (
	pk "go-mcbots/pkg/protocol/net/packet"
)

func (b *Bot) handleKeepAlive(p pk.Packet) error {
	var keepAliveID pk.Long
	if err := p.Scan(&keepAliveID); err != nil {
		return err
	}

	err := b.conn.WritePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_KeepAlive),
		keepAliveID,
	))

	return err
}
