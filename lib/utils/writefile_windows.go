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

//go:build windows

package utils

import (
	"os"

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
	return trace.Wrap(os.WriteFile(path, content, perm))
}
