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

package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// CompressDirToZipFile compresses a source directory into `.zip` format and stores at `archivePath`,
// preserving the relative file path structure of the source directory.
func CompressDirToZipFile(ctx context.Context, sourceDir, archivePath string) (err error) {
	archive, err := os.Create(archivePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			err = trace.NewAggregate(err, closeErr)
			return
		}
		if err != nil {
			if err := os.Remove(archivePath); err != nil {
				slog.ErrorContext(ctx, "failed to remove archive", "error", err)
			}
		}
	}()

	zipWriter := zip.NewWriter(archive)
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
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
		defer file.Close()
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return trace.Wrap(err)
		}
		zipFileWriter, err := zipWriter.Create(relPath)
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err = io.Copy(zipFileWriter, file); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(file.Close())
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err = zipWriter.Close(); err != nil {
		return trace.Wrap(err)
	}

	return
}

// CompressDirToTarGzFile compresses a source directory into .tar.gz format and stores at `archivePath`,
// preserving the relative file path structure of the source directory.
func CompressDirToTarGzFile(ctx context.Context, sourceDir, archivePath string) (err error) {
	archive, err := os.Create(archivePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			err = trace.NewAggregate(err, closeErr)
			return
		}
		if err != nil {
			if err := os.Remove(archivePath); err != nil {
				slog.ErrorContext(ctx, "failed to remove archive", "error", err)
			}
		}
	}()
	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
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
		defer file.Close()
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return trace.Wrap(err)
		}
		header.Name, err = filepath.Rel(sourceDir, path)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return trace.Wrap(err)
		}
		if _, err = io.Copy(tarWriter, file); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(file.Close())
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err = tarWriter.Close(); err != nil {
		return trace.Wrap(err)
	}
	if err = gzipWriter.Close(); err != nil {
		return trace.Wrap(err)
	}

	return
}

// CompressDirToPkgFile runs for the macOS `pkgbuild` command to generate a .pkg
// archive file from the source directory.
func CompressDirToPkgFile(ctx context.Context, sourceDir, archivePath, identifier string) error {
	if runtime.GOOS != "darwin" {
		return trace.BadParameter("only darwin platform is supported for pkg file")
	}
	cmd := exec.CommandContext(
		ctx,
		"pkgbuild",
		"--root", sourceDir,
		"--identifier", identifier,
		"--version", teleport.Version,
		archivePath,
	)

	return trace.Wrap(cmd.Run())
}
