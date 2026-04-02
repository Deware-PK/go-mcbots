package chat_test

import (
	"testing"

	"go-mcbots/pkg/protocol/chat"
	en_us "go-mcbots/pkg/protocol/data/lang/en-us"
	"go-mcbots/pkg/protocol/nbt"
)

func TestMessage_UnmarshalJSON_string(t *testing.T) {
	snbts := []string{
		"{translate: sleep.players_sleeping, with: [I; 1, 37]}",
	}

	texts := []string{
		"1/37 players sleeping",
	}

	chat.SetLanguage(en_us.Map)
	for i, v := range snbts {
		bytes, err := nbt.Marshal(nbt.StringifiedMessage(v))
		if err != nil {
			t.Errorf("Invalid SNBT: %v", err)
			continue
		}

		var cm chat.Message
		if err := nbt.Unmarshal(bytes, &cm); err != nil {
			t.Error(err)
		}
		if str := cm.String(); str != texts[i] {
			t.Errorf("gets %q, wants %q", str, texts[i])
		}
	}
}
