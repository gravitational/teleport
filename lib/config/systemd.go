// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"io"
	"os"
	"text/template"

	"github.com/gravitational/trace"
)

const (
	// SystemdDefaultEnvironmentFile is the default path to the env file for the systemd unit file config
	SystemdDefaultEnvironmentFile = "/etc/default/teleport"
	// SystemdDefaultPIDFile is the default path to the PID file for the systemd unit file config
	SystemdDefaultPIDFile = "/run/teleport.pid"
	// SystemdDefaultFileDescriptorLimit is the default max number of open file descriptors for the systemd unit file config
	SystemdDefaultFileDescriptorLimit = 8192
)

// systemdUnitFileTemplate is the systemd unit file configuration template.
var systemdUnitFileTemplate = template.Must(template.New("").Parse(`[Unit]
Description=Teleport Service
After=network.target

[Service]
Type=simple
Restart=on-failure
EnvironmentFile=-{{ .EnvironmentFile }}
ExecStart={{ .TeleportInstallationFile }} start --pid-file={{ .PIDFile }}
ExecReload=/bin/kill -HUP $MAINPID
PIDFile={{ .PIDFile }}
LimitNOFILE={{ .FileDescriptorLimit }}

[Install]
WantedBy=multi-user.target`))

// SystemdFlags specifies configuration parameters for a systemd unit file.
type SystemdFlags struct {
	// EnvironmentFile is the environment file path provided by the user.
	EnvironmentFile string
	// PIDFile is the process ID (PID) file path provided by the user.
	PIDFile string
	// FileDescriptorLimit is the maximum number of open file descriptors provided by the user.
	FileDescriptorLimit int
	// TeleportInstallationFile is the teleport installation path provided by the user.
	TeleportInstallationFile string
}

// CheckAndSetDefaults checks and sets default values for the flags.
func (f *SystemdFlags) CheckAndSetDefaults() error {
	if f.TeleportInstallationFile == "" {
		teleportPath, err := os.Readlink("/proc/self/exe")
		if err != nil {
			return trace.Wrap(err, "Can't find Teleport binary. Please specify the path.")
		}
		f.TeleportInstallationFile = teleportPath
	}

	return nil
}

// WriteSystemdUnitFile accepts flags and an io.Writer
// and writes the systemd unit file configuration to it
func WriteSystemdUnitFile(flags SystemdFlags, dest io.Writer) error {
	err := flags.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(systemdUnitFileTemplate.Execute(dest, flags))
}
