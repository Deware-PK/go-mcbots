package bot

import (
	"fmt"
	"strings"
	"time"

	pk "github.com/deware-pk/go-mcbots/pkg/protocol/net/packet"
)

func (b *Bot) Chat(msg string) error {
	if strings.HasPrefix(msg, "/") {
		return b.Command(msg[1:])
	}
	timestamp := time.Now().UnixMilli()
	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_Chat),
		pk.String(msg),
		pk.Long(timestamp),
		pk.Long(0),                      // salt
		pk.Boolean(false),               // no signature
		pk.VarInt(0),                    // message count
		pk.FixedBitSet([]byte{0, 0, 0}), // acknowledged (20 bits = 3 bytes)
		pk.Byte(0),                      // checksum
	))
}

func (b *Bot) Command(cmd string) error {
	cmd = strings.TrimPrefix(cmd, "/")
	return b.writePacket(pk.Marshal(
		pk.VarInt(b.version.IDs.SB_ChatCommand),
		pk.String(cmd),
	))
}

func (b *Bot) handlePlayerChat(p pk.Packet) error {
	var senderUUID pk.UUID
	var index pk.VarInt
	var sigPresent pk.Boolean

	if err := p.Scan(&senderUUID, &index, &sigPresent); err != nil {
		return nil
	}

	if bool(sigPresent) {
		var sig pk.ByteArray
		if err := p.Scan(&sig); err != nil {
			return nil
		}
	}

	var body pk.String
	if err := p.Scan(&body); err != nil {
		return nil
	}

	senderName := fmt.Sprintf("%x", senderUUID[:4])
	b.Events.emit("chat", senderName, string(body))
	return nil
}

func (b *Bot) handleSystemChat(p pk.Packet) error {
	var content pk.String
	var isActionBar pk.Boolean

	if err := p.Scan(&content, &isActionBar); err != nil {
		return nil
	}

	if !bool(isActionBar) {
		b.Events.emit("system_message", string(content))
	}
	return nil
}
