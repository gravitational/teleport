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
	"bytes"
	"compress/gzip"
	"io/fs"

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
