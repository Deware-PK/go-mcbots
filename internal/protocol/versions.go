package protocol

import (
	"fmt"
	"sort"
	"strconv"

	v774 "github.com/deware-pk/go-mcbots/internal/protocol/versions/v774"
	v776 "github.com/deware-pk/go-mcbots/internal/protocol/versions/v776"
)

var mcVersionMap = map[string]int{
	"1.21.11": 774,
	"26.2":    776,
}

var registry = map[int]VersionInfo{
	774: v774.Info,
	776: v776.Info,
}

// Latest is the newest supported Minecraft version.
const Latest = "26.2"

// Resolve looks up a version by Minecraft version ("26.2", "1.21.11") or by
// protocol number ("776").
func Resolve(version string) (VersionInfo, error) {
	if protoNum, ok := mcVersionMap[version]; ok {
		return registry[protoNum], nil
	}
	if protoNum, err := strconv.Atoi(version); err == nil {
		if info, ok := registry[protoNum]; ok {
			return info, nil
		}
		return VersionInfo{}, fmt.Errorf("unsupported protocol: %d", protoNum)
	}
	return VersionInfo{}, fmt.Errorf("unsupported MC version: %q (supported: %v)", version, SupportedVersions())
}

// SupportedVersions lists the supported Minecraft versions, oldest first.
func SupportedVersions() []string {
	versions := make([]string, 0, len(mcVersionMap))
	for v := range mcVersionMap {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool { return mcVersionMap[versions[i]] < mcVersionMap[versions[j]] })
	return versions
}
