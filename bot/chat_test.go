package bot

import (
	"io"
	"testing"

	"github.com/google/uuid"

	pk "github.com/deware-pk/go-mcbots/internal/protocol/net/packet"
)

func newChatTestBot(t *testing.T) *Bot {
	t.Helper()
	ver, err := ResolveVersion("1.21.11")
	if err != nil {
		t.Fatal(err)
	}
	return New("TestBot", ver)
}

func playerChatPacket(t *testing.T, msg string, withSig bool, senderName any) pk.Packet {
	t.Helper()
	fields := []pk.FieldEncoder{
		pk.VarInt(7), // global index
		pk.UUID(uuid.MustParse("8667ba71-b85a-4004-af54-457a9734eed7")),
		pk.VarInt(0), // index
		pk.Boolean(withSig),
	}
	if withSig {
		sig := make([]byte, 256)
		fields = append(fields, rawBytes(sig))
	}
	fields = append(fields,
		pk.String(msg),
		pk.Long(1700000000000),
		pk.Long(42),
		pk.VarInt(1), // one previous message, by id
		pk.VarInt(5),
		pk.Boolean(false), // no unsigned content
		pk.VarInt(0),      // pass-through filter
		pk.VarInt(1),      // chat type: registry id 0 (+1)
		pk.NBT(senderName),
		pk.Boolean(false), // no target name
	)
	return pk.Marshal(pk.VarInt(0x3F), fields...)
}

type rawBytes []byte

func (b rawBytes) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b)
	return int64(n), err
}

func TestHandlePlayerChat(t *testing.T) {
	steve := map[string]any{
		"text":      "Steve",
		"insertion": "Steve",
		"click_event": map[string]any{
			"action":  "suggest_command",
			"command": "/tell Steve ",
		},
		"hover_event": map[string]any{
			"action": "show_entity",
			"id":     "minecraft:player",
			"uuid":   []int32{1, 2, 3, 4},
			"name":   map[string]any{"text": "Steve"},
		},
	}
	for _, tc := range []struct {
		name   string
		sig    bool
		sender any
		want   string
	}{
		{"unsigned, compound name", false, steve, "Steve"},
		{"signed, string name", true, "Alex", "Alex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newChatTestBot(t)
			var gotSender, gotMsg string
			b.Events.OnChat = func(s, m string) { gotSender, gotMsg = s, m }
			p := playerChatPacket(t, "!goto 1 2 3", tc.sig, tc.sender)
			if err := b.handlePlayerChat(p); err != nil {
				t.Fatal(err)
			}
			if gotMsg != "!goto 1 2 3" {
				t.Fatalf("message = %q", gotMsg)
			}
			if gotSender != tc.want {
				t.Fatalf("sender = %q, want %q", gotSender, tc.want)
			}
		})
	}
}

func TestHandleSystemChat(t *testing.T) {
	b := newChatTestBot(t)
	var got string
	b.Events.OnSystemMessage = func(m string) { got = m }
	p := pk.Marshal(pk.VarInt(0x77),
		pk.NBT(map[string]any{
			"translate": "multiplayer.player.joined",
			"color":     "yellow",
			"with":      []map[string]any{{"text": "Steve"}},
		}),
		pk.Boolean(false),
	)
	if err := b.handleSystemChat(p); err != nil {
		t.Fatal(err)
	}
	if got != "Steve joined the game" {
		t.Fatalf("system message = %q", got)
	}

	got = ""
	p = pk.Marshal(pk.VarInt(0x77), pk.NBT("plain text"), pk.Boolean(false))
	b.handleSystemChat(p)
	if got != "plain text" {
		t.Fatalf("plain system message = %q", got)
	}
}
