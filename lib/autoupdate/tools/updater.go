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

package tools

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
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/packaging"
)

const (
	// teleportToolsVersionEnv is environment name for requesting specific version for update.
	teleportToolsVersionEnv = "TELEPORT_TOOLS_VERSION"
	// reservedFreeDisk is the predefined amount of free disk space (in bytes) required
	// to remain available after downloading archives.
	reservedFreeDisk = 10 * 1024 * 1024 // 10 Mb
	// lockFileName is file used for locking update process in parallel.
	lockFileName = ".lock"
	// updatePackageSuffix is directory suffix used for package extraction in tools directory.
	updatePackageSuffix = "-update-pkg"
)

var (
	// pattern is template for response on version command for client tools {tsh, tctl}.
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)

// Option applies an option value for the Updater.
type Option func(u *Updater)

// WithBaseURL defines custom base url for the updater.
func WithBaseURL(baseURL string) Option {
	return func(u *Updater) {
		u.baseURL = baseURL
	}
}

// WithURITemplate defines custom URI template for the updater.
func WithURITemplate(uriTemplate string) Option {
	return func(u *Updater) {
		u.uriTemplate = uriTemplate
	}
}

// WithClient defines custom http client for the Updater.
func WithClient(client *http.Client) Option {
	return func(u *Updater) {
		u.client = client
	}
}

// WithTools defines custom list of the tools has to be installed by updater.
func WithTools(tools []string) Option {
	return func(u *Updater) {
		u.tools = tools
	}
}

// Updater is updater implementation for the client tools auto updates.
type Updater struct {
	toolsDir     string
	localVersion string
	tools        []string
	uriTemplate  string
	baseURL      string

	client *http.Client
}

// NewUpdater initializes the updater for client tools auto updates. We need to specify the tools directory
// path where we download, extract package archives with the new version, and replace symlinks
// (e.g., `$TELEPORT_HOME/bin`).
// The base URL of the CDN with Teleport packages, the `http.Client` and  the list of tools (e.g., `tsh`, `tctl`)
// that must be updated can be customized via options.
func NewUpdater(toolsDir, localVersion string, options ...Option) *Updater {
	updater := &Updater{
		tools:        DefaultClientTools(),
		toolsDir:     toolsDir,
		localVersion: localVersion,
		uriTemplate:  autoupdate.DefaultCDNURITemplate,
		baseURL:      autoupdate.DefaultBaseURL,
		client:       http.DefaultClient,
	}
	for _, option := range options {
		option(updater)
	}

	return updater
}

// CheckLocal is run at client tool startup and will only perform local checks.
// Returns the version needs to be updated and re-executed, by re-execution flag we
// understand that update and re-execute is required.
func (u *Updater) CheckLocal() (version string, reExec bool, err error) {
	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv(teleportToolsVersionEnv)
	switch requestedVersion {
	// The user has turned off any form of automatic updates.
	case "off":
		return "", false, nil
	// Requested version already the same as client version.
	case u.localVersion:
		return u.localVersion, false, nil
	// No requested version, we continue.
	case "":
	// Requested version that is not the local one.
	default:
		if _, err := semver.NewVersion(requestedVersion); err != nil {
			return "", false, trace.Wrap(err, "checking that request version is semantic")
		}
		return requestedVersion, true, nil
	}

	// If a version of client tools has already been downloaded to
	// tools directory, return that.
	toolsVersion, err := CheckToolVersion(u.toolsDir)
	if trace.IsNotFound(err) || toolsVersion == u.localVersion {
		return u.localVersion, false, nil
	}
	if err != nil {
		return "", false, trace.Wrap(err)
	}

	return toolsVersion, true, nil
}

// CheckRemote first checks the version set by the environment variable. If not set or disabled,
// it checks against the Proxy Service to determine if client tools need updating by requesting
// the `webapi/find` handler, which stores information about the required client tools version to
// operate with this cluster. It returns the semantic version that needs updating and whether
// re-execution is necessary, by re-execution flag we understand that update and re-execute is required.
func (u *Updater) CheckRemote(ctx context.Context, proxyAddr string, insecure bool) (version string, reExec bool, err error) {
	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv(teleportToolsVersionEnv)
	switch requestedVersion {
	// The user has turned off any form of automatic updates.
	case "off":
		return "", false, nil
	// Requested version already the same as client version.
	case u.localVersion:
		return u.localVersion, false, nil
	// No requested version, we continue.
	case "":
	// Requested version that is not the local one.
	default:
		if _, err := semver.NewVersion(requestedVersion); err != nil {
			return "", false, trace.Wrap(err, "checking that request version is semantic")
		}
		return requestedVersion, true, nil
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return "", false, trace.Wrap(err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: proxyAddr,
		Pool:      certPool,
		Timeout:   10 * time.Second,
		Insecure:  insecure,
	})
	if err != nil {
		return "", false, trace.Wrap(err)
	}

	// If a version of client tools has already been downloaded to
	// tools directory, return that.
	toolsVersion, err := CheckToolVersion(u.toolsDir)
	if err != nil && !trace.IsNotFound(err) {
		return "", false, trace.Wrap(err)
	}

	switch {
	case !resp.AutoUpdate.ToolsAutoUpdate || resp.AutoUpdate.ToolsVersion == "":
		if toolsVersion == "" {
			return u.localVersion, false, nil
		}
	case u.localVersion == resp.AutoUpdate.ToolsVersion:
		return u.localVersion, false, nil
	case resp.AutoUpdate.ToolsVersion != toolsVersion:
		return resp.AutoUpdate.ToolsVersion, true, nil
	}

	return toolsVersion, true, nil
}

