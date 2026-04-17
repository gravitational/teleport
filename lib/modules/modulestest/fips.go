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

//go:build !boringcrypto

package modulestest

import (
	"testing"

	"github.com/gravitational/teleport/lib/modules"
)

// SetFIPS sets the value to be returned by modules.IsFIPSBuild for the
// purposes of testing FIPS-related functionality.
func SetFIPS(t *testing.T, isFIPS bool) {
	t.Helper()
	t.Setenv("TELEPORT_TEST_NOT_SAFE_FOR_PARALLEL", "true")
	t.Cleanup(func() { modules.FIPSTestOverride = nil })
	modules.FIPSTestOverride = &isFIPS
}
