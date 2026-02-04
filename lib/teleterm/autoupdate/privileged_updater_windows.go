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
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/windowsservice"
)

const (
	serviceName          = "TeleportConnectUpdater"
	ServiceCommand       = "connect-updater-service"
	eventSource          = "updater-service"
	serviceAccessFlags   = windows.SERVICE_START | windows.SERVICE_QUERY_STATUS
	pipePath             = `\\.\pipe\TeleportConnectUpdaterPipe`
	maxMetadataSize      = 1 * 1024 * 1024        // 1 MiB
	maxUpdatePayloadSize = 1 * 1024 * 1024 * 1024 // 1 GiB
	serviceTimeout       = 30 * time.Second

	// Allow SYSTEM/Admins Full Control, Authenticated Users read/write, implicitly denies everyone else.
	namedPipeSecurityDescriptor = "D:" + // DACL
		"(A;;GA;;;SY)" + // Allow (A);; Generic All (GA);;; SYSTEM (SY)
		"(A;;GA;;;BA)" + // Allow (A);; Generic All (GA);;; Built-in Admins (BA)
		"(A;;GRGW;;;AU)" // Allow (A);; Generic Read/Write (GRGW);;; Authenticated Users (AU)
	updateDirSecurityDescriptor = "O:SY" + // Owner SYSTEM
		"D:P" + // 'P' blocks permissions inheritance from the parent directory
		"(A;OICI;GA;;;SY)" + // Allow System Full Access
		"(A;OICI;GA;;;BA)" // Allow Built-in Administrators Full Access

)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "autoupdate")

func InstallService(ctx context.Context) (err error) {
	return trace.Wrap(windowsservice.Install(ctx, &windowsservice.InstallConfig{
		Name:              serviceName,
		Command:           ServiceCommand,
		EventSourceName:   eventSource,
		AccessPermissions: serviceAccessFlags,
	}))
}

func UninstallService(ctx context.Context) (err error) {
	return trace.Wrap(windowsservice.Uninstall(ctx, &windowsservice.UninstallConfig{
		Name:            serviceName,
		EventSourceName: eventSource,
	}))
}

func ServiceMain() error {
	closeLogger, err := windowsservice.InitSlogEventLogger(eventSource)
	if err != nil {
		return trace.Wrap(err)
	}
	logger := slog.With(teleport.ComponentKey, teleport.Component("privileged-updater"))

	err = windowsservice.Run(&windowsservice.RunConfig{
		Name:    serviceName,
		Handler: &handler{logger},
		Logger:  logger,
	})
	return trace.NewAggregate(err, closeLogger())
}

type handler struct {
	logger *slog.Logger
}

func (h *handler) Execute(ctx context.Context, _ []string) error {
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	baseURL, enforcedVersion, err := getUpdaterConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	updateMetadata, updatePath, err := receiveUpdateFromClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if enforcedVersion != "" && updateMetadata.Version != enforcedVersion {
		return trace.BadParameter("update version %q does not match policy version %q", updateMetadata.Version, enforcedVersion)
	}

	if err = VerifySignature(updatePath); err != nil {
		return trace.Wrap(err, "invalid update signature")
	}

	if err = ensureIsUpgrade(updateMetadata.Version); err != nil {
		return trace.Wrap(err)
	}

	hash, err := downloadChecksum(ctx, baseURL, updateMetadata.Version)
	if err != nil {
		return trace.Wrap(err, "downloading update hash")
	}

	if err = verifyUpdateChecksum(updatePath, hash); err != nil {
		return trace.Wrap(err, "verifying update hash")
	}

	return trace.Wrap(runInstaller(updatePath, updateMetadata, h.logger), "running admin process")
}

func getUpdaterConfig() (string, string, error) {
	policyValues, err := readRegistryPolicyValues(registry.LOCAL_MACHINE)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	versionFromPolicy := policyValues.version
	if versionFromPolicy == teleportToolsVersionOff {
		return "", "", trace.BadParameter("ToolsVersion is off, cannot install update")
	}

	cdnBaseURL := policyValues.cdnBaseURL
	defaultBaseUrlValue := getDefaultBaseURL()
	if cdnBaseURL == "" {
		cdnBaseURL = defaultBaseUrlValue
	}
	if cdnBaseURL == "" {
		return "", "", trace.BadParameter("Client tools updates are disabled as they are licensed under AGPL. To use Community Edition builds or custom binaries, set CdnBaseUrl in HKLM\\SOFTWARE\\Policies\\Teleport\\TeleportConnect")
	}

	return cdnBaseURL, versionFromPolicy, nil
}

