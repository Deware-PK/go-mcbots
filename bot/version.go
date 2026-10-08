package bot

import "github.com/deware-pk/go-mcbots/internal/protocol"

// Version describes a supported Minecraft version and its protocol data.
type Version = protocol.VersionInfo

// LatestVersion is the newest supported Minecraft version.
const LatestVersion = protocol.Latest

// ResolveVersion looks up a supported Minecraft version ("26.2", "1.21.11")
// or protocol number ("776").
func ResolveVersion(version string) (Version, error) {
	return protocol.Resolve(version)
}

// SupportedVersions lists the Minecraft versions this library can connect with.
func SupportedVersions() []string {
	return protocol.SupportedVersions()
}
