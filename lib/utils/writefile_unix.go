// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

//go:build unix

package utils

import (
	"os"
	"path/filepath"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
)

// MaybeAtomicWriteFile mirrors [os.WriteFile] to write a file atomically
// where possible.
//
// On UNIX it will create the file atomically by writing to a temp file in
// the target directory and then moving it into place once the file is
// complete. Note that if the there is an existing symlink at `path` it will
// *not* be followed, but will be overwritten with the new file.
//
// Files cannot be reliably written atomically under Windows (See
// https://github.com/google/renameio/issues/1), so on Windows this function
// falls back to just using [os.WriteFile].
func MaybeAtomicWriteFile(path string, content []byte, perm os.FileMode) error {
	// Given that
	// 1. If there is an existing file already at `path`, renameio will preserve
	//    that file's permissions rather than use the permissions from `perm`,
	// 2. The temp file will be created using the effective permissions for the
	//    final destination file, and
	// 3. Renameio will use the current system temp dir if it is on the same
	//    volume as the destination file
	// it is possible for the creation of the temp file to leak sensitive info
	// into the system temp dir if an existing file has lax permissions but is
	// protected by tighter permissions at the directory level.
	//
	// To avoid this, we force renameio to use the destination directory as its
	// temp dir.
	tmpdir := filepath.Dir(path)
	err := renameio.WriteFile(path, content, perm, renameio.WithTempDir(tmpdir))

	return trace.Wrap(err)
}
