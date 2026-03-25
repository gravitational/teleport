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
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// ReplaceToolsBinaries extracts executables specified by execNames from archivePath into
// extractDir. After each executable is extracted, it is symlinked from extractDir/[name] to
// toolsDir/[name].
//
// For Darwin, archivePath must be a .pkg file.
// For other POSIX, archivePath must be a gzipped tarball.
func ReplaceToolsBinaries(archivePath string, extractDir string, execNames []string) (map[string]string, error) {
	switch runtime.GOOS {
	case constants.DarwinOS:
		return replacePkg(archivePath, extractDir, execNames)
	default:
		return replaceTarGz(archivePath, extractDir, execNames)
	}
}

// replaceTarGz un-archives the Teleport package in .tar.gz format, iterates through
// the compressed content, and ignores everything not matching the app binaries specified
// in the apps argument. The data is extracted to extractDir, and symlinks are created
// in toolsDir pointing to the extractDir path with binaries.
func replaceTarGz(archivePath string, extractDir string, execNames []string) (map[string]string, error) {
	execPaths := make(map[string]string, len(execNames))
	if err := validateFreeSpaceTarGz(archivePath, extractDir, execNames); err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		baseName := filepath.Base(header.Name)
		// Skip over any files in the archive that are not in execNames.
		if !slices.Contains(execNames, baseName) {
			continue
		}

		if err = func(header *tar.Header) error {
			dest := filepath.Join(extractDir, header.Name)
			// Preserve the archive directory structure.
			if err := os.MkdirAll(filepath.Dir(dest), directoryPerm); err != nil {
				return trace.Wrap(err)
			}
			destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, binaryPerm)
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := io.Copy(destFile, tarReader); err != nil {
				return trace.NewAggregate(err, destFile.Close())
			}
			return trace.Wrap(destFile.Close())
		}(header); err != nil {
			return nil, trace.Wrap(err)
		}
		execPaths[baseName] = header.Name
	}

	return execPaths, trace.Wrap(gzipReader.Close())
}

// validateFreeSpaceTarGz validates that extraction size match available disk space in `extractDir`.
func validateFreeSpaceTarGz(archivePath string, extractDir string, execNames []string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	var totalSize uint64
	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return trace.Wrap(err)
	}
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return trace.Wrap(err)
		}
		baseName := filepath.Base(header.Name)
		// Skip over any files in the archive that are not defined execNames.
		if !slices.Contains(execNames, baseName) {
			continue
		}
		totalSize += uint64(header.Size)
	}

	return trace.Wrap(checkFreeSpace(extractDir, totalSize))
}

// replacePkg expands the Teleport package in .pkg format using the platform-dependent pkgutil utility.
// The data is extracted to extractDir, and symlinks are created in toolsDir pointing to the binaries
// in extractDir. Before creating the symlinks, each binary must be executed at least once to pass
// OS signature verification.
func replacePkg(archivePath string, extractDir string, execNames []string) (map[string]string, error) {
	execPaths := make(map[string]string, len(execNames))
	// Use "pkgutil" from the filesystem to expand the archive. In theory .pkg
	// files are xz archives, however it's still safer to use "pkgutil" in-case
	// Apple makes non-standard changes to the format.
	//
	// Full command: pkgutil --expand-full NAME.pkg DIRECTORY/
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err = exec.Command(pkgutil, "--expand-full", archivePath, extractDir).Run(); err != nil {
		return nil, trace.Wrap(err)
	}

	err = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if info.IsDir() {
			return nil
		}
		// Skip over any files in the archive that are not in execNames.
		if !slices.ContainsFunc(execNames, func(s string) bool {
			return filepath.Base(info.Name()) == s
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
		command := exec.Command(path, "version")
		command.Env = []string{"TELEPORT_TOOLS_VERSION=off"}
		if err := command.Run(); err != nil {
			return trace.WrapWithMessage(err, "failed to validate binary")
		}
		relPath, err := filepath.Rel(extractDir, path)
		if err != nil {
			return trace.Wrap(err)
		}
		execPaths[info.Name()] = relPath

		return nil
	})

	return execPaths, trace.Wrap(err)
}
