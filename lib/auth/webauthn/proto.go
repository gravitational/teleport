// Copyright 2021 Gravitational, Inc
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

package webauthn

import (
	"encoding/base64"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"

	wantypes "github.com/gravitational/teleport/api/types/webauthn"
)

// CredentialAssertionToProto converts a CredentialAssertion to its proto
// counterpart.
func CredentialAssertionToProto(assertion *CredentialAssertion) *wantypes.CredentialAssertion {
	if assertion == nil {
		return nil
	}
	return &wantypes.CredentialAssertion{
		PublicKey: &wantypes.PublicKeyCredentialRequestOptions{
			Challenge:        assertion.Response.Challenge,
			TimeoutMs:        int64(assertion.Response.Timeout),
			RpId:             assertion.Response.RelyingPartyID,
			AllowCredentials: credentialDescriptorsToProto(assertion.Response.AllowedCredentials),
			Extensions:       inputExtensionsToProto(assertion.Response.Extensions),
		},
	}
}

// CredentialAssertionResponseToProto converts a CredentialAssertionResponse to
// its proto counterpart.
func CredentialAssertionResponseToProto(car *CredentialAssertionResponse) *wantypes.CredentialAssertionResponse {
	if car == nil {
		return nil
	}
	return &wantypes.CredentialAssertionResponse{
		Type:  car.Type,
		RawId: car.RawID,
		Response: &wantypes.AuthenticatorAssertionResponse{
			ClientDataJson:    car.AssertionResponse.ClientDataJSON,
			AuthenticatorData: car.AssertionResponse.AuthenticatorData,
			Signature:         car.AssertionResponse.Signature,
			UserHandle:        car.AssertionResponse.UserHandle,
		},
		Extensions: outputExtensionsToProto(car.Extensions),
	}
}

// CredentialCreationToProto converts a CredentialCreation to its proto
// counterpart.
func CredentialCreationToProto(cc *CredentialCreation) *wantypes.CredentialCreation {
	if cc == nil {
		return nil
	}
	return &wantypes.CredentialCreation{
		PublicKey: &wantypes.PublicKeyCredentialCreationOptions{
			Challenge:            cc.Response.Challenge,
			Rp:                   rpEntityToProto(cc.Response.RelyingParty),
			User:                 userEntityToProto(cc.Response.User),
			CredentialParameters: credentialParametersToProto(cc.Response.Parameters),
			TimeoutMs:            int64(cc.Response.Timeout),
			ExcludeCredentials:   credentialDescriptorsToProto(cc.Response.CredentialExcludeList),
			Attestation:          string(cc.Response.Attestation),
			Extensions:           inputExtensionsToProto(cc.Response.Extensions),
		},
	}
}

// CredentialCreationResponseToProto converts a CredentialCreationResponse to
// its proto counterpart.
func CredentialCreationResponseToProto(ccr *CredentialCreationResponse) *wantypes.CredentialCreationResponse {
	if ccr == nil {
		return nil
	}
	return &wantypes.CredentialCreationResponse{
		Type:  ccr.Type,
		RawId: ccr.RawID,
		Response: &wantypes.AuthenticatorAttestationResponse{
			ClientDataJson:    ccr.AttestationResponse.ClientDataJSON,
			AttestationObject: ccr.AttestationResponse.AttestationObject,
		},
		Extensions: outputExtensionsToProto(ccr.Extensions),
	}
}

// CredentialAssertionFromProto converts a CredentialAssertion proto to its lib
// counterpart.
func CredentialAssertionFromProto(assertion *wantypes.CredentialAssertion) *CredentialAssertion {
	if assertion == nil {
		return nil
	}
	return &CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:          assertion.PublicKey.Challenge,
			Timeout:            int(assertion.PublicKey.TimeoutMs),
			RelyingPartyID:     assertion.PublicKey.RpId,
			AllowedCredentials: credentialDescriptorsFromProto(assertion.PublicKey.AllowCredentials),
			Extensions:         inputExtensionsFromProto(assertion.PublicKey.Extensions),
		},
	}
}

// CredentialAssertionResponseFromProto converts a CredentialAssertionResponse
// proto to its lib counterpart.
func CredentialAssertionResponseFromProto(car *wantypes.CredentialAssertionResponse) *CredentialAssertionResponse {
	if car == nil {
		return nil
	}
	return &CredentialAssertionResponse{
		PublicKeyCredential: PublicKeyCredential{
			Credential: Credential{
				ID:   base64.RawURLEncoding.EncodeToString(car.RawId),
				Type: car.Type,
			},
			RawID:      car.RawId,
			Extensions: outputExtensionsFromProto(car.Extensions),
		},
		AssertionResponse: AuthenticatorAssertionResponse{
			AuthenticatorResponse: AuthenticatorResponse{
				ClientDataJSON: car.Response.ClientDataJson,
			},
			AuthenticatorData: car.Response.AuthenticatorData,
			Signature:         car.Response.Signature,
			UserHandle:        car.Response.UserHandle,
		},
	}
}

// CredentialCreationFromProto converts a CredentialCreation proto to its lib
// counterpart.
func CredentialCreationFromProto(cc *wantypes.CredentialCreation) *CredentialCreation {
	if cc == nil {
		return nil
	}
	if cc.PublicKey == nil {
		return &CredentialCreation{}
	}
	return &CredentialCreation{
		Response: protocol.PublicKeyCredentialCreationOptions{
			Challenge:             cc.PublicKey.Challenge,
			RelyingParty:          rpEntityFromProto(cc.PublicKey.Rp),
			User:                  userEntityFromProto(cc.PublicKey.User),
			Parameters:            credentialParametersFromProto(cc.PublicKey.CredentialParameters),
			Timeout:               int(cc.PublicKey.TimeoutMs),
			CredentialExcludeList: credentialDescriptorsFromProto(cc.PublicKey.ExcludeCredentials),
			Extensions:            inputExtensionsFromProto(cc.PublicKey.Extensions),
			Attestation:           protocol.ConveyancePreference(cc.PublicKey.Attestation),
		},
	}
}

