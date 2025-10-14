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

// ToProto returns a matching protobuf type or VerifyFull for empty value.
func (m TLSMode) ToProto() (types.DatabaseTLSMode, error) {
	switch m {
	case VerifyCA:
		return types.DatabaseTLSMode_VERIFY_CA, nil
	case Insecure:
		return types.DatabaseTLSMode_INSECURE, nil
	case VerifyFull:
		return types.DatabaseTLSMode_VERIFY_FULL, nil
	case "": // default to verify-full.
		return types.DatabaseTLSMode_VERIFY_FULL, nil
	default:
		return 0, trace.BadParameter("provided invalid TLS mode %q. Correct values are: %v", string(m), []TLSMode{VerifyFull, VerifyCA, Insecure})
	}
}
