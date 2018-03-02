/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"github.com/coreos/go-semver/semver"

	"github.com/gravitational/trace"
)

// CheckVersions compares client and server versions
// and makes sure they are compatible using Teleport convention
func CheckVersions(clientVersion, serverVersion string) error {
	clientSemver, err := semver.NewVersion(clientVersion)
	if err != nil {
		return trace.Wrap(err,
			"unsupported version format, need semver format: %q, e.g 1.0.0", clientVersion)
	}

	serverSemver, err := semver.NewVersion(serverVersion)
	if err != nil {
		return trace.Wrap(err,
			"unsupported version format, need semver format: %q, e.g 1.0.0", clientVersion)
	}

	// for now only do simple check that client is not newer version
	// than server, ignore patch versions

	switch {
	case serverSemver.Major != clientSemver.Major:
		return trace.BadParameter("client version %q is not compatible server version %q, please make client and server versions use the same major versions", clientVersion, serverVersion)
	case serverSemver.Minor < clientSemver.Minor:
		return trace.BadParameter("client version %q is newer than server version %q, please downgrade client or upgrade server", clientVersion, serverVersion)
		// for now let's not enforce minor versions diff as it's too harsh
	}

	return nil
}
