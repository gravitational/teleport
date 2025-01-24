// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	ServiceCommand     = "vnet-service"
	serviceName        = "TeleportVNet"
	serviceDescription = "This service manages networking and OS configuration for Teleport VNet."
	serviceAccessFlags = windows.SERVICE_START | windows.SERVICE_STOP | windows.SERVICE_QUERY_STATUS
)

// runService is called from the normal user process to run the VNet Windows in
// the background and wait for it to exit. It will terminate the service and
// return immediately if [ctx] is canceled.
func runService(ctx context.Context) error {
	service, err := startService(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer service.Close()
	log.InfoContext(ctx, "Started Windows service", "service", service.Name)
	ticker := time.Tick(time.Second)
	for {
		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "Context canceled, stopping Windows service")
			if _, err := service.Control(svc.Stop); err != nil {
				return trace.Wrap(err, "sending stop request to Windows service %s", service.Name)
			}
			return nil
		case <-ticker:
			status, err := service.Query()
			if err != nil {
				return trace.Wrap(err, "querying admin service")
			}
			if status.State != svc.Running && status.State != svc.StartPending {
				return trace.Errorf("service stopped running prematurely, status: %+v", status)
			}
		}
	}
}

// startService starts the Windows VNet admin service in the background.
func startService(ctx context.Context) (*mgr.Service, error) {
	// Avoid [mgr.Connect] because it requests elevated permissions.
	scManager, err := windows.OpenSCManager(nil /*machine*/, nil /*database*/, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, trace.Wrap(err, "opening Windows service manager")
	}
	defer windows.CloseServiceHandle(scManager)
	serviceNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return nil, trace.Wrap(err, "converting service name to UTF16")
	}
	serviceHandle, err := windows.OpenService(scManager, serviceNamePtr, serviceAccessFlags)
	if err != nil {
		return nil, trace.Wrap(err, "opening Windows service %v", serviceName)
	}
	service := &mgr.Service{
		Name:   serviceName,
		Handle: serviceHandle,
	}
	if err := service.Start(ServiceCommand); err != nil {
		return nil, trace.Wrap(err, "starting Windows service %s", serviceName)
	}
	return service, nil
}

// ServiceMain runs the Windows VNet admin service.
func ServiceMain() error {
	if err := setupServiceLogger(); err != nil {
		return trace.Wrap(err, "setting up logger for service")
	}
	if err := svc.Run(serviceName, &windowsService{}); err != nil {
		return trace.Wrap(err, "running Windows service")
	}
	return nil
}

// windowsService implements [svc.Handler].
type windowsService struct{}

// Execute implements [svc.Handler.Execute], the GoDoc is copied below.
//
// Execute will be called by the package code at the start of the service, and
// the service will exit once Execute completes.  Inside Execute you must read
// service change requests from [requests] and act accordingly. You must keep
// service control manager up to date about state of your service by writing
// into [status] as required.  args contains service name followed by argument
// strings passed to the service.
// You can provide service exit code in exitCode return parameter, with 0 being
// "no error". You can also indicate if exit code, if any, is service specific
// or not by using svcSpecificEC parameter.
func (s *windowsService) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop // Interrogate is always accepted and there is no const for it.
	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error)
	go func() { errCh <- s.run(ctx, args) }()

loop:
	for {
		select {
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				state := svc.Running
				if ctx.Err() != nil {
					state = svc.StopPending
				}
				status <- svc.Status{State: state, Accepts: cmdsAccepted}
			case svc.Stop:
				slog.InfoContext(ctx, "Received stop command, shutting down service")
				cancel()
				status <- svc.Status{State: svc.StopPending}
			}
		case err := <-errCh:
			slog.ErrorContext(ctx, "Windows VNet service terminated", "error", err)
			if err != nil {
				exitCode = 1
			}
			break loop
		}
	}
	status <- svc.Status{State: svc.Stopped, Win32ExitCode: exitCode}
	return false, exitCode
}

func (s *windowsService) run(ctx context.Context, args []string) error {
	if err := runWindowsAdminProcess(ctx); err != nil {
		return trace.Wrap(err, "running admin process")
	}
	return nil
}

func setupServiceLogger() error {
	logFile, err := serviceLogFile()
	if err != nil {
		return trace.Wrap(err, "creating log file for service")
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	return nil
}

func serviceLogFile() (*os.File, error) {
	// TODO(nklaassen): find a better path for Windows service logs.
	exePath, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err, "getting current executable path")
	}
	dir := filepath.Dir(exePath)
	logFile, err := os.Create(filepath.Join(dir, "logs.txt"))
	if err != nil {
		return nil, trace.Wrap(err, "creating log file")
	}
	return logFile, nil
}
