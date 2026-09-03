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

package privilegedupdater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/teleterm/autoupdate/common"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/windowsservice"
)

// ServiceCommand is the tsh subcommand that the Windows service manager invokes when starting the
// updater service.
var ServiceCommand = []string{"connect-updater", ServiceSubCommand}

const (
	// ServiceSubCommand is the tsh subcommand under "connect-updater" that runs the updater service.
	ServiceSubCommand  = "service"
	serviceName        = "TeleportConnectUpdater"
	serviceDescription = "Installs Teleport Connect updates without requiring administrator privileges."
	eventSource        = "connect-updater"
	serviceAccessFlags = windows.SERVICE_START | windows.SERVICE_QUERY_STATUS
	serviceRunTimeout  = 30 * time.Second

	// SafePipeReadWriteAccess defines access for Authenticated Users (AU).
	//According to https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights
	// and https://stackoverflow.com/questions/29947524/c-let-user-process-write-to-local-system-named-pipe-custom-security-descrip
	// the pipe should not set GENERIC_WRITE for standard users as it would allow them to create the pipe.
	SafePipeReadWriteAccess = windows.GENERIC_READ | windows.FILE_WRITE_DATA

	// updateRootDirName is the staging directory under SystemTemp.
	updateRootDirName = "TeleportConnectUpdater"
)

var (
	modkernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procGetTempPath2W = modkernel32.NewProc("GetTempPath2W")
)

// getSystemTempPath resolves the SystemTemp path (%WINDIR%\SystemTemp) without creating it.
//
// Staging updates here is what keeps the privileged updater safe: SystemTemp lives under
// %WINDIR% and is writable only by SYSTEM and Administrators, so a standard user cannot
// pre-create the staging root, plant files, or open the staged installer to tamper with it.
// This relies on the service running as LocalSystem: GetTempPath2W returns the secure
// SystemTemp path only for SYSTEM callers (other callers get a regular temp directory).
//
// The path GetTempPath2W for SYSTEM callers comes from %SYSTEMTEMP%, which defaults
// to %WINDIR%\SystemTemp. An administrator can point it at a custom location, and we do not
// check that location's ACLs, since doing so reliably is complex. Keeping a custom location
// locked down is therefore the administrator's responsibility; Microsoft documents how to
// change the variable safely in
// https://support.microsoft.com/en-us/servicing/dotnetframework/2024/12/gettemppath-changes-in-windows-february-cumulative-update-preview.
//
// We prefer GetTempPath2W and fall back to %WINDIR%\SystemTemp on older builds. The caller
// verifies the resolved path exists and is a real directory, aborting the update otherwise.
func getSystemTempPath() (string, error) {
	// When Windows updates released in March 2025 and later are installed, this API will be supported on Windows 10,
	// version 1607 (Build 14393.7876) and later and Windows Server 2016 Build 14393.7876 and later versions.
	if err := procGetTempPath2W.Find(); err == nil {
		return getTempPath2()
	}

	// Fallback for older builds: %WINDIR%\SystemTemp was introduced sometime before Windows 10 build 19042.
	windowsDir, err := windows.KnownFolderPath(windows.FOLDERID_Windows, 0)
	if err != nil {
		return "", trace.Wrap(err, "reading Windows path")
	}
	return filepath.Join(windowsDir, "SystemTemp"), nil
}

func getTempPath2() (string, error) {
	buf := make([]uint16, windows.MAX_PATH+1)
	ret, _, err := procGetTempPath2W.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if ret == 0 {
		return "", trace.Wrap(err, "GetTempPath2W failed")
	}
	return filepath.Clean(windows.UTF16ToString(buf[:ret])), nil
}