// CredentialCreationResponseFromProto converts a CredentialCreationResponse
// proto to its lib counterpart.
func CredentialCreationResponseFromProto(ccr *wantypes.CredentialCreationResponse) *CredentialCreationResponse {
	if ccr == nil {
		return nil
	}
	return &CredentialCreationResponse{
		PublicKeyCredential: PublicKeyCredential{
			Credential: Credential{
				ID:   base64.RawURLEncoding.EncodeToString(ccr.RawId),
				Type: ccr.Type,
			},
			RawID:      ccr.RawId,
			Extensions: outputExtensionsFromProto(ccr.Extensions),
		},
		AttestationResponse: AuthenticatorAttestationResponse{
			AuthenticatorResponse: AuthenticatorResponse{
				ClientDataJSON: ccr.Response.ClientDataJson,
			},
			AttestationObject: ccr.Response.AttestationObject,
		},
	}
}

func credentialDescriptorsToProto(creds []protocol.CredentialDescriptor) []*wantypes.CredentialDescriptor {
	res := make([]*wantypes.CredentialDescriptor, len(creds))
	for i, cred := range creds {
		res[i] = &wantypes.CredentialDescriptor{
			Type: string(cred.Type),
			Id:   cred.CredentialID,
		}
	}
	return res
}

func credentialParametersToProto(params []protocol.CredentialParameter) []*wantypes.CredentialParameter {
	res := make([]*wantypes.CredentialParameter, len(params))
	for i, p := range params {
		res[i] = &wantypes.CredentialParameter{
			Type: string(p.Type),
			Alg:  int32(p.Algorithm),
		}
	}
	return res
}

func inputExtensionsToProto(exts protocol.AuthenticationExtensions) *wantypes.AuthenticationExtensionsClientInputs {
	if len(exts) == 0 {
		return nil
	}
	res := &wantypes.AuthenticationExtensionsClientInputs{}
	if value, ok := exts[AppIDExtension]; ok {
		// Type should always be string, since we are the ones setting it, but let's
		// play it safe and check anyway.
		if appID, ok := value.(string); ok {
			res.AppId = appID
		}
	}
	return res
}

func outputExtensionsToProto(exts *AuthenticationExtensionsClientOutputs) *wantypes.AuthenticationExtensionsClientOutputs {
	if exts == nil {
		return nil
	}
	return &wantypes.AuthenticationExtensionsClientOutputs{
		AppId: exts.AppID,
	}
}

func rpEntityToProto(rp protocol.RelyingPartyEntity) *wantypes.RelyingPartyEntity {
	return &wantypes.RelyingPartyEntity{
		Id:   rp.ID,
		Name: rp.Name,
		Icon: rp.Icon,
	}
}

func userEntityToProto(user protocol.UserEntity) *wantypes.UserEntity {
	return &wantypes.UserEntity{
		Id:          user.ID,
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Icon:        user.Icon,
	}
}

func credentialDescriptorsFromProto(creds []*wantypes.CredentialDescriptor) []protocol.CredentialDescriptor {
	res := make([]protocol.CredentialDescriptor, len(creds))
	for i, cred := range creds {
		res[i] = protocol.CredentialDescriptor{
			Type:         protocol.CredentialType(cred.Type),
			CredentialID: cred.Id,
		}
	}
	return res
}

func credentialParametersFromProto(params []*wantypes.CredentialParameter) []protocol.CredentialParameter {
	res := make([]protocol.CredentialParameter, len(params))
	for i, p := range params {
		res[i] = protocol.CredentialParameter{
			Type:      protocol.CredentialType(p.Type),
			Algorithm: webauthncose.COSEAlgorithmIdentifier(p.Alg),
		}
	}
	return res
}

func inputExtensionsFromProto(exts *wantypes.AuthenticationExtensionsClientInputs) protocol.AuthenticationExtensions {
	if exts == nil {
		return nil
	}
	res := make(map[string]interface{})
	if exts.AppId != "" {
		res[AppIDExtension] = exts.AppId
	}
	return res
}

func outputExtensionsFromProto(exts *wantypes.AuthenticationExtensionsClientOutputs) *AuthenticationExtensionsClientOutputs {
	if exts == nil {
		return nil
	}
	return &AuthenticationExtensionsClientOutputs{
		AppID: exts.AppId,
	}
}

func rpEntityFromProto(rp *wantypes.RelyingPartyEntity) protocol.RelyingPartyEntity {
	if rp == nil {
		return protocol.RelyingPartyEntity{}
	}
	return protocol.RelyingPartyEntity{
		CredentialEntity: protocol.CredentialEntity{
			Name: rp.Name,
			Icon: rp.Icon,
		},
		ID: rp.Id,
	}
}

func userEntityFromProto(user *wantypes.UserEntity) protocol.UserEntity {
	if user == nil {
		return protocol.UserEntity{}
	}
	return protocol.UserEntity{
		CredentialEntity: protocol.CredentialEntity{
			Name: user.Name,
			Icon: user.Icon,
		},
		DisplayName: user.DisplayName,
		ID:          user.Id,
	}
}
