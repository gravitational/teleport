package windows_service

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	eventlogutils "github.com/gravitational/teleport/lib/utils/log/eventlog"
)

const serviceName = "TeleportUpdateService"
const ServiceCommand = "update-service"
const serviceAccessFlags = windows.SERVICE_START | windows.SERVICE_STOP | windows.SERVICE_QUERY_STATUS
const terminateTimeout = 30 * time.Second

var log = logutils.NewPackageLogger(teleport.ComponentKey, "update-service")

const eventSource = "updater-service"

func installEventSource() error {
	exe, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	// Assume that the message file is shipped next to tsh.exe.
	msgFilePath := filepath.Join(filepath.Dir(exe), "msgfile.dll")

	// This should create a registry entry under
	// SYSTEM\CurrentControlSet\Services\EventLog\Teleport\vnet with an absolute path to msgfile.dll.
	// If the user moves Teleport Connect to some other directory, logs will still be captured, but
	// they might display a message about missing event ID until the user reinstalls the app.
	err = eventlogutils.Install(eventlogutils.LogName, eventSource, msgFilePath, false /* useExpandKey */)
	return trace.Wrap(err)
}

func InstallService(ctx context.Context) (err error) {
	tshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting current exe Path")
	}
	//if err := assertTshInProgramFiles(tshPath); err != nil {
	//	return trace.Wrap(err, "checking if tsh.exe is installed under %%PROGRAMFILES%%")
	//}

	svcMgr, err := mgr.Connect()
	if err != nil {
		return trace.Wrap(err, "connecting to Windows service manager")
	}
	svc, err := svcMgr.OpenService(serviceName)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return trace.Wrap(err, "unexpected error checking if Windows service %s exists", serviceName)
		}
		// The service has not been created yet and must be installed.
		svc, err = svcMgr.CreateService(
			serviceName,
			tshPath,
			mgr.Config{
				StartType: mgr.StartManual,
			},
			ServiceCommand,
		)
		if err != nil {
			return trace.Wrap(err, "creating updater Windows service")
		}
	}
	if err := svc.Close(); err != nil {
		return trace.Wrap(err, "closing updater Windows service")
	}
	if err := grantServiceRights(); err != nil {
		return trace.Wrap(err, "granting authenticated users permission to control the VNet Windows service")
	}
	if err := installEventSource(); err != nil {
		return trace.Wrap(err, "creating event source for logging")
	}
	return nil
}

func grantServiceRights() error {
	// Get the current security info for the service, requesting only the DACL
	// (discretionary access control list).
	si, err := windows.GetNamedSecurityInfo(serviceName, windows.SE_SERVICE, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return trace.Wrap(err, "getting current service security information")
	}
	// Get the DACL from the security info.
	dacl, _ /*defaulted*/, err := si.DACL()
	if err != nil {
		return trace.Wrap(err, "getting current service DACL")
	}
	// This is the universal well-known SID for "Authenticated Users".
	authenticatedUsersSID, err := windows.StringToSid("S-1-5-11")
	if err != nil {
		return trace.Wrap(err, "parsing authenticated users SID")
	}
	// Build an explicit access entry allowing authenticated users to start,
	// stop, and query the service.
	ea := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.SERVICE_QUERY_STATUS | windows.SERVICE_START | windows.SERVICE_STOP,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(authenticatedUsersSID),
		},
	}}
	// Merge the new explicit access entry with the existing DACL.
	dacl, err = windows.ACLFromEntries(ea, dacl)
	if err != nil {
		return trace.Wrap(err, "merging service DACL entries")
	}
	// Set the DACL on the service security info.
	if err := windows.SetNamedSecurityInfo(
		serviceName,
		windows.SE_SERVICE,
		windows.DACL_SECURITY_INFORMATION,
		nil,  // owner
		nil,  // group
		dacl, // dacl
		nil,  // sacl
	); err != nil {
		return trace.Wrap(err, "setting service DACL")
	}
	return nil
}

