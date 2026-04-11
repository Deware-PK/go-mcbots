package bot

import (
	pk "github.com/deware-pk/go-mcbots/pkg/protocol/net/packet"
)

func (b *Bot) Respawn() error {
	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_ClientCommand),
		pk.VarInt(0),
	))
}