// UpdateWithLock acquires filesystem lock, downloads requested version package,
// unarchive and replace existing one.
func (u *Updater) UpdateWithLock(ctx context.Context, updateToolsVersion string) (err error) {
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
	toolsVersion, err := CheckToolVersion(u.toolsDir)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if updateToolsVersion == toolsVersion {
		return nil
	}

	// Download and update client tools in tools directory.
	if err := u.Update(ctx, updateToolsVersion); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Update downloads requested version and replace it with existing one and cleanups the previous downloads
// with defined updater directory suffix.
func (u *Updater) Update(ctx context.Context, toolsVersion string) error {
	// Get platform specific download URLs.
	packages, err := teleportPackageURLs(u.uriTemplate, u.baseURL, toolsVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	var pkgNames []string
	for _, pkg := range packages {
		pkgName := fmt.Sprint(uuid.New().String(), updatePackageSuffix)
		if err := u.update(ctx, pkg, pkgName); err != nil {
			return trace.Wrap(err)
		}
		pkgNames = append(pkgNames, pkgName)
	}

	// Cleanup the tools directory with previously downloaded and un-archived versions.
	if err := packaging.RemoveWithSuffix(u.toolsDir, updatePackageSuffix, pkgNames); err != nil {
		slog.WarnContext(ctx, "failed to clean up tools directory", "error", err)
	}

	return nil
}

// update downloads the archive and validate against the hash. Download to a
// temporary path within tools directory.
func (u *Updater) update(ctx context.Context, pkg packageURL, pkgName string) error {
	f, err := os.CreateTemp("", "teleport-")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		_ = f.Close()
		if err := os.Remove(f.Name()); err != nil {
			slog.WarnContext(ctx, "failed to remove temporary archive file", "error", err)
		}
	}()

	archiveHash, err := u.downloadArchive(ctx, pkg.Archive, f)
	if pkg.Optional && trace.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	hash, err := u.downloadHash(ctx, pkg.Hash)
	if pkg.Optional && trace.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	if !bytes.Equal(archiveHash, hash) {
		return trace.BadParameter("hash of archive does not match downloaded archive")
	}

	extractDir := filepath.Join(u.toolsDir, pkgName)
	if runtime.GOOS != constants.DarwinOS {
		if err := os.Mkdir(extractDir, 0o755); err != nil {
			return trace.Wrap(err)
		}
	}

	// Perform atomic replace so concurrent exec do not fail.
	if err := packaging.ReplaceToolsBinaries(u.toolsDir, f.Name(), extractDir, u.tools); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Exec re-executes tool command with same arguments and environ variables.
func (u *Updater) Exec(args []string) (int, error) {
	path, err := toolName(u.toolsDir)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	// To prevent re-execution loop we have to disable update logic for re-execution,
	// by unsetting current tools version env variable and setting it to "off".
	if err := os.Unsetenv(teleportToolsVersionEnv); err != nil {
		return 0, trace.Wrap(err)
	}

	env := os.Environ()
	executablePath, err := os.Executable()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if path == executablePath {
		env = append(env, teleportToolsVersionEnv+"=off")
	}

	if runtime.GOOS == constants.WindowsOS {
		cmd := exec.Command(path, args...)
		cmd.Env = env
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 0, trace.Wrap(err)
		}

		return cmd.ProcessState.ExitCode(), nil
	}

	if err := syscall.Exec(path, append([]string{path}, args...), env); err != nil {
		return 0, trace.Wrap(err)
	}

	return 0, nil
}

// downloadHash downloads the hash file `.sha256` for package checksum validation and return the hash sum.
func (u *Updater) downloadHash(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, trace.NotFound("hash file is not found: %q", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("bad status when downloading archive hash: %v", resp.StatusCode)
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

// downloadArchive downloads the archive package by `url` and writes content to the writer interface,
// return calculated sha256 hash sum of the content.
func (u *Updater) downloadArchive(ctx context.Context, url string, f io.Writer) ([]byte, error) {
	// Display a progress bar before initiating the update request to inform the user that
	// an update is in progress, allowing them the option to cancel before actual response
	// which might be delayed with slow internet connection or complete isolation to CDN.
	pw, finish := newProgressWriter(10)
	defer finish()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, trace.NotFound("archive file is not found: %v", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("bad status when downloading archive: %v", resp.StatusCode)
	}

	if resp.ContentLength != -1 {
		if err := checkFreeSpace(u.toolsDir, uint64(resp.ContentLength)); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	h := sha256.New()
	// It is a little inefficient to download the file to disk and then re-load
	// it into memory to unarchive later, but this is safer as it allows client
	// tools to validate the hash before trying to operate on the archive.
	if _, err := pw.CopyLimit(f, io.TeeReader(resp.Body, h), resp.ContentLength); err != nil {
		return nil, trace.Wrap(err)
	}

	return h.Sum(nil), nil
}
