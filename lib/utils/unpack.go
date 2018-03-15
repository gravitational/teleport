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
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Extract extracts the contents of the specified tarball under dir.
// The resulting files and directories are created using the current user context.
func Extract(r io.Reader, dir string) error {
	tarball := tar.NewReader(r)

	for {
		header, err := tarball.Next()
		if err == io.EOF {
			break
		} else if err != nil {
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
	case tar.TypeBlock, tar.TypeChar, tar.TypeReg, tar.TypeRegA, tar.TypeFifo:
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

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	err := withDir(path, func() error {
		out, err := os.Create(path)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		defer out.Close()

		err = out.Chmod(mode)
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		_, err = io.Copy(out, r)
		return trace.Wrap(err)
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
