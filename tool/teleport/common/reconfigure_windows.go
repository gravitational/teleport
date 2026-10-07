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

package common

import (
	"os"

	"github.com/gravitational/teleport/lib/service"
)

func overwriteFile(filename string, data []byte) (err error) {
	return service.WriteOwnerRestrictedFile(filename, data)
}

// createExclusiveFile creates path for writing, failing if it already exists.
// The file is created with an owner-only DACL in the same call, so there is
// no window where it exists with the ACL inherited from the parent directory.
func createExclusiveFile(path string) (*os.File, error) {
	return service.CreateOwnerRestrictedFile(path)
}
