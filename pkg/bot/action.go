package bot

import (
	pk "go-mcbots/pkg/protocol/net/packet"
	"time"
)

func (b *Bot) Chat(msg string) error {
    timestamp := time.Now().UnixMilli()
    return b.conn.WritePacket(pk.Marshal(
        pk.VarInt(b.version.IDs.SB_Chat),
        pk.String(msg),
        pk.Long(timestamp),
        pk.Long(0),                      // salt
        pk.Boolean(false),               // no signature
        pk.VarInt(0),                    // message count
        pk.FixedBitSet([]byte{0, 0, 0}), // acknowledged (20 bits = 3 bytes)
        pk.Byte(0),                      // checksum ← เพิ่มตัวนี้
    ))
}
