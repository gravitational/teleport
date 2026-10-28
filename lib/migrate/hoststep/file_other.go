//go:build !windows

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

package hoststep

import (
	"path/filepath"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
)

// writePrivate atomically replaces p with body at mode 0600, whatever mode p
// had. The temp file is beside p so the rename stays on one filesystem.
func writePrivate(p string, body []byte) error {
	f, err := renameio.NewPendingFile(p, renameio.WithTempDir(filepath.Dir(p)), renameio.WithPermissions(0o600), renameio.WithStaticPermissions(0o600))
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Cleanup()
	if _, err := f.Write(body); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(f.CloseAtomicallyReplace())
}
