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
	// directoryPerm defines the permissions used when creating the directory
	// structure, matching those from the archive.
	directoryPerm = 0o755
	// binaryPerm defines the permissions applied to extracted binaries.
	binaryPerm = 0o755
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
// in the execNames argument. The data is extracted to extractDir, and copies are created in toolsDir.
func replaceZip(archivePath string, extractDir string, execNames []string) (map[string]string, error) {
	execPaths := make(map[string]string, len(execNames))
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	zipReader, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return nil, trace.Wrap(err)
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
		return nil, trace.Wrap(err)
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

			dest := filepath.Join(extractDir, zipFile.Name)
			// Preserve the archive directory structure.
			if err := os.MkdirAll(filepath.Dir(dest), directoryPerm); err != nil {
				return trace.Wrap(err)
			}
			destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, binaryPerm)
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := io.Copy(destFile, file); err != nil {
				return trace.NewAggregate(err, destFile.Close())
			}
			return trace.Wrap(destFile.Close())
		}(zipFile); err != nil {
			return nil, trace.Wrap(err)
		}
		execPaths[baseName] = zipFile.Name
	}

	return execPaths, nil
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
