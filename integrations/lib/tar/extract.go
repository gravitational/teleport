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

package tar

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
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
			outFileName = filepath.Join(parts[strip:]...)
		}

		outFilePath := filepath.Join(outDir, outFileName)
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