// makePipeServerSecurityDescriptor allows SYSTEM/Admins Full Control and grants Authenticated Users the passed access mask.
func makePipeServerSecurityDescriptor(authenticatedUsersAccess uint32) string {
	return "D:" + // DACL
		"(A;;GA;;;SY)" + // Allow (A);; Generic All (GA);;; SYSTEM (SY)
		"(A;;GA;;;BA)" + // Allow (A);; Generic All (GA);;; Built-in Admins (BA)
		fmt.Sprintf("(A;;%#x;;;AU)", authenticatedUsersAccess) // Allow (A);; authenticatedUsersAccess ;;; Authenticated Users (AU)
}

var log = logutils.NewPackageLogger(teleport.ComponentKey, "autoupdate")

// ServiceTestConfig allows overriding certain updater config properties.
// For test use only.
type ServiceTestConfig struct {
	// SystemTempDir overrides the default %WINDIR%\SystemTemp root directory. Tests point this at t.TempDir().
	SystemTempDir string
	// ProgramDataDir overrides the default %ProgramData% root used for legacy
	// update-dir cleanup. Tests point this at t.TempDir().
	ProgramDataDir string
	// PolicyToolsVersion overrides ToolsVersion in HKLM\SOFTWARE\Policies\Teleport\TeleportConnect.
	PolicyToolsVersion string
	// PolicyCDNBaseURL overrides CdnBaseUrl in HKLM\SOFTWARE\Policies\Teleport\TeleportConnect.
	PolicyCDNBaseURL string
	// HTTPClient overrides the client used for checksum download.
	HTTPClient *http.Client
	// PipeAuthenticatedUsersAccess overrides Authenticated Users access mask in
	// the named pipe DACL. If zero, SafePipeReadWriteAccess is used.
	PipeAuthenticatedUsersAccess uint32
	// VerifySignature overrides signature verification.
	// If nil, verifySignature is used.
	VerifySignature func(updatePath string) error
}

// InstallService installs the Teleport Connect privileged update service.
// This service enables installing updates without prompting the user for administrator permissions.
func InstallService(ctx context.Context) (err error) {
	return trace.Wrap(windowsservice.Install(ctx, &windowsservice.InstallConfig{
		Name:              serviceName,
		Command:           ServiceCommand,
		Description:       serviceDescription,
		EventSourceName:   eventSource,
		AccessPermissions: serviceAccessFlags,
	}))
}

// UninstallService uninstalls Teleport Connect privileged update service.
func UninstallService(ctx context.Context) (err error) {
	return trace.Wrap(windowsservice.Uninstall(ctx, &windowsservice.UninstallConfig{
		Name:            serviceName,
		EventSourceName: eventSource,
	}))
}

// RunService implements Teleport Connect privileged update service.
// This service enables installing updates without prompting the user for administrator permissions.
func RunService() error {
	h := &handler{
		testCfg: &ServiceTestConfig{},
	}

	closeLogger, err := windowsservice.InitSlogEventLogger(eventSource)
	if err != nil {
		return trace.Wrap(err)
	}

	err = windowsservice.Run(&windowsservice.RunConfig{
		Name:    serviceName,
		Handler: h,
		Logger:  log,
	})
	return trace.NewAggregate(err, closeLogger())
}

// RunServiceTest implements Teleport Connect privileged update service.
// It runs the service implementation directly.
// For test use only.
func RunServiceTest(ctx context.Context, cfg *ServiceTestConfig) error {
	h := &handler{
		testCfg: cfg,
	}
	return trace.Wrap(h.Execute(ctx, nil))
}

type handler struct {
	testCfg *ServiceTestConfig
}

