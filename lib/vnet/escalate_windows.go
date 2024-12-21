// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName        = "TeleportVNet"
	serviceDescription = "This service manages networking and OS configuration for Teleport VNet."
	serviceAccessFlags = windows.SERVICE_START | windows.SERVICE_STOP | windows.SERVICE_QUERY_STATUS
)

// execAdminProcess is called from the normal user process start the VNet admin
// service, installing it first if necessary.
func execAdminProcess(ctx context.Context, cfg AdminProcessConfig) error {
	service, err := startService(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer service.Close()
	log.InfoContext(ctx, "Started Windows service", "service", serviceName)
	for {
		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "Context canceled, stopping Windows service")
			if _, err := service.Control(svc.Stop); err != nil {
				return trace.Wrap(err, "sending stop request to Windows service %s", serviceName)
			}
			return nil
		case <-time.After(time.Second):
			if status, err := service.Query(); err != nil {
				return trace.Wrap(err, "querying admin service")
			} else {
				if status.State != svc.Running && status.State != svc.StartPending {
					return trace.Errorf("service stopped running prematurely, status: %v", status)
				}
			}
		}
	}
}

func startService(ctx context.Context, cfg AdminProcessConfig) (*mgr.Service, error) {
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
		log.InfoContext(ctx, "Failed to open Windows service, trying to install the service", "error", err)
		if err := escalateAndInstallService(); err != nil {
			return nil, trace.Wrap(err, "installing Windows service")
		}
		if serviceHandle, err = waitForService(ctx, scManager, serviceNamePtr); err != nil {
			return nil, trace.Wrap(err, "waiting for service immediately after installation")
		}
	}
	service := &mgr.Service{
		Name:   serviceName,
		Handle: serviceHandle,
	}
	if err := service.Start("vnet-service", "--pipe", cfg.NamedPipe); err != nil {
		return nil, trace.Wrap(err, "starting Windows service %s", serviceName)
	}
	return service, nil
}

func escalateAndInstallService() error {
	user, err := user.Current()
	if err != nil {
		return trace.Wrap(err, "getting current user")
	}
	return trace.Wrap(escalateAndRunSubcommand("vnet-install-service", "--userSID", user.Uid))
}

func escalateAndRunSubcommand(args ...string) error {
	tshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}
	argPtrs, err := ptrsFromStrings(
		"runas",
		shsprintf.EscapeDefaultContext(tshPath),
		escapeAndJoinArgs(args...),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := windows.ShellExecute(
		0,          // parent window handle (default is no window)
		argPtrs[0], // verb
		argPtrs[1], // file
		argPtrs[2], // args
		nil,        // cwd (default is current directory)
		1,          // showCmd (1 is normal)
	); err != nil {
		return trace.Wrap(err, "running subcommand as administrator via runas")
	}
	return nil
}

func ptrsFromStrings(strs ...string) ([]*uint16, error) {
	ptrs := make([]*uint16, len(strs))
	for i := range ptrs {
		var err error
		ptrs[i], err = syscall.UTF16PtrFromString(strs[i])
		if err != nil {
			return nil, trace.Wrap(err, "converting string to UTF16")
		}
	}
	return ptrs, nil
}

func escapeAndJoinArgs(args ...string) string {
	for i := range args {
		args[i] = shsprintf.EscapeDefaultContext(args[i])
	}
	return strings.Join(args, " ")
}

func waitForService(ctx context.Context, scManager windows.Handle, serviceNamePtr *uint16) (windows.Handle, error) {
	deadline := time.After(30 * time.Second)
	for {
		serviceHandle, err := windows.OpenService(scManager, serviceNamePtr, serviceAccessFlags)
		if err == nil {
			return serviceHandle, nil
		}
		select {
		case <-ctx.Done():
			return 0, trace.Wrap(ctx.Err())
		case <-deadline:
			return 0, trace.Errorf("timeout waiting for service to start")
		case <-time.After(time.Second):
		}
	}
}

// InstallService implements the vnet-install-service command, it must run as
// administrator and installs the TeleportVNet Windows service.
func InstallService(ctx context.Context, userSID string) error {
	m, err := mgr.Connect()
	if err != nil {
		return trace.Wrap(err, "connecting to Windows service manager")
	}
	defer m.Disconnect()
	service, err := installService(m)
	if err != nil {
		return trace.Wrap(err, "installing Windows service")
	}
	defer service.Close()
	if err := configureServicePermissions(service, userSID); err != nil {
		slog.ErrorContext(ctx, "Error configuring permissions for the Windows service, will attempt to delete the service", "error", err)
		return trace.Wrap(service.Delete(), "deleting Windows service after failing to configure permissions")
	}
	return nil
}

