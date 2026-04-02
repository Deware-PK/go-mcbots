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
	SB_LoginStart   int32
	SB_LoginAck     int32
	CB_LoginSuccess int32
	CB_Disconnect   int32
	CB_SetCompression int32 

	// — Configuration —
	SB_KnownPacks     int32
	SB_FinishConfig   int32
	SB_PluginResponse int32
	CB_KnownPacks     int32
	CB_RegistryData   int32
	CB_FinishConfig   int32
	CB_PluginRequest  int32

	// — Play —
	SB_KeepAlive      int32
	SB_Chat           int32
	SB_PlayerPosition int32
	SB_PlayerAction   int32
	SB_UseItem        int32
	SB_AcceptTeleport int32

	CB_KeepAlive       int32
	CB_ChatMessage     int32
	CB_SyncPosition    int32
	CB_BlockUpdate     int32
	CB_SpawnEntity     int32
	CB_GameEvent       int32
	CB_Disconnect_Play int32

	// Others
	CB_FeatureFlags int32
	
}

type VersionInfo struct {
	MCVersion      string // "1.21.11"
	ProtocolNumber int    // 774
	IDs            PacketIDs
}
