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
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

// GenerateZipFile compresses the file into a `.zip` format. This format intended to be
// used only for windows platform and mocking paths for windows archive.
func GenerateZipFile(sourcePath, destPath string) error {
	archive, err := os.Create(destPath)
	if err != nil {
		return trace.Wrap(err)
	}

	zipWriter := zip.NewWriter(archive)
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		zipFileWriter, err := zipWriter.Create(filepath.Base(path))
		if err != nil {
			return trace.NewAggregate(err, file.Close())
		}

		_, err = io.Copy(zipFileWriter, file)
		return trace.NewAggregate(err, file.Close())
	})

	return trace.NewAggregate(err, zipWriter.Close(), archive.Close())
}

// GenerateTarGzFile compresses files into a `.tar.gz` format specifically in file
// structure related to linux packaging.
func GenerateTarGzFile(sourcePath, destPath string) error {
	archive, err := os.Create(destPath)
	if err != nil {
		return trace.Wrap(err)
	}
	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return trace.NewAggregate(err, file.Close())
		}
		header.Name = filepath.Join("teleport", filepath.Base(info.Name()))
		if err := tarWriter.WriteHeader(header); err != nil {
			return trace.NewAggregate(err, file.Close())
		}

		_, err = io.Copy(tarWriter, file)
		return trace.NewAggregate(err, file.Close())
	})

	return trace.NewAggregate(err, tarWriter.Close(), gzipWriter.Close(), archive.Close())
}

// GeneratePkgFile runs the macOS `pkgbuild` command to generate a .pkg file from the source.
func GeneratePkgFile(sourcePath, destPath, identifier string) error {
	cmd := exec.Command("pkgbuild",
		"--root", sourcePath,
		"--identifier", identifier,
		"--version", teleport.Version,
		destPath,
	)

	return trace.Wrap(cmd.Run())
}