func receiveUpdateFromClient(ctx context.Context) (*UpdateMetadata, string, error) {
	conn, err := waitForSingleClient(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer conn.Close()

	dir, err := getSecureUpdateDir()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	meta, updatePath, err := readUpdate(conn, dir)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return meta, updatePath, nil
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

	checksumBytes, err := io.ReadAll(utils.LimitReader(resp.Body, sha256.Size*2)) // SHA bytes to hex
	if err != nil {
		return nil, trace.Wrap(err)
	}
	expected, err := hex.DecodeString(strings.TrimSpace(string(checksumBytes)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return expected, trace.Wrap(err)
}

func runInstaller(updatePath string, meta *UpdateMetadata, logger *slog.Logger) error {
	args := []string{"--updated", "/S"}
	if meta.ForceRun {
		args = append(args, "--force-run")
	}
	cmd := exec.Command(updatePath, args...)

	logger.Info("Running command", "command", cmd.String())

	err := cmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	// Release the handle to the process so the parent doesn't keep a reference to the child in its process table.
	if cmd.Process != nil {
		err = cmd.Process.Release()
	}

	return trace.Wrap(err)
}

func verifyUpdateChecksum(updatePath string, expectedHash []byte) error {
	file, err := os.Open(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return trace.Wrap(err)
	}
	actual := hasher.Sum(nil)
	if !bytes.Equal(actual, expectedHash) {
		return trace.BadParameter("hash of an update does not match downloaded checksum")
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

type acceptResult struct {
	conn net.Conn
	err  error
}

func waitForSingleClient(ctx context.Context) (net.Conn, error) {
	l, err := winio.ListenPipe(pipePath, &winio.PipeConfig{
		SecurityDescriptor: namedPipeSecurityDescriptor,
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
		// Stop listening immediately. This removes the pipe name from the system.
		// Any new client trying to connect now will get an error.
		if err = l.Close(); err != nil {
			return nil, trace.Wrap(err)
		}
		return res.conn, nil
	}
}

type UpdateMetadata struct {
	ForceRun bool   `json:"force_run"`
	Version  string `json:"version"`
}

// readUpdate reads:
// 1. Length of UpdateMetadata header.
// 2. The actual UpdateMetadata.
// 3. The update binary until EOF.
// It writes the installer into destinationDir and returns the parsed metadata and the full path to the written file.
func readUpdate(conn io.ReadWriteCloser, destinationDir string) (*UpdateMetadata, string, error) {
	// Read UpdateMetadata length.
	var jsonLen uint32
	if err := binary.Read(conn, binary.LittleEndian, &jsonLen); err != nil {
		return nil, "", trace.Wrap(err)
	}
	if jsonLen > maxMetadataSize {
		return nil, "", trace.BadParameter("metadata payload too large")
	}

	meta := &UpdateMetadata{}
	if err := json.NewDecoder(io.LimitReader(conn, int64(jsonLen))).Decode(meta); err != nil {
		return nil, "", trace.Wrap(err)
	}
	if meta.Version == "" {
		return nil, "", trace.BadParameter("update version is required")
	}

	outFilePath := filepath.Join(destinationDir, "teleport-update.exe")

	outFile, err := os.OpenFile(outFilePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer outFile.Close()

	// io.Copy reads outFilePath EOF (connection closed), but cap the payload size.
	payloadReader := utils.LimitReader(conn, maxUpdatePayloadSize)
	_, err = io.Copy(outFile, payloadReader)
	if err != nil {
		// Remove the target file on error.
		return nil, "", trace.NewAggregate(err, outFile.Close(), os.Remove(outFilePath))
	}

	return meta, outFilePath, nil
}

func getSecureUpdateDir() (string, error) {
	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return "", trace.Wrap(err, "reading ProgramData path")
	}

	sd, err := windows.SecurityDescriptorFromString(updateDirSecurityDescriptor)
	if err != nil {
		return "", trace.Wrap(err, "creating security descriptor")
	}

	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}

	dir := filepath.Join(programData, "TeleportConnectUpdater")
	if err = ensureDirIsSecure(dir, sa); err != nil {
		return "", trace.Wrap(err, "securing TeleportConnectUpdater directory")
	}

	err = cleanupOldUpdates(dir)
	if err != nil {
		return "", trace.Wrap(err, "cleaning up old updates")
	}

	// CREATE FRESH WORKSPACE (Guaranteed Empty)
	// We create a random directory. This cannot conflict with locked files
	// because the name is new.
	newGUID := uuid.New().String()
	updateDir := filepath.Join(dir, newGUID)
	updateDirPtr, err := windows.UTF16PtrFromString(updateDir)
	if err != nil {
		return "", trace.Wrap(err, "creating secure directory")
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
		return trace.Wrap(err, "creating secure directory")
	}

	// Try to create the directory with the secure ACLs immediately.
	err = windows.CreateDirectory(namePtr, sa)
	if err == nil {
		return nil
	}
	if !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return trace.Wrap(err, "creating directory")
	}

	// If the directory exists, open a handle with SECURITY modification rights
	// We use FILE_FLAG_OPEN_REPARSE_POINT to ensure we open the directory itself,
	// not a target it might point to (if it's a junction).
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

	// Verify it is a real directory (Not a Symlink/Junction)
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
		// "User Write" permissions from the parent (e.g. ProgramData).
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		owner,
		nil,
		dacl,
		nil,
	)

	return trace.Wrap(err, "resetting directory security")
}

// cleanupOldUpdates removes stale update directories and files from the cache.
//
// SECURITY:
// This runs with SYSTEM privileges. To prevent "Junction/Symlink Crossing" attacks:
//  1. We rely on the Go standard library's os.RemoveAll implementation for Windows.
//  2. Go's os.RemoveAll explicitly detects reparse points (Symlinks/Junctions)
//     and calls RemoveDirectory on the link itself, NEVER recursing into the target.
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
