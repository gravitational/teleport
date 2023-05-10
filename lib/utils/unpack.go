/*
Copyright 2018 Gravitational, Inc.

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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
)

// Extract extracts the contents of the specified tarball under dir. The
// resulting files and directories are created using the current user context.
// Extract will only unarchive files into dir, and will fail if the tarball
// tries to write files outside of dir.
func Extract(r io.Reader, dir string) error {
	tarball := tar.NewReader(r)

	for {
		header, err := tarball.Next()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return trace.Wrap(err)
		}

		err = sanitizeTarPath(header, dir)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := extractFile(tarball, header, dir); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// extractFile extracts a single file or directory from tarball into dir.
// Uses header to determine the type of item to create
// Based on https://github.com/mholt/archiver
func extractFile(tarball *tar.Reader, header *tar.Header, dir string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return withDir(filepath.Join(dir, header.Name), nil)
	case tar.TypeBlock, tar.TypeChar, tar.TypeReg, tar.TypeFifo:
		return writeFile(filepath.Join(dir, header.Name), tarball, header.FileInfo().Mode())
	case tar.TypeLink:
		return writeHardLink(filepath.Join(dir, header.Name), filepath.Join(dir, header.Linkname))
	case tar.TypeSymlink:
		return writeSymbolicLink(filepath.Join(dir, header.Name), header.Linkname)
	default:
		log.Warnf("Unsupported type flag %v for %v.", header.Typeflag, header.Name)
	}
	return nil
}

// sanitizeTarPath checks that the tar header paths resolve to a subdirectory
// path, and don't contain file paths or links that could escape the tar file
// like ../../etc/password.
func sanitizeTarPath(header *tar.Header, dir string) error {
	// Sanitize all tar paths resolve to within the destination directory.
	destPath := filepath.Join(dir, header.Name)
	if !strings.HasPrefix(destPath, filepath.Clean(dir)+string(os.PathSeparator)) {
		return trace.BadParameter("%s: illegal file path", header.Name)
	}

	// Ensure link destinations resolve to within the destination directory.
	if header.Linkname != "" {
		if filepath.IsAbs(header.Linkname) {
			if !strings.HasPrefix(filepath.Clean(header.Linkname), filepath.Clean(dir)+string(os.PathSeparator)) {
				return trace.BadParameter("%s: illegal link path", header.Linkname)
			}
		} else {
			// Relative paths are relative to filename after extraction to directory.
			linkPath := filepath.Join(dir, filepath.Dir(header.Name), header.Linkname)
			if !strings.HasPrefix(linkPath, filepath.Clean(dir)+string(os.PathSeparator)) {
				return trace.BadParameter("%s: illegal link path", header.Linkname)
			}
		}
	}

	return nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	err := withDir(path, func() error {
		// Create file only if it does not exist to prevent overwriting existing
		// files (like session recordings).
		out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		_, err = io.Copy(out, r)
		return trace.NewAggregate(err, out.Close())
	})
	return trace.Wrap(err)
}

func writeSymbolicLink(path string, target string) error {
	err := withDir(path, func() error {
		err := os.Symlink(target, path)
		return trace.ConvertSystemError(err)
	})
	return trace.Wrap(err)
}

func writeHardLink(path string, target string) error {
	err := withDir(path, func() error {
		err := os.Link(target, path)
		return trace.ConvertSystemError(err)
	})
	return trace.Wrap(err)
}

func withDir(path string, fn func() error) error {
	err := os.MkdirAll(filepath.Dir(path), teleport.DirMaskSharedGroup)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if fn == nil {
		return nil
	}
	err = fn()
	return trace.Wrap(err)
}
