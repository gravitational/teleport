// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package devicetpm

import "github.com/google/go-attestation/attest"

// OpenTPM calls [attest.OpenTPM] using TPM version 2.0.
func OpenTPM(config *attest.OpenConfig) (*attest.TPM, error) {
	return attest.OpenTPM(config)
}

// ActivationParameters prepares ap for TPM version 2.0.
func ActivationParameters(ap *attest.ActivationParameters) *attest.ActivationParameters {
	return ap
}

// ParseAKPublic calls [attest.ParseAKPublic] using TPM version 2.0.
func ParseAKPublic(public []byte) (*attest.AKPublic, error) {
	return attest.ParseAKPublic(public)
}
