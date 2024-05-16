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
package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"

	"github.com/gravitational/trace"
)

// repack copies the contents of a compressed tar archive into a
// zip archive. Only regular  files are copied into the Zip
// archive - symlinks, etc are discarded.
func repack(dst io.Writer, src io.Reader) error {
	uncompressedReader, err := gzip.NewReader(src)
	if err != nil {
		return trace.Wrap(err)
	}
	defer uncompressedReader.Close()
	tarReader := tar.NewReader(uncompressedReader)

	zipWriter := zip.NewWriter(dst)
	defer zipWriter.Close()

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return trace.Wrap(err)
		}

		// if the header represents a "regular file"...
		if header.Typeflag == tar.TypeReg {
			if err = copyfile(zipWriter, header, tarReader); err != nil {
				return trace.Wrap(err, "failed repacking tar file item")
			}
		}
	}

	return nil
}

// copyfile copies a file from a tar archive into a zip archive, preserving
// the file attributes as far as possible.
func copyfile(zipfile *zip.Writer, header *tar.Header, src io.Reader) error {
	zipHeader, err := zip.FileInfoHeader(header.FileInfo())
	if err != nil {
		return trace.Wrap(err, "failed initializing zipfile header")
	}
	zipHeader.Name = header.Name
	zipHeader.Method = zip.Deflate

	dst, err := zipfile.CreateHeader(zipHeader)
	if err != nil {
		return trace.Wrap(err, "failed writing zipfile header")
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		return trace.Wrap(err, "failed adding data to zipfile")
	}

	return nil
}
