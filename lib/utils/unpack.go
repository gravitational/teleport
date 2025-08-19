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
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// Extract extracts the contents of the specified tarball under dir. The
// resulting files and directories are created using the current user context.
// Extract will only unarchive files into dir, and will fail if the tarball
// tries to write files outside of dir.
//
// If any paths are specified, only the specified paths are extracted.
// The destination specified in the first matching path is selected.
func Extract(r io.Reader, dir string, paths ...ExtractPath) error {
	tarball := tar.NewReader(r)

	for {
		header, err := tarball.Next()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return trace.Wrap(err)
		}
		dirMode, ok := filterHeader(header, paths)
		if !ok {
			continue
		}
		err = sanitizeTarPath(header, dir)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := extractFile(tarball, header, dir, dirMode); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ExtractPath specifies a path to be extracted.
type ExtractPath struct {
	// Src path and Dst path within the archive to extract files to.
	// Directories in the Src path are not included in the extraction dir.
	// For example, given foo/bar/file.txt with Src=foo/bar Dst=baz, baz/file.txt results.
	// Trailing slashes are always ignored.
	Src, Dst string
	// Skip extracting the Src path and ignore Dst.
	Skip bool
	// DirMode is the file mode for implicit parent directories in Dst.
	DirMode os.FileMode
}

// filterHeader modifies the tar header by filtering it through the ExtractPaths.
// filterHeader returns false if the tar header should be skipped.
// If no paths are provided, filterHeader assumes the header should be included, and sets
// the mode for implicit parent directories to teleport.DirMaskSharedGroup.
func filterHeader(hdr *tar.Header, paths []ExtractPath) (dirMode os.FileMode, include bool) {
	name := path.Clean(hdr.Name)
	for _, p := range paths {
		src := path.Clean(p.Src)
		switch hdr.Typeflag {
		case tar.TypeDir:
			// If name is a directory, then
			// assume src is a directory prefix, or the directory itself,
			// and replace that prefix with dst.
			if src != "/" {
				src += "/" // ensure HasPrefix does not match partial names
			}
			if !strings.HasPrefix(name, src) {
				continue
			}
			dst := path.Join(p.Dst, strings.TrimPrefix(name, src))
			if dst != "/" {
				dst += "/" // tar directory headers end in /
			}
			hdr.Name = dst
			return p.DirMode, !p.Skip
		default:
			// If name is a file, then
			// if src is an exact match to the file name, assume src is a file and write directly to dst,
			// otherwise, assume src is a directory prefix, and replace that prefix with dst.
			if src == name {
				hdr.Name = path.Clean(p.Dst)
				return p.DirMode, !p.Skip
			}
			if src != "/" {
				src += "/" // ensure HasPrefix does not match partial names
			}
			if !strings.HasPrefix(name, src) {
				continue
			}
			hdr.Name = path.Join(p.Dst, strings.TrimPrefix(name, src))
			return p.DirMode, !p.Skip

		}
	}
	return teleport.DirMaskSharedGroup, len(paths) == 0
}

// extractFile extracts a single file or directory from tarball into dir.
// Uses header to determine the type of item to create
// Based on https://github.com/mholt/archiver
func extractFile(tarball *tar.Reader, header *tar.Header, dir string, dirMode os.FileMode) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return withDir(filepath.Join(dir, header.Name), dirMode, nil)
	case tar.TypeBlock, tar.TypeChar, tar.TypeReg, tar.TypeFifo:
		return writeFile(filepath.Join(dir, header.Name), tarball, header.FileInfo().Mode(), dirMode)
	case tar.TypeLink:
		return writeHardLink(filepath.Join(dir, header.Name), filepath.Join(dir, header.Linkname), dirMode)
	case tar.TypeSymlink:
		return writeSymbolicLink(filepath.Join(dir, header.Name), header.Linkname, dirMode)
	default:
		slog.WarnContext(context.Background(), "Unsupported type flag for tarball",
			"type_flag", header.Typeflag,
			"header", header.Name,
		)
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

func writeFile(path string, r io.Reader, mode, dirMode os.FileMode) error {
	err := withDir(path, dirMode, func() error {
		// Create file only if it does not exist to prevent overwriting existing
		// files (like session recordings).
		out, err := CreateExclusiveFile(path, mode)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		_, err = io.Copy(out, r)
		return trace.NewAggregate(err, out.Close())
	})
	return trace.Wrap(err)
}

func writeSymbolicLink(path, target string, dirMode os.FileMode) error {
	err := withDir(path, dirMode, func() error {
		err := os.Symlink(target, path)
		return trace.ConvertSystemError(err)
	})
	return trace.Wrap(err)
}

func writeHardLink(path, target string, dirMode os.FileMode) error {
	err := withDir(path, dirMode, func() error {
		err := os.Link(target, path)
		return trace.ConvertSystemError(err)
	})
	return trace.Wrap(err)
}

func withDir(path string, mode os.FileMode, fn func() error) error {
	err := os.MkdirAll(filepath.Dir(path), mode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if fn == nil {
		return nil
	}
	err = fn()
	return trace.Wrap(err)
}
