// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/utils"
)

// clientTooOldError indicates this client is older than the cluster's minimum
// client version.
type clientTooOldError struct {
	ClientVersion string
	MinVersion    string
}

func (e *clientTooOldError) Error() string {
	return fmt.Sprintf("Teleport instance is too old. This instance is running v%s. The server requires a minimum version of v%s. To connect anyway pass the --skip-version-check flag.", e.ClientVersion, e.MinVersion)
}

// clientTooNewError indicates this client is a newer major version than the cluster.
type clientTooNewError struct {
	LocalMajorVersion   int64
	ClusterMajorVersion int64
}

func (e *clientTooNewError) Error() string {
	return fmt.Sprintf("Teleport instance is too new. This instance is running v%d. The server is running v%d and only supports instances on v%d or v%d. To connect anyway pass the --skip-version-check flag.", e.LocalMajorVersion, e.ClusterMajorVersion, e.ClusterMajorVersion, e.ClusterMajorVersion-1)
}

// isVersionIncompatible reports whether err indicates this client's version is
// incompatible with the cluster, either a [clientTooOldError] or [clientTooNewError].
func isVersionIncompatible(err error) bool {
	var tooOld *clientTooOldError
	var tooNew *clientTooNewError
	return errors.As(err, &tooOld) || errors.As(err, &tooNew)
}

// enforceVersionPolicy implements this agent's policy for an incompatible cluster
// version. Any version incompatibility is fatal unless the --skip-version-check
// flag is provided. Parse errors fail open so malformed version information
// advertised by the cluster can't block joining.
func (process *TeleportProcess) enforceVersionPolicy(ctx context.Context, info joinclient.VersionInfo) error {
	for _, err := range []error{
		checkClientMeetsMinVersion(teleport.Version, info.MinClientVersion),
		checkClientMeetsMaxVersion(teleport.Version, info.ServerVersion),
	} {
		if err == nil {
			continue
		}
		if !isVersionIncompatible(err) {
			process.logger.WarnContext(ctx,
				"Could not determine version compatibility with the cluster, skipping check.",
				"error", err,
				"min_client_version", info.MinClientVersion,
				"server_version", info.ServerVersion,
				"instance_version", teleport.Version,
			)
			continue
		}
		if process.Config.SkipVersionCheck {
			process.logger.WarnContext(ctx,
				"Instance version is incompatible with the cluster, continuing anyway because --skip-version-check flag was provided.",
				"error", err,
				"min_client_version", info.MinClientVersion,
				"server_version", info.ServerVersion,
				"instance_version", teleport.Version,
			)
			continue
		}
		return trace.Wrap(err)
	}
	return nil
}

func checkClientMeetsMinVersion(clientVersion, minVersion string) error {
	if minVersion == "" {
		return nil
	}
	// Stripping the pre-release both validates the advertised minimum and yields a
	// clean version for the user-facing error message.
	minWithoutPreRelease, err := utils.VersionWithoutPreRelease(minVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	if !utils.MeetsMinVersion(clientVersion, minVersion) {
		return &clientTooOldError{ClientVersion: clientVersion, MinVersion: minWithoutPreRelease}
	}
	return nil
}

func checkClientMeetsMaxVersion(clientVersion, serverVersion string) error {
	if serverVersion == "" {
		return nil
	}
	client, err := semver.NewVersion(clientVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	server, err := semver.NewVersion(serverVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	if server.Major < client.Major {
		return &clientTooNewError{LocalMajorVersion: client.Major, ClusterMajorVersion: server.Major}
	}
	return nil
}
