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

package tdpb

import (
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// IsScreenFragment reports whether msg is a TDPB message carrying a fragment of
// the remote desktop's screen. The legacy TDP messages are intentionally not
// recognized - this code is meant to sit above the translation layer where everything
// is already TDPB.
func IsScreenFragment(msg tdp.Message) bool {
	switch msg.(type) {
	case *PNGFrame, *FastPathPDU:
		return true
	default:
		return false
	}
}
