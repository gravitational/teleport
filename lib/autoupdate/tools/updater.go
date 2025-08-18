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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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
	// teleportToolsVersionEnvDisabled is a special value that disables teleport tools updates
	// when assigned to the teleportToolsVersionEnv environment variable.
	teleportToolsVersionEnvDisabled = "off"
	// teleportToolsVersionReExecEnv is internal environment name for transferring original
	// version to re-executed ones.
	teleportToolsVersionReExecEnv = "TELEPORT_TOOLS_VERSION_REEXEC"
	// teleportToolsDirsEnv overrides Teleport tools directory for saving updated
	// versions.
	teleportToolsDirsEnv = "TELEPORT_TOOLS_DIR"
	// reservedFreeDisk is the predefined amount of free disk space (in bytes) required
	// to remain available after downloading archives.
	reservedFreeDisk = 10 * 1024 * 1024 // 10 Mb
	// updatePackageSuffix is directory suffix used for package extraction in tools directory for v1.
	updatePackageSuffix = "-update-pkg"
	// updatePackageSuffix is directory suffix used for package extraction in tools directory for v2.
	updatePackageSuffixV2 = "-update-pkg-v2"
)

var (
	// pattern is template for response on version command for client tools {tsh, tctl}.
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)

// UpdateResponse contains information about after update process.
type UpdateResponse struct {
	Version  string `json:"version,omitempty"`
	ReExec   bool   `json:"reExec,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

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
func (u *Updater) CheckLocal(ctx context.Context, profileName string) (resp *UpdateResponse, err error) {
	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv(teleportToolsVersionEnv)
	switch requestedVersion {
	// The user has turned off any form of automatic updates.
	case teleportToolsVersionEnvDisabled:
		return &UpdateResponse{Version: "", ReExec: false}, nil
	// Requested version already the same as client version.
	case u.localVersion:
		return &UpdateResponse{Version: u.localVersion, ReExec: false}, nil
	// No requested version, we continue.
	case "":
	// Requested version that is not the local one.
	default:
		if _, err := semver.NewVersion(requestedVersion); err != nil {
			return nil, trace.Wrap(err, "checking that request version is semantic")
		}
		return &UpdateResponse{Version: requestedVersion, ReExec: true}, nil
	}

	// We should acquire and release the lock before checking the version
	// by executing the binary, as it might block tool execution until the version
	// check is completed, which can take several seconds.
	ctc, err := getToolsConfig(u.toolsDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config, ok := ctc.Configs[profileName]; ok {
		if config.Disabled || config.Version == u.localVersion {
			return &UpdateResponse{Version: config.Version, ReExec: false}, nil
		} else {
			return &UpdateResponse{Version: config.Version, ReExec: true}, nil
		}
	}

	// Backward compatibility check. If a version of the client tools has already been downloaded
	// to the tools directory, return it. Version check failures should be ignored, as EDR software
	// might block execution or a broken version may already exist in the tools' directory.
	toolsVersion, err := CheckExecutedToolVersion(u.toolsDir)
	if trace.IsNotFound(err) || errors.Is(err, ErrVersionCheck) || toolsVersion == u.localVersion {
		return &UpdateResponse{Version: u.localVersion, ReExec: false}, nil
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !ctc.HasVersion(toolsVersion) {
		if err := migrateV1AndUpdateConfig(u.toolsDir, u.tools); err != nil {
			// Execution should not be interrupted if migration fails. Instead, it's better to
			// re-download the version that was supposed to be migrated but failed for some reason.
			slog.WarnContext(ctx, "Failed to migrate client tools", "error", err)
		}
	}

	return &UpdateResponse{Version: toolsVersion, ReExec: false}, nil
}

// CheckRemote first checks the version set by the environment variable. If not set or disabled,
// it checks against the Proxy Service to determine if client tools need updating by requesting
// the `webapi/find` handler, which stores information about the required client tools version to
// operate with this cluster. It returns the semantic version that needs updating and whether
// re-execution is necessary, by re-execution flag we understand that update and re-execute is required.
func (u *Updater) CheckRemote(ctx context.Context, proxyAddr string, insecure bool) (response *UpdateResponse, err error) {
	proxyHost := utils.TryHost(proxyAddr)
	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv(teleportToolsVersionEnv)
	switch requestedVersion {
	// The user has turned off any form of automatic updates.
	case teleportToolsVersionEnvDisabled:
		return &UpdateResponse{Version: "", ReExec: false}, nil
	// Requested version already the same as client version.
	case u.localVersion:
		if err := updateToolsConfig(u.toolsDir, func(ctc *ClientToolsConfig) error {
			ctc.SetConfig(proxyHost, requestedVersion, false)
			return nil
		}); err != nil {
			return nil, trace.Wrap(err)
		}
		return &UpdateResponse{Version: u.localVersion, ReExec: false}, nil
	// No requested version, we continue.
	case "":
	// Requested version that is not the local one.
	default:
		if _, err := semver.NewVersion(requestedVersion); err != nil {
			return nil, trace.Wrap(err, "checking that request version is semantic")
		}
		// If the environment variable is set during a remote check,
		// prioritize this version for the current host and use it as the default
		// for all commands under the current profile.
		if err := updateToolsConfig(u.toolsDir, func(ctc *ClientToolsConfig) error {
			ctc.SetConfig(proxyHost, requestedVersion, false)
			return nil
		}); err != nil {
			return nil, trace.Wrap(err)
		}
		return &UpdateResponse{Version: requestedVersion, ReExec: true}, nil
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: proxyAddr,
		Pool:      certPool,
		Timeout:   10 * time.Second,
		Insecure:  insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateResp := &UpdateResponse{Version: u.localVersion, ReExec: false}

	switch {
	case !resp.AutoUpdate.ToolsAutoUpdate || resp.AutoUpdate.ToolsVersion == "":
		updateResp = &UpdateResponse{Version: u.localVersion, ReExec: false, Disabled: true}
	case u.localVersion == resp.AutoUpdate.ToolsVersion:
		updateResp = &UpdateResponse{Version: u.localVersion, ReExec: false}
	default:
		updateResp = &UpdateResponse{Version: resp.AutoUpdate.ToolsVersion, ReExec: true}
	}

	if err := updateToolsConfig(u.toolsDir, func(ctc *ClientToolsConfig) error {
		ctc.SetConfig(proxyHost, updateResp.Version, updateResp.Disabled)
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return updateResp, nil
}

// Update acquires filesystem lock, downloads requested version package, unarchive, replace
// existing one and cleanups the previous downloads with defined updater directory suffix.
func (u *Updater) Update(ctx context.Context, toolsVersion string) error {
	err := updateToolsConfig(u.toolsDir, func(ctc *ClientToolsConfig) error {
		// ignoreTools is the list of tools installed and tracked by the config.
		// They should be preserved during cleanup. If we have more than [defaultSizeStoredVersion]
		// versions, the updater will forget about the least used version.
		var ignoreTools []string
		for _, tool := range ctc.Tools {
			// If the version of the running binary or the version downloaded to
			// tools directory is the same as the requested version of client tools,
			// nothing to be done, exit early.
			if tool.Version == toolsVersion {
				return nil
			}
			ignoreTools = append(ignoreTools, tool.PackageNames()...)
		}

		// Get platform specific download URLs.
		packages, err := teleportPackageURLs(ctx, u.uriTemplate, u.baseURL, toolsVersion)
		if err != nil {
			return trace.Wrap(err)
		}

		var pkgNames []string
		for _, pkg := range packages {
			pkgName := fmt.Sprint(uuid.New().String(), updatePackageSuffixV2)
			if err := u.update(ctx, ctc, pkg, pkgName); err != nil {
				return trace.Wrap(err)
			}
			pkgNames = append(pkgNames, pkgName)
		}
		// Cleanup all tools in directory with the specific prefix by ignoring tools
		// that are currently recorded in the configuration.
		if err := packaging.RemoveWithSuffix(u.toolsDir, updatePackageSuffixV2, append(ignoreTools, pkgNames...)); err != nil {
			slog.WarnContext(ctx, "failed to clean up tools directory", "error", err)
		}

		return nil
	})

	return trace.Wrap(err)
}

// update downloads the archive and validate against the hash. Download to a
// temporary path within tools directory.
func (u *Updater) update(ctx context.Context, ctc *ClientToolsConfig, pkg packageURL, pkgName string) error {
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
	toolsMap, err := packaging.ReplaceToolsBinaries(f.Name(), extractDir, u.tools)
	if err != nil {
		return trace.Wrap(err)
	}

	for key, val := range toolsMap {
		toolsMap[key] = filepath.Join(pkgName, val)
	}
	ctc.AddTool(Tool{Version: pkg.Version, PathMap: toolsMap})

	return nil
}

// ToolPath loads full path from config file to specific tool and version.
func (u *Updater) ToolPath(toolName, toolVersion string) (path string, err error) {
	var tool *Tool
	if err := updateToolsConfig(u.toolsDir, func(ctc *ClientToolsConfig) error {
		tool = ctc.SelectVersion(toolVersion)
		return nil
	}); err != nil {
		return "", trace.Wrap(err)
	}
	if tool == nil {
		return "", trace.NotFound("tool version %q not found", toolVersion)
	}
	relPath, ok := tool.PathMap[toolName]
	if !ok {
		return "", trace.NotFound("tool %q not found", toolName)
	}

	return filepath.Join(u.toolsDir, relPath), nil
}

// Exec re-executes tool command with same arguments and environ variables.
func (u *Updater) Exec(ctx context.Context, toolsVersion string, args []string) (int, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	path, err := u.ToolPath(filepath.Base(executablePath), toolsVersion)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	for _, unset := range []string{
		teleportToolsVersionReExecEnv,
		teleportToolsDirsEnv,
	} {
		if err := os.Unsetenv(unset); err != nil {
			return 0, trace.Wrap(err)
		}
	}
	env := append(os.Environ(), fmt.Sprintf("%s=%s", teleportToolsDirsEnv, u.toolsDir))
	// To prevent re-execution loop we have to disable update logic for re-execution,
	// by unsetting current tools version env variable and setting it to "off".
	// The re-execution path and tools directory are absolute. Since the v2 logic
	// no longer uses a static path, any re-execution from the tools directory
	// must disable further re-execution.
	if path == executablePath || strings.HasPrefix(path, u.toolsDir) {
		if err := os.Unsetenv(teleportToolsVersionEnv); err != nil {
			return 0, trace.Wrap(err)
		}
		env = append(env, teleportToolsVersionEnv+"="+teleportToolsVersionEnvDisabled)
		slog.DebugContext(ctx, "Disable next re-execution")
	}
	env = append(env, fmt.Sprintf("%s=%s", teleportToolsVersionReExecEnv, u.localVersion))

	slog.DebugContext(ctx, "Re-execute updated version", "execute", path, "from", executablePath)
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
	pw, finish := newProgressWriter(10)
	defer finish()

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
