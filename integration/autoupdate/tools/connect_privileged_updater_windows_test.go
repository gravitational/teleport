/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tools_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/teleterm/autoupdate/privilegedupdater"
)

func TestPrivilegedUpdateServiceSuccess(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}
	err := runPrivilegedUpdaterFlow(t, up)
	require.NoError(t, err)
}

func TestPrivilegedUpdateServiceRejectsDowngrade(t *testing.T) {
	up := update{
		// The version is a downgrade compared to the current api.Version.
		version: "0.0.1",
		binary:  []byte("payload"),
	}
	err := runPrivilegedUpdaterFlow(t, up)
	require.ErrorIs(t, err, trace.BadParameter("update version 0.0.1 is not newer than current version %s", teleport.SemVer()))
}

func TestPrivilegedUpdateServiceRejectsChecksumMismatch(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	otherHash := sha256.Sum256([]byte("different-payload"))
	err := runPrivilegedUpdaterFlow(t, up, withChecksumServerResponseWriter(func(w http.ResponseWriter) {
		_, err := w.Write([]byte(hex.EncodeToString(otherHash[:])))
		require.NoError(t, err)
	}))
	require.ErrorIs(t, err, trace.BadParameter("hash of the update does not match downloaded checksum"))
}

func TestPrivilegedUpdateServiceRejectsInvalidVersionFormat(t *testing.T) {
	up := update{
		version: "not-a-semver",
		binary:  []byte("payload"),
	}
	err := runPrivilegedUpdaterFlow(t, up)
	require.Error(t, err)
	require.Contains(t, err.Error(), `invalid update version "not-a-semver"`)
}

func TestPrivilegedUpdateServiceRejectsChecksumRequestFailure(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	err := runPrivilegedUpdaterFlow(t, up, withChecksumServerResponseWriter(func(w http.ResponseWriter) {
		http.Error(w, "failure", http.StatusInternalServerError)
	}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "downloading update checksum")
}

func TestPrivilegedUpdateServicePolicyOffRejectsUpdate(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}
	err := runPrivilegedUpdaterFlow(t, up, withServiceTestPolicyToolsVersion("off"))
	require.Error(t, err)
	require.ErrorIs(t, err, trace.AccessDenied(`ToolsVersion in HKLM\SOFTWARE\Policies\Teleport\TeleportConnect is "off", automatic updates are disabled by system policy`))
}

func TestPrivilegedUpdateServicePolicyVersionMismatch(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}
	err := runPrivilegedUpdaterFlow(t, up, withServiceTestPolicyToolsVersion("999.0.1"))
	require.ErrorIs(t, err, trace.BadParameter("update version 999.0.0 does not match policy version 999.0.1"))
}

func TestPrivilegedUpdateServiceRejectsMalformedMetadata(t *testing.T) {
	cfg := getDefaultConfig(t)

	serviceErr := make(chan error, 1)
	go func() {
		serviceErr <- privilegedupdater.RunServiceTest(t.Context(), cfg)
	}()

	conn := dialUpdaterPipe(t, 5*time.Second)
	defer conn.Close()

	// Send malformed JSON metadata.
	malformedMetadata := []byte("{")
	require.NoError(t, binary.Write(conn, binary.LittleEndian, uint32(len(malformedMetadata))))
	n, err := conn.Write(malformedMetadata)
	require.NoError(t, err)
	require.Len(t, malformedMetadata, n)
	require.NoError(t, conn.Close())

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	select {
	case err := <-serviceErr:
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal update metadata")
	case <-ctx.Done():
		t.Fatal("timed out")
	}
}

func TestPrivilegedUpdateServiceRejectsNonExistingSystemTempDir(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	baseDir := filepath.Join(t.TempDir(), "does-not-exist")

	err := runPrivilegedUpdaterFlow(t, up, withServiceTestSystemTempDir(baseDir))
	require.ErrorIs(t, err, trace.BadParameter("the updater requires %s to be available, ensure your system is up-to-date", baseDir))
}

func TestPrivilegedUpdateServiceRejectsInvalidSystemTempDir(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	baseDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(baseDir, []byte("x"), 0o600))

	err := runPrivilegedUpdaterFlow(t, up, withServiceTestSystemTempDir(baseDir))
	require.ErrorIs(t, err, trace.BadParameter("security violation: %s exists but is not a directory", baseDir))
}

func TestPrivilegedUpdateServiceRejectsSystemTempDirReparsePoint(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	targetDir := t.TempDir()
	baseDir := filepath.Join(t.TempDir(), "junction-base")
	createJunction(t, baseDir, targetDir)

	err := runPrivilegedUpdaterFlow(t, up, withServiceTestSystemTempDir(baseDir))
	require.ErrorIs(t, err, trace.BadParameter("security violation: %s is a reparse point", baseDir))
}