func installService(m *mgr.Mgr) (*mgr.Service, error) {
	if _, err := m.OpenService(serviceName); err == nil {
		return nil, trace.Errorf("Windows service %s is already installed", serviceName)
	}
	serviceCfg := mgr.Config{
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		StartType:    mgr.StartManual,
		ErrorControl: mgr.ErrorNormal,
		DisplayName:  serviceName,
		Description:  serviceDescription,
	}
	tshPath, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err, "getting executable path")
	}
	service, err := m.CreateService(serviceName, tshPath, serviceCfg, "vnet-service")
	if err != nil {
		return nil, trace.Wrap(err, "creating Windows service")
	}
	return service, nil
}

// configureServicePermissions sets the security descriptor DACL on the service
// such that the user who installed the service (identified by userSIDStr) is
// allowed to start, stop, and query the service.
func configureServicePermissions(service *mgr.Service, userSIDStr string) error {
	userSID, err := windows.StringToSid(userSIDStr)
	if err != nil {
		return trace.Wrap(err, "parsing user SID from string")
	}
	securityDescriptor, err := windows.GetNamedSecurityInfo(
		service.Name, windows.SE_SERVICE, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return trace.Wrap(err, "getting current security descriptor for %s", service.Name)
	}
	currentDACL, _, err := securityDescriptor.DACL()
	if err != nil {
		return trace.Wrap(err, "getting DACL from security descriptor")
	}
	explicitAccess := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.ACCESS_MASK(serviceAccessFlags),
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(userSID),
		},
	}}
	newDACL, err := windows.ACLFromEntries(explicitAccess, currentDACL)
	if err != nil {
		return trace.Wrap(err, "preparing explicit access DACL")
	}
	if err := windows.SetNamedSecurityInfo(
		service.Name,
		windows.SE_SERVICE,
		windows.DACL_SECURITY_INFORMATION,
		nil,     // don't change owner
		nil,     // don't change group
		newDACL, // only set DACL
		nil,     // don't change SACL
	); err != nil {
		return trace.Wrap(err, "setting security descriptor for %s", service.Name)
	}
	return nil
}

// UninstallService implements the vnet-uninstall-service command to uninstall
// the TeleportVNet Windows service. If it does not have sufficient permissions,
// it tries to re-execute itself with administrator rights via a UAC prompt.
func UninstallService(ctx context.Context) error {
	m, err := mgr.Connect()
	if err != nil {
		slog.ErrorContext(ctx, "Error connecting to service manager, attempting to escalate to administrator",
			"error", err)
		err := escalateAndRunSubcommand("vnet-uninstall-service")
		return trace.Wrap(err, "escalating to administrator to uninstall service")
	}
	defer m.Disconnect()
	service, err := m.OpenService(serviceName)
	if err != nil {
		return trace.Wrap(err, "unable to open service, it may not be installed")
	}
	defer service.Close()
	if err := service.Delete(); err != nil {
		return trace.Wrap(err, "deleting Windows service")
	}
	return nil
}

// ServiceMain runs with Windows VNet service.
func ServiceMain() error {
	cleanup, err := setupServiceLogger()
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()
	if err := svc.Run(serviceName, &windowsService{}); err != nil {
		return trace.Wrap(err, "running Windows service")
	}
	return nil
}

type windowsService struct{}

// Execute implements [svc.Handler].
func (s *windowsService) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending, Accepts: cmdsAccepted}
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
				status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
			case svc.Stop, svc.Shutdown:
				slog.InfoContext(ctx, "Shutting down service, received command", "cmd", request.Cmd)
				break loop
			}
		case err := <-errCh:
			slog.ErrorContext(ctx, "Running Windows VNet service", "error", err)
			const exitCode = 1
			status <- svc.Status{State: svc.Stopped, Win32ExitCode: exitCode}
			return false, exitCode
		}
	}
	cancel()
	status <- svc.Status{State: svc.StopPending}
	<-errCh
	const exitCode = 0
	status <- svc.Status{State: svc.Stopped, Win32ExitCode: exitCode}
	return false, exitCode
}

func (s *windowsService) run(ctx context.Context, args []string) error {
	var pipePath string
	app := kingpin.New(serviceName, "Teleport Windows Service")
	serviceCmd := app.Command("vnet-service", "Start the VNet service.")
	serviceCmd.Flag("pipe", "pipe path").Required().StringVar(&pipePath)
	cmd, err := app.Parse(args[1:])
	if err != nil {
		return trace.Wrap(err, "parsing arguments")
	}
	if cmd != serviceCmd.FullCommand() {
		return trace.BadParameter("executed arguments did not match vnet-service")
	}
	cfg := AdminProcessConfig{
		NamedPipe: pipePath,
	}
	if err := RunAdminProcess(ctx, cfg); err != nil {
		return trace.Wrap(err, "running admin process")
	}
	return nil
}

func setupServiceLogger() (func(), error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err, "getting current executable path")
	}
	dir := filepath.Dir(exePath)
	logFile, err := os.Create(filepath.Join(dir, "logs.txt"))
	if err != nil {
		return nil, trace.Wrap(err, "creating log file")
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	return func() { logFile.Close() }, nil
}
