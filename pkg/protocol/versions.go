package protocol

import (
	"fmt"

	v774 "go-mcbots/pkg/protocol/versions/v774"
)

var mcVersionMap = map[string]int{
	"1.21.11": 774,
}

var registry = map[int]VersionInfo{
	774: v774.Info,
}

func Resolve(version string) (VersionInfo, error) {
	var protoNum int
	if _, err := fmt.Sscanf(version, "%d", &protoNum); err != nil {
		if info, ok := registry[protoNum]; ok {
			return info, nil
		}
		return VersionInfo{}, fmt.Errorf("unsupported protocol: %d", protoNum)
	}

	protoNum, ok := mcVersionMap[version]
	if !ok {
		return VersionInfo{}, fmt.Errorf("unsupported MC version: %q", version)
	}
	return registry[protoNum], nil
}

func SupportedVersions() []string {
	versions := make([]string, 0, len(mcVersionMap))
	for v := range mcVersionMap {
		versions = append(versions, v)
	}
	return versions
}
