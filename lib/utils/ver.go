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
	"fmt"

	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
)

// CheckVersions compares client and server versions and makes sure that the
// client version is greater than or equal to the minimum version supported
// by the server.
func CheckVersions(clientVersion string, minClientVersion string) error {
	clientSemver, err := semver.NewVersion(clientVersion)
	if err != nil {
		return trace.Wrap(err,
			"unsupported version format, need semver format: %q, e.g 1.0.0", clientVersion)
	}

	minClientSemver, err := semver.NewVersion(minClientVersion)
	if err != nil {
		return trace.Wrap(err,
			"unsupported version format, need semver format: %q, e.g 1.0.0", minClientVersion)
	}

	if clientSemver.Compare(*minClientSemver) < 0 {
		errorMessage := fmt.Sprintf("minimum client version supported by the server "+
			"is %v. Please upgrade the client, downgrade the server, or use the "+
			"--skip-version-check flag to by-pass this check.", minClientVersion)
		return trace.BadParameter(errorMessage)
	}

	return nil
}
