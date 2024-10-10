/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

// TestMarshalUnmarshalRequireMFAType tests encoding/decoding of the RequireMFAType.
func TestEncodeDecodeRequireMFAType(t *testing.T) {
	for _, tt := range []struct {
		requireMFAType RequireMFAType
		encoded        any
	}{
		{
			requireMFAType: RequireMFAType_OFF,
			encoded:        false,
		}, {
			requireMFAType: RequireMFAType_SESSION,
			encoded:        true,
		}, {
			requireMFAType: RequireMFAType_SESSION_AND_HARDWARE_KEY,
			encoded:        RequireMFATypeHardwareKeyString,
		}, {
			requireMFAType: RequireMFAType_HARDWARE_KEY_TOUCH,
			encoded:        RequireMFATypeHardwareKeyTouchString,
		}, {
			requireMFAType: RequireMFAType_HARDWARE_KEY_PIN,
			encoded:        RequireMFATypeHardwareKeyPINString,
		}, {
			requireMFAType: RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN,
			encoded:        RequireMFATypeHardwareKeyTouchAndPINString,
		},
	} {
		t.Run(tt.requireMFAType.String(), func(t *testing.T) {
			t.Run("encode", func(t *testing.T) {
				encoded, err := tt.requireMFAType.encode()
				require.NoError(t, err)
				require.Equal(t, tt.encoded, encoded)
			})

			t.Run("decode", func(t *testing.T) {
				var decoded RequireMFAType
				err := decoded.decode(tt.encoded)
				require.NoError(t, err)
				require.Equal(t, tt.requireMFAType, decoded)
			})
		})
	}
}

func TestNewAuthPreference_secondFactors(t *testing.T) {
	for _, tt := range []struct {
		name           string
		spec           AuthPreferenceSpecV2
		assertErr      require.ErrorAssertionFunc
		assertAuthPref func(t *testing.T, authPref AuthPreference)
	}{
		{
			name: "OK default to OTP",
			spec: AuthPreferenceSpecV2{},
			assertAuthPref: func(t *testing.T, authPref AuthPreference) {
				assert.Equal(t, []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_OTP}, authPref.GetSecondFactors())
			},
		},
		{
			name: "OK OTP default settings",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_OTP,
				},
			},
			assertAuthPref: func(t *testing.T, authPref AuthPreference) {
				assert.False(t, authPref.GetAllowPasswordless())
				assert.False(t, authPref.GetAllowHeadless())
				assert.True(t, authPref.GetAllowLocalAuth())
				assert.False(t, authPref.IsAdminActionMFAEnforced())
			},
		},
		{
			name: "OK WebAuthn default settings",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
				},
				Webauthn: &Webauthn{
					RPID: "localhost",
				},
			},
			assertAuthPref: func(t *testing.T, authPref AuthPreference) {
				assert.True(t, authPref.GetAllowPasswordless())
				assert.True(t, authPref.GetAllowHeadless())
				assert.True(t, authPref.GetAllowLocalAuth())
				assert.True(t, authPref.IsAdminActionMFAEnforced())
			},
		},
		{
			name: "OK SSO default settings",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				AllowLocalAuth: NewBoolOption(false),
			},
			assertAuthPref: func(t *testing.T, authPref AuthPreference) {
				assert.False(t, authPref.GetAllowPasswordless())
				assert.False(t, authPref.GetAllowHeadless())
				assert.False(t, authPref.GetAllowLocalAuth())
				assert.True(t, authPref.IsAdminActionMFAEnforced())
			},
		},
		{
			name: "OK all second factors",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_OTP,
					SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
					SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				Webauthn: &Webauthn{
					RPID: "localhost",
				},
			},
			assertAuthPref: func(t *testing.T, authPref AuthPreference) {
				assert.True(t, authPref.GetAllowPasswordless())
				assert.True(t, authPref.GetAllowHeadless())
				assert.True(t, authPref.GetAllowLocalAuth())
				// enabling OTP disables admin mfa.
				assert.False(t, authPref.IsAdminActionMFAEnforced())
			},
		},
		{
			name: "OK U2F config provided",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
				},
				U2F: &U2F{
					AppID: "https://localhost",
				},
			},
			assertAuthPref: func(t *testing.T, authPref AuthPreference) {
				w, err := authPref.GetWebauthn()
				assert.NoError(t, err)
				assert.Equal(t, &Webauthn{RPID: "localhost"}, w)
			},
		},
		{
			name: "NOK SecondFactor and SecondFactors both set",
			spec: AuthPreferenceSpecV2{
				SecondFactor: constants.SecondFactorWebauthn,
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
				},
			},
			assertErr: func(t require.TestingT, err error, vals ...interface{}) {
				assert.ErrorAs(t, err, new(*trace.BadParameterError))
			},
		},
		{
			name: "NOK WebAuthn config missing",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
				},
			},
			assertErr: func(t require.TestingT, err error, vals ...interface{}) {
				assert.ErrorAs(t, err, new(*trace.BadParameterError))
			},
		},
		{
			name: "NOK prevent passwordless without WebAuthn",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_OTP,
				},
				AllowPasswordless: NewBoolOption(true),
			},
			assertErr: func(t require.TestingT, err error, vals ...interface{}) {
				assert.ErrorAs(t, err, new(*trace.BadParameterError))
			},
		},
		{
			name: "NOK prevent headless without WebAuthn",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_OTP,
				},
				AllowHeadless: NewBoolOption(true),
			},
			assertErr: func(t require.TestingT, err error, vals ...interface{}) {
				assert.ErrorAs(t, err, new(*trace.BadParameterError))
			},
		},
		{
			name: "NOK prevent local lockout with second factor SSO",
			spec: AuthPreferenceSpecV2{
				SecondFactors: []SecondFactorType{
					SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				AllowLocalAuth: NewBoolOption(true),
			},
			assertErr: func(t require.TestingT, err error, vals ...interface{}) {
				assert.ErrorAs(t, err, new(*trace.BadParameterError))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			authPref, err := NewAuthPreference(tt.spec)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.assertAuthPref != nil {
				tt.assertAuthPref(t, authPref)
			}
		})
	}
}
