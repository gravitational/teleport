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

import "github.com/gravitational/trace"

// WriteFile writes host UUID into a file
func WriteFile(dataDir string, id string) error {
	return trace.NotImplemented("host id writing is not supported on windows")
}

// ReadOrCreateFile looks for a hostid file in the data dir. If present,
// returns the UUID from it, otherwise generates one
func ReadOrCreateFile(dataDir string) (string, error) {
	return "", trace.NotImplemented("host id writing is not supported on windows")
}
