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

//go:build fips

//go:debug fips140=on

package modules

import (
	"crypto/fips140"
	"fmt"
	"os"
)

func init() {
	if !fips140.Enabled() {
		fmt.Fprintln(
			os.Stderr,
			`Teleport FIPS builds don't allow setting the fips140 GODEBUG flag to "off". Please check your GODEBUG environment variable.`,
		)
		os.Exit(1)
	}
}

// IsBoringBinary checks if the binary was compiled with FIPS support.
func IsBoringBinary() bool {
	return true
}
