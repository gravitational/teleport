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
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/windowsservice"
)

const (
	ServiceCommand     = "vnet-service"
	serviceName        = "TeleportVNet"
	serviceDescription = "This service manages networking and OS configuration for Teleport VNet."
	serviceAccessFlags = windows.SERVICE_START | windows.SERVICE_STOP | windows.SERVICE_QUERY_STATUS
	terminateTimeout   = 30 * time.Second
)

// runService is called from the normal user process to run the VNet Windows in
// the background and wait for it to exit. It will terminate the service and
// return immediately if [ctx] is canceled.
func runService(ctx context.Context, cfg *windowsAdminProcessConfig) error {
	service, err := startService(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer service.Close()
	log.InfoContext(ctx, "Started Windows service", "service", service.Name)
	ticker := time.Tick(time.Second)
loop:
	for {
		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "Context canceled, stopping Windows service")
			if _, err := service.Control(svc.Stop); err != nil {
				return trace.Wrap(err, "sending stop request to Windows service %s", service.Name)
			}
			break loop
		case <-ticker:
			status, err := service.Query()
			if err != nil {
				return trace.Wrap(err, "querying Windows service %s", service.Name)
			}
			if status.State != svc.Running && status.State != svc.StartPending {
				return trace.Errorf("service stopped running prematurely, status: %+v", status)
			}
		}
	}
	// Wait for the service to actually stop. Add some buffer to
	// terminateTimeout which the service also uses to terminate itself to
	// hopefully allow it to exit on its own.
	deadline := time.After(terminateTimeout + 5*time.Second)
	for {
		select {
		case <-deadline:
			return trace.Errorf("Windows service %s failed to stop with %v", service.Name, terminateTimeout)
		case <-ticker:
			status, err := service.Query()
			if err != nil {
				return trace.Wrap(err, "querying Windows service %s", service.Name)
			}
			if status.State == svc.Stopped {
				return nil
			}
		}
	}
}

// startService starts the Windows VNet admin service in the background.
func startService(ctx context.Context, cfg *windowsAdminProcessConfig) (*mgr.Service, error) {
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
	if err := service.Start(ServiceCommand,
		"--addr", cfg.clientApplicationServiceAddr,
		"--cred-path", cfg.serviceCredentialPath,
		"--user-sid", cfg.userSID,
	); err != nil {
		return nil, trace.Wrap(err, "starting Windows service %s", serviceName)
	}
	return service, nil
}

// ServiceMain runs the Windows VNet admin service.
func ServiceMain() error {
	closeLogger, err := windowsservice.InitSlogEventLogger(eventSource)
	if err != nil {
		return trace.Wrap(err)
	}
	logger := slog.With(teleport.ComponentKey, teleport.Component("vnet", "windows-service"))

	err = windowsservice.Run(&windowsservice.RunConfig{
		Name:    serviceName,
		Handler: &handler{},
		Logger:  logger,
	})
	return trace.NewAggregate(err, closeLogger())
}

type handler struct{}

func (w *handler) Execute(ctx context.Context, args []string) error {
	var cfg windowsAdminProcessConfig
	app := kingpin.New(serviceName, "Teleport VNet Windows Service")
	serviceCmd := app.Command("vnet-service", "Start the VNet service.")
	serviceCmd.Flag("addr", "client application service address").Required().StringVar(&cfg.clientApplicationServiceAddr)
	serviceCmd.Flag("cred-path", "path to TLS credentials for connecting to client application").Required().StringVar(&cfg.serviceCredentialPath)
	serviceCmd.Flag("user-sid", "SID of the user running the client application").Required().StringVar(&cfg.userSID)
	cmd, err := app.Parse(args[1:])
	if err != nil {
		return trace.Wrap(err, "parsing runtime arguments to Windows service")
	}
	if cmd != serviceCmd.FullCommand() {
		return trace.BadParameter("Windows service runtime arguments did not match \"vnet-service\", args: %v", args[1:])
	}
	if err := runWindowsAdminProcess(ctx, &cfg); err != nil {
		return trace.Wrap(err, "running admin process")
	}
	return nil
}

// VerifyServiceInstalled checks whether the service is installed and running the expected version.
// It returns nil on success, or an error otherwise.
func VerifyServiceInstalled() error {
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
	serviceArgs, err := windows.DecomposeCommandLine(config.BinaryPathName)
	if err != nil {
		return trace.Wrap(err, "parsing Windows service binary command line")
	}
	if len(serviceArgs) == 0 {
		return trace.BadParameter("Windows service has empty binary command line")
	}
	exe, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	// Run the same check as the service.
	err = compareFiles(exe, serviceArgs[0])
	return trace.Wrap(err, "comparing running executable with service executable")
}
