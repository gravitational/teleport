/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package systemd

import (
	"io"
	"os"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

const (
	// DefaultEnvironmentFile is the default path to the env file for the systemd unit file config
	DefaultEnvironmentFile = "/etc/default/teleport"
	// DefaultPIDFile is the default path to the PID file for the systemd unit file config
	DefaultPIDFile = "/run/teleport.pid"
	// DefaultFileDescriptorLimit is the default max number of open file descriptors for the systemd unit file config
	DefaultFileDescriptorLimit = 524288
)

// unitFileTemplate is the systemd unit file configuration template.
var unitFileTemplate = template.Must(template.New("").Parse(`[Unit]
Description=Teleport Service
After=network.target

[Service]
Type=simple
Restart=always
RestartSec=5
EnvironmentFile=-{{ .EnvironmentFile }}
ExecStart={{ .TeleportInstallationFile }} start {{ if .FIPS }}--fips {{ end }}--config {{ .TeleportConfigPath }} --pid-file={{ .PIDFile }}
# systemd before 239 needs an absolute path
ExecReload=/bin/sh -c "exec pkill -HUP -L -F {{ .PIDFile }}"
PIDFile={{ .PIDFile }}
LimitNOFILE={{ .FileDescriptorLimit }}

[Install]
WantedBy=multi-user.target
`))

// Flags specifies configuration parameters for a systemd unit file.
type Flags struct {
	// EnvironmentFile is the environment file path provided by the user.
	EnvironmentFile string
	// PIDFile is the process ID (PID) file path provided by the user.
	PIDFile string
	// FileDescriptorLimit is the maximum number of open file descriptors provided by the user.
	FileDescriptorLimit int
	// TeleportInstallationFile is the teleport installation path provided by the user.
	TeleportInstallationFile string
	// TeleportConfigPath is the path to the teleport config file (as set by Teleport defaults)
	TeleportConfigPath string
	// FIPS configures teleport to run in a FIPS compliant mode.
	FIPS bool
}

// CheckAndSetDefaults checks and sets default values for the flags.
func (f *Flags) CheckAndSetDefaults() error {
	if f.TeleportInstallationFile == "" {
		teleportPath, err := os.Readlink("/proc/self/exe")
		if err != nil {
			return trace.Wrap(err, "Can't find Teleport binary. Please specify the path.")
		}
		f.TeleportInstallationFile = teleportPath
	}
	// set Teleport config path to the default
	if f.TeleportConfigPath == "" {
		f.TeleportConfigPath = defaults.ConfigFilePath
	}
	return nil
}

// WriteUnitFile accepts flags and an io.Writer
// and writes the systemd unit file configuration to it
func WriteUnitFile(flags Flags, dest io.Writer) error {
	err := flags.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(unitFileTemplate.Execute(dest, flags))
}
