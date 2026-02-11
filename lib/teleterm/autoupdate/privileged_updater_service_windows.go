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
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/windowsservice"
)

const (
	ServiceCommand     = "connect-updater-service"
	serviceName        = "TeleportConnectUpdater"
	serviceDescription = "Installs Teleport Connect updates without requiring administrator privileges."
	eventSource        = "connect-updater"
	serviceAccessFlags = windows.SERVICE_START | windows.SERVICE_QUERY_STATUS
	serviceRunTimeout  = 30 * time.Second

	// Allow SYSTEM/Admins Full Control, Authenticated Users read/write, implicitly denies everyone else.
	pipeSecurityDescriptor = "D:" + // DACL
		"(A;;GA;;;SY)" + // Allow (A);; Generic All (GA);;; SYSTEM (SY)
		"(A;;GA;;;BA)" + // Allow (A);; Generic All (GA);;; Built-in Admins (BA)
		"(A;;GRGW;;;AU)" // Allow (A);; Generic Read/Write (GRGW);;; Authenticated Users (AU)
	updateDirSecurityDescriptor = "O:SY" + // Owner SYSTEM
		"D:P" + // 'P' blocks permissions inheritance from the parent directory
		"(A;OICI;GA;;;SY)" + // Allow System Full Access
		"(A;OICI;GA;;;BA)" // Allow Built-in Administrators Full Access

)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "autoupdate")

// PrivilegedServiceTestConfig allows overriding certain updater config properties.
// Must be used only in tests.
type PrivilegedServiceTestConfig struct {
	// UpdateDirSecurityDescriptor overrides updateDirSecurityDescriptor.
	UpdateDirSecurityDescriptor string
	// UpdateBaseDir overrides the default %ProgramData%\TeleportConnectUpdater update path.
	UpdateBaseDir string
	// PolicyToolsVersion overrides ToolsVersion in HKLM\SOFTWARE\Policies\Teleport\TeleportConnect.
	PolicyToolsVersion string
	// PolicyToolsVersion overrides CdnBaseUrl in HKLM\SOFTWARE\Policies\Teleport\TeleportConnect.
	PolicyCDNBaseURL string
}

// InstallPrivilegedService installs Teleport Connect privileged update service.
// The service allows installing updates with asking for admin permissions.
func InstallPrivilegedService(ctx context.Context) (err error) {
	return trace.Wrap(windowsservice.Install(ctx, &windowsservice.InstallConfig{
		Name:              serviceName,
		Command:           ServiceCommand,
		Description:       serviceDescription,
		EventSourceName:   eventSource,
		AccessPermissions: serviceAccessFlags,
	}))
}

// UninstallPrivilegedService uninstalls Teleport Connect privileged update service.
func UninstallPrivilegedService(ctx context.Context) (err error) {
	return trace.Wrap(windowsservice.Uninstall(ctx, &windowsservice.UninstallConfig{
		Name:            serviceName,
		EventSourceName: eventSource,
	}))
}

