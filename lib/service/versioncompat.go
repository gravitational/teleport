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
	return fmt.Sprintf("This Teleport instance (v%s) is older than the minimum version supported by the cluster (v%s), which may be the cause of this connection failure.", e.ClientVersion, e.MinVersion)
}

// clientTooNewError indicates this client is a newer major version than the cluster.
type clientTooNewError struct {
	LocalMajorVersion   int64
	ClusterMajorVersion int64
}

func (e *clientTooNewError) Error() string {
	return fmt.Sprintf("Teleport instance is too new. This instance is running v%d. The server is running v%d and only supports instances on v%d or v%d. To connect anyway pass the --skip-version-check flag.", e.LocalMajorVersion, e.ClusterMajorVersion, e.ClusterMajorVersion, e.ClusterMajorVersion-1)
}

// isVersionIncompatible reports whether err is a fatal client-side version
// incompatibility (a [clientTooNewError]).
func isVersionIncompatible(err error) bool {
	var tooNew *clientTooNewError
	return errors.As(err, &tooNew)
}

// enforceVersionPolicy refuses an instance that is a newer major version than
// the server. If --skip-version-check is provided, version incompatibility is
// detected and logged but not enforced. Parse errors fail open to prevent
// blocking a connection from an otherwise valid instance.
//
// Too-old is deliberately not checked here. The server is the authority and
// will close connections unless it is running with TELEPORT_UNSTABLE_ALLOW_OLD_CLIENTS=yes.
func (process *TeleportProcess) enforceVersionPolicy(ctx context.Context, info joinclient.VersionInfo) error {
	err := checkClientMeetsMaxVersion(teleport.Version, info.ServerVersion)
	if err == nil {
		return nil
	}
	if !isVersionIncompatible(err) {
		process.logger.WarnContext(ctx,
			"Could not determine version compatibility with the cluster, skipping check.",
			"error", err,
			"server_version", info.ServerVersion,
			"instance_version", teleport.Version,
		)
		return nil
	}
	if process.Config.SkipVersionCheck {
		process.logger.WarnContext(ctx,
			"Instance is a newer major version than the cluster, continuing anyway because --skip-version-check flag was provided.",
			"error", err,
			"server_version", info.ServerVersion,
			"instance_version", teleport.Version,
		)
		return nil
	}
	return trace.Wrap(err)
}

// clientTooOld returns a [clientTooOldError] if this instance is older than the
// server's advertised minimum client version, or nil if it is not (including an
// empty or unparseable version). A parse error is treated as nil so uncertainty
// never drives behavior.
func clientTooOld(minClientVersion string) error {
	var tooOld *clientTooOldError
	if errors.As(checkClientMeetsMinVersion(teleport.Version, minClientVersion), &tooOld) {
		return tooOld
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
