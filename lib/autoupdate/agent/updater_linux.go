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

package agent

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/lib/client/debug"
)

const (
	// BinaryName specifies the name of the updater binary.
	BinaryName = "teleport-update"
)

// targetServices returns the systemd services that are linked alongside the Teleport binaries.
func targetServices(ns *Namespace) []ServiceFile {
	return []ServiceFile{
		{
			Path:        ns.teleportServiceFile,
			Binary:      "teleport",
			ExampleName: teleportServiceName,
			ExampleFunc: ns.ReplaceTeleportService, // required for TryLinkSystem
		},
		{
			Path:   ns.tbotServiceFile,
			Binary: "tbot",
		},
	}
}

func createTeleportProcess(ns *Namespace, debugClient *debug.Client, logger *slog.Logger) Process {
	return &SystemdService{
		ServiceName: filepath.Base(ns.teleportServiceFile),
		PIDFile:     ns.teleportPIDFile,
		Ready:       debugClient,
		Log:         logger,
	}
}

func createTbotProcess(ns *Namespace, debugClient *debug.Client, logger *slog.Logger) Process {
	return &SystemdService{
		ServiceName:  filepath.Base(ns.tbotServiceFile),
		PIDFile:      ns.tbotPIDFile,
		Ready:        debugClient,
		Log:          logger,
		ForceRestart: true,
	}
}

func (u *Updater) readMachineID() ([]byte, error) {
	return os.ReadFile(u.MachineIDFile)
}
