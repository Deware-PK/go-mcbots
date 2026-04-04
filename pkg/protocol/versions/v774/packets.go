package v774

import "go-mcbots/pkg/protocol/types"

// source: minecraft.wiki/w/Java_Edition_protocol/Packets (protocol 774, MC 1.21.11)
var Info = types.VersionInfo{
	MCVersion:      "1.21.11",
	ProtocolNumber: 774,
	IDs: types.PacketIDs{
		// — Handshake —
		SB_Handshake: 0x00,

		// — Login —
		SB_LoginStart:     0x00,
		SB_LoginAck:       0x03,
		CB_LoginSuccess:   0x02,
		CB_Disconnect:     0x02,
		CB_SetCompression: 0x03,

		// — Configuration —
		SB_KnownPacks:     0x07,
		SB_FinishConfig:   0x03,
		SB_PluginResponse: 0x02,
		CB_KnownPacks:     0x0E,
		CB_RegistryData:   0x07,
		CB_FinishConfig:   0x03,
		CB_PluginRequest:  0x01,
		CB_FeatureFlags:   0x0C,

		// — Play (Serverbound) —
		SB_AcceptTeleport:         0x00,
		SB_ChatCommand:            0x06,
		SB_Chat:                   0x08,
		SB_ClientCommand:          0x0B,
		SB_ClientTickEnd:          0x0C,
		SB_ClientInformation:      0x0D,
		SB_KeepAlive:              0x1B,
		SB_PlayerPosition:         0x1D,
		SB_PlayerPositionRotation: 0x1E,
		SB_PlayerRotation:         0x1F,
		SB_PlayerOnGround:         0x20,
		SB_PlayerCommand:          0x29,
		SB_PlayerAction:           0x28,
		SB_PlayerInput:            0x2A,
		SB_ChunkBatchReceived:     0x0A,
		SB_PlayerLoaded:           0x2B,
		SB_UseItem:                0x38,

		// — Play (Clientbound) —
		CB_ChunkBatchFinished:      0x0B,
		CB_ChunkBatchStart:         0x0C,
		CB_SpawnEntity:             0x01,
		CB_BlockUpdate:             0x08,
		CB_Disconnect_Play:         0x20,
		CB_UnloadChunk:             0x25,
		CB_GameEvent:               0x26,
		CB_KeepAlive:               0x2B,
		CB_ChunkData:               0x2C,
		CB_Login:                   0x30,
		CB_PlayerChat:              0x3F,
		CB_CombatDeath:             0x42,
		CB_SyncPosition:            0x46,
		CB_Respawn:                 0x50,
		CB_SetDefaultSpawnPosition: 0x5F,
		CB_UpdateHealth:            0x66,
		CB_SystemChat:              0x77,
	},
}
