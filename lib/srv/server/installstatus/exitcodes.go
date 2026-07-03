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

	"github.com/gravitational/teleport/api/types/usertasks"
)

// ExitCode represents a classified exit code from the auto-enrollment installer pipeline.
type ExitCode int

const (
	// Preflight exit codes (shell-side, 100–149).
	BashNotFound                    ExitCode = 100
	SudoNotFound                    ExitCode = 101
	CurlNotFound                    ExitCode = 102
	InsufficientDiskSpace           ExitCode = 103
	ProxyPingError                  ExitCode = 104
	InvokeWebRequestNotFound        ExitCode = 105
	AdministratorPrivilegesRequired ExitCode = 106
	WindowsInsufficientDiskSpace    ExitCode = 107
	UnsupportedWindowsVersion       ExitCode = 108

	// Post-install exit codes (Go binary, 150–199).
	JoinFailure ExitCode = 150

	// Install exit codes (200-249).
	WindowsInstallerDownloadFailure  ExitCode = 200
	WindowsInstallerExecutionFailure ExitCode = 201
)

// InstallerMinFreeDiskMB is the minimum free disk space in megabytes required for Teleport installation.
// This value might change over time as Teleport's binary size changes, but currently sits at around 990MB: 210MB for the tarball, and around 780MB for the extracted files.
// Setting this value to 1250MB provides a buffer for future updates, while still being reasonable for most systems.
//
// If this value is updated, ensure you also update the docs at "Installation script exit codes" in the Teleport EC2 Server Discovery documentation.
const InstallerMinFreeDiskMB = 1250

// WindowsDesktopInstallerMinFreeDiskMB is the minimum free disk space in megabytes required for Teleport Authentication Package installation on Windows.
// The .exe is very small at 5.9MB. The installer drops a .dll and a few certificates which are a few megabytes each, at most.
// 50 MB is more than enough and provides some buffer.
//
// As above, if this value is updated, ensure you also update the docs at "Installation script exit codes" in the Teleport Server Discovery documentation.
const WindowsDesktopInstallerMinFreeDiskMB = 50

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
	case InvokeWebRequestNotFound:
		return "Invoke-WebRequest is not available in the instance. " +
			"Please ensure Invoke-WebRequest is installed and try again."
	case AdministratorPrivilegesRequired:
		return "Administrator privileges are required to run the installer. " +
			"Please run the installer as an administrator and try again."
	case WindowsInsufficientDiskSpace:
		return fmt.Sprintf(
			"Insufficient disk space for installation. "+
				"Teleport requires at least %dMB in the system drive.",
			WindowsDesktopInstallerMinFreeDiskMB)
	case UnsupportedWindowsVersion:
		return "Unsupported Windows version. " +
			"Please ensure you are running a supported version of Windows " +
			"(Windows Server 2016 or later, Windows 10 or later) and try again."
	case JoinFailure:
		return "Teleport was installed successfully but the agent " +
			"did not become ready within the configured timeout. " +
			"Check standard error output for join diagnostics."
	case WindowsInstallerDownloadFailure:
		return "Failed to download the Teleport authentication package installer. " +
			"Ensure this host can reach https://cdn.teleport.dev and try again."
	case WindowsInstallerExecutionFailure:
		return "The Teleport authentication package installer returned an error. " +
			"Check the standard output and standard error for details."
	default:
		return fmt.Sprintf(
			"Installation failed with exit code %d. "+
				"Please check stdout and stderr and try again.",
			int(c))
	}
}

// IssueType returns the user task issue type for the exit code. Unrecognized codes default to ec2-ssm-script-failure.
func (c ExitCode) IssueType() string {
	switch c {
	case JoinFailure:
		return usertasks.AutoDiscoverEC2IssueJoinFailure
	default:
		return usertasks.AutoDiscoverEC2IssueSSMScriptFailure
	}
}
