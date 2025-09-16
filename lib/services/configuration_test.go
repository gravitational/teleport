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
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestAuthPreferenceValidate(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, ValidateAuthPreference(types.DefaultAuthPreference()))
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
				spec:  types.AuthPreferenceSpecV2{SecondFactor: constants.SecondFactorOff},
				check: disableSecondFactorAssertion,
			},
			{
				name:   "cloud prevents disabling",
				bypass: true,
				spec:   types.AuthPreferenceSpecV2{SecondFactor: constants.SecondFactorOff},
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
				name:   "bypass self hosted",
				spec:   types.AuthPreferenceSpecV2{SecondFactor: constants.SecondFactorOff},
				check:  require.NoError,
				bypass: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					t.Setenv(teleport.EnvVarAllowNoSecondFactor, "true")
				}

				modulestest.SetTestModules(t, modulestest.Modules{
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
