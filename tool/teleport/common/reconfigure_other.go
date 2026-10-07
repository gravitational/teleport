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

//go:build !windows

package common

import (
	"os"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

func overwriteFile(path string, data []byte) error {
	// Temp file + atomic rename, forcing owner-only permissions:
	// renameio.WriteFile would keep an existing file's broader mode.
	pending, err := renameio.NewPendingFile(path, renameio.WithPermissions(teleport.FileMaskOwnerOnly))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer pending.Cleanup()
	if _, err := pending.Write(data); err != nil {
		return trace.ConvertSystemError(err)
	}
	return trace.ConvertSystemError(pending.CloseAtomicallyReplace())
}

// createExclusiveFile creates path with owner-only permissions, failing if it
// already exists. O_EXCL atomically rejects anything already at the path,
// symlinks included.
func createExclusiveFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, teleport.FileMaskOwnerOnly)
}
