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
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// FileName is the file name where the host UUID file is stored
	FileName = "host_uuid"
)

// GetPath returns the path to the host UUID file given the data directory.
func GetPath(dataDir string) string {
	return filepath.Join(dataDir, FileName)
}

// ExistsLocally checks if dataDir/host_uuid file exists in local storage.
func ExistsLocally(dataDir string) bool {
	_, err := ReadFile(dataDir)
	return err == nil
}

// ReadFile reads host UUID from the file in the data dir
func ReadFile(dataDir string) (string, error) {
	out, err := utils.ReadPath(GetPath(dataDir))
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			//do not convert to system error as this loses the ability to compare that it is a permission error
			return "", trace.Wrap(err)
		}
		return "", trace.ConvertSystemError(err)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", trace.NotFound("host uuid is empty")
	}
	return id, nil
}
