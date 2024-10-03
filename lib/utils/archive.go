/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

// ReadStatFS combines two interfaces: fs.ReadFileFS and fs.StatFS
// We need both when creating the archive to be able to:
// - read file contents - `ReadFile` provided by fs.ReadFileFS
// - set the correct file permissions - `Stat() ... Mode()` provided by fs.StatFS
type ReadStatFS interface {
	fs.ReadFileFS
	fs.StatFS
}

// CompressTarGzArchive creates a Tar Gzip archive in memory, reading the files using the provided file reader
func CompressTarGzArchive(files []string, fileReader ReadStatFS) (*bytes.Buffer, error) {
	archiveBytes := &bytes.Buffer{}

	gzipWriter, err := gzip.NewWriterLevel(archiveBytes, gzip.BestSpeed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, filename := range files {
		bs, err := fileReader.ReadFile(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fileStat, err := fileReader.Stat(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := tarWriter.WriteHeader(&tar.Header{
			Name: filename,
			Size: int64(len(bs)),
			Mode: int64(fileStat.Mode()),
		}); err != nil {
			return nil, trace.Wrap(err)
		}

		if _, err := tarWriter.Write(bs); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return archiveBytes, nil
}

// CompressDirToZipFile compresses directory into a `.zip` format.
func CompressDirToZipFile(sourcePath, destPath string) error {
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
			return trace.NewAggregate(err, file.Close())
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return trace.NewAggregate(err, file.Close())
		}

		_, err = io.Copy(tarWriter, file)
		return trace.NewAggregate(err, file.Close())
	})

	return trace.NewAggregate(err, tarWriter.Close(), gzipWriter.Close(), archive.Close())
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
