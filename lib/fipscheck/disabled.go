//go:build !fips

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

package fipscheck

import (
	"crypto/fips140"
	"fmt"
	"os"
)

func init() {
	// This guards against a user running teleport with `GODEBUG=fips140=on` set
	// in their environment. They may be expecting this would enable FIPS140 mode
	// with Teleport, but for that they need the fips build, as there is also a
	// rust component that needs to have FIPS140 enabled.
	if fips140.Enabled() {
		fmt.Fprintln(os.Stderr, "FIPS140 mode is active in a non-FIPS build (GODEBUG=fips140=on).")
		fmt.Fprintln(os.Stderr, "Install the Teleport Enterprise FIPS edition to use FIPS140 mode.")
		os.Exit(1)
	}
}
