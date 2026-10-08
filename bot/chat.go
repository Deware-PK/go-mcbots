package bot

import (
	"bytes"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/deware-pk/go-mcbots/internal/protocol/chat"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
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

// handlePlayerChat parses a clientbound Player Chat packet (1.21.5+ layout,
// protocol 774). All fields must be read from ONE reader in order:
// packet.Scan restarts from the beginning of the packet on every call.
func (b *Bot) handlePlayerChat(p pk.Packet) error {
	r := bytes.NewReader(p.Data)

	var (
		globalIndex pk.VarInt
		senderUUID  pk.UUID
		index       pk.VarInt
		hasSig      pk.Boolean
	)
	if err := readFields(r, &globalIndex, &senderUUID, &index, &hasSig); err != nil {
		return nil
	}
	if hasSig {
		if _, err := io.CopyN(io.Discard, r, 256); err != nil {
			return nil
		}
	}

	var (
		body      pk.String
		timestamp pk.Long
		salt      pk.Long
		prevCount pk.VarInt
	)
	if err := readFields(r, &body, &timestamp, &salt, &prevCount); err != nil {
		return nil
	}

	// From here on, failures only cost us the sender name, not the message.
	sender := uuid.UUID(senderUUID).String()
	if name, ok := readPlayerChatSender(r, int(prevCount)); ok && name != "" {
		sender = name
	}

	b.Events.emit("chat", sender, string(body))
	return nil
}

// readPlayerChatSender skips the rest of the signed-message data and returns
// the decorated sender name from the chat type.
func readPlayerChatSender(r *bytes.Reader, prevCount int) (string, bool) {
	for i := 0; i < prevCount; i++ {
		var id pk.VarInt
		if _, err := id.ReadFrom(r); err != nil {
			return "", false
		}
		if id == 0 { // full signature follows
			if _, err := io.CopyN(io.Discard, r, 256); err != nil {
				return "", false
			}
		}
	}

	var hasUnsigned pk.Boolean
	if _, err := hasUnsigned.ReadFrom(r); err != nil {
		return "", false
	}
	if hasUnsigned {
		var unsigned chat.Message
		if _, err := unsigned.ReadFrom(r); err != nil {
			return "", false
		}
	}

	var filterType pk.VarInt
	if _, err := filterType.ReadFrom(r); err != nil {
		return "", false
	}
	if filterType == 2 { // partially filtered
		var mask pk.BitSet
		if _, err := mask.ReadFrom(r); err != nil {
			return "", false
		}
	}

	var chatType chat.Type
	if _, err := chatType.ReadFrom(r); err != nil {
		return "", false
	}
	return chatType.SenderName.ClearString(), true
}

// handleSystemChat parses a clientbound System Chat packet. Since 1.20.3 the
// content is an NBT text component, not a JSON string.
func (b *Bot) handleSystemChat(p pk.Packet) error {
	var (
		content     chat.Message
		isActionBar pk.Boolean
	)
	if err := p.Scan(&content, &isActionBar); err != nil {
		return nil
	}
	if !bool(isActionBar) {
		b.Events.emit("system_message", content.ClearString())
	}
	return nil
}

func readFields(r io.Reader, fields ...io.ReaderFrom) error {
	for _, f := range fields {
		if _, err := f.ReadFrom(r); err != nil {
			return err
		}
	}
	return nil
}
