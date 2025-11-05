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
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/packaging"
)

var (
	// ErrNoBaseURL is returned when `TELEPORT_CDN_BASE_URL` must be set
	// in order to proceed with managed updates.
	ErrNoBaseURL = errors.New("baseURL is not defined")
	// ErrVersionCheck is returned when the downloaded version fails
	// to execute for version identification.
	ErrVersionCheck = errors.New("version check failed")
)

// Dir returns the client tools installation directory path, using the following fallback order:
// $TELEPORT_TOOLS_DIR, $TELEPORT_HOME/bin, and $HOME/.tsh/bin.
func Dir() (string, error) {
	toolsDir := os.Getenv(teleportToolsDirsEnv)
	if toolsDir == "" {
		toolsDir = os.Getenv(types.HomeEnvVar)
		if toolsDir == "" {
			var err error
			toolsDir, err = os.UserHomeDir()
			if err != nil {
				return "", trace.Wrap(err)
			}
			toolsDir = filepath.Join(toolsDir, ".tsh", "bin")
		} else {
			toolsDir = filepath.Join(toolsDir, "bin")
		}
	}

	toolsDir, err := filepath.Abs(toolsDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return toolsDir, nil
}

// DefaultClientTools list of the client tools needs to be updated by default.
func DefaultClientTools() []string {
	switch runtime.GOOS {
	case constants.WindowsOS:
		return []string{"tsh.exe", "tctl.exe"}
	default:
		return []string{"tsh", "tctl"}
	}
}

// CheckExecutedToolVersion invokes the exact executable from the tools directory to retrieve its version.
func CheckExecutedToolVersion(toolsDir string) (string, error) {
	path, err := toolName(toolsDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return CheckToolVersion(path)
}

// CheckToolVersion returns client tools version, must return NotFoundError if
// the client tools is not found in specified path.
func CheckToolVersion(toolPath string) (string, error) {
	if _, err := os.Stat(toolPath); errors.Is(err, os.ErrNotExist) {
		return "", trace.NotFound("autoupdate tool not found in %q", toolPath)
	} else if err != nil {
		return "", trace.Wrap(err)
	}

	// Set a timeout to not let "{tsh, tctl} version" block forever. Allow up
	// to 10 seconds because sometimes MDM tools like Jamf cause a lot of
	// latency in launching binaries.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute "{tsh, tctl} version" and pass in TELEPORT_TOOLS_VERSION=off to
	// turn off all automatic updates code paths to prevent any recursion.
	command := exec.CommandContext(ctx, toolPath, "version")
	command.Env = []string{teleportToolsVersionEnv + "=" + teleportToolsVersionEnvDisabled}
	output, err := command.Output()
	if err != nil {
		slog.DebugContext(context.Background(), "failed to determine version",
			"tool", toolPath, "error", err, "output", string(output))
		return "", ErrVersionCheck
	}

	// The output for "{tsh, tctl} version" can be multiple lines. Find the
	// actual version line and extract the version.
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "Teleport") {
			continue
		}

		matches := pattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			return "", trace.BadParameter("invalid version line: %v", line)
		}
		version, err := semver.NewVersion(matches[1])
		if err != nil {
			return "", trace.Wrap(err)
		}
		return version.String(), nil
	}

	return "", trace.BadParameter("unable to determine version")
}

// GetReExecFromVersion returns the version if current execution binary is re-executed from
// another version.
func GetReExecFromVersion(ctx context.Context) string {
	reExecFromVersion := os.Getenv(teleportToolsVersionReExecEnv)
	if reExecFromVersion != "" {
		if _, err := semver.NewVersion(reExecFromVersion); err != nil {
			slog.WarnContext(ctx, "Failed to parse teleport 'TELEPORT_TOOLS_VERSION_REEXEC'", "error", err)
			return ""
		}
	}
	return reExecFromVersion
}

// GetReExecPath returns the execution path if current execution binary is re-executed from
// another version.
func GetReExecPath() string {
	return os.Getenv(teleportToolsPathReExecEnv)
}

