/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package autoupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/packaging"
)

const (
	// teleportToolsVersionEnv is environment name for requesting specific version for update.
	teleportToolsVersionEnv = "TELEPORT_TOOLS_VERSION"
	// baseURL is CDN URL for downloading official Teleport packages.
	baseURL = "https://cdn.teleport.dev"
	// checksumHexLen is length of the hash sum.
	checksumHexLen = 64
	// reservedFreeDisk is the predefined amount of free disk space (in bytes) required
	// to remain available after downloading archives.
	reservedFreeDisk = 10 * 1024 * 1024 // 10 Mb
	// lockFileName is file used for locking update process in parallel.
	lockFileName = ".lock"
	// updatePackageSuffix is directory suffix used for package extraction in tools directory.
	updatePackageSuffix = "-update-pkg"
)

var (
	// // pattern is template for response on version command for client tools {tsh, tctl}.
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)

// ClientOption applies an option value for the ClientUpdater.
type ClientOption func(u *ClientUpdater)

// WithBaseURL defines custom base url for the updater.
func WithBaseURL(baseUrl string) ClientOption {
	return func(u *ClientUpdater) {
		u.baseUrl = baseUrl
	}
}

// WithClient defines custom http client for the ClientUpdater.
func WithClient(client *http.Client) ClientOption {
	return func(u *ClientUpdater) {
		u.client = client
	}
}

// ClientUpdater is updater implementation for the client tools auto updates.
type ClientUpdater struct {
	toolsDir     string
	localVersion string
	tools        []string

	baseUrl string
	client  *http.Client
}

// NewClientUpdater initiate updater for the client tools auto updates.
func NewClientUpdater(tools []string, toolsDir string, localVersion string, options ...ClientOption) *ClientUpdater {
	updater := &ClientUpdater{
		tools:        tools,
		toolsDir:     toolsDir,
		localVersion: localVersion,
		baseUrl:      baseURL,
		client:       http.DefaultClient,
	}
	for _, option := range options {
		option(updater)
	}

	return updater
}

// CheckLocal is run at client tool startup and will only perform local checks.
func (u *ClientUpdater) CheckLocal() (string, bool) {
	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv(teleportToolsVersionEnv)
	switch {
	// The user has turned off any form of automatic updates.
	case requestedVersion == "off":
		return "", false
	// Requested version already the same as client version.
	case u.localVersion == requestedVersion:
		return requestedVersion, false
	}

	// If a version of client tools has already been downloaded to
	// tools directory, return that.
	toolsVersion, err := checkClientToolVersion(u.toolsDir)
	if err != nil {
		return "", false
	}
	// The user has requested a specific version of client tools.
	if requestedVersion != "" && requestedVersion != toolsVersion {
		return requestedVersion, true
	}

	return toolsVersion, false
}

// CheckRemote will check against Proxy Service if client tools need to be
// updated.
func (u *ClientUpdater) CheckRemote(ctx context.Context, proxyAddr string) (string, bool, error) {
	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv(teleportToolsVersionEnv)
	switch {
	// The user has turned off any form of automatic updates.
	case requestedVersion == "off":
		return "", false, nil
	// Requested version already the same as client version.
	case u.localVersion == requestedVersion:
		return requestedVersion, false, nil
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return "", false, trace.Wrap(err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: proxyAddr,
		Pool:      certPool,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		return "", false, trace.Wrap(err)
	}

	// If a version of client tools has already been downloaded to
	// tools directory, return that.
	toolsVersion, err := checkClientToolVersion(u.toolsDir)
	if err != nil {
		return "", false, trace.Wrap(err)
	}

	switch {
	case requestedVersion != "" && requestedVersion != toolsVersion:
		return requestedVersion, true, nil
	case !resp.AutoUpdate.ToolsAutoUpdate || resp.AutoUpdate.ToolsVersion == "":
		return "", false, nil
	case u.localVersion == resp.AutoUpdate.ToolsVersion:
		return resp.AutoUpdate.ToolsVersion, false, nil
	case resp.AutoUpdate.ToolsVersion != toolsVersion:
		return resp.AutoUpdate.ToolsVersion, true, nil
	}

	return toolsVersion, false, nil
}

// UpdateWithLock acquires filesystem lock, downloads requested version package, unarchive and replace existing one.
func (u *ClientUpdater) UpdateWithLock(ctx context.Context, toolsVersion string) (err error) {
	// Create tools directory if it does not exist.
	if err := os.MkdirAll(u.toolsDir, 0o755); err != nil {
		return trace.Wrap(err)
	}
	// Lock concurrent client tools execution util requested version is updated.
	unlock, err := utils.FSWriteLock(filepath.Join(u.toolsDir, lockFileName))
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err = trace.NewAggregate(err, unlock())
	}()

	// If the version of the running binary or the version downloaded to
	// tools directory is the same as the requested version of client tools,
	// nothing to be done, exit early.
	teleportVersion, err := checkClientToolVersion(u.toolsDir)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)

	}
	if toolsVersion == u.localVersion || toolsVersion == teleportVersion {
		return nil
	}

	// Download and update client tools in tools directory.
	if err := u.Update(ctx, toolsVersion); err != nil {
		return trace.Wrap(err)
	}

	return
}