func (h *handler) Execute(ctx context.Context, _ []string) (err error) {
	ctx, cancel := context.WithTimeout(ctx, serviceRunTimeout)
	defer cancel()

	updaterConfig, err := h.getUpdaterConfig()
	if err != nil {
		return trace.Wrap(err, "getting updater config")
	}

	updateMeta, updatePath, err := h.readUpdateMeta(ctx)
	if err != nil {
		return trace.Wrap(err, "reading update metadata")
	}

	if updaterConfig.Version != "" && updateMeta.Version != updaterConfig.Version {
		return trace.BadParameter("update version %s does not match policy version %s", updateMeta.Version, updaterConfig.Version)
	}

	if err = ensureIsUpgrade(updateMeta.Version); err != nil {
		return trace.Wrap(err, "checking if update is upgrade")
	}

	verifySignatureFn := verifySignature
	if h.testCfg.VerifySignature != nil {
		verifySignatureFn = h.testCfg.VerifySignature
	}
	if err = verifySignatureFn(updatePath); err != nil {
		return trace.Wrap(err, "verifying update signature")
	}

	hash, err := h.downloadChecksum(ctx, updaterConfig.CDNBaseURL, updateMeta.Version)
	if err != nil {
		return trace.Wrap(err, "downloading update checksum")
	}

	if err = verifyUpdateChecksum(updatePath, hash); err != nil {
		return trace.Wrap(err, "verifying update checksum")
	}

	// TODO(gzdunek): REMOVE IN 20.0.0.
	h.removeLegacyUpdateRoots()

	return trace.Wrap(runInstaller(updatePath, updateMeta.ForceRun), "running installer")
}

// getUpdaterConfig reads the per-machine config.
func (h *handler) getUpdaterConfig() (*common.PolicyValues, error) {
	policyValues, err := common.ReadRegistryPolicyValues(registry.LOCAL_MACHINE)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	versionFromPolicy := policyValues.Version
	if h.testCfg.PolicyToolsVersion != "" {
		versionFromPolicy = h.testCfg.PolicyToolsVersion
	}
	if versionFromPolicy == common.TeleportToolsVersionOff {
		return nil, trace.AccessDenied("%s in HKLM\\%s is %q, automatic updates are disabled by system policy", common.RegistryValueToolsVersion, common.TeleportConnectPoliciesKeyPath, common.TeleportToolsVersionOff)
	}

	cdnBaseURL := policyValues.CDNBaseURL
	if h.testCfg.PolicyCDNBaseURL != "" {
		cdnBaseURL = h.testCfg.PolicyCDNBaseURL
	}
	if cdnBaseURL == "" {
		cdnBaseURL = common.GetDefaultBaseURL()
	}
	if cdnBaseURL == "" {
		return nil, trace.AccessDenied("client tools updates are disabled as they are licensed under AGPL. To use Community Edition builds or custom binaries, set %s in HKLM\\%s", common.RegistryValueCDNBaseURL, common.TeleportConnectPoliciesKeyPath)
	}

	return &common.PolicyValues{
		CDNBaseURL: cdnBaseURL,
		Version:    versionFromPolicy,
	}, nil
}

type acceptResult struct {
	conn net.Conn
	err  error
}

func (h *handler) readUpdateMeta(ctx context.Context) (_ *updateMetadata, _ string, err error) {
	pipeAuthenticatedUsersAccess := uint32(SafePipeReadWriteAccess)
	if h.testCfg.PipeAuthenticatedUsersAccess != 0 {
		pipeAuthenticatedUsersAccess = h.testCfg.PipeAuthenticatedUsersAccess
	}

	conn, err := waitForSingleClient(ctx, pipeAuthenticatedUsersAccess)
	if err != nil {
		return nil, "", trace.Wrap(err, "waiting for client")
	}
	closeConnOnce := sync.OnceValue(conn.Close)
	// Always defer conn.Close and return the error.
	defer func() {
		err = trace.NewAggregate(err, trace.Wrap(closeConnOnce(), "closing conn"))
	}()
	// Close conn early to unblock reads if ctx is canceled.
	defer context.AfterFunc(ctx, func() { _ = closeConnOnce() })()

	dir, err := h.getSecureUpdateDir()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	updatePath := filepath.Join(dir, "update.exe")
	updateMeta, err := readUpdate(conn, updatePath)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return updateMeta, updatePath, nil
}

