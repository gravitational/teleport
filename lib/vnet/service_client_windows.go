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

package vnet

import (
	"os"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// VerifyServiceInstalledAndMatchesClient returns nil if the service is installed and matches the client version.
// Called by the client.
func VerifyServiceInstalledAndMatchesClient() error {
	// Avoid [mgr.Connect] because it requests elevated permissions.
	scManager, err := windows.OpenSCManager(nil /*machine*/, nil /*database*/, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return trace.Wrap(err, "opening Windows service manager")
	}
	defer windows.CloseServiceHandle(scManager)
	serviceNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return trace.Wrap(err, "converting service name to UTF16")
	}
	serviceHandle, err := windows.OpenService(scManager, serviceNamePtr, windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return trace.Wrap(err, "opening Windows service %v", serviceName)
	}
	service := &mgr.Service{
		Name:   serviceName,
		Handle: serviceHandle,
	}
	defer service.Close()

	config, err := service.Config()
	if err != nil {
		return trace.Wrap(err, "getting service config")
	}
	exe, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}
	serviceArgs, err := windows.DecomposeCommandLine(config.BinaryPathName)
	if err != nil {
		return trace.Wrap(err, "parsing Windows service binary command line")
	}
	if len(serviceArgs) == 0 {
		return trace.BadParameter("Windows service has empty binary command line")
	}

	// Require exact binary match between the client and service.
	// Hash comparison is enough here because same binary means same version.
	if err = compareFiles(exe, serviceArgs[0]); err != nil {
		return trace.Wrap(err, "comparing tsh.exe executable with service executable")
	}
	return nil
}
