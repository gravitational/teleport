//go:build !windows

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

package packaging

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
)

// Replace un-archives package from tools directory and replaces defined apps by symlinks.
func Replace(toolsDir string, archivePath string, hash string, apps []string) error {
	switch runtime.GOOS {
	case "darwin":
		return replacePkg(toolsDir, archivePath, hash, apps)
	default:
		return replaceTarGz(toolsDir, archivePath, apps)
	}
}

func replaceTarGz(toolsDir string, archivePath string, apps []string) error {
	tempDir := renameio.TempDir(toolsDir)
	f, err := os.Open(archivePath)
	if err != nil {
		return trace.Wrap(err)
	}

	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return trace.NewAggregate(err, f.Close())
	}

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		// Skip over any files in the archive that are not in apps.
		if !slices.ContainsFunc(apps, func(s string) bool {
			return strings.HasSuffix(header.Name, s)
		}) {
			if _, err := io.Copy(io.Discard, tarReader); err != nil {
				slog.DebugContext(context.Background(), "failed to discard", "name", header.Name, "error", err)
			}
			continue
		}
		// Verify that we have enough space for uncompressed file.
		if err := checkFreeSpace(tempDir, uint64(header.Size)); err != nil {
			return trace.NewAggregate(err, gzipReader.Close(), f.Close())
		}

		dest := filepath.Join(toolsDir, strings.TrimPrefix(header.Name, "teleport/"))
		t, err := renameio.TempFile(tempDir, dest)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := os.Chmod(t.Name(), 0o755); err != nil {
			return trace.NewAggregate(err, t.Cleanup(), gzipReader.Close(), f.Close())
		}

		if _, err := io.Copy(t, tarReader); err != nil {
			return trace.NewAggregate(err, t.Cleanup(), gzipReader.Close(), f.Close())
		}
		if err := t.CloseAtomicallyReplace(); err != nil {
			return trace.NewAggregate(err, t.Cleanup(), gzipReader.Close(), f.Close())
		}
		if err := t.Cleanup(); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.NewAggregate(gzipReader.Close(), f.Close())
}

func replacePkg(toolsDir string, archivePath string, hash string, apps []string) error {
	// Use "pkgutil" from the filesystem to expand the archive. In theory .pkg
	// files are xz archives, however it's still safer to use "pkgutil" in-case
	// Apple makes non-standard changes to the format.
	//
	// Full command: pkgutil --expand-full NAME.pkg DIRECTORY/
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		return trace.Wrap(err)
	}
	expandPath := filepath.Join(toolsDir, hash+"-pkg")
	out, err := exec.Command(pkgutil, "--expand-full", archivePath, expandPath).Output()
	if err != nil {
		slog.DebugContext(context.Background(), "failed to run pkgutil:", "output", out)
		return trace.Wrap(err)
	}

	err = filepath.Walk(expandPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if info.IsDir() {
			return nil
		}
		// Skip over any files in the archive that are not in apps.
		if !slices.ContainsFunc(apps, func(s string) bool {
			return strings.HasSuffix(info.Name(), s)
		}) {
			return nil
		}

		// The first time a signed and notarized binary macOS application is run,
		// execution is paused while it gets sent to Apple to verify. Once Apple
		// approves the binary, the "com.apple.macl" extended attribute is added
		// and the process is allowed to execute. This process is not concurrent, any
		// other operations (like moving the application) on the application during
		// this time will lead to the application being sent SIGKILL.
		//
		// Since apps have to be concurrent, execute app before performing any
		// swap operations. This ensures that the "com.apple.macl" extended
		// attribute is set and macOS will not send a SIGKILL to the process
		// if multiple processes are trying to operate on it.
		command := exec.Command(path, "version", "--client")
		if err := command.Run(); err != nil {
			return trace.Wrap(err)
		}

		// Due to macOS applications not being a single binary (they are a
		// directory), atomic operations are not possible. To work around this, use
		// a symlink (which can be atomically swapped), then do a cleanup pass
		// removing any stale copies of the expanded package.
		newName := filepath.Join(toolsDir, filepath.Base(path))
		if err := renameio.Symlink(path, newName); err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	return trace.Wrap(err)
}
