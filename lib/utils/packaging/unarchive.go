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
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// reservedFreeDisk is the predefined amount of free disk space (in bytes) required
	// to remain available after extracting Teleport binaries.
	reservedFreeDisk = 10 * 1024 * 1024
)

// RemoveWithSuffix removes all that matches the provided suffix, except for file or directory with `skipName`.
func RemoveWithSuffix(dir, suffix string, skipNames []string) error {
	var removePaths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if slices.Contains(skipNames, info.Name()) {
			return nil
		}
		if !strings.HasSuffix(info.Name(), suffix) {
			return nil
		}
		removePaths = append(removePaths, path)
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	var aggErr []error
	for _, path := range removePaths {
		if err := os.RemoveAll(path); err != nil {
			aggErr = append(aggErr, err)
		}
	}
	return trace.NewAggregate(aggErr...)
}

// replaceZip un-archives the Teleport package in .zip format, iterates through
// the compressed content, and ignores everything not matching the binaries specified
// in the execNames argument. The data is extracted to extractDir, and symlinks are created
// in toolsDir pointing to the extractDir path with binaries.
func replaceZip(toolsDir string, archivePath string, extractDir string, execNames []string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return trace.Wrap(err)
	}
	zipReader, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return trace.Wrap(err)
	}

	var totalSize uint64 = 0
	for _, zipFile := range zipReader.File {
		baseName := filepath.Base(zipFile.Name)
		// Skip over any files in the archive that are not defined execNames.
		if !slices.ContainsFunc(execNames, func(s string) bool {
			return baseName == s
		}) {
			continue
		}
		totalSize += zipFile.UncompressedSize64
	}
	// Verify that we have enough space for uncompressed zipFile.
	if err := checkFreeSpace(extractDir, totalSize); err != nil {
		return trace.Wrap(err)
	}

	for _, zipFile := range zipReader.File {
		baseName := filepath.Base(zipFile.Name)
		// Skip over any files in the archive that are not defined execNames.
		if !slices.Contains(execNames, baseName) {
			continue
		}

		if err := func(zipFile *zip.File) error {
			file, err := zipFile.Open()
			if err != nil {
				return trace.Wrap(err)
			}
			defer file.Close()

			dest := filepath.Join(extractDir, baseName)
			destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
			if err != nil {
				return trace.Wrap(err)
			}
			defer destFile.Close()

			if _, err := io.Copy(destFile, file); err != nil {
				return trace.Wrap(err)
			}
			appPath := filepath.Join(toolsDir, baseName)
			// For the Windows build, we need to copy the binary to perform updates without requiring
			// administrative access, which would otherwise be needed for creating symlinks.
			// Since symlinks are not used on the Windows platform, there's no need to remove appPath
			// before copying the new binary — it will simply be replaced.
			if err := utils.CopyFile(dest, appPath, 0o755); err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(destFile.Close())
		}(zipFile); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// checkFreeSpace verifies that we have enough requested space (in bytes) at specific directory.
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
