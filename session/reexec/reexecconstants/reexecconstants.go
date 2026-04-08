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

package reexecconstants

const (
	// ExecSubCommand is the sub-command Teleport uses to re-exec itself for
	// command execution (exec and shells).
	ExecSubCommand = "exec"

	// NetworkingSubCommand is the sub-command Teleport uses to re-exec itself
	// for networking operations. e.g. local/remote port forwarding, agent forwarding,
	// or x11 forwarding.
	NetworkingSubCommand = "networking"

	// CheckHomeDirSubCommand is the sub-command Teleport uses to re-exec itself
	// to check if the user's home directory exists.
	CheckHomeDirSubCommand = "checkhomedir"

	// ParkSubCommand is the sub-command Teleport uses to re-exec itself as a
	// specific UID to prevent the matching user from being deleted before
	// spawning the intended child process.
	ParkSubCommand = "park"

	// SFTPSubCommand is the sub-command Teleport uses to re-exec itself to
	// handle SFTP connections.
	SFTPSubCommand = "sftp"
)

const (
	// RemoteCommandSuccess is returned when a command has successfully executed.
	RemoteCommandSuccess = 0
	// RemoteCommandFailure is returned when a command has failed to execute and
	// we don't have another status code for it.
	RemoteCommandFailure = 255
	// HomeDirNotFound is returned when the "teleport checkhomedir" command cannot
	// find the user's home directory.
	HomeDirNotFound = 254
	// HomeDirNotAccessible is returned when the "teleport checkhomedir" command has
	// found the user's home directory, but the user does NOT have permissions to
	// access it.
	HomeDirNotAccessible = 253
	// UnexpectedCredentials is returned when a command is no longer running with the expected
	// credentials.
	UnexpectedCredentials = 252
)
