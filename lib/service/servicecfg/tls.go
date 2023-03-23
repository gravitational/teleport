// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
