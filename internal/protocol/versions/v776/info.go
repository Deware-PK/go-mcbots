package v776

import "github.com/deware-pk/go-mcbots/internal/protocol/types"

// Info describes Minecraft 26.2 (protocol 776).
//
// Packet IDs and block state IDs are generated from the vanilla data
// generator's reports. Packet layouts were checked against the 26.2 server
// classes; the only change that affects this library since 1.21.11 is the
// fluid count in chunk sections.
var Info = types.VersionInfo{
	MCVersion:         "26.2",
	ProtocolNumber:    776,
	IDs:               IDs,
	BlockClasses:      BlockClasses,
	SectionFluidCount: true,
}
