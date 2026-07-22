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
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateLocalAuthSecrets_deviceTypes(t *testing.T) {
	addedAt := time.Now()

	otp, err := services.NewTOTPDevice("otp", "supersecretkeyLLAMA", addedAt)
	require.NoError(t, err, "NewTOTPDevice failed")

	u2f, err := types.NewMFADevice("u2f", "u2fID", addedAt, &types.MFADevice_U2F{
		U2F: &types.U2FDevice{
			KeyHandle: []byte{1, 2, 3, 4, 5}, // Contents don't matter.
			PubKey:    []byte{1, 2, 3, 4, 5},
			Counter:   1,
		},
	})
	require.NoError(t, err, "NewMFADevice failed")

	wan, err := types.NewMFADevice("webauthn", "webauthbID", addedAt, &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:     []byte{1, 2, 3, 4, 5}, // Arbitrary
			PublicKeyCbor:    []byte{1, 2, 3, 4, 5}, // Arbitrary
			Aaguid:           []byte{1, 2, 3, 4, 5}, // Arbitrary
			SignatureCounter: 1,
		},
	})
	require.NoError(t, err, "NewMFADevice failed")

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
		rawValue string
		expected types.SignatureAlgorithmSuite
	}{
		{
			rawValue: `""`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
		},
		{
			rawValue: `"SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED,
		},
		{
			rawValue: `"legacy"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			rawValue: `"SIGNATURE_ALGORITHM_SUITE_LEGACY"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			rawValue: `1`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			rawValue: `1.0`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_LEGACY,
		},
		{
			rawValue: `"balanced-v1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		{
			rawValue: `"SIGNATURE_ALGORITHM_SUITE_BALANCED_V1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		{
			rawValue: `2`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		{
			rawValue: `"fips-v1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		},
		{
			rawValue: `"SIGNATURE_ALGORITHM_SUITE_FIPS_V1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		},
		{
			rawValue: `3`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_FIPS_V1,
		},
		{
			rawValue: `"hsm-v1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		},
		{
			rawValue: `"SIGNATURE_ALGORITHM_SUITE_HSM_V1"`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		},
		{
			rawValue: `4`,
			expected: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1,
		},
	} {
		t.Run(tc.rawValue, func(t *testing.T) {
			t.Run("json", func(t *testing.T) {
				rawJSON := fmt.Sprintf(`{"spec":{"signature_algorithm_suite":%s}}`, tc.rawValue)
				unmarshaled, err := services.UnmarshalAuthPreference([]byte(rawJSON))
				require.NoError(t, err)
				require.Equal(t, tc.expected, unmarshaled.GetSignatureAlgorithmSuite())
			})

			t.Run("yaml", func(t *testing.T) {
				rawYAML := fmt.Sprintf(`spec: { signature_algorithm_suite: %s }`, tc.rawValue)

				decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(rawYAML), defaults.LookaheadBufSize)
				var authPref types.AuthPreferenceV2
				err := decoder.Decode(&authPref)
				require.NoError(t, err)
				require.Equal(t, tc.expected, authPref.GetSignatureAlgorithmSuite())
			})
		})
	}
}
