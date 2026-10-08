package bot

import "github.com/deware-pk/go-mcbots/internal/protocol"

// Version describes a supported Minecraft version and its protocol data.
type Version = protocol.VersionInfo

// ResolveVersion looks up a supported Minecraft version, e.g. "1.21.11".
func ResolveVersion(version string) (Version, error) {
	return protocol.Resolve(version)
}

// SupportedVersions lists the Minecraft versions this library can connect with.
func SupportedVersions() []string {
	return protocol.SupportedVersions()
}
