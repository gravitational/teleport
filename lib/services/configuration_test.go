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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

func TestAuthPreferenceValidate(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateAuthPreference(types.DefaultAuthPreference()))
	})

	t.Run("stable_unix_users", func(t *testing.T) {
		t.Parallel()

		type testCase struct {
			name   string
			config *types.StableUNIXUserConfig
			check  require.ErrorAssertionFunc
		}

		testCases := []testCase{
			{
				name:   "missing",
				config: nil,
				check:  require.NoError,
			},
			{
				name: "disabled",
				config: &types.StableUNIXUserConfig{
					Enabled: false,
				},
				check: require.NoError,
			},
			{
				name: "enabled",
				config: &types.StableUNIXUserConfig{
					Enabled:  true,
					FirstUid: 30000,
					LastUid:  40000,
				},
				check: require.NoError,
			},
			{
				name: "empty_range",
				config: &types.StableUNIXUserConfig{
					Enabled:  true,
					FirstUid: 30000,
					LastUid:  29000,
				},
				check: require.Error,
			},
			{
				name: "empty_range_disabled",
				config: &types.StableUNIXUserConfig{
					Enabled:  false,
					FirstUid: 30000,
					LastUid:  29000,
				},
				check: require.NoError,
			},
			{
				name: "system_range",
				config: &types.StableUNIXUserConfig{
					Enabled:  true,
					FirstUid: 100,
					LastUid:  40000,
				},
				check: require.Error,
			},
			{
				name: "negative_range",
				config: &types.StableUNIXUserConfig{
					Enabled:  true,
					FirstUid: -100,
					LastUid:  40000,
				},
				check: require.Error,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				authPref := &types.AuthPreferenceV2{
					Spec: types.AuthPreferenceSpecV2{
						StableUnixUserConfig: tc.config,
						SecondFactors:        []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_OTP},
					},
				}
				tc.check(t, ValidateAuthPreference(authPref))
				tc.check(t, ValidateStableUNIXUserConfig(authPref.Spec.StableUnixUserConfig))
			})
		}
	})

	disableSecondFactorAssertion := func(t require.TestingT, err error, a ...any) {
		require.ErrorIs(t, err, modules.ErrCannotDisableSecondFactor, a...)
	}
	t.Run("second_factors", func(t *testing.T) {
		type testCase struct {
			name     string
			spec     types.AuthPreferenceSpecV2
			features modules.Features
			bypass   bool
			check    require.ErrorAssertionFunc
		}

		testCases := []testCase{
			{
				name:  "disabling prevented",
				check: disableSecondFactorAssertion,
			},
			{
				name:   "cloud prevents disabling",
				bypass: true,
				features: modules.Features{
					Cloud: true,
				},
				check: disableSecondFactorAssertion,
			},
			{
				name: "webauthn allowed",
				spec: types.AuthPreferenceSpecV2{
					SecondFactor: constants.SecondFactorWebauthn,
					Webauthn: &types.Webauthn{
						RPID: "test.example.com",
					},
				},
				check: require.NoError,
			},
			{
				name: "otp allowed",
				spec: types.AuthPreferenceSpecV2{
					SecondFactor: constants.SecondFactorOTP,
				},
				check: require.NoError,
			},
			{
				name: "sso and webauthn allowed",
				spec: types.AuthPreferenceSpecV2{
					SecondFactors: []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_SSO, types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN},
				},
				check: require.NoError,
			},
			{
				name:   "bypass self hosted",
				check:  require.NoError,
				bypass: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					t.Setenv(teleport.EnvVarAllowNoSecondFactor, "true")
				}

				modules.SetTestModules(t, &modules.TestModules{
					TestFeatures: tc.features,
				})

				authPref := &types.AuthPreferenceV2{
					Spec: tc.spec,
				}
				tc.check(t, ValidateAuthPreference(authPref))
			})
		}
	})
}
