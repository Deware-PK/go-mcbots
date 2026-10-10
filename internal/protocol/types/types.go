package types

type State int

const (
	Handshaking State = iota
	Play
	Login
	Status
)

type PacketIDs struct {
	// — Handshake —
	SB_Handshake int32

	// — Login —
	SB_LoginStart     int32
	SB_LoginAck       int32
	CB_LoginSuccess   int32
	CB_Disconnect     int32
	CB_SetCompression int32

	CB_LoginPluginRequest   int32
	SB_LoginPluginResponse  int32
	CB_CookieRequest_Login  int32
	SB_CookieResponse_Login int32

	// — Configuration —
	SB_KnownPacks     int32
	SB_FinishConfig   int32
	SB_PluginResponse int32
	CB_KnownPacks     int32
	CB_RegistryData   int32
	CB_FinishConfig   int32
	CB_PluginRequest  int32
	CB_FeatureFlags   int32

	SB_ClientInformation_Config int32
	CB_Ping_Config              int32
	SB_Pong_Config              int32
	CB_ResourcePackPush_Config  int32
	SB_ResourcePack_Config      int32
	CB_CookieRequest_Config     int32
	SB_CookieResponse_Config    int32
	CB_CodeOfConduct            int32
	SB_AcceptCodeOfConduct      int32

	// — Play (Serverbound) —
	SB_AcceptTeleport         int32
	SB_ChatCommand            int32
	SB_Chat                   int32
	SB_ClientCommand          int32
	SB_ClientTickEnd          int32
	SB_ClientInformation      int32
	SB_KeepAlive              int32
	SB_PlayerPosition         int32
	SB_PlayerPositionRotation int32
	SB_PlayerRotation         int32
	SB_PlayerOnGround         int32
	SB_PlayerCommand          int32
	SB_PlayerAction           int32
	SB_PlayerInput            int32
	SB_ChunkBatchReceived     int32
	SB_PlayerLoaded           int32
	SB_UseItem                int32
	SB_ConfigurationAck       int32
	SB_Pong                   int32
	SB_ResourcePack           int32
	SB_CookieResponse         int32

	// — Play (Clientbound) —
	CB_ChunkBatchFinished      int32
	CB_ChunkBatchStart         int32
	CB_SpawnEntity             int32
	CB_BlockUpdate             int32
	CB_Disconnect_Play         int32
	CB_SectionBlocksUpdate     int32
	CB_Disconnect_Config       int32
	CB_KeepAlive_Config        int32
	SB_KeepAlive_Config        int32
	CB_UnloadChunk             int32
	CB_GameEvent               int32
	CB_KeepAlive               int32
	CB_ChunkData               int32
	CB_Login                   int32
	CB_PlayerChat              int32
	CB_SyncPosition            int32
	CB_SetDefaultSpawnPosition int32
	CB_Respawn                 int32
	CB_UpdateHealth            int32
	CB_SystemChat              int32
	CB_CombatDeath             int32
	CB_SetEntityMotion         int32
	CB_Explode                 int32
	CB_StartConfiguration      int32
	CB_Ping                    int32
	CB_ResourcePackPush        int32
	CB_CookieRequest           int32
}

type VersionInfo struct {
	MCVersion      string // "1.21.11"
	ProtocolNumber int    // 774
	IDs            PacketIDs

	// BlockClasses holds one physics/pathfinding class per block state ID
	// (generated, see internal/tools/genblocks).
	BlockClasses string

	// BlockShapes holds the collision boxes of every block state, encoded
	// by internal/tools/genblocks.
	BlockShapes string

	// SectionFluidCount: chunk sections carry a fluid count (Short) after
	// the non-empty block count (26.1+).
	SectionFluidCount bool
}
