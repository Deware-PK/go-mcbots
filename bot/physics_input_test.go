package bot

import "testing"

// Flag values from PrismarineJS/minecraft-data pc/1.21.11 protocol.json
// (packet_player_input: forward, backward, left, right, jump, shift, sprint).
func TestInputFlags(t *testing.T) {
	tests := []struct {
		c    ControlState
		want byte
	}{
		{ControlState{}, 0},
		{ControlState{Forward: true}, 0x01},
		{ControlState{Back: true}, 0x02},
		{ControlState{Left: true}, 0x04},
		{ControlState{Right: true}, 0x08},
		{ControlState{Jump: true}, 0x10},
		{ControlState{Sneak: true}, 0x20},
		{ControlState{Sprint: true}, 0x40},
		{ControlState{Forward: true, Jump: true, Sprint: true}, 0x51},
	}
	for _, tt := range tests {
		if got := tt.c.inputFlags(); got != tt.want {
			t.Errorf("%+v.inputFlags() = %#x, want %#x", tt.c, got, tt.want)
		}
	}
}