// runService is called from the normal user process to run the VNet Windows in
// the background and wait for it to exit. It will terminate the service and
// return immediately if [ctx] is canceled.
func RunService(ctx context.Context, cfg *Config) error {
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
func startService(ctx context.Context, cfg *Config) (*mgr.Service, error) {
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
		"--path", cfg.Path,
		//"--proxyHost", cfg.ProxyHost,
		//"--user-sid", cfg.UserSID,
	); err != nil {
		return nil, trace.Wrap(err, "starting Windows service %s", serviceName)
	}
	return service, nil
}

func setupServiceLogger() (func() error, error) {
	level := slog.LevelInfo
	if envVar := os.Getenv(teleport.VerboseLogsEnvVar); envVar != "" {
		isDebug, err := strconv.ParseBool(envVar)
		if err != nil {
			return nil, trace.Wrap(err, "parsing %s", teleport.VerboseLogsEnvVar)
		}
		if isDebug {
			level = slog.LevelDebug
		}
	}

	handler, close, err := logutils.NewSlogEventLogHandler(eventSource, level)
	if err != nil {
		return nil, trace.Wrap(err, "initializing log handler")
	}
	slog.SetDefault(slog.New(handler))
	return close, nil
}

// ServiceMain runs the Windows VNet admin service.
func ServiceMain() error {
	closeFn, err := setupServiceLogger()
	if err != nil {
		return trace.Wrap(err, "setting up logger for service")
	}

	if err := svc.Run(serviceName, &windowsService{}); err != nil {
		closeFn()
		return trace.Wrap(err, "running Windows service")
	}

	return trace.Wrap(closeFn(), "closing logger")
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
	logger := slog.With(teleport.ComponentKey, teleport.Component("vnet", "windows-service"))
	const cmdsAccepted = svc.AcceptStop // Interrogate is always accepted and there is no const for it.
	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error)
	go func() { errCh <- s.runInstall(ctx, args) }()

	var terminateTimedOut <-chan time.Time
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
				logger.InfoContext(ctx, "Received stop command, shutting down service")
				// Cancel the context passed to s.run to terminate the
				// networking stack.
				cancel()
				terminateTimedOut = cmp.Or(terminateTimedOut, time.After(terminateTimeout))
				status <- svc.Status{State: svc.StopPending}
			}
		case <-terminateTimedOut:
			logger.ErrorContext(ctx, "Networking stack failed to terminate within timeout, exiting process",
				slog.Duration("timeout", terminateTimeout))
			exitCode = 1
			break loop
		case err := <-errCh:
			if err == nil || errors.Is(err, context.Canceled) {
				logger.InfoContext(ctx, "Service terminated")
			} else {
				logger.ErrorContext(ctx, "Service terminated", "error", err)
				exitCode = 1
			}
			break loop
		}
	}
	status <- svc.Status{State: svc.Stopped, Win32ExitCode: exitCode}
	return false, exitCode
}

type Config struct {
	UserSID   string
	Path      string
	ProxyHost string
}

func (s *windowsService) runInstall(ctx context.Context, args []string) error {
	var cfg Config
	app := kingpin.New(serviceName, "Teleport Updater Windows Service")
	serviceCmd := app.Command("update-service", "Start the VNet service.")
	//serviceCmd.Flag("user-sid", "SID of the user running the client application").Required().StringVar(&cfg.UserSID)
	serviceCmd.Flag("path", "SID of the user running the client application").Required().StringVar(&cfg.Path)
	//serviceCmd.Flag("proxyHost", "SID of the user running the client application").Required().StringVar(&cfg.ProxyHost)
	cmd, err := app.Parse(args[1:])
	if err != nil {
		return trace.Wrap(err, "parsing runtime arguments to Windows service")
	}
	if cmd != serviceCmd.FullCommand() {
		return trace.BadParameter("Windows service runtime arguments did not match \"update-service\", args: %v", args[1:])
	}
	if err := performInstall(ctx, &cfg); err != nil {
		return trace.Wrap(err, "running admin process")
	}
	return nil
}

func performInstall(ctx context.Context, cfg *Config) error {
	cmd := exec.Command(cfg.Path, "--updated", "/S", "--force-run")
	err := cmd.Start()
	return trace.Wrap(err, "running admin process")
}
