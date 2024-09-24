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

package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
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
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
)

const (
	teleportToolsVersion = "TELEPORT_TOOLS_VERSION"
	checksumHexLen       = 64
	reservedFreeDisk     = 10 * 1024 * 1024

	FlagEnt  = 1 << 0
	FlagFips = 1 << 1
)

var (
	pattern       = regexp.MustCompile(`(?m)Teleport v(.*) git`)
	baseUrl       = "https://cdn.teleport.dev"
	defaultClient = newClient(&downloadConfig{})
	featureFlag   int
)

// CheckLocal is run at client tool startup and will only perform local checks.
func CheckLocal() (string, bool) {
	// If a version of client tools has already been downloaded to
	// $TELEPORT_HOME/bin, return that.
	toolsVersion, err := version()
	if err != nil {
		return "", false
	}

	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv(teleportToolsVersion)
	switch {
	// The user has turned off any form of automatic updates.
	case requestedVersion == "off":
		return "", false
	// Requested version already the same as client version.
	case teleport.Version == requestedVersion:
		return requestedVersion, false
	// The user has requested a specific version of client tools.
	case requestedVersion != "" && requestedVersion != toolsVersion:
		return requestedVersion, true
	}

	return toolsVersion, false
}

// CheckRemote will check against Proxy Service if client tools need to be
// updated.
func CheckRemote(ctx context.Context, proxyAddr string) (string, bool, error) {
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
	// $TELEPORT_HOME/bin, return that.
	toolsVersion, err := version()
	if err != nil {
		return "", false, nil
	}

	requestedVersion := os.Getenv(teleportToolsVersion)
	switch {
	// The user has turned off any form of automatic updates.
	case requestedVersion == "off":
		return "", false, nil
	// Requested version already the same as client version.
	case teleport.Version == requestedVersion:
		return requestedVersion, false, nil
	case requestedVersion != "" && requestedVersion != toolsVersion:
		return requestedVersion, true, nil
	case !resp.ToolsAutoupdate || resp.ToolsVersion == "":
		return "", false, nil
	case teleport.Version == resp.ToolsVersion:
		return resp.ToolsVersion, false, nil
	case resp.ToolsVersion != toolsVersion:
		return resp.ToolsVersion, true, nil
	}

	return toolsVersion, false, nil
}

