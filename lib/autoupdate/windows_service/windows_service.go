package windows_service

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
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
	logger.Info("Started applying update", args)
	const cmdsAccepted = svc.AcceptStop // Interrogate is always accepted and there is no const for it.
	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error)
	go func() { errCh <- s.runInstall(ctx, args, logger) }()

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
	ProxyHost []string
}

func (s *windowsService) runInstall(ctx context.Context, args []string, logger *slog.Logger) error {
	var cfg Config
	app := kingpin.New(serviceName, "Teleport Updater Windows Service")
	serviceCmd := app.Command("update-service", "Start the VNet service.")
	//serviceCmd.Flag("user-sid", "SID of the user running the client application").Required().StringVar(&cfg.UserSID)
	serviceCmd.Flag("path", "SID of the user running the client application").Required().StringVar(&cfg.Path)
	serviceCmd.Flag("proxy-hosts", "SID of the user running the client application").Required().StringsVar(&cfg.ProxyHost)
	cmd, err := app.Parse(args[1:])
	if err != nil {
		return trace.Wrap(err, "parsing runtime arguments to Windows service")
	}
	if cmd != serviceCmd.FullCommand() {
		return trace.BadParameter("Windows service runtime arguments did not match \"update-service\", args: %v", args[1:])
	}
	if err := performInstall(ctx, &cfg, logger); err != nil {
		return trace.Wrap(err, "running admin process")
	}
	return nil
}

// RunMeElevated attempts to re-launch the current executable as Administrator.
// It returns true if the elevation was triggered (parent should exit),
// or false if it failed.
func runMeElevated() (bool, error) {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argsPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 1 // SW_NORMAL

	err := windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, showCmd)
	if err != nil {
		// If the user clicks "No" on the UAC dialog, this error will trigger.
		return false, err
	}

	return true, nil
}

// AmIAdmin checks if the current process has Admin privileges.
func AmIAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false
	}
	return true
}

func RelaunchAsAdmin(origin string, allowed bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// Command: myapp.exe --internal-update-origin "example.com" --internal-allowed=true
	args := fmt.Sprintf(`modify-reg --cluster=%s`, origin)

	// Construct arguments for the new process.
	// We use hidden internal flags to pass the data.
	if allowed {
		args = fmt.Sprintf(`%s --enabled`, args)
	}

	verbPtr, _ := syscall.UTF16PtrFromString("runas") // "runas" triggers UAC
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argsPtr, _ := syscall.UTF16PtrFromString(args)
	cwdPtr, _ := syscall.UTF16PtrFromString(".")

	// ShellExecute is fire-and-forget regarding the child process exit code.
	// It returns nil if the user clicked "Yes" on the UAC prompt.
	err = windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, 1) // 1 = SW_NORMAL
	if err != nil {
		// User clicked "No" or OS error
		return fmt.Errorf("user denied elevation or shell error: %w", err)
	}

	return nil
}

func UpdateOrigin(origin string, allowed bool) error {
	keyPath := `SOFTWARE\Policies\Teleport\TeleportConnect`
	valName := "AllowedUpdateOrigins"

	// 1. Open the Key with WRITE permissions.
	// We use CreateKey:
	// - If adding: it ensures the key exists.
	// - If removing: it opens the existing key.
	// Note: This requires Administrator/SYSTEM privileges.
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("access denied or failed to open key: %w", err)
	}
	defer k.Close()

	// 2. Read the existing Multi-String Value
	// If the value doesn't exist, GetStringsValue returns registry.ErrNotExist.
	// We treat that as an empty list.
	origins, _, err := k.GetStringsValue(valName)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to read value: %w", err)
	}

	// 3. Modify the list (Add or Remove)
	newOrigins := make([]string, 0, len(origins)+1)
	exists := false

	for _, o := range origins {
		if o == origin {
			exists = true
			if !allowed {
				// Case: Remove. We found the item, so we SKIP adding it to the new list.
				continue
			}
		}
		// Keep existing items
		newOrigins = append(newOrigins, o)
	}

	// Case: Add. If it wasn't found in the loop, append it now.
	if allowed && !exists {
		newOrigins = append(newOrigins, origin)
	}

	// 4. Write back to Registry
	if err := k.SetStringsValue(valName, newOrigins); err != nil {
		return fmt.Errorf("failed to write value: %w", err)
	}

	return nil
}

func GetAllowedOrigins() ([]string, error) {
	// 1. Open the Policy Key (Read-Only)
	keyPath := `SOFTWARE\Policies\Teleport\TeleportConnect`

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		// If the key doesn't exist, it means no policy is set.
		// You should fallback to your default hardcoded origin.
		return nil, trace.NotFound("policy key not found: %w", err)
	}
	defer k.Close()

	// 2. Read the Multi-String Value
	origins, _, err := k.GetStringsValue("AllowedUpdateOrigins")
	if err != nil {
		return nil, fmt.Errorf("failed to read value: %w", err)
	}

	return origins, nil
}

