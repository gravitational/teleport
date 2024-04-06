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
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/google/renameio/v2"
	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
	log "github.com/sirupsen/logrus"
)

// CheckLocal is run at client tool startup and will only perform local checks.
func CheckLocal() (string, bool) {
	// Check if the user has requested a specific version of client tools.
	requestedVersion := os.Getenv("TELEPORT_TOOLS_VERSION")
	switch {
	// The user has turned off any form of automatic updates.
	case requestedVersion == "off":
		return "", false
	// The user has requested a specific version of client tools.
	case requestedVersion != "":
		return requestedVersion, true
	}

	// If a version of client tools has already been downloaded to
	// $TELEPORT_HOME/bin, return that.
	toolsVersion, err := version()
	if err != nil {
		return "", false
	}
	return toolsVersion, true
}

// CheckRemote will check against Proxy Service if client tools need to be
// updated.
func CheckRemote() (string, bool) {
	// TODO(russjones): CheckRemote should still honor TELEPORT_TOOLS_VERSION
	// as it allows the user to override what the server has requested.
	return "", false
}

func Download(toolsVersion string) error {
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

	// Create $TELEPORT_HOME/bin if it does not exist.
	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return trace.Wrap(err)
	}

	// Download and update {tsh, tctl} in $TELEPORT_HOME/bin.
	if err := update(toolsVersion); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// TODO(russjones): Add edition check here as well.
func update(toolsVersion string) error {
	// Lock to allow multiple concurrent {tsh, tctl} to run.
	unlock, err := lock()
	defer unlock()

	// TODO(russjones): Cleanup any partial downloads first.

	// Get platform specific download URLs.
	archiveURL, hashURL, err := urls(toolsVersion, "")
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
	if err := atomicReplace(path); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// urls returns the URL for the Teleport archive to download. The format is:
// https://cdn.teleport.dev/teleport-{, ent-}v15.3.0-{linux, darwin, windows}-{amd64,arm64,arm,386}-{fips-}bin.tar.gz
func urls(toolsVersion string, toolsEdition string) (string, string, error) {
	var archive string

	switch runtime.GOOS {
	case "darwin":
		// TODO(russjones): Once tctl.app is created, update this to whatever the
		// new package will be called. Maybe back to teleport-{toolsVersion}.pkg?
		archive = "https://cdn.teleport.dev/tsh-" + toolsVersion + ".pkg"
	case "windows":
		// TODO(russjones): Update this to whatever this package will be called.
		archive = "https://cdn.teleport.dev/teleport-v" + toolsVersion + "-windows-amd64-bin.zip"
	case "linux":
		edition := ""
		if toolsEdition == "ent" || toolsEdition == "fips" {
			edition = "ent-"
		}
		fips := ""
		if toolsEdition == "fips" {
			fips = "fips-"
		}

		var b strings.Builder
		b.WriteString("https://cdn.teleport.dev/teleport-")
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
	resp, err := http.Get(url)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("request failed with: %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Hash is the first 64 bytes of the response.
	return string(body)[0:64], nil
}

func downloadArchive(url string, hash string) (string, error) {
	resp, err := http.Get(url)
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

	// Caller of this function will remove this file after the atomic swap has
	// occured.
	f, err := os.CreateTemp(dir, "tmp-")
	if err != nil {
		return "", trace.Wrap(err)
	}

	// TODO(russjones): Add ability to Ctrl-C cancel here.
	h := sha256.New()
	pw := &progressWriter{n: 0, limit: resp.ContentLength}
	body := io.TeeReader(io.TeeReader(resp.Body, h), pw)

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

func atomicReplace(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return trace.Wrap(replaceDarwin(path))
	case "linux":
		return trace.Wrap(replaceLinux(path))
	case "windows":
		return trace.Wrap(replaceWindows(path))
	default:
		return trace.BadParameter("unsupported runtime: %v", runtime.GOOS)
	}
}

func replaceDarwin(path string) error {
	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}

	// Use "pkgutil" from the filesystem to expand the archive. In theory .pkg
	// files are xz archives, however it's still safer to use "pkgutil" in-case
	// Apple makes non-standard changes to the format.
	//
	// Full command: pkgutil --expand-full NAME.pkg DIRECTORY/
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		return trace.Wrap(err)
	}
	expandUUID := uuid.NewString()
	expandPath := filepath.Join(dir, expandUUID+"-pkg")
	out, err := exec.Command(pkgutil, "--expand-full", path, expandPath).Output()
	if err != nil {
		log.Debugf("Failed to run pkgutil: %v.", out)
		return trace.Wrap(err)
	}

	// The first time a signed and notarized binary macOS application is run,
	// execution is paused while it gets sent to Apple to verify. Once Apple
	// approves the binary, the "com.apple.macl" extended attribute is added
	// and the process is allow to execute. This process is not concurrent, any
	// other operations (like moving the application) on the application during
	// this time will lead the the application being sent SIGKILL.
	//
	// Since {tsh, tctl} have to be concurrent, execute {tsh, tctl} before
	// performing any swap operations. This ensures that the "com.apple.macl"
	// extended attribute is set and macOS will not send a SIGKILL to the
	// process if multiple processes are trying to operate on it.
	//
	// TODO(russjones): Update to support tctl.app as well.
	expandExecPath := filepath.Join(expandPath, "Payload", "tsh.app", "Contents", "MacOS", "tsh")
	if _, err := exec.Command(expandExecPath, "version", "--client").Output(); err != nil {
		return trace.Wrap(err)
	}

	// Due to macOS applications not being a single binary (they are a
	// directory), atomic operations are not possible. To work around this, use
	// a symlink (which can be atomically swapped), then do a cleanup pass
	// removing any stale copies of the expanded package.
	oldName := filepath.Join(expandPath, "Payload", "tsh.app", "Contents", "MacOS", "tsh")
	newName := filepath.Join(dir, "tsh")
	if err := renameio.Symlink(oldName, newName); err != nil {
		return trace.Wrap(err)
	}

	// Perform a cleanup pass to remove any old copies of "{tsh, tctl}.app".
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if expandUUID+"-pkg" == info.Name() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), "-pkg") {
			return nil
		}

		// Found a stale expanded package.
		if err := os.RemoveAll(filepath.Join(dir, info.Name())); err != nil {
			return err
		}

		return nil
	})

	return nil
}

