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
	"github.com/gravitational/teleport/lib/teleterm/autoupdate"
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
	require.ErrorIs(t, err, trace.BadParameter(`ToolsVersion in HKLM\SOFTWARE\Policies\Teleport\TeleportConnect is "off", the update will not be installed`))
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
		serviceErr <- autoupdate.PrivilegedServiceMainTest(t.Context(), cfg)
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

func TestPrivilegedUpdateServiceRejectsUpdateBaseDirFile(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	baseDir := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(baseDir, []byte("x"), 0o600))

	err := runPrivilegedUpdaterFlow(t, up, withServiceTestUpdateBaseDir(baseDir))
	require.ErrorIs(t, err, trace.BadParameter("security violation: %s exists but is not a directory", baseDir))
}

func TestPrivilegedUpdateServiceRejectsUpdateBaseDirReparsePoint(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	targetDir := t.TempDir()
	baseDir := filepath.Join(t.TempDir(), "junction-base")
	createJunction(t, baseDir, targetDir)

	err := runPrivilegedUpdaterFlow(t, up, withServiceTestUpdateBaseDir(baseDir))
	require.ErrorIs(t, err, trace.BadParameter("security violation: %s is a reparse point", baseDir))
}

func TestPrivilegedUpdateServiceSafelyCleanupOldUpdates(t *testing.T) {
	updateBaseDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "must-stay.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0o600))

	staleDir := filepath.Join(updateBaseDir, "stale-update")
	require.NoError(t, os.MkdirAll(staleDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(staleDir, "update.exe"), []byte("stale"), 0o600))

	junctionPath := filepath.Join(updateBaseDir, "outside-junction")
	createJunction(t, junctionPath, outsideDir)

	updateBinary := []byte("payload")
	up := update{
		version: "999.0.0",
		binary:  updateBinary,
	}
	err := runPrivilegedUpdaterFlow(t, up, withServiceTestUpdateBaseDir(updateBaseDir))
	require.NoError(t, err)

	_, err = os.Stat(staleDir)
	require.ErrorIs(t, err, os.ErrNotExist, "stale update directory should be removed")

	_, err = os.Lstat(junctionPath)
	require.ErrorIs(t, err, os.ErrNotExist, "junction entry should be removed")

	_, err = os.Stat(outsideFile)
	require.NoError(t, err, "cleanup must not remove files outside base dir via junction traversal")
}

func TestPrivilegedUpdateServiceCorrectsUpdateBaseDirACL(t *testing.T) {
	up := update{
		version: "999.0.0",
		binary:  []byte("payload"),
	}

	defaultConfig := getDefaultConfig(t)
	baseDir := filepath.Join(t.TempDir(), "new-dir")
	require.NoError(t, os.MkdirAll(baseDir, 0o777))
	// Everyone has Full Control over this object,
	// and the permission is inherited by all subfolders and files.
	// This access will be corrected by the service.
	setDirectoryDACL(t, baseDir, "D:(A;OICI;GA;;;WD)")

	err := runPrivilegedUpdaterFlow(t, up, withServiceTestUpdateBaseDir(baseDir))
	require.NoError(t, err)

	assertDirectorySecurityDescriptor(t, baseDir, defaultConfig.UpdateDirSecurityDescriptor)
}

