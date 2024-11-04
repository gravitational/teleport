/*
Copyright 2024 Gravitational, Inc.

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
	"encoding/json"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// secondFactorsFromLegacySecondFactor returns the list of SecondFactorTypes supported by the given second factor type.
func secondFactorsFromLegacySecondFactor(sf constants.SecondFactorType) []SecondFactorType {
	switch sf {
	case constants.SecondFactorOff:
		return nil
	case constants.SecondFactorOptional, constants.SecondFactorOn:
		return []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_OTP, SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN}
	case constants.SecondFactorOTP:
		return []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_OTP}
	case constants.SecondFactorWebauthn:
		return []SecondFactorType{SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN}
	default:
		return nil
	}
}

// LegacySecondFactorFromSecondFactors returns a suitable legacy second factor for the given list of second factors.
func LegacySecondFactorFromSecondFactors(secondFactors []SecondFactorType) constants.SecondFactorType {
	hasOTP := slices.Contains(secondFactors, SecondFactorType_SECOND_FACTOR_TYPE_OTP)
	hasWebAuthn := slices.Contains(secondFactors, SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN)
	hasSSO := slices.Contains(secondFactors, SecondFactorType_SECOND_FACTOR_TYPE_SSO)

	switch {
	case hasOTP && hasWebAuthn:
		return constants.SecondFactorOn
	case hasWebAuthn:
		return constants.SecondFactorWebauthn
	case hasOTP:
		return constants.SecondFactorOTP
	case hasSSO:
		// In the WebUI, we can treat exclusive SSO MFA as disabled. In practice this means
		// things like the "add MFA device" button is disabled, but SSO MFA prompts will still work.
		// TODO(Joerger): Ensure that SSO MFA flows work in the WebUI with this change, once implemented.
		return constants.SecondFactorOff
	default:
		return constants.SecondFactorOff
	}
}

// MarshalJSON marshals SecondFactorType to string.
func (s *SecondFactorType) MarshalYAML() (interface{}, error) {
	val, err := s.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return val, nil
}

// UnmarshalYAML supports parsing SecondFactorType from string.
func (s *SecondFactorType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val interface{}
	err := unmarshal(&val)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.decode(val)
	return trace.Wrap(err)
}

// MarshalJSON marshals SecondFactorType to string.
func (s *SecondFactorType) MarshalJSON() ([]byte, error) {
	val, err := s.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := json.Marshal(val)
	return out, trace.Wrap(err)
}

// UnmarshalJSON supports parsing SecondFactorType from string.
func (s *SecondFactorType) UnmarshalJSON(data []byte) error {
	var val interface{}
	err := json.Unmarshal(data, &val)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.decode(val)
	return trace.Wrap(err)
}

const (
	// secondFactorTypeOTPString is the string representation of SecondFactorType_SECOND_FACTOR_TYPE_OTP
	secondFactorTypeOTPString = "otp"
	// secondFactorTypeWebauthnString is the string representation of SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN
	secondFactorTypeWebauthnString = "webauthn"
	// secondFactorTypeSSOString is the string representation of SecondFactorType_SECOND_FACTOR_TYPE_SSO
	secondFactorTypeSSOString = "sso"
)

// Encode encodes the SecondFactorType in string form.
func (s *SecondFactorType) Encode() (string, error) {
	switch *s {
	case SecondFactorType_SECOND_FACTOR_TYPE_UNSPECIFIED:
		return "", nil
	case SecondFactorType_SECOND_FACTOR_TYPE_OTP:
		return secondFactorTypeOTPString, nil
	case SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN:
		return secondFactorTypeWebauthnString, nil
	case SecondFactorType_SECOND_FACTOR_TYPE_SSO:
		return secondFactorTypeSSOString, nil
	default:
		return "", trace.BadParameter("invalid SecondFactorType value %v", *s)
	}
}

func (s *SecondFactorType) decode(val any) error {
	switch v := val.(type) {
	case string:
		switch v {
		case secondFactorTypeOTPString:
			*s = SecondFactorType_SECOND_FACTOR_TYPE_OTP
		case secondFactorTypeWebauthnString:
			*s = SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN
		case secondFactorTypeSSOString:
			*s = SecondFactorType_SECOND_FACTOR_TYPE_SSO
		case "":
			*s = SecondFactorType_SECOND_FACTOR_TYPE_UNSPECIFIED
		default:
			return trace.BadParameter("invalid SecondFactorType value %v", val)
		}
	case int32:
		return trace.Wrap(s.setFromEnum(v))
	case int64:
		return trace.Wrap(s.setFromEnum(int32(v)))
	case int:
		return trace.Wrap(s.setFromEnum(int32(v)))
	case float64:
		return trace.Wrap(s.setFromEnum(int32(v)))
	case float32:
		return trace.Wrap(s.setFromEnum(int32(v)))
	default:
		return trace.BadParameter("invalid SecondFactorType type %T", val)
	}
	return nil
}

// setFromEnum sets the value from enum value as int32.
func (s *SecondFactorType) setFromEnum(val int32) error {
	if _, ok := SecondFactorType_name[val]; !ok {
		return trace.BadParameter("invalid SecondFactorType enum %v", val)
	}
	*s = SecondFactorType(val)
	return nil
}