func TestPrivilegedUpdateServiceSafelyCleanupOldUpdates(t *testing.T) {
	updateBaseDir := t.TempDir()
	updateRoot := filepath.Join(updateBaseDir, "TeleportConnectUpdater")
	require.NoError(t, os.MkdirAll(updateRoot, 0o700))

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "must-stay.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0o600))

	staleDir := filepath.Join(updateRoot, "stale-update")
	require.NoError(t, os.MkdirAll(staleDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(staleDir, "update.exe"), []byte("stale"), 0o600))

	junctionPath := filepath.Join(updateRoot, "outside-junction")
	createJunction(t, junctionPath, outsideDir)

	updateBinary := []byte("payload")
	up := update{
		version: "999.0.0",
		binary:  updateBinary,
	}
	err := runPrivilegedUpdaterFlow(t, up, withServiceTestSystemTempDir(updateBaseDir))
	require.NoError(t, err)

	_, err = os.Stat(staleDir)
	require.ErrorIs(t, err, os.ErrNotExist, "stale update directory should be removed")

	_, err = os.Lstat(junctionPath)
	require.ErrorIs(t, err, os.ErrNotExist, "junction entry should be removed")

	_, err = os.Stat(outsideFile)
	require.NoError(t, err, "cleanup must not remove files outside base dir via junction traversal")
}

func TestPrivilegedUpdateServiceRemovesLegacyUpdateRoot(t *testing.T) {
	programData := t.TempDir()
	legacyRoot := filepath.Join(programData, "TeleportConnectUpdater")
	staleDir := filepath.Join(legacyRoot, "stale")
	require.NoError(t, os.MkdirAll(staleDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(staleDir, "update.exe"), []byte("stale"), 0o600))

	// Plant a junction inside the legacy root pointing outside it. Cleanup must
	// remove the junction entry without recursing into and deleting the target.
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "must-stay.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0o600))
	junctionPath := filepath.Join(legacyRoot, "outside-junction")
	createJunction(t, junctionPath, outsideDir)

	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}
	err := runPrivilegedUpdaterFlow(t, up, withServiceTestProgramDataDir(programData))
	require.NoError(t, err)

	require.NoDirExists(t, legacyRoot, "legacy update root should be removed during the update flow")
	require.DirExists(t, programData, "ProgramData root itself must be left untouched")

	_, err = os.Lstat(junctionPath)
	require.ErrorIs(t, err, os.ErrNotExist, "junction entry should be removed")

	require.FileExists(t, outsideFile, "cleanup must not remove files outside the legacy root via junction traversal")
}

// TestPrivilegedUpdateServiceLegacyUpdateRootIsJunction verifies that when the
// legacy root itself is a junction, cleanup unlinks it without recursing into
// and deleting the target.
func TestPrivilegedUpdateServiceLegacyUpdateRootIsJunction(t *testing.T) {
	programData := t.TempDir()
	legacyRoot := filepath.Join(programData, "TeleportConnectUpdater")

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "must-stay.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0o600))
	createJunction(t, legacyRoot, outsideDir)

	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}
	err := runPrivilegedUpdaterFlow(t, up, withServiceTestProgramDataDir(programData))
	require.NoError(t, err)

	_, err = os.Lstat(legacyRoot)
	require.ErrorIs(t, err, os.ErrNotExist, "legacy update root junction should be unlinked")

	require.FileExists(t, outsideFile, "cleanup must not remove the junction target")
}

func TestPrivilegedUpdateServiceAllowOnlyOneClientConnection(t *testing.T) {
	serviceErr := make(chan error, 1)
	cfg := getDefaultConfig(t)

	go func() {
		serviceErr <- privilegedupdater.RunServiceTest(t.Context(), cfg)
	}()

	// First client connects and keeps the pipe open. This blocks the service in readUpdate.
	firstConn := dialUpdaterPipe(t, 2*time.Second)

	// Second client should fail because waitForSingleClient closes the listener after first accept.
	clientCtx2, cancel2 := context.WithTimeout(t.Context(), 2*time.Second)
	t.Cleanup(cancel2)
	secondConn, err := winio.DialPipeAccess(clientCtx2, privilegedupdater.PipePath, privilegedupdater.SafePipeReadWriteAccess)
	if secondConn != nil {
		_ = secondConn.Close()
	}
	require.Error(t, err, "second client unexpectedly connected")

	// Let the service exit cleanly from the blocked read path.
	require.NoError(t, firstConn.Close())
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	select {
	case err := <-serviceErr:
		require.Error(t, err)
	case <-ctx.Done():
		t.Fatal("timed out")
	}
}

func TestPrivilegedUpdateServiceRejectsUnsignedUpdate(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	// Keep signature verification enabled for this test to ensure unsigned
	// updates are rejected.
	err := runPrivilegedUpdaterFlow(t, up, withServiceTestVerifySignature(nil))
	require.Error(t, err)
	require.ErrorContains(t, err, "verifying update signature")
}

type serviceConfig struct {
	privilegedupdater.ServiceTestConfig
	checksumServerResponseWriter func(http.ResponseWriter)
}

type privilegedServiceMainConfigOption func(*serviceConfig)

func withServiceTestSystemTempDir(path string) privilegedServiceMainConfigOption {
	return func(cfg *serviceConfig) {
		cfg.SystemTempDir = path
	}
}

