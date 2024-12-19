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

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
)

const (
	serviceName        = "TeleportVNet"
	serviceDescription = "This service manages networking and OS configuration for Teleport VNet."
)

var (
	// ErrVnetNotImplemented is an error indicating that VNet is not implemented on the host OS.
	ErrVnetNotImplemented = &trace.NotImplementedError{Message: "VNet is not implemented on windows"}
)

// execAdminProcess is called from the normal user process to execute the admin
// subcommand as root.
func execAdminProcess(ctx context.Context, cfg AdminProcessConfig) error {
	service, err := startService(cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer service.Close()
	for {
		select {
		case <-ctx.Done():
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

func startService(cfg AdminProcessConfig) (*mgr.Service, error) {
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
	serviceHandle, err := windows.OpenService(scManager, serviceNamePtr, windows.SERVICE_START)
	if err != nil {
		if installErr := escalateAndInstallService(cfg); installErr != nil {
			return nil, trace.NewAggregate(
				trace.Wrap(err, "opening Windows service %s", serviceName),
				trace.Wrap(installErr, "installing Windows service"))
		}
		serviceHandle, err = windows.OpenService(scManager, serviceNamePtr, windows.SERVICE_START)
		if err != nil {
			return nil, trace.Wrap(err, "opening Windows service immediately after installation")
		}
	}
	service := &mgr.Service{
		Name:   serviceName,
		Handle: serviceHandle,
	}
	if err := service.Start(serviceArgs(cfg)...); err != nil {
		return nil, trace.Wrap(err, "starting Windows service %s", serviceName)
	}
	return service, nil
}

func escalateAndInstallService(cfg AdminProcessConfig) error {
	tshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}
	user, err := user.Current()
	if err != nil {
		return trace.Wrap(err, "getting current user")
	}
	args, err := ptrsFromStrings(
		"runas",
		shsprintf.EscapeDefaultContext(tshPath),
		escapeAndJoinArgs("vnet-install-service", "--userSID", user.Uid, "--home", cfg.HomePath),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := windows.ShellExecute(
		0,       // parent window handle (default is no window)
		args[0], // verb
		args[1], // file
		args[2], // args
		nil,     // cwd (default is current directory)
		1,       // showCmd (1 is normal)
	); err != nil {
		return trace.Wrap(err, "installing Windows service as administrator via runas")
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

func serviceArgs(cfg AdminProcessConfig) []string {
	return []string{
		"vnet-service",
		"--pipe", cfg.NamedPipe,
		"--ipv6-prefix", cfg.IPv6Prefix,
		"--dns-addr", cfg.DNSAddr,
	}
}

func InstallService(userSID, home string) error {
	m, err := mgr.Connect()
	if err != nil {
		return trace.Wrap(err, "connecting to Windows service manager")
	}
	defer m.Disconnect()
	service, err := installService(m, home)
	if err != nil {
		return trace.Wrap(err, "installing Windows service")
	}
	if err := configureServicePermissions(service, userSID); err != nil {
		return trace.Wrap(err, "configuring Windows service permissions")
	}
	return nil
}

func installService(m *mgr.Mgr, home string) (*mgr.Service, error) {
	if service, err := m.OpenService(serviceName); err == nil {
		// Service is already installed.
		return service, nil
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
	args := []string{
		"vnet-service",
		"--home", profile.FullProfilePath(os.Getenv(types.HomeEnvVar)),
	}
	service, err := m.CreateService(serviceName, tshPath, serviceCfg, args...)
	if err != nil {
		return nil, trace.Wrap(err, "creating Windows service")
	}
	return service, nil
}

func configureServicePermissions(service *mgr.Service, userSIDStr string) error {
	userSID, err := windows.StringToSid(userSIDStr)
	if err != nil {
		return trace.Wrap(err, "converting SID from string")
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
		AccessPermissions: windows.ACCESS_MASK(
			windows.SERVICE_QUERY_STATUS | windows.SERVICE_INTERROGATE | windows.SERVICE_START | windows.SERVICE_STOP),
		AccessMode:  windows.GRANT_ACCESS,
		Inheritance: windows.NO_INHERITANCE,
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

// ServiceMain runs with Windows VNet service.
func ServiceMain(ctx context.Context) error {
	cleanup := setupServiceLogger()
	defer cleanup()
	s := &windowsService{
		done: ctx.Done(),
	}
	if err := svc.Run(serviceName, s); err != nil {
		return trace.Wrap(err, "running Windows service")
	}
	return nil
}

type windowsService struct {
	done <-chan struct{}
}

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
				break loop
			}
		case <-s.done:
			break loop
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
	slog.InfoContext(ctx, "Initial arguments", "args", os.Args)
	slog.InfoContext(ctx, "Executed arguments", "args", args)
	homePath := os.Getenv(types.HomeEnvVar)
	var (
		debug      bool
		pipePath   string
		ipv6Prefix string
		dnsAddr    string
	)
	app := kingpin.New(serviceName, "Teleport Windows Service")
	app.Flag("debug", "Enable verbose logging").Short('d').BoolVar(&debug)
	serviceCmd := app.Command("vnet-service", "Start the VNet service.")
	serviceCmd.Flag("pipe", "pipe path").Required().StringVar(&pipePath)
	serviceCmd.Flag("ipv6-prefix", "IPv6 prefix for the VNet").Required().StringVar(&ipv6Prefix)
	serviceCmd.Flag("dns-addr", "VNet DNS address").Required().StringVar(&dnsAddr)
	cmd, err := app.Parse(args[1:])
	if err != nil {
		return trace.Wrap(err, "parsing arguments")
	}
	slog.InfoContext(ctx, "Full command", "cmd", cmd)
	if err := RunAdminProcess(ctx, AdminProcessConfig{
		NamedPipe:  pipePath,
		IPv6Prefix: ipv6Prefix,
		DNSAddr:    dnsAddr,
		HomePath:   homePath,
	}); err != nil {
		return trace.Wrap(err, "running admin process")
	}
	return nil
}

func setupServiceLogger() func() {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	dir := filepath.Dir(exePath)
	logFile, err := os.Create(filepath.Join(dir, "logs.txt"))
	if err != nil {
		panic(err)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	return func() { logFile.Close() }
}
