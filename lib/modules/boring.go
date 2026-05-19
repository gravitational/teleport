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

//go:build boringcrypto

package modules

import "crypto/boring"

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
//
// It's possible to enable the boringcrypto GOEXPERIMENT (which will enable the
// boringcrypto build tag) even on platforms that don't support the boringcrypto
// module, which results in crypto packages being available and working, but not
// actually using a certified cryptographic module, so we have to check
// [boring.Enabled] even if this is compiled in.
func IsBoringBinary() bool {
	return boring.Enabled()
}