func replaceLinux(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return trace.Wrap(err)
	}

	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}
	tempDir := renameio.TempDir(dir)

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		// Skip over any files in the archive that are not {tsh, tctl}.
		if header.Name != "teleport/tctl" &&
			header.Name != "teleport/tsh" &&
			header.Name != "teleport/tbot" {
			if _, err := io.Copy(ioutil.Discard, tarReader); err != nil {
				log.Debugf("Failed to discard %v: %v.", header.Name, err)
			}
			continue
		}

		dest := filepath.Join(dir, strings.TrimPrefix(header.Name, "teleport/"))
		t, err := renameio.TempFile(tempDir, dest)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := os.Chmod(t.Name(), 0755); err != nil {
			return trace.Wrap(err)
		}
		defer t.Cleanup()

		if _, err := io.Copy(t, tarReader); err != nil {
			return trace.Wrap(err)
		}
		if err := t.CloseAtomicallyReplace(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func replaceWindows(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	fi, err := f.Stat()
	if err != nil {
		return trace.Wrap(err)
	}
	zipReader, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return trace.Wrap(err)
	}

	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}
	tempDir := renameio.TempDir(dir)

	for _, r := range zipReader.File {
		if r.Name != "tsh.exe" {
			continue
		}
		rr, err := r.Open()
		if err != nil {
			return trace.Wrap(err)
		}
		defer rr.Close()

		dest := filepath.Join(dir, r.Name)
		t, err := renameio.TempFile(tempDir, dest)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := os.Chmod(t.Name(), 0755); err != nil {
			return trace.Wrap(err)
		}
		defer t.Cleanup()

		if _, err := io.Copy(t, rr); err != nil {
			return trace.Wrap(err)
		}
		if err := t.CloseAtomicallyReplace(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func Exec() (int, error) {
	path, err := toolName()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	//// On macOS and Linux, this may be better.
	//err := syscall.Exec(path, os.Args[1:], os.Environ())
	//if err != nil {
	//	return trace.Wrap(err)
	//}

	cmd := exec.Command(path, os.Args[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return 0, trace.Wrap(err)
	}

	return cmd.ProcessState.ExitCode(), nil
}

func lock() (func(), error) {
	// Build the path to the lock file that will be used by flock.
	dir, err := toolsDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lockFile := filepath.Join(dir, ".lock")

	// Create the advisory lock using flock.
	// TODO(russjones): Use os.CreateTemp here?
	lf, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return nil, trace.Wrap(err)
	}

	return func() {
		if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_UN); err != nil {
			log.Debugf("Failed to unlock file: %v: %v.", lockFile, err)
		}
		//if err := os.Remove(lockFile); err != nil {
		//	log.Debugf("Failed to remove lock file: %v: %v.", lockFile, err)
		//}
		if err := lf.Close(); err != nil {
			log.Debugf("Failed to close lock file %v: %v.", lockFile, err)
		}
	}, nil
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

	// Execue "{tsh, tctl} version" and pass in TELEPORT_TOOLS_VERSION=off to
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

type progressWriter struct {
	n     int64
	limit int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.n = w.n + int64(len(p))

	n := int((w.n*100)/w.limit) / 10
	bricks := strings.Repeat("▒", n) + strings.Repeat(" ", 10-n)
	fmt.Printf("\rUpdate progress: [" + bricks + "] (Ctrl-C to cancel update)")

	if w.n == w.limit {
		fmt.Printf("\n")
	}

	return len(p), nil
}

const (
	teleportToolsVersion = "TELEPORT_TOOLS_VERSION"
)

var (
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)
