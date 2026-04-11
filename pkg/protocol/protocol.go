package protocol

import "github.com/deware-pk/go-mcbots/pkg/protocol/types"

// Re-export types for backward compatibility
type State = types.State
type PacketIDs = types.PacketIDs
type VersionInfo = types.VersionInfo

const (
	Handshaking = types.Handshaking
	Play        = types.Play
	Login       = types.Login
	Status      = types.Status
)