// Update downloads requested version and replace it with existing one and cleanups the previous downloads
// with defined updater directory suffix.
func (u *ClientUpdater) Update(ctx context.Context, toolsVersion string) error {
	// Get platform specific download URLs.
	archiveURL, hashURL, err := urls(u.baseUrl, toolsVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Download the archive and validate against the hash. Download to a
	// temporary path within tools directory.
	hash, err := u.downloadHash(signalCtx, hashURL)
	if err != nil {
		return trace.Wrap(err)
	}
	archivePath, archiveHash, err := u.downloadArchive(signalCtx, u.toolsDir, archiveURL)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := os.Remove(archivePath); err != nil {
			slog.WarnContext(ctx, "failed to remove archive", "error", err)
		}
	}()
	if archiveHash != hash {
		return trace.BadParameter("hash of archive does not match downloaded archive")
	}

	pkgName := fmt.Sprint(uuid.New().String(), updatePackageSuffix)
	extractDir := filepath.Join(u.toolsDir, pkgName)
	if runtime.GOOS != constants.DarwinOS {
		if err := os.Mkdir(extractDir, 0o755); err != nil {
			return trace.Wrap(err)
		}
	}

	// Perform atomic replace so concurrent exec do not fail.
	if err := packaging.ReplaceToolsBinaries(u.toolsDir, archivePath, extractDir, u.tools); err != nil {
		return trace.Wrap(err)
	}
	// Cleanup the tools directory with previously downloaded and un-archived versions.
	if err := packaging.RemoveWithSuffix(u.toolsDir, updatePackageSuffix, pkgName); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Exec re-executes tool command with same arguments and environ variables.
func (u *ClientUpdater) Exec() (int, error) {
	path, err := toolName(u.toolsDir)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	// To prevent re-execution loop we have to disable update logic for re-execution.
	env := append(os.Environ(), teleportToolsVersionEnv+"=off")

	if runtime.GOOS == constants.WindowsOS {
		cmd := exec.Command(path, os.Args[1:]...)
		cmd.Env = env
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 0, trace.Wrap(err)
		}

		return cmd.ProcessState.ExitCode(), nil
	}

	if err := syscall.Exec(path, append([]string{path}, os.Args[1:]...), env); err != nil {
		return 0, trace.Wrap(err)
	}

	return 0, nil
}

func (u *ClientUpdater) downloadHash(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("bad status when downloading archive hash: %v", resp.StatusCode)
	}

	var buf bytes.Buffer
	_, err = io.CopyN(&buf, resp.Body, checksumHexLen)
	if err != nil {
		return "", trace.Wrap(err)
	}
	raw := buf.String()
	if _, err = hex.DecodeString(raw); err != nil {
		return "", trace.Wrap(err)
	}
	return raw, nil
}

func (u *ClientUpdater) downloadArchive(ctx context.Context, downloadDir string, url string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", trace.BadParameter("bad status when downloading archive: %v", resp.StatusCode)
	}

	if resp.ContentLength != -1 {
		if err := checkFreeSpace(u.toolsDir, uint64(resp.ContentLength)); err != nil {
			return "", "", trace.Wrap(err)
		}
	}

	// Caller of this function will remove this file after the atomic swap has
	// occurred.
	f, err := os.CreateTemp(downloadDir, "tmp-")
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	h := sha256.New()
	pw := &progressWriter{n: 0, limit: resp.ContentLength}
	body := io.TeeReader(io.TeeReader(resp.Body, h), pw)

	// It is a little inefficient to download the file to disk and then re-load
	// it into memory to unarchive later, but this is safer as it allows client
	// tools to validate the hash before trying to operate on the archive.
	_, err = io.CopyN(f, body, resp.ContentLength)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	return f.Name(), fmt.Sprintf("%x", h.Sum(nil)), nil
}