func withServiceTestProgramDataDir(path string) privilegedServiceMainConfigOption {
	return func(cfg *serviceConfig) {
		cfg.ProgramDataDir = path
	}
}

func withChecksumServerResponseWriter(checksumResponseWriter func(w http.ResponseWriter)) privilegedServiceMainConfigOption {
	return func(cfg *serviceConfig) {
		cfg.checksumServerResponseWriter = checksumResponseWriter
	}
}

func withServiceTestPolicyToolsVersion(version string) privilegedServiceMainConfigOption {
	return func(cfg *serviceConfig) {
		cfg.PolicyToolsVersion = version
	}
}

func withServiceTestVerifySignature(fn func(updatePath string) error) privilegedServiceMainConfigOption {
	return func(cfg *serviceConfig) {
		cfg.VerifySignature = fn
	}
}

type update struct {
	version string
	binary  []byte
}

// runPrivilegedUpdaterFlow runs the service implementation and sends the update via the named pipe.
func runPrivilegedUpdaterFlow(t *testing.T, update update, opts ...privilegedServiceMainConfigOption) error {
	t.Helper()

	defaultCfg := getDefaultConfig(t)
	cfg := &serviceConfig{
		ServiceTestConfig: *defaultCfg,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	checksumPath := "/Teleport Connect Setup-" + update.version + ".exe.sha256"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != checksumPath {
			http.NotFound(w, r)
			return
		}
		if cfg.checksumServerResponseWriter != nil {
			cfg.checksumServerResponseWriter(w)
		} else {
			hash := sha256.Sum256(update.binary)
			// By default, return a checksum for the passed file.
			_, _ = w.Write([]byte(hex.EncodeToString(hash[:])))
		}
	}))
	t.Cleanup(server.Close)

	payloadPath := filepath.Join(t.TempDir(), "client-update.exe")
	require.NoError(t, os.WriteFile(payloadPath, update.binary, 0o600))

	serviceErr := make(chan error, 1)
	installUpdateFromClientErr := make(chan error, 1)
	go func() {
		err := privilegedupdater.RunServiceTest(t.Context(), &privilegedupdater.ServiceTestConfig{
			SystemTempDir:                cfg.SystemTempDir,
			ProgramDataDir:               cfg.ProgramDataDir,
			PolicyToolsVersion:           cfg.PolicyToolsVersion,
			PolicyCDNBaseURL:             server.URL,
			HTTPClient:                   server.Client(),
			PipeAuthenticatedUsersAccess: cfg.PipeAuthenticatedUsersAccess,
			VerifySignature:              cfg.VerifySignature,
		})
		// We are attempting to run a non-exe file.
		// It will fail, so we check if we ran the correct file.
		// The pattern should match: <base-update-dir>\<guid>\update.exe.
		// In the production code, base-update-dir is %WINDIR%\SystemTemp\TeleportConnectUpdater.
		if err != nil && strings.Contains(err.Error(), "running installer") {
			pattern := fmt.Sprintf(
				`.*starting installer path=%s\\TeleportConnectUpdater\\[0-9a-fA-F-]{36}\\update\.exe`,
				regexp.QuoteMeta(cfg.SystemTempDir),
			)
			require.Regexp(t, pattern, err.Error())
			require.Contains(t, err.Error(), "args=\"--updated /S /allusers\"")
			serviceErr <- nil
			return
		}
		serviceErr <- err
	}()
	go func() {
		installUpdateFromClientErr <- privilegedupdater.InstallUpdateFromClient(t.Context(), payloadPath, false, update.version)
	}()

	for i := 0; i < 2; i++ {
		select {
		case err := <-serviceErr:
			return err
		case err := <-installUpdateFromClientErr:
			if err != nil {
				return err
			}
		case <-t.Context().Done():
			t.Fatal("timed out")
			return nil
		}
	}
	return nil
}

func dialUpdaterPipe(t *testing.T, timeout time.Duration) net.Conn {
	t.Helper()

	var conn net.Conn
	err := retryutils.RetryStaticFor(timeout, 25*time.Millisecond, func() error {
		c, err := winio.DialPipeAccess(t.Context(), privilegedupdater.PipePath, privilegedupdater.SafePipeReadWriteAccess)
		if err != nil {
			return err
		}
		conn = c
		return nil
	})
	require.NoError(t, err, "failed to connect to updater pipe before timeout")
	return conn
}

// getDefaultConfig returns a base service-test config with a per-test staging dir.
func getDefaultConfig(t *testing.T) *privilegedupdater.ServiceTestConfig {
	t.Helper()

	return &privilegedupdater.ServiceTestConfig{
		SystemTempDir:  t.TempDir(),
		ProgramDataDir: t.TempDir(),
		// Allow Authenticated Users to create the pipe in tests.
		PipeAuthenticatedUsersAccess: windows.GENERIC_READ | windows.GENERIC_WRITE,
		// Integration test updates are unsigned.
		VerifySignature: func(string) error { return nil },
	}
}

func createJunction(t *testing.T, linkPath, targetPath string) {
	t.Helper()

	cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetPath)
	_, err := cmd.CombinedOutput()
	require.NoError(t, err)
}