// CleanUp cleans the tools directory with downloaded versions.
func CleanUp(toolsDir string, tools []string) error {
	var aggErr []error
	for _, tool := range tools {
		if err := os.Remove(filepath.Join(toolsDir, tool)); err != nil && !os.IsNotExist(err) {
			aggErr = append(aggErr, err)
		}
	}
	if err := os.Remove(filepath.Join(toolsDir, lockFileName)); err != nil && !os.IsNotExist(err) {
		aggErr = append(aggErr, err)
	}
	if err := os.Remove(filepath.Join(toolsDir, configFileName)); err != nil && !os.IsNotExist(err) {
		aggErr = append(aggErr, err)
	}
	if err := packaging.RemoveWithSuffix(toolsDir, updatePackageSuffix, nil); err != nil {
		aggErr = append(aggErr, err)
	}
	if err := packaging.RemoveWithSuffix(toolsDir, updatePackageSuffixV2, nil); err != nil {
		aggErr = append(aggErr, err)
	}

	return trace.NewAggregate(aggErr...)
}

// packageURL defines URLs to the archive and their archive sha256 hash file, and marks
// if this package is optional, for such case download needs to be ignored if package
// not found in CDN.
type packageURL struct {
	Version  string
	Archive  string
	Hash     string
	Optional bool
}

// teleportPackageURLs returns URLs for the Teleport archives to download.
func teleportPackageURLs(ctx context.Context, uriTmpl string, baseURL, requestedVersion string) ([]packageURL, error) {
	semVersion, err := version.EnsureSemver(requestedVersion)
	if err != nil {
		return nil, trace.BadParameter("version %q is not following semver", requestedVersion)
	}

	m := modules.GetModules()
	envBaseURL := os.Getenv(autoupdate.BaseURLEnvVar)
	if m.BuildType() == modules.BuildOSS && envBaseURL == "" {
		slog.WarnContext(ctx, "Client tools updates are disabled as they are licensed under AGPL. To use Community Edition builds or custom binaries, set the 'TELEPORT_CDN_BASE_URL' environment variable.")
		return nil, ErrNoBaseURL
	}

	var flags autoupdate.InstallFlags
	if m.IsBoringBinary() {
		flags |= autoupdate.FlagFIPS
	}
	if m.IsEnterpriseBuild() || m.IsBoringBinary() {
		flags |= autoupdate.FlagEnterprise
	}

	// TODO(vapopov): DELETE in v22.0.0 version check - the separate `teleport-tools` package
	// will be included in all supported versions.
	pkg := autoupdate.DefaultPackage
	if runtime.GOOS == constants.DarwinOS &&
		(semVersion.Major > 18 ||
			semVersion.Major == 18 && semVersion.Compare(semver.Version{Major: 18, Minor: 1, Patch: 5}) >= 0 ||
			semVersion.Major == 17 && semVersion.Compare(semver.Version{Major: 17, Minor: 7, Patch: 2}) >= 0) {
		pkg = autoupdate.DefaultToolsPackage
	}

	teleportURL, err := autoupdate.MakeURL(uriTmpl, baseURL, pkg, requestedVersion, flags)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(vapopov): DELETE in v20.0.0 - the separate `tsh` package will no longer be supported.
	if runtime.GOOS == constants.DarwinOS && semVersion.Major < 17 {
		tshURL, err := autoupdate.MakeURL(uriTmpl, baseURL, "tsh", requestedVersion, flags)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return []packageURL{
			{Version: requestedVersion, Archive: teleportURL, Hash: teleportURL + ".sha256"},
			{Version: requestedVersion, Archive: tshURL, Hash: tshURL + ".sha256", Optional: true},
		}, nil
	}

	return []packageURL{
		{Version: requestedVersion, Archive: teleportURL, Hash: teleportURL + ".sha256"},
	}, nil
}

// toolName returns the path to {tsh, tctl} for the executable that started
// the current process.
func toolName(toolsDir string) (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return filepath.Join(toolsDir, filepath.Base(executablePath)), nil
}

// checkFreeSpace verifies that we have enough requested space at specific directory.
func checkFreeSpace(path string, requested uint64) error {
	free, err := utils.FreeDiskWithReserve(path, reservedFreeDisk)
	if err != nil {
		return trace.Errorf("failed to calculate free disk in %q: %v", path, err)
	}
	// Bail if there's not enough free disk space at the target.
	if requested > free {
		return trace.Errorf("%q needs %d additional bytes of disk space", path, requested-free)
	}

	return nil
}

// filterEnvs excludes environment variables by the list of the keys.
func filterEnvs(envs []string, excludeKeys []string) []string {
	return slices.DeleteFunc(envs, func(e string) bool {
		parts := strings.SplitN(e, "=", 2)
		return slices.Contains(excludeKeys, parts[0])
	})
}
