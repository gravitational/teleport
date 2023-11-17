/*
Copyright 2021 Gravitational, Inc.

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

package tar

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/lib/stringset"
)

// Compression is a compression flag.
type Compression int

const (
	NoCompression Compression = iota
	GzipCompression
)

// ExtractOptions is the options for Extract and ExtractFile functions.
type ExtractOptions struct {
	// OutDir is a directory where to extract the files. Current working directory by default.
	OutDir string
	// Compression indicates is the tarball compressed or not.
	Compression Compression
	// StripComponents is like --strip-components of tar utility. It strips first N components from the path by a given depth.
	StripComponents uint
	// Files is a list of files to extract. If empty, extract all the files.
	Files []string
	// OutFiles is a resulting map passed by user. If non-nil then it will store the mapping from tar file names to file system paths.
	OutFiles map[string]string
}

// ExtractFile extracts a tar file contents.
func ExtractFile(fileName string, options ExtractOptions) error {
	file, err := os.Open(fileName)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.NewAggregate(
		Extract(file, options),
		file.Close(),
	)
}

// Extract extracts a tarball given as a reader interface.
func Extract(reader io.Reader, options ExtractOptions) error {
	var err error

	outDir := options.OutDir
	if outDir == "" {
		outDir, err = os.Getwd()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	switch options.Compression {
	case NoCompression:
	case GzipCompression:
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown compression options %v", options.Compression)
	}

	tarReader := tar.NewReader(reader)

	var filesDone stringset.StringSet
	if len(options.Files) > 0 {
		filesDone = stringset.New(options.Files...)
	}
	for filesDone == nil || filesDone.Len() > 0 {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return trace.Wrap(err)
		}

		if filesDone != nil && !filesDone.Contains(header.Name) {
			continue
		}
		filesDone.Del(header.Name)

		outFileName := header.Name
		if strip := int(options.StripComponents); strip > 0 {
			parts := strings.Split(outFileName, "/")
			if strip > len(parts)-1 {
				strip = len(parts) - 1
			}
			outFileName = path.Join(parts[strip:]...)
		}

		outFilePath := path.Join(outDir, outFileName)
		outFilePerm := os.FileMode(header.Mode).Perm()

		// fail if the outFilePath is outside outDir, see the "zip slip" vulnerability
		if !strings.HasPrefix(filepath.Clean(outFilePath), filepath.Clean(outDir)+string(os.PathSeparator)) {
			return trace.Errorf("extraction target outside the root: %s", header.Name)
		}
		outFile, err := os.OpenFile(outFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, outFilePerm)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = io.Copy(outFile, tarReader)
		if err = trace.NewAggregate(err, outFile.Close()); err != nil {
			return trace.Wrap(err)
		}
		if options.OutFiles != nil {
			options.OutFiles[header.Name] = outFile.Name()
		}
	}

	if filesDone.Len() > 0 {
		return trace.Errorf("files not found in the archive: %s", strings.Join(filesDone.ToSlice(), ", "))
	}

	return nil
}
