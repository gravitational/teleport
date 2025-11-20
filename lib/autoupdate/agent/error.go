/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package agent

import "errors"

var (
	// ErrLinked is returned when a linked version cannot be operated on.
	ErrLinked = errors.New("version is linked")
	// ErrNotSupported is returned when the operation is not supported on the platform.
	ErrNotSupported = errors.New("not supported on this platform")
	// ErrNotAvailable is returned when the operation is not available at the current version of the platform.
	ErrNotAvailable = errors.New("not available at this version")
	// ErrNoBinaries is returned when no binaries are available to be linked.
	ErrNoBinaries = errors.New("no binaries available to link")
	// ErrFilePresent is returned when a file is present.
	ErrFilePresent = errors.New("file present")
	// ErrNotInstalled is returned when Teleport is not installed.
	ErrNotInstalled = errors.New("not installed")

	// ErrNoSpaceLeft is returned when there are no disk space on the agent host.
	ErrNoSpaceLeft = errors.New("no space left on device")
	// ErrSystemdReload is returned when systemd was failed to response.
	ErrSystemdReload = errors.New("failed to reload systemd service")
)
