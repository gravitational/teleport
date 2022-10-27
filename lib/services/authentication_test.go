// Copyright 2022 Gravitational, Inc
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

package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateLocalAuthSecrets_deviceTypes(t *testing.T) {
	addedAt := time.Now()

	otp, err := services.NewTOTPDevice("otp", "supersecretkeyLLAMA", addedAt)
	require.NoError(t, err, "NewTOTPDevice failed")

	u2f := types.NewMFADevice("u2f", "u2fID", addedAt)
	u2f.Device = &types.MFADevice_U2F{
		U2F: &types.U2FDevice{
			KeyHandle: []byte{1, 2, 3, 4, 5}, // Contents don't matter.
			PubKey:    []byte{1, 2, 3, 4, 5},
			Counter:   1,
		},
	}

	wan := types.NewMFADevice("webauthn", "webauthbID", addedAt)
	wan.Device = &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:     []byte{1, 2, 3, 4, 5}, // Arbitrary
			PublicKeyCbor:    []byte{1, 2, 3, 4, 5}, // Arbitrary
			Aaguid:           []byte{1, 2, 3, 4, 5}, // Arbitrary
			SignatureCounter: 1,
		},
	}

	err = services.ValidateLocalAuthSecrets(&types.LocalAuthSecrets{
		MFA: []*types.MFADevice{
			otp,
			u2f,
			wan,
		},
	})
	assert.NoError(t, err, "ValidateLocalAuthSecrets failed")
}