// waitForSingleClient waits for the first client and then closes the listener.
func waitForSingleClient(ctx context.Context, authenticatedUsersAccess uint32) (net.Conn, error) {
	l, err := winio.ListenPipe(PipePath, &winio.PipeConfig{
		SecurityDescriptor: makePipeServerSecurityDescriptor(authenticatedUsersAccess),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resCh := make(chan acceptResult, 1)

	go func() {
		conn, acceptErr := l.Accept()
		resCh <- acceptResult{conn: conn, err: acceptErr}
	}()

	select {
	case <-ctx.Done():
		err = l.Close()
		// Drain the goroutine — l.Close() unblocks Accept().
		res := <-resCh
		if res.conn != nil {
			_ = res.conn.Close()
		}
		return nil, trace.NewAggregate(ctx.Err(), err)
	case res := <-resCh:
		if res.err != nil {
			return nil, trace.Wrap(res.err)
		}
		if err = l.Close(); err != nil {
			return nil, trace.NewAggregate(err, res.conn.Close())
		}
		return res.conn, nil
	}
}

// getSecureUpdateDir returns a fresh per-update staging directory under
// SystemTemp\TeleportConnectUpdater\<GUID>.
func (h *handler) getSecureUpdateDir() (string, error) {
	systemTempDir := h.testCfg.SystemTempDir
	if systemTempDir == "" {
		temp, err := getSystemTempPath()
		if err != nil {
			return "", trace.Wrap(err, "resolving SystemTemp")
		}
		systemTempDir = temp
	}
	// SystemTemp is provisioned by Windows; verify it before staging.
	if err := verifyRealDirectory(systemTempDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", trace.BadParameter("the updater requires %s to be available, ensure your system is up-to-date", systemTempDir)
		}
		return "", trace.Wrap(err, "verifying SystemTemp %s", systemTempDir)
	}

	updateRoot := filepath.Join(systemTempDir, updateRootDirName)

	// Inherit SystemTemp's locked-down ACL.
	if err := os.MkdirAll(updateRoot, 0); err != nil {
		return "", trace.Wrap(err, "creating staging root %s", updateRoot)
	}

	if err := cleanupOldUpdates(updateRoot); err != nil {
		return "", trace.Wrap(err, "cleaning up old updates")
	}

	// Isolate each install.
	newGUID := uuid.New().String()
	updateDir := filepath.Join(updateRoot, newGUID)
	if err := os.Mkdir(updateDir, 0); err != nil {
		return "", trace.Wrap(err, "creating update dir %s", updateDir)
	}
	return updateDir, nil
}

func verifyRealDirectory(path string) error {
	namePtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return trace.Wrap(err)
	}

	handle, err := windows.CreateFile(
		namePtr,
		windows.READ_CONTROL,
		// Metadata-only verification handle, closed immediately after the attribute
		// check, so we don't block concurrent readers/writers/deleters.
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return trace.Wrap(err, "opening directory %s", path)
	}
	defer windows.CloseHandle(handle)

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return trace.Wrap(err, "getting file information")
	}
	// Defense-in-depth: although SystemTemp is considered trusted, reject reparse points
	// to prevent file I/O from being redirected through a junction, mount point, or
	// symbolic link.
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return trace.BadParameter("security violation: %s is a reparse point", path)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return trace.BadParameter("security violation: %s exists but is not a directory", path)
	}
	return nil
}

// cleanupOldUpdates removes stale update directories and files from the cache.
// Failures to remove individual entries are logged and ignored so cleanup can continue.
//
// This is fine, as updates are always stored in freshly generated, random subdirectories.
// This saves us from accidentally executing attacker-controlled files (e.g., planted DLLs),
//
// Important:
// This function runs with SYSTEM privileges and relies on the Go standard library’s
// os.RemoveAll implementation on Windows. It detects reparse points (symlinks and
// junctions) and removes the link itself without ever recursing into the target,
// mitigating junction/symlink crossing attacks.
func cleanupOldUpdates(baseDir string) error {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, entry := range entries {
		fullPath := filepath.Join(baseDir, entry.Name())

		err = os.RemoveAll(fullPath)
		if err != nil {
			log.Error("Failed to remove old update file", "path", fullPath, "error", err)
		}
	}
	return nil
}

