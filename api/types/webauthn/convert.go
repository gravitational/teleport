// Copyright 2026 Gravitational, Inc
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

package webauthnpb

import webauthnv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/webauthn/v2"

// CredentialAssertionV2ToV1 converts a v2 CredentialAssertion to v1.
func CredentialAssertionV2ToV1(v2 *webauthnv2.CredentialAssertion) *CredentialAssertion {
	if v2 == nil {
		return nil
	}

	pubKey := v2.GetPublicKey()

	return &CredentialAssertion{
		PublicKey: &PublicKeyCredentialRequestOptions{
			Challenge:        pubKey.GetChallenge(),
			TimeoutMs:        pubKey.GetTimeoutMs(),
			RpId:             pubKey.GetRpId(),
			AllowCredentials: credentialDescriptorsV2ToV1(pubKey.GetAllowCredentials()),
			UserVerification: pubKey.GetUserVerification(),
			Extensions:       inputExtensionsV2ToV1(pubKey.GetExtensions()),
		},
	}
}

// CredentialAssertionResponseV1ToV2 converts a v1 CredentialAssertionResponse to v2.
func CredentialAssertionResponseV1ToV2(v1 *CredentialAssertionResponse) *webauthnv2.CredentialAssertionResponse {
	if v1 == nil {
		return nil
	}

	resp := v1.GetResponse()

	return webauthnv2.CredentialAssertionResponse_builder{
		Type:  v1.GetType(),
		RawId: v1.GetRawId(),
		Response: webauthnv2.AuthenticatorAssertionResponse_builder{
			ClientDataJson:    resp.GetClientDataJson(),
			AuthenticatorData: resp.GetAuthenticatorData(),
			Signature:         resp.GetSignature(),
			UserHandle:        resp.GetUserHandle(),
		}.Build(),
		Extensions: outputExtensionsV1ToV2(v1.GetExtensions()),
	}.Build()
}

func credentialDescriptorsV2ToV1(v2 []*webauthnv2.CredentialDescriptor) []*CredentialDescriptor {
	if len(v2) == 0 {
		return nil
	}

	v1 := make([]*CredentialDescriptor, len(v2))

	for i, c := range v2 {
		v1[i] = &CredentialDescriptor{
			Type: c.GetType(),
			Id:   c.GetId(),
		}
	}

	return v1
}

func inputExtensionsV2ToV1(v2 *webauthnv2.AuthenticationExtensionsClientInputs) *AuthenticationExtensionsClientInputs {
	if v2 == nil {
		return nil
	}

	return &AuthenticationExtensionsClientInputs{
		AppId:     v2.GetAppId(),
		CredProps: v2.GetCredProps(),
	}
}

func outputExtensionsV1ToV2(v1 *AuthenticationExtensionsClientOutputs) *webauthnv2.AuthenticationExtensionsClientOutputs {
	if v1 == nil {
		return nil
	}

	v2 := webauthnv2.AuthenticationExtensionsClientOutputs_builder{
		AppId: v1.GetAppId(),
	}.Build()

	if credProps := v1.GetCredProps(); credProps != nil {
		v2.SetCredProps(
			webauthnv2.CredentialPropertiesOutput_builder{
				Rk: credProps.GetRk(),
			}.Build(),
		)
	}

	return v2
}
