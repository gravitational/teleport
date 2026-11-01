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

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/registry"
)

const (
	// BinaryName specifies the name of the updater binary.
	BinaryName = "teleport-update.exe"
)

// targetServices returns the Windows services whose binPath is linked to the Teleport binaries.
// Path is the Windows service name, and Binary the name of the binary it runs.
// tbot is not managed on Windows.
func targetServices(ns *Namespace) []ServiceFile {
	return []ServiceFile{
		{
			Path:   ns.teleportServiceName(),
			Binary: "teleport",
		},
	}
}

func createTeleportProcess(ns *Namespace, readyChecker ReadyChecker, logger *slog.Logger) Process {
	return &scmService{
		serviceName: ns.teleportServiceName(),
		ready:       readyChecker,
		log:         logger,
	}
}

func createTbotProcess(ns *Namespace, _ ReadyChecker, _ *slog.Logger) Process {
	return unsupportedProcess{name: ns.name}
}

func (u *Updater) readMachineID() ([]byte, error) {
	return readMachineID()
}

func readMachineID() ([]byte, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []byte(value), nil
}