func performInstall(ctx context.Context, cfg *Config, logger *slog.Logger) error {
	path, err := secureCopy(cfg.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	ver, err := verifyFile(path)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Info("Found", "subject", ver.Subject, "version", ver.FileVersion, "status", ver.Status)

	allowedOrigins, err := GetAllowedOrigins()
	if err != nil {
		return trace.Wrap(err)
	}
	found := false
	for _, origin := range allowedOrigins {
		for _, pp := range cfg.ProxyHost {
			if origin == pp {
				found = true
				break
			}
		}
	}
	if !found {
		return trace.BadParameter("allowed origin not found in %s", cfg.ProxyHost)
	}

	var versions []string
	for _, ao := range allowedOrigins {
		resp, err := webclient.Ping(&webclient.Config{
			Context:   ctx,
			ProxyAddr: ao,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		versions = append(versions, resp.AutoUpdate.ToolsVersion)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	foundVer := false
	for _, ver1 := range versions {
		if ver1 == ver.FileVersion {
			foundVer = true
		}
	}

	if !foundVer {
		return trace.BadParameter("Updating to this version of Teleport is not allowed")
	}

	cmd := exec.Command(path, "--updated", "/S", "--force-run")
	// SysProcAttr holds Windows-specific attributes
	//cmd.SysProcAttr = &windows.SysProcAttr{
	//	// DETACHED_PROCESS: The new process does not inherit the parent's console.
	//	// CREATE_NEW_PROCESS_GROUP: Ensures the child doesn't receive Ctrl+C signals sent to the parent.
	//	CreationFlags:    windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	//	NoInheritHandles: true,
	//}
	logger.Info("Running command", "command", cmd.String())

	// Use Start() instead of Run().
	// Start() returns immediately after the process is launched.
	err = cmd.Start()
	if err != nil {
		return err
	}

	// Important: Release the handle to the process so the parent
	// doesn't keep a reference to the child in its process table.
	if cmd.Process != nil {
		logger.Info("Releasing resources")
		err = cmd.Process.Release()
	}

	os.Exit(0)

	return trace.Wrap(err, "running admin process")
}

func secureCopy(userPath string) (string, error) {
	// 1. Define source (Insecure User Space)
	// e.g. C:\Users\Alice\AppData\Local\Temp\update.exe

	// 2. Define destination (Secure SYSTEM Space)
	secureDir := os.TempDir()
	err := os.MkdirAll(secureDir, 0755) // ensure dir exists
	if err != nil {
		return "", trace.Wrap(err)
	}

	securePath := filepath.Join(secureDir, "update_secure.exe")

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 1. Impersonate the client who called the RPC/Service
	// Note: In a real service, this is often RpcImpersonateClient()
	// or getting a token from a specific process ID.
	err = windows.ImpersonateSelf(windows.SecurityImpersonation)
	if err != nil {
		return "", fmt.Errorf("failed to impersonate: %v", err)
	}

	// Ensure we ALWAYS revert to the service's own identity (SYSTEM/Admin)
	defer windows.RevertToSelf()

	// 2. Open the source file as the USER
	// Using specific flags to prevent Symlink attacks (FILE_FLAG_OPEN_REPARSE_POINT)
	srcHandle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(userPath),
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)

	if err != nil {
		// If this fails, the user likely doesn't have permissions for this file
		return "", fmt.Errorf("user access denied to source file: %v", err)
	}

	// Convert the Windows handle to a Go file pointer
	srcFile := os.NewFile(uintptr(srcHandle), userPath)
	defer srcFile.Close()

	// 3. Revert to Service Context to perform the Write
	// We call this explicitly now (instead of waiting for defer) so the next
	// commands run with high privileges.
	windows.RevertToSelf()

	// 4. Create the destination file in the protected %ProgramData% folder
	// This succeeds because we are back to being a privileged Service.
	dstFile, err := os.OpenFile(securePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create protected destination: %v", err)
	}
	defer dstFile.Close()

	// 5. Manual Byte Copy
	// This reads through the handle validated as the user, but writes as the Service.
	_, err = io.Copy(dstFile, srcFile)

	return securePath, nil
}

type FileInfo struct {
	Status      string `json:"Status"`      // Valid, NotSigned, etc.
	Subject     string `json:"Subject"`     // "CN=My Company, O=..."
	FileVersion string `json:"FileVersion"` // "1.2.3.4"
}

func verifyFile(path string) (*FileInfo, error) {
	// PowerShell Script to run
	// 1. Get Signature
	// 2. Get Version Info
	// 3. Output as compressed JSON
	psScript := fmt.Sprintf(`
		$path = "%s"
		$sig = Get-AuthenticodeSignature -FilePath $path
		$ver = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($path)
		
		$obj = @{
			Status = $sig.Status.ToString()
			Subject = if ($sig.SignerCertificate) { $sig.SignerCertificate.Subject } else { "" }
			FileVersion = $ver.FileVersion
		}
		
		$obj | ConvertTo-Json -Compress
	`, path)

	// Wrap in a command
	// -NoProfile: Speeds up load time
	// -NonInteractive: Prevents prompts
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("powershell error: %v, output: %s", err, string(output))
	}

	// Parse JSON
	var info FileInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse json: %v | raw: %s", err, string(output))
	}

	return &info, nil
}
