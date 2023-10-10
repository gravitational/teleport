/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
