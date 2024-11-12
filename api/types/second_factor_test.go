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

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/constants"
)

func TestEncodeDecodeSecondFactorType(t *testing.T) {
	for _, tt := range []struct {
		secondFactorType SecondFactorType
		encoded          string
	}{
		{
			secondFactorType: SecondFactorType_SECOND_FACTOR_TYPE_OTP,
			encoded:          secondFactorTypeOTPString,
		}, {
			secondFactorType: SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
			encoded:          secondFactorTypeWebauthnString,
		}, {
			secondFactorType: SecondFactorType_SECOND_FACTOR_TYPE_SSO,
			encoded:          secondFactorTypeSSOString,
		},
	} {
		t.Run(tt.secondFactorType.String(), func(t *testing.T) {
			t.Run("encode", func(t *testing.T) {
				encoded, err := tt.secondFactorType.Encode()
				assert.NoError(t, err)
				assert.Equal(t, tt.encoded, encoded)
			})

			t.Run("decode", func(t *testing.T) {
				var decoded SecondFactorType
				err := decoded.decode(tt.encoded)
				assert.NoError(t, err)
				assert.Equal(t, tt.secondFactorType, decoded)
			})
		})
	}
}

func TestSecondFactorsFromLegacySecondFactor(t *testing.T) {
	for _, tt := range []struct {
		sf  constants.SecondFactorType
		sfs []SecondFactorType
	}{
		{
			sf:  "",
			sfs: nil,
		},
		{
			sf:  constants.SecondFactorOff,
			sfs: nil,
		},
		{
			sf:  constants.SecondFactorOptional,
			sfs: []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_OTP, SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN},
		},
		{
			sf:  constants.SecondFactorOn,
			sfs: []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_OTP, SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN},
		},
		{
			sf:  constants.SecondFactorOTP,
			sfs: []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_OTP},
		},
		{
			sf:  constants.SecondFactorWebauthn,
			sfs: []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN},
		},
	} {
		t.Run(string(tt.sf), func(t *testing.T) {
			assert.Equal(t, tt.sfs, secondFactorsFromLegacySecondFactor(tt.sf))
		})
	}
}

func TestLegacySecondFactorFromSecondFactors(t *testing.T) {
	for _, tt := range []struct {
		sfs []SecondFactorType
		sf  constants.SecondFactorType
	}{
		{
			sfs: nil,
			sf:  constants.SecondFactorOff,
		},
		{
			sfs: []SecondFactorType{},
			sf:  constants.SecondFactorOff,
		},
		{
			sfs: []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_OTP},
			sf:  constants.SecondFactorOTP,
		},
		{
			sfs: []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN},
			sf:  constants.SecondFactorWebauthn,
		},
		{
			sfs: []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_SSO},
			sf:  constants.SecondFactorOff,
		},
		{
			sfs: []SecondFactorType{
				SecondFactorType_SECOND_FACTOR_TYPE_OTP,
				SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
				SecondFactorType_SECOND_FACTOR_TYPE_SSO,
			},
			sf: constants.SecondFactorOn,
		},
	} {
		t.Run(string(tt.sfs), func(t *testing.T) {
			assert.Equal(t, tt.sf, LegacySecondFactorFromSecondFactors(tt.sfs))
		})
	}
}
