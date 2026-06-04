/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package types

import "slices"

const (
	// GitProtocolSSH is the SSH protocol for git proxying.
	GitProtocolSSH = "ssh"
	// GitProtocolHTTP is the HTTPS/API protocol for git proxying.
	GitProtocolHTTP = "http"
)

// GitServerSSHEnabled returns whether SSH is allowed for a git server.
// If AllowProtocols is empty, defaults to true for backwards compatibility.
func GitServerSSHEnabled(github *GitHubServerMetadata) bool {
	if github == nil {
		return false
	}
	if len(github.AllowProtocols) == 0 {
		return true
	}
	return slices.Contains(github.AllowProtocols, GitProtocolSSH)
}

// GitServerHTTPEnabled returns whether HTTP is allowed for a git server.
func GitServerHTTPEnabled(github *GitHubServerMetadata) bool {
	if github == nil {
		return false
	}
	return slices.Contains(github.AllowProtocols, GitProtocolHTTP)
}