// removeLegacyUpdateRoots best-effort cleans up old %ProgramData% staging dir.
// The new SystemTemp staging path does not depend on this succeeding.
func (h *handler) removeLegacyUpdateRoots() {
	programData := h.testCfg.ProgramDataDir
	if programData == "" {
		var err error
		programData, err = windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
		if err != nil {
			log.WarnContext(context.Background(), "Skipping legacy update-root cleanup; cannot resolve ProgramData", "error", err)
			return
		}
	}
	name := "TeleportConnectUpdater"
	path := filepath.Join(programData, name)
	// This function runs with SYSTEM privileges and relies on the Go standard library’s
	// os.RemoveAll implementation on Windows. It detects reparse points (symlinks and
	// junctions) and removes the link itself without ever recursing into the target,
	// mitigating junction/symlink crossing attacks.
	if err := os.RemoveAll(path); err != nil {
		log.WarnContext(context.Background(), "Failed to remove legacy update root", "path", path, "error", err)
	}
}

func ensureIsUpgrade(updateVersion string) error {
	updateSemver, err := semver.NewVersion(updateVersion)
	if err != nil {
		return trace.Wrap(err, "invalid update version %q", updateVersion)
	}
	current := teleport.SemVer()
	if current == nil {
		return trace.BadParameter("current version is not available")
	}
	if updateSemver.Compare(*current) <= 0 {
		return trace.BadParameter("update version %s is not newer than current version %s", updateSemver, current)
	}
	return nil
}

func (h *handler) downloadChecksum(ctx context.Context, baseUrl string, version string) ([]byte, error) {
	parsedBaseURL, err := url.Parse(baseUrl)
	if err != nil {
		return nil, trace.Wrap(err, "parsing base URL")
	}
	// Keep updater policy aligned with Service.GetConfig RPC validation and reject non-TLS CDNs even if this path is called outside the UI flow.
	if parsedBaseURL.Scheme != "https" {
		return nil, trace.BadParameter("CDN base URL must be https")
	}
	filename := fmt.Sprintf("Teleport Connect Setup-%s.exe.sha256", version)
	downloadURL := parsedBaseURL.JoinPath(filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := http.DefaultClient
	if h.testCfg.HTTPClient != nil {
		client = h.testCfg.HTTPClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("update hash request failed with status %s", resp.Status)
	}

	var buf bytes.Buffer
	_, err = io.CopyN(&buf, resp.Body, sha256.Size*2) // SHA bytes to hex
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hexBytes, err := hex.DecodeString(buf.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return hexBytes, nil
}

func verifyUpdateChecksum(updatePath string, expectedHash []byte) error {
	file, err := os.Open(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return trace.Wrap(err)
	}
	actual := hasher.Sum(nil)
	if !bytes.Equal(actual, expectedHash) {
		return trace.BadParameter("hash of the update does not match downloaded checksum")
	}
	return nil
}

func runInstaller(updatePath string, forceRun bool) error {
	args := []string{"--updated", "/S", "/allusers"}
	if forceRun {
		args = append(args, "--force-run")
	}
	cmd := exec.Command(updatePath, args...)

	log.Info("Running command", "command", cmd.String())

	err := cmd.Start()
	if err != nil {
		return trace.Wrap(err, "starting installer path=%s args=%q", updatePath, strings.Join(args, " "))
	}

	// Release the handle so the parent process can exit and the installer will continue.
	return trace.Wrap(cmd.Process.Release())
}
