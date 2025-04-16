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
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

var errNoBaseURL = errors.New("baseURL is not defined")

// Dir returns the path to client tools in $TELEPORT_HOME/bin.
func Dir() (string, error) {
	home := os.Getenv(types.HomeEnvVar)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return filepath.Join(home, ".tsh", "bin"), nil
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

// CheckToolVersion returns current installed client tools version, must return NotFoundError if
// the client tools is not found in tools directory.
func CheckToolVersion(toolsDir string) (string, error) {
	// Find the path to the current executable.
	path, err := toolName(toolsDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", trace.NotFound("autoupdate tool not found in %q", toolsDir)
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
	command := exec.CommandContext(ctx, path, "version")
	command.Env = []string{teleportToolsVersionEnv + "=off"}
	output, err := command.Output()
	if err != nil {
		return "", trace.WrapWithMessage(err, "failed to determine version of %q tool", path)
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

// packageURL defines URLs to the archive and their archive sha256 hash file, and marks
// if this package is optional, for such case download needs to be ignored if package
// not found in CDN.
type packageURL struct {
	Archive  string
	Hash     string
	Optional bool
}

// teleportPackageURLs returns URLs for the Teleport archives to download.
func teleportPackageURLs(ctx context.Context, uriTmpl string, baseURL, version string) ([]packageURL, error) {
	m := modules.GetModules()
	envBaseURL := os.Getenv(autoupdate.BaseURLEnvVar)
	if m.BuildType() == modules.BuildOSS && envBaseURL == "" {
		slog.WarnContext(ctx, "Client tools updates are disabled as they are licensed under AGPL. To use Community Edition builds or custom binaries, set the 'TELEPORT_CDN_BASE_URL' environment variable.")
		return nil, errNoBaseURL
	}

	var flags autoupdate.InstallFlags
	if m.IsBoringBinary() {
		flags |= autoupdate.FlagFIPS
	}
	if m.IsEnterpriseBuild() || m.IsBoringBinary() {
		flags |= autoupdate.FlagEnterprise
	}

	teleportURL, err := autoupdate.MakeURL(uriTmpl, baseURL, autoupdate.DefaultPackage, version, flags)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if runtime.GOOS == constants.DarwinOS {
		tshURL, err := autoupdate.MakeURL(uriTmpl, baseURL, "tsh", version, flags)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return []packageURL{
			{Archive: teleportURL, Hash: teleportURL + ".sha256"},
			{Archive: tshURL, Hash: tshURL + ".sha256", Optional: true},
		}, nil
	}

	return []packageURL{
		{Archive: teleportURL, Hash: teleportURL + ".sha256"},
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
