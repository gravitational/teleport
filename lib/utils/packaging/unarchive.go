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
	reservedFreeDisk = 10 * 1024 * 1024
)

// RemoveWithSuffix removes all files in dir that have the provided suffix, except for files named `skipName`
func RemoveWithSuffix(dir, suffix, skipName string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if !info.IsDir() {
			return nil
		}
		if skipName == info.Name() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), suffix) {
			return nil
		}
		// Found a stale expanded package.
		if err := os.RemoveAll(path); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return trace.Wrap(err)
}

func replaceZip(toolsDir string, archivePath string, hash string, apps []string) error {
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
	tempDir, err := os.MkdirTemp(toolsDir, hash)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, zipFile := range zipReader.File {
		// Skip over any files in the archive that are not defined apps.
		if !slices.ContainsFunc(apps, func(s string) bool {
			return strings.HasSuffix(zipFile.Name, s)
		}) {
			continue
		}
		// Verify that we have enough space for uncompressed zipFile.
		if err := checkFreeSpace(tempDir, zipFile.UncompressedSize64); err != nil {
			return trace.NewAggregate(err, f.Close())
		}

		file, err := zipFile.Open()
		if err != nil {
			return trace.Wrap(err)
		}
		defer file.Close()

		dest := filepath.Join(tempDir, zipFile.Name)
		destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return trace.Wrap(err)
		}
		defer destFile.Close()

		if _, err := io.Copy(destFile, file); err != nil {
			return trace.Wrap(err)
		}
		appPath := filepath.Join(toolsDir, zipFile.Name)
		if err := os.Remove(appPath); err != nil && !os.IsNotExist(err) {
			return trace.Wrap(err)
		}
		if err := os.Symlink(dest, appPath); err != nil {
			return trace.Wrap(err)
		}
		if err := destFile.Close(); err != nil {
			return trace.Wrap(err)
		}
		if err := file.Close(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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
