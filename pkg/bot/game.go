package bot

import (
	"fmt"
	pk "go-mcbots/pkg/protocol/net/packet"
)

func (b *Bot) HandleGame() error {

	if b.OnJoin != nil {
		if err := b.OnJoin(); err != nil {
			return err
		}
	}

	for {
		var p pk.Packet
		if err := b.conn.ReadPacket(&p); err != nil {
			return err
		}

		switch p.ID {

		case b.version.IDs.CB_KeepAlive:
			if err := b.handleKeepAlive(p); err != nil {
				return fmt.Errorf("keepalive error: %w", err)
			}
			
		case b.version.IDs.CB_Disconnect_Play:
			var reason pk.String
			p.Scan(&reason)
			return fmt.Errorf("disconnected: %s", reason)

		default:
			// ignore packet
		}
	}
}
