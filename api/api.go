package api

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
)

// MinServerVersion is the minimum server version required by the client.
var MinServerVersion string

func init() {
	// Per https://github.com/gravitational/teleport/blob/master/rfd/0012-teleport-versioning.md,
	// only one major version backwards is supported for servers.
	ver := semver.New(Version)
	MinServerVersion = fmt.Sprintf("%d.0.0", ver.Major-1)
}
