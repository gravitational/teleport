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

package autoupdate

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceStartTimeout = 5 * time.Second
	servicePollInterval = 500 * time.Millisecond
)

// RunServiceAndInstallFromClient is called by the client.
func RunServiceAndInstallFromClient(ctx context.Context, path string, forceRun bool, version string) error {
	if err := ensureServiceRunning(ctx); err != nil {
		return trace.Wrap(err)
	}

	conn, err := winio.DialPipeContext(ctx, pipePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	meta := UpdateMetadata{ForceRun: forceRun, Version: version}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = binary.Write(conn, binary.LittleEndian, uint32(len(metaBytes))); err != nil {
		return trace.Wrap(err)
	}
	if _, err = conn.Write(metaBytes); err != nil {
		return trace.Wrap(err)
	}

	file, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()

	_, err = io.Copy(conn, file)
	return trace.Wrap(err)
}

func ensureServiceRunning(ctx context.Context) error {
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
	serviceHandle, err := windows.OpenService(scManager, serviceNamePtr, serviceAccessFlags)
	if err != nil {
		return trace.Wrap(err, "opening Windows service %v", serviceName)
	}
	service := &mgr.Service{
		Name:   serviceName,
		Handle: serviceHandle,
	}
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return trace.Wrap(err, "querying service status")
	}
	if status.State == svc.Running {
		return nil
	}

	if err = service.Start(ServiceCommand); err != nil {
		return trace.Wrap(err, "starting Windows service %s", serviceName)
	}

	deadline := time.Now().Add(serviceStartTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		default:
		}

		status, err = service.Query()
		if err == nil && status.State == svc.Running {
			return nil
		}
		time.Sleep(servicePollInterval)
	}

	return trace.LimitExceeded("timed out waiting for service to start")
}