func TestPrivilegedUpdateServiceAllowOnlyOneClientConnection(t *testing.T) {
	serviceErr := make(chan error, 1)
	go func() {
		serviceErr <- autoupdate.PrivilegedServiceMainTest(t.Context(), &autoupdate.PrivilegedServiceTestConfig{})
	}()

	// First client connects and keeps the pipe open. This blocks the service in readUpdate.
	firstConn := dialUpdaterPipe(t, 2*time.Second)

	// Second client should fail because waitForSingleClient closes the listener after first accept.
	clientCtx2, cancel2 := context.WithTimeout(t.Context(), 2*time.Second)
	t.Cleanup(cancel2)
	secondConn, err := winio.DialPipeContext(clientCtx2, autoupdate.UpdaterPipePath)
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

type serviceConfig struct {
	autoupdate.PrivilegedServiceTestConfig
	checksumServerResponseWriter func(http.ResponseWriter)
}

type privilegedServiceMainConfigOption func(*serviceConfig)

func withServiceTestUpdateBaseDir(path string) privilegedServiceMainConfigOption {
	return func(cfg *serviceConfig) {
		cfg.UpdateBaseDir = path
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

type update struct {
	version string
	binary  []byte
}

// runPrivilegedUpdaterFlow runs the service implementation and sends the update via the named pipe.
func runPrivilegedUpdaterFlow(t *testing.T, update update, opts ...privilegedServiceMainConfigOption) error {
	t.Helper()

	defaultCfg := getDefaultConfig(t)
	cfg := &serviceConfig{
		PrivilegedServiceTestConfig: autoupdate.PrivilegedServiceTestConfig{
			UpdateDirSecurityDescriptor: defaultCfg.UpdateDirSecurityDescriptor,
			UpdateBaseDir:               defaultCfg.UpdateBaseDir,
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	checksumPath := "/Teleport Connect Setup-" + update.version + ".exe.sha256"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		err := autoupdate.PrivilegedServiceMainTest(t.Context(), &autoupdate.PrivilegedServiceTestConfig{
			UpdateDirSecurityDescriptor: cfg.UpdateDirSecurityDescriptor,
			UpdateBaseDir:               cfg.UpdateBaseDir,
			PolicyToolsVersion:          cfg.PolicyToolsVersion,
			PolicyCDNBaseURL:            server.URL,
		})
		// We are attempting to run a non-exe file.
		// It will fail, so we check if we ran the correct file.
		// The pattern should match: <base-update-dir>\<guid>\update.exe.
		// In the production code, base-update-dir is %ProgramData%\TeleportConnectUpdater.
		if err != nil && strings.Contains(err.Error(), "running installer") {
			pattern := fmt.Sprintf(
				`.*starting installer path=%s\\[0-9a-fA-F-]{36}\\update\.exe`,
				regexp.QuoteMeta(cfg.UpdateBaseDir),
			)
			require.Regexp(t, pattern, err.Error())
			require.Contains(t, err.Error(), "args=\"--updated /S /allusers\"")
			serviceErr <- nil
			return
		}
		serviceErr <- err
	}()
	go func() {
		installUpdateFromClientErr <- autoupdate.InstallUpdateFromClient(t.Context(), payloadPath, false, update.version)
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
		c, err := winio.DialPipeContext(t.Context(), autoupdate.UpdaterPipePath)
		if err != nil {
			return err
		}
		conn = c
		return nil
	})
	require.NoError(t, err, "failed to connect to updater pipe before timeout")
	return conn
}

// getDefaultConfig returns a base dir and a security descriptor.
func getDefaultConfig(t *testing.T) *autoupdate.PrivilegedServiceTestConfig {
	t.Helper()

	token := windows.GetCurrentProcessToken()
	tokenUser, err := token.GetTokenUser()
	require.NoError(t, err)
	require.NotNil(t, tokenUser.User.Sid)

	ownerSID := tokenUser.User.Sid.String()

	// We can't use the production security descriptor as it requires the process to run with elevated privileges.
	// Here we create a descriptor that restrict a bit the regular rights for authenticated users.
	descriptor := "O:" + ownerSID +
		"D:P" +
		"(A;;FA;;;SY)" +
		"(A;;FA;;;BA)" +
		"(A;OICI;0x1301bf;;;AU)" // 0x1301bf - modify rights for AU (authenticated users) for dir and sub dirs (OICI)

	return &autoupdate.PrivilegedServiceTestConfig{
		UpdateDirSecurityDescriptor: descriptor,
		UpdateBaseDir:               t.TempDir(),
	}
}

func createJunction(t *testing.T, linkPath, targetPath string) {
	t.Helper()

	cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetPath)
	_, err := cmd.CombinedOutput()
	require.NoError(t, err)
}

func assertDirectorySecurityDescriptor(t *testing.T, path string, expectedDescriptor string) {
	t.Helper()

	actualSD, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION)
	require.NoError(t, err)

	expectedSD, err := windows.SecurityDescriptorFromString(expectedDescriptor)
	require.NoError(t, err)

	// Comparing ACLs is non-trivial.
	//
	// In SDDL, "D:" starts the DACL section.
	// "D:P" means the DACL is protected (no inheritance).
	// After ACL changes, Windows may apply "D:PAI", where "AI" indicates
	// auto-inherited ACEs. The descriptors are functionally equivalent
	// for our purposes, so normalize before comparison.
	expectedSDString := strings.Replace(expectedSD.String(), "D:P", "D:PAI", 1)
	require.Equal(t, expectedSDString, actualSD.String(), "directory DACL does not match expected descriptor")
}

func setDirectoryDACL(t *testing.T, path string, descriptor string) {
	t.Helper()

	sd, err := windows.SecurityDescriptorFromString(descriptor)
	require.NoError(t, err)
	dacl, _, err := sd.DACL()
	require.NoError(t, err)

	err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
	require.NoError(t, err)
}
