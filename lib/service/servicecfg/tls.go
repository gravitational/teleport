/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package servicecfg

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// TLSMode defines all possible database verification modes.
type TLSMode string

const (
	// VerifyFull is the strictest. Verifies certificate and server name.
	VerifyFull TLSMode = "verify-full"
	// VerifyCA checks the certificate, but skips the server name verification.
	VerifyCA TLSMode = "verify-ca"
	// Insecure accepts any certificate.
	Insecure TLSMode = "insecure"
)

// AllTLSModes keeps all possible database TLS modes for easy access.
var AllTLSModes = []TLSMode{VerifyFull, VerifyCA, Insecure}

// CheckAndSetDefaults check if TLSMode holds a correct value. If the value is not set
// VerifyFull is set as a default. BadParameter error is returned if value set is incorrect.
func (m *TLSMode) CheckAndSetDefaults() error {
	switch *m {
	case "": // Use VerifyFull if not set.
		*m = VerifyFull
	case VerifyFull, VerifyCA, Insecure:
		// Correct value, do nothing.
	default:
		return trace.BadParameter("provided incorrect TLSMode value. Correct values are: %v", AllTLSModes)
	}

	return nil
}

// ToProto returns a matching protobuf type or VerifyFull for empty value.
func (m TLSMode) ToProto() types.DatabaseTLSMode {
	switch m {
	case VerifyCA:
		return types.DatabaseTLSMode_VERIFY_CA
	case Insecure:
		return types.DatabaseTLSMode_INSECURE
	default: // VerifyFull
		return types.DatabaseTLSMode_VERIFY_FULL
	}
}
