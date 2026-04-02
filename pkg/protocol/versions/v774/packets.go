package v774

import "go-mcbots/pkg/protocol/types"

// source: minecraft.wiki/w/Java_Edition_protocol (protocol 774)
var Info = types.VersionInfo{
	MCVersion:      "1.21.11",
	ProtocolNumber: 774,
	IDs: types.PacketIDs{
		// Handshake
		SB_Handshake: 0x00,

		// Login
		SB_LoginStart:     0x00,
		SB_LoginAck:       0x03,
		CB_LoginSuccess:   0x02,
		CB_Disconnect:     0x02,
		CB_SetCompression: 0x03,

		// Configuration
		SB_KnownPacks:     0x07,
		SB_FinishConfig:   0x03,
		SB_PluginResponse: 0x02,
		CB_KnownPacks:     0x0E,
		CB_RegistryData:   0x07,
		CB_FinishConfig:   0x03,
		CB_PluginRequest:  0x01,

		// Play
		SB_KeepAlive:      0x1B,
		SB_Chat:           0x08,
		SB_PlayerPosition: 0x1A,
		SB_PlayerAction:   0x2A,
		SB_UseItem:        0x38,
		SB_AcceptTeleport: 0x00,

		CB_KeepAlive:       0x2B,
		CB_ChatMessage:     0x1C,
		CB_SyncPosition:    0x46,
		CB_BlockUpdate:     0x09,
		CB_SpawnEntity:     0x01,
		CB_GameEvent:       0x22,
		CB_Disconnect_Play: 0x1B,

		// Other
		CB_FeatureFlags: 0x0C,
	},
}
