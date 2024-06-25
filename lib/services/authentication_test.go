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

package services_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
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

func TestValidateLocalAuthSecrets_empty(t *testing.T) {
	assert.NoError(t, services.ValidateLocalAuthSecrets(&types.LocalAuthSecrets{}))
}

func TestValidateLocalAuthSecrets_passwordHash(t *testing.T) {
	assert.NoError(t, services.ValidateLocalAuthSecrets(&types.LocalAuthSecrets{
		// bcrypt hash of "foobar"
		PasswordHash: []byte("$2y$10$d3BZ9tUDA5vD1hUL8iSfC.ADGj.WS4VRTLVtEWkZrD76pRZFJZ5f2"),
	}))

	err := services.ValidateLocalAuthSecrets(&types.LocalAuthSecrets{
		PasswordHash: []byte("$hashimpo$tor"),
	})
	assert.Error(t, err)
	assert.True(t, trace.IsBadParameter(err),
		"ValidateLocalAuthSecrets returned err=%v (%T), want BadParameter", err, trace.Unwrap(err))
}

func TestSignatureAlgorithmSuiteRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		str  string
		enum types.SignatureAlgorithmSuite
	}{
		{
			str:  "",
			enum: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
		},
		{
			str:  "legacy",
			enum: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			str:  "balanced-v1",
			enum: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		{
			str:  "fips-v1",
			enum: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		},
		{
			str:  "hsm-v1",
			enum: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		},
	} {
		t.Run(tc.str, func(t *testing.T) {
			prefs := &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					SignatureAlgorithmSuite: tc.enum,
				},
			}
			err := prefs.CheckAndSetDefaults()
			require.NoError(t, err)

			marshaled, err := services.MarshalAuthPreference(prefs)
			require.NoError(t, err)

			require.Contains(t, string(marshaled), tc.str)

			unmarshaled, err := services.UnmarshalAuthPreference(marshaled)
			require.NoError(t, err)

			require.Equal(t, tc.enum, unmarshaled.GetSignatureAlgorithmSuite())
		})
	}

}

func TestParseSignatureAlgorithmSuite(t *testing.T) {
	for _, tc := range []struct {
		json     string
		expected types.SignatureAlgorithmSuite
	}{
		{
			json:     `""`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
		},
		{
			json:     `"SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
		},
		{
			json:     `"legacy"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			json:     `"SIGNATURE_ALGORITHM_SUITE_LEGACY"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			json:     `1`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			json:     `1.0`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			json:     `"balanced-v1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		{
			json:     `"SIGNATURE_ALGORITHM_SUITE_BALANCED_V1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		{
			json:     `2`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		{
			json:     `"fips-v1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		},
		{
			json:     `"SIGNATURE_ALGORITHM_SUITE_FIPS_V1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		},
		{
			json:     `3`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		},
		{
			json:     `"hsm-v1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		},
		{
			json:     `"SIGNATURE_ALGORITHM_SUITE_HSM_V1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		},
		{
			json:     `4`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		},
	} {
		t.Run(tc.json, func(t *testing.T) {
			raw := fmt.Sprintf(`{"spec":{"signature_algorithm_suite":%s}}`, tc.json)
			unmarshaled, err := services.UnmarshalAuthPreference([]byte(raw))
			require.NoError(t, err)
			require.Equal(t, tc.expected, unmarshaled.GetSignatureAlgorithmSuite())
		})
	}
}
