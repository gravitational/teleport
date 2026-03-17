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

package installstatus

import (
	"fmt"
)

// ExitCode represents a classified exit code from the auto-enrollment installer pipeline.
type ExitCode int

const (
	// Preflight exit codes (shell-side, 100–149).
	BashNotFound          ExitCode = 100
	SudoNotFound          ExitCode = 101
	CurlNotFound          ExitCode = 102
	InsufficientDiskSpace ExitCode = 103
	ProxyPingError        ExitCode = 104
)

// InstallerMinFreeDiskMB is the minimum free disk space in megabytes required for Teleport installation.
// This value might change over time as Teleport's binary size changes, but currently sits at around 990MB: 210MB for the tarball, and around 780MB for the extracted files.
// Setting this value to 1250MB provides a buffer for future updates, while still being reasonable for most systems.
//
// If this value is updated, ensure you also update the docs at "Installation script exit codes" in the Teleport EC2 Server Discovery documentation.
const InstallerMinFreeDiskMB = 1250

// String returns a human-readable description for the exit code.
// Unrecognized codes get a generic fallback.
func (c ExitCode) String() string {
	switch c {
	case BashNotFound:
		return "bash is not installed in the instance. " +
			"Please install all required tools (bash, sudo, curl) and try again."
	case SudoNotFound:
		return "sudo is not installed in the instance. " +
			"Please install all required tools (bash, sudo, curl) and try again."
	case CurlNotFound:
		return "curl is not installed in the instance. " +
			"Please install all required tools (bash, sudo, curl) and try again."
	case InsufficientDiskSpace:
		return fmt.Sprintf(
			"Insufficient disk space for installation. "+
				"Teleport requires at least %dMB in /opt directory.",
			InstallerMinFreeDiskMB)
	case ProxyPingError:
		return "Failed to connect to Teleport cluster HTTPS endpoint. " +
			"Ensure this host can access your cluster and its certificate is trusted."
	default:
		return fmt.Sprintf(
			"Installation failed with exit code %d. "+
				"Please check stdout and stderr and try again.",
			int(c))
	}
}