// PrivilegedServiceMain implements Teleport Connect privileged update service.
// The service allows installing updates with asking for admin permissions.
func PrivilegedServiceMain() error {
	h := &handler{
		testCfg: &PrivilegedServiceTestConfig{},
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

// PrivilegedServiceMainTest implements Teleport Connect privileged update service.
// It runs the service implementation directly.
// Must be used only in tests.
func PrivilegedServiceMainTest(ctx context.Context, cfg *PrivilegedServiceTestConfig) error {
	h := &handler{
		testCfg: cfg,
	}
	return trace.Wrap(h.Execute(ctx, nil))
}

type handler struct {
	testCfg *PrivilegedServiceTestConfig
}

func (h *handler) Execute(ctx context.Context, _ []string) (err error) {
	ctx, cancel := context.WithTimeout(ctx, serviceRunTimeout)
	defer cancel()

	updaterConfig, err := h.getUpdaterConfig()
	if err != nil {
		return trace.Wrap(err, "getting updater config")
	}

	conn, err := waitForSingleClient(ctx)
	if err != nil {
		return trace.Wrap(err, "waiting for client")
	}

	dir, err := h.getSecureUpdateDir()
	if err != nil {
		return trace.NewAggregate(err, conn.Close())
	}

	updatePath := filepath.Join(dir, "update.exe")
	updateMeta, err := readUpdate(conn, updatePath)
	if err != nil {
		return trace.NewAggregate(err, conn.Close())
	}
	if err = conn.Close(); err != nil {
		return trace.Wrap(err)
	}

	if updaterConfig.version != "" && updateMeta.Version != updaterConfig.version {
		return trace.BadParameter("update version %s does not match policy version %s", updateMeta.Version, updaterConfig.version)
	}

	if err = ensureIsUpgrade(updateMeta.Version); err != nil {
		return trace.Wrap(err, "checking if update is upgrade")
	}

	// TODO(gzdunek): Add signature verification.

	hash, err := downloadChecksum(ctx, updaterConfig.cdnBaseURL, updateMeta.Version)
	if err != nil {
		return trace.Wrap(err, "downloading update checksum")
	}

	if err = verifyUpdateChecksum(updatePath, hash); err != nil {
		return trace.Wrap(err, "verifying update checksum")
	}

	return trace.Wrap(runInstaller(updatePath, updateMeta.ForceRun), "running installer")
}

// getUpdaterConfig reads the per-machine config.
func (h *handler) getUpdaterConfig() (*policyValue, error) {
	policyValues, err := readRegistryPolicyValues(registry.LOCAL_MACHINE)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	versionFromPolicy := policyValues.version
	if h.testCfg.PolicyToolsVersion != "" {
		versionFromPolicy = h.testCfg.PolicyToolsVersion
	}
	if versionFromPolicy == teleportToolsVersionOff {
		return nil, trace.BadParameter(`ToolsVersion in HKLM\SOFTWARE\Policies\Teleport\TeleportConnect is "off", the update will not be installed`)
	}

	cdnBaseURL := policyValues.cdnBaseURL
	if h.testCfg.PolicyCDNBaseURL != "" {
		cdnBaseURL = h.testCfg.PolicyCDNBaseURL
	}
	if cdnBaseURL == "" {
		cdnBaseURL = getDefaultBaseURL()
	}
	if cdnBaseURL == "" {
		return nil, trace.BadParameter("client tools updates are disabled as they are licensed under AGPL. To use Community Edition builds or custom binaries, set CdnBaseUrl in HKLM\\SOFTWARE\\Policies\\Teleport\\TeleportConnect")
	}

	return &policyValue{
		cdnBaseURL: cdnBaseURL,
		version:    versionFromPolicy,
	}, nil
}

type acceptResult struct {
	conn net.Conn
	err  error
}

// waitForSingleClient waits for the first client and then closes the listener.
func waitForSingleClient(ctx context.Context) (net.Conn, error) {
	l, err := winio.ListenPipe(UpdaterPipePath, &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor,
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
		return nil, trace.NewAggregate(err, ctx.Err())
	case res := <-resCh:
		if res.err != nil {
			return nil, trace.Wrap(res.err)
		}

		if err = l.Close(); err != nil {
			return nil, trace.Wrap(err)
		}
		return res.conn, nil
	}
}

// getSecureUpdateDir secures %ProgramData%\TeleportConnectUpdater directory and then returns
// a unique  %ProgramData%\TeleportConnectUpdater\<GUID> path.
func (h *handler) getSecureUpdateDir() (string, error) {
	updateRoot := h.testCfg.UpdateBaseDir
	if updateRoot == "" {
		programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
		if err != nil {
			return "", trace.Wrap(err, "reading ProgramData path")
		}
		updateRoot = filepath.Join(programData, "TeleportConnectUpdater")
	}

	descriptor := updateDirSecurityDescriptor
	if h.testCfg.UpdateDirSecurityDescriptor != "" {
		descriptor = h.testCfg.UpdateDirSecurityDescriptor
	}
	sd, err := windows.SecurityDescriptorFromString(descriptor)
	if err != nil {
		return "", trace.Wrap(err, "creating security descriptor")
	}

	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}

	if err = ensureDirIsSecure(updateRoot, sa); err != nil {
		return "", trace.Wrap(err, "securing TeleportConnectUpdater directory")
	}

	err = cleanupOldUpdates(updateRoot)
	if err != nil {
		return "", trace.Wrap(err, "cleaning up old updates")
	}

	// Create a per-update random directory. This prevents DLL planting attacks, as the update is executed from its own directory.
	newGUID := uuid.New().String()
	updateDir := filepath.Join(updateRoot, newGUID)
	updateDirPtr, err := windows.UTF16PtrFromString(updateDir)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if err = windows.CreateDirectory(updateDirPtr, sa); err != nil {
		return "", trace.Wrap(err, "failed to create update dir")
	}

	return updateDir, nil
}

// ensureDirIsSecure guarantees that the directory exists and is locked down to SYSTEM/Admins only.
func ensureDirIsSecure(dir string, sa *windows.SecurityAttributes) error {
	namePtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return trace.Wrap(err)
	}

	// Try to create the directory with the secure ACLs immediately.
	err = windows.CreateDirectory(namePtr, sa)
	if err == nil {
		return nil
	}
	if !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return trace.Wrap(err, "creating directory")
	}

	// If the directory exists, open a handle with DACL modification rights
	// We use FILE_FLAG_OPEN_REPARSE_POINT to ensure we open the directory itself,
	// not a target it might point to (it could be a junction).
	dirHandle, err := windows.CreateFile(
		namePtr,
		windows.READ_CONTROL|windows.WRITE_DAC|windows.WRITE_OWNER,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return trace.Wrap(err, "failed to open handle to existing directory")
	}
	defer windows.CloseHandle(dirHandle)

	// Verify it is a real directory (not a symlink/junction)
	// This prevents redirection attacks where we might unexpectedly secure a system folder.
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(dirHandle, &info); err != nil {
		return trace.Wrap(err, "getting file information")
	}

	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return trace.BadParameter("security violation: %s is a reparse point", dir)
	}

	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return trace.BadParameter("security violation: %s exists but is not a directory", dir)
	}

	owner, _, err := sa.SecurityDescriptor.Owner()
	if err != nil {
		return trace.Wrap(err, "reading owner from security descriptor")
	}
	dacl, _, err := sa.SecurityDescriptor.DACL()
	if err != nil {
		return trace.Wrap(err, "reading DACL from security descriptor")
	}

	// Reapply directory ACLs.
	err = windows.SetSecurityInfo(
		dirHandle,
		windows.SE_FILE_OBJECT,
		// PROTECTED_DACL_SECURITY_INFORMATION stops the directory from inheriting
		// "User Write" permissions from the parent (%ProgramData%).
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner,
		nil,
		dacl,
		nil,
	)

	return trace.Wrap(err, "resetting directory security")
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
			log.Error("Failed to remove old update file", fullPath)
		}
	}
	return nil
}

func ensureIsUpgrade(updateVersion string) error {
	updateSemver, err := semver.NewVersion(updateVersion)
	if err != nil {
		return trace.Wrap(err)
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

func downloadChecksum(ctx context.Context, baseUrl string, version string) ([]byte, error) {
	parsedBaseURL, err := url.Parse(baseUrl)
	if err != nil {
		return nil, trace.Wrap(err, "parsing base URL")
	}
	filename := fmt.Sprintf("Teleport Connect Setup-%s.exe.sha256", version)
	downloadURL := parsedBaseURL.JoinPath(filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
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
	return hexBytes, trace.Wrap(err)
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

	// Release the handle to the parent process can exit and the installer will continue.
	return trace.Wrap(cmd.Process.Release())
}
