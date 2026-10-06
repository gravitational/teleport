// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package hostid

import (
	"os"

	"github.com/gravitational/trace"
)

// WriteFile writes host UUID into a file
func WriteFile(dataDir string, id string) error {
	f, err := os.CreateTemp(dataDir, FileName+".tmp")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	tmpName := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = f.Write([]byte(id)); err != nil {
		return trace.ConvertSystemError(err)
	}
	// Flush the contents before the move, so that a crash can't leave filename
	// pointing at a file whose data never reached the disk.
	if err = f.Sync(); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err = f.Close(); err != nil {
		return trace.ConvertSystemError(err)
	}

	// there's no way to do atomic rename in Windows, we do best effort here
	err = os.Rename(tmpName, GetPath(dataDir))
	return trace.ConvertSystemError(err)
}