// Download downloads requested version package, unarchive and replace existing one.
func Download(toolsVersion string) error {
	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}
	// Create $TELEPORT_HOME/bin if it does not exist.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return trace.Wrap(err)
	}
	// Lock to allow multiple concurrent {tsh, tctl} to run.
	unlock, err := lock(dir)
	if err != nil {
		return trace.Wrap(err)
	}
	defer unlock()

	// If the version of the running binary or the version downloaded to
	// $TELEPORT_HOME/bin is the same as the requested version of client tools,
	// nothing to be done, exit early.
	teleportVersion, err := version()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	if toolsVersion == teleport.Version || toolsVersion == teleportVersion {
		return nil
	}

	// Download and update {tsh, tctl} in $TELEPORT_HOME/bin.
	if err := update(toolsVersion); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func update(toolsVersion string) error {
	// Get platform specific download URLs.
	archiveURL, hashURL, err := urls(toolsVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("Archive download path: %v.", archiveURL)

	// Download the archive and validate against the hash. Download to a
	// temporary path within $TELEPORT_HOME/bin.
	hash, err := downloadHash(hashURL)
	if err != nil {
		return trace.Wrap(err)
	}
	path, err := downloadArchive(archiveURL, hash)
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.Remove(path)

	// Perform atomic replace so concurrent exec do not fail.
	if err := replace(path, hash); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// urls returns the URL for the Teleport archive to download. The format is:
// https://cdn.teleport.dev/teleport-{, ent-}v15.3.0-{linux, darwin, windows}-{amd64,arm64,arm,386}-{fips-}bin.tar.gz
func urls(toolsVersion string) (string, string, error) {
	var archive string

	switch runtime.GOOS {
	case "darwin":
		archive = baseUrl + "/tsh-" + toolsVersion + ".pkg"
	case "windows":
		archive = baseUrl + "/teleport-v" + toolsVersion + "-windows-amd64-bin.zip"
	case "linux":
		edition := ""
		if featureFlag&FlagEnt != 0 || featureFlag&FlagFips != 0 {
			edition = "ent-"
		}
		fips := ""
		if featureFlag&FlagFips != 0 {
			fips = "fips-"
		}

		var b strings.Builder
		b.WriteString(baseUrl + "/teleport-")
		if edition != "" {
			b.WriteString(edition)
		}
		b.WriteString("v" + toolsVersion + "-" + runtime.GOOS + "-" + runtime.GOARCH + "-")
		if fips != "" {
			b.WriteString(fips)
		}
		b.WriteString("bin.tar.gz")
		archive = b.String()
	default:
		return "", "", trace.BadParameter("unsupported runtime: %v", runtime.GOOS)
	}

	return archive, archive + ".sha256", nil
}

func downloadHash(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	resp, err := defaultClient.Do(req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("request failed with: %v", resp.StatusCode)
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

func downloadArchive(url string, hash string) (string, error) {
	resp, err := defaultClient.Get(url)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("bad status when downloading archive: %v", resp.StatusCode)
	}

	dir, err := toolsDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if resp.ContentLength != -1 {
		if err := checkFreeSpace(dir, uint64(resp.ContentLength)); err != nil {
			return "", trace.Wrap(err)
		}
	}

	// Caller of this function will remove this file after the atomic swap has
	// occurred.
	f, err := os.CreateTemp(dir, "tmp-")
	if err != nil {
		return "", trace.Wrap(err)
	}

	h := sha256.New()
	pw := &progressWriter{n: 0, limit: resp.ContentLength}
	body := cancelableTeeReader(io.TeeReader(resp.Body, h), pw, syscall.SIGINT, syscall.SIGTERM)

	// It is a little inefficient to download the file to disk and then re-load
	// it into memory to unarchive later, but this is safer as it allows {tsh,
	// tctl} to validate the hash before trying to operate on the archive.
	_, err = io.Copy(f, body)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != hash {
		return "", trace.BadParameter("hash of archive does not match downloaded archive")
	}

	return f.Name(), nil
}

// Exec re-executes tool command with same arguments and environ variables.
func Exec() (int, error) {
	path, err := toolName()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	cmd := exec.Command(path, os.Args[1:]...)
	// To prevent re-execution loop we have to disable update logic for re-execution.
	cmd.Env = append(os.Environ(), teleportToolsVersion+"=off")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return 0, trace.Wrap(err)
	}

	return cmd.ProcessState.ExitCode(), nil
}

func version() (string, error) {
	// Find the path to the current executable.
	path, err := toolName()
	if err != nil {
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
	command.Env = []string{teleportToolsVersion + "=off"}
	output, err := command.Output()
	if err != nil {
		return "", trace.Wrap(err)
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

// toolsDir returns the path to {tsh, tctl} in $TELEPORT_HOME/bin.
func toolsDir() (string, error) {
	home := os.Getenv(types.HomeEnvVar)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return filepath.Join(filepath.Clean(home), ".tsh", "bin"), nil
}

// toolName returns the path to {tsh, tctl} for the executable that started
// the current process.
func toolName() (string, error) {
	base, err := toolsDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", trace.Wrap(err)
	}
	toolName := filepath.Base(executablePath)

	return filepath.Join(base, toolName), nil
}

// checkFreeSpace verifies that we have enough requested space at specific directory.
func checkFreeSpace(path string, requested uint64) error {
	free, err := freeDiskWithReserve(path)
	if err != nil {
		return trace.Errorf("failed to calculate free disk in %q: %v", path, err)
	}
	// Bail if there's not enough free disk space at the target.
	if requested > free {
		return trace.Errorf("%q needs %d additional bytes of disk space", path, requested-free)
	}
	return nil
}
