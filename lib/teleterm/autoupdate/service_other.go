//go:build !windows

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

package autoupdate

import (
	"os"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/auto_update/v1"
	"github.com/gravitational/teleport/lib/autoupdate"
)

// When tsh runs as a daemon, auto-updates must be disabled. Connect enforces this by
// launching tsh with TELEPORT_TOOLS_VERSION=off, and forwards the real value via
// FORWARDED_TELEPORT_TOOLS_VERSION.
const forwardedTeleportToolsEnvVar = "FORWARDED_TELEPORT_TOOLS_VERSION"

// platformGetConfig retrieves the local auto updates configuration.
func platformGetConfig() (*api.GetConfigResponse, error) {
	envBaseURL := os.Getenv(autoupdate.BaseURLEnvVar)
	envTeleportToolsVersion := os.Getenv(forwardedTeleportToolsEnvVar)

	return &api.GetConfigResponse{
		CdnBaseUrl:   envBaseURL,
		ToolsVersion: envTeleportToolsVersion,
	}, nil
}
