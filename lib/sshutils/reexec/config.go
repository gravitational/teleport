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

package reexec

import (
	"io"
	"log/slog"

	"github.com/gravitational/teleport/lib/utils"
)

// Config contains the payload to "teleport exec" which will be used to
// construct and execute a shell.
type Config struct {
	// LogConfig is the log configuration for the child process.
	LogConfig LogConfig `json:"log_config"`

	// LogWriter is a logger that logs from the child process should be streamed to.
	LogWriter io.Writer `json:"-"`

	// ReexecCommand is the reexec subcommand to execute.
	ReexecCommand string `json:"reexec_command"`

	// Command is the command to execute in the child process. If an interactive
	// session is being requested, will be empty. If a subsystem is requested, it
	// will contain the subsystem name
	Command string `json:"command"`

	// DestinationAddress is the target address to dial to.
	DestinationAddress string `json:"dst_addr"`

	// Username is the username associated with the Teleport identity.
	Username string `json:"username"`

	// Login is the local *nix account.
	Login string `json:"login"`

	// Roles is the list of Teleport roles assigned to the Teleport identity.
	Roles []string `json:"roles"`

	// ClusterName is the name of the Teleport cluster.
	ClusterName string `json:"cluster_name"`

	// Terminal indicates if a TTY has been allocated for the session. This is
	// typically set if either a shell was requested or a TTY was explicitly
	// allocated for an exec request.
	Terminal bool `json:"term"`

	// TerminalName is the name of TTY terminal, ex: /dev/tty1.
	// Currently, this field is used by auditd.
	TerminalName string `json:"terminal_name"`

	// ClientAddress contains IP address of the connected client.
	// Currently, this field is used by auditd.
	ClientAddress string `json:"client_address"`

	// RequestType is the type of request: either "exec" or "shell". This will
	// be used to control where to connect std{out,err} based on the request
	// type: "exec", "shell" or "subsystem".
	RequestType string `json:"request_type"`

	// PAMConfig is the configuration data that needs to be passed to the child and then to PAM modules.
	PAMConfig *PAMConfig `json:"pam_config,omitempty"`

	// Environment is a list of environment variables to add to the defaults.
	Environment []string `json:"environment"`

	// PermitUserEnvironment is set to allow reading in ~/.tsh/environment
	// upon login.
	PermitUserEnvironment bool `json:"permit_user_environment"`

	// IsTestStub is used by tests to mock the shell.
	IsTestStub bool `json:"is_test_stub"`

	// UserCreatedByTeleport is true when the system user was created by Teleport user auto-provision.
	UserCreatedByTeleport bool

	// UaccMetadata contains metadata needed for user accounting.
	UaccMetadata UaccMetadata `json:"uacc_meta"`

	// SetSELinuxContext is true when the SELinux context should be set
	// for the child.
	SetSELinuxContext bool `json:"set_selinux_context"`

	// IsSFTPRequest indicates whether this is an sftp request. Used to differentiate
	// between `tsh ssh sftp` and `tsh scp`.
	IsSFTPRequest bool `json"is_sftp_request"`
}

// LogConfig represents all the logging configuration data that
// needs to be passed to the child.
type LogConfig struct {
	// Level is the log level to use.
	Level *slog.LevelVar
	// Format defines the output format. Possible values are 'text' and 'json'.
	Format string
	// ExtraFields lists the output fields from KnownFormatFields. Example format: [timestamp, component, caller].
	ExtraFields []string
	// EnableColors dictates if output should be colored when Format is set to "text".
	EnableColors bool
	// Padding to use for various components when Format is set to "text".
	Padding int
}

// PAMConfig represents all the configuration data that needs to be passed to the child.
type PAMConfig struct {
	// UsePAMAuth specifies whether to trigger the "auth" PAM modules from the
	// policy.
	UsePAMAuth bool `json:"use_pam_auth"`

	// ServiceName is the name of the PAM service requested if PAM is enabled.
	ServiceName string `json:"service_name"`

	// Environment represents env variables to pass to PAM.
	Environment map[string]string `json:"environment"`
}

// UaccMetadata contains information the child needs from the parent for user accounting.
type UaccMetadata struct {
	// RemoteAddr is the address of the remote host.
	RemoteAddr utils.NetAddr `json:"remote_addr"`

	// UtmpPath is the path of the system utmp database.
	UtmpPath string `json:"utmp_path,omitempty"`

	// WtmpPath is the path of the system wtmp log.
	WtmpPath string `json:"wtmp_path,omitempty"`

	// BtmpPath is the path of the system btmp log.
	BtmpPath string `json:"btmp_path,omitempty"`

	// WtmpdbPath is the path of the system wtmpdb database.
	WtmpdbPath string `json:"wtmpdb_path,omitempty"`
}
