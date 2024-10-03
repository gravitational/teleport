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

package helpers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

// CompressDirToZipFile compresses directory into a `.zip` format.
func CompressDirToZipFile(sourcePath, destPath string) error {
	archive, err := os.Create(destPath)
	if err != nil {
		return trace.Wrap(err)
	}

	zipWriter := zip.NewWriter(archive)
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return trace.Wrap(err)
		}
		zipFileWriter, err := zipWriter.Create(filepath.Base(path))
		if err != nil {
			_ = file.Close()
			return trace.Wrap(err)
		}
		if _, err = io.Copy(zipFileWriter, file); err != nil {
			_ = file.Close()
			return trace.Wrap(err)
		}
		return trace.Wrap(file.Close())
	})
	if err != nil {
		_ = archive.Close()
		return trace.Wrap(err)
	}
	if err := zipWriter.Close(); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(archive.Close())
}

// CompressDirToTarGzFile compresses directory into a `.tar.gz` format.
func CompressDirToTarGzFile(sourcePath, destPath string) error {
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
			_ = file.Close()
			return trace.Wrap(err)
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = file.Close()
			return trace.NewAggregate(err, file.Close())
		}
		if _, err = io.Copy(tarWriter, file); err != nil {
			_ = file.Close()
			return trace.Wrap(err)
		}
		return trace.Wrap(file.Close())
	})
	if err != nil {
		_ = archive.Close()
		return trace.Wrap(err)
	}
	if err := tarWriter.Close(); err != nil {
		_ = archive.Close()
		return trace.Wrap(err)
	}
	if err := gzipWriter.Close(); err != nil {
		_ = archive.Close()
		return trace.Wrap(err)
	}

	return trace.Wrap(archive.Close())
}

// CompressDirToPkgFile runs for the macOS `pkgbuild` command to generate a .pkg file from the source.
func CompressDirToPkgFile(sourcePath, destPath, identifier string) error {
	if runtime.GOOS != "darwin" {
		return trace.BadParameter("only darwin packaging is supported")
	}
	cmd := exec.Command("pkgbuild",
		"--root", sourcePath,
		"--identifier", identifier,
		"--version", teleport.Version,
		destPath,
	)

	return trace.Wrap(cmd.Run())
}
