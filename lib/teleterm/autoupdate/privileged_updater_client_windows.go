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
	"errors"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

const (
	serviceStartTimeout   = 5 * time.Second
	serviceStartRetryStep = 500 * time.Millisecond
	serviceStartRetryMax  = 500 * time.Millisecond
	pipeDialTimeout       = 3 * time.Second
	pipeDialRetryStep     = 100 * time.Millisecond
	pipeDialRetryMax      = 300 * time.Millisecond
)

// RunServiceAndInstallUpdateFromClient is called by the client.
// It starts the update service, sends update metadata, and transfers the binary for validation and installation.
func RunServiceAndInstallUpdateFromClient(ctx context.Context, path string, forceRun bool, version string) error {
	if err := ensureServiceRunning(ctx); err != nil {
		// Service failed to start; fall back to client-side install (UAC).
		if installErr := runInstaller(path, forceRun); installErr != nil {
			return trace.Wrap(installErr, "fallback install failed after service start error: %v", err)
		}
		return nil
	}

	err := InstallUpdateFromClient(ctx, path, forceRun, version)
	return trace.Wrap(err)
}

// InstallUpdateFromClient sends update metadata, and transfers the binary for validation and installation.
func InstallUpdateFromClient(ctx context.Context, path string, forceRun bool, version string) error {
	conn, err := dialPipeWithRetry(ctx, UpdaterPipePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	// The update must be read by the client running as a standard user.
	// Passing the path directly to the SYSTEM service could cause it to read
	// files the user is not permitted to access.
	file, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()

	meta := updateMetadata{ForceRun: forceRun, Version: version}
	return trace.Wrap(writeUpdate(conn, meta, file))
}

func dialPipeWithRetry(ctx context.Context, path string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, pipeDialTimeout)
	defer cancel()
	linearRetry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step: pipeDialRetryStep,
		Max:  pipeDialRetryMax,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	isRetryError := func(err error) bool {
		return errors.Is(err, windows.ERROR_FILE_NOT_FOUND)
	}

	var conn net.Conn
	err = linearRetry.For(ctx, func() error {
		conn, err = winio.DialPipeContext(ctx, path)
		if err != nil && !isRetryError(err) {
			return retryutils.PermanentRetryError(trace.Wrap(err))
		}
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

func ensureServiceRunning(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, serviceStartTimeout)
	defer cancel()
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

	linearRetry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step: serviceStartRetryStep,
		Max:  serviceStartRetryMax,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = linearRetry.For(ctx, func() error {
		status, err = service.Query()
		if err != nil {
			return retryutils.PermanentRetryError(trace.Wrap(err))
		}
		if status.State != svc.Running {
			return trace.LimitExceeded("service not running yet")
		}
		return nil
	})
	return trace.Wrap(err)
}
