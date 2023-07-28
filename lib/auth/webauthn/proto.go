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

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"

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
			UserVerification: string(assertion.Response.UserVerification),
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
			Challenge:              cc.Response.Challenge,
			Rp:                     rpEntityToProto(cc.Response.RelyingParty),
			User:                   userEntityToProto(cc.Response.User),
			CredentialParameters:   credentialParametersToProto(cc.Response.Parameters),
			TimeoutMs:              int64(cc.Response.Timeout),
			ExcludeCredentials:     credentialDescriptorsToProto(cc.Response.CredentialExcludeList),
			Attestation:            string(cc.Response.Attestation),
			Extensions:             inputExtensionsToProto(cc.Response.Extensions),
			AuthenticatorSelection: authenticatorSelectionToProto(cc.Response.AuthenticatorSelection),
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
		Response: publicKeyCredentialRequestOptionsFromProto(assertion.PublicKey),
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
		AssertionResponse: authenticatorAssertionResponseFromProto(car.Response),
	}
}

// CredentialCreationFromProto converts a CredentialCreation proto to its lib
// counterpart.
func CredentialCreationFromProto(cc *wantypes.CredentialCreation) *CredentialCreation {
	if cc == nil {
		return nil
	}
	return &CredentialCreation{
		Response: publicKeyCredentialCreationOptionsFromProto(cc.PublicKey),
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
		AttestationResponse: authenticatorAttestationResponseFromProto(ccr.Response),
	}
}

func authenticatorSelectionToProto(a AuthenticatorSelection) *wantypes.AuthenticatorSelection {
	return &wantypes.AuthenticatorSelection{
		AuthenticatorAttachment: string(a.AuthenticatorAttachment),
		RequireResidentKey:      a.RequireResidentKey != nil && *a.RequireResidentKey,
		UserVerification:        string(a.UserVerification),
	}
}

func credentialDescriptorsToProto(creds []CredentialDescriptor) []*wantypes.CredentialDescriptor {
	res := make([]*wantypes.CredentialDescriptor, len(creds))
	for i, cred := range creds {
		res[i] = &wantypes.CredentialDescriptor{
			Type: string(cred.Type),
			Id:   cred.CredentialID,
		}
	}
	return res
}

func credentialParametersToProto(params []CredentialParameter) []*wantypes.CredentialParameter {
	res := make([]*wantypes.CredentialParameter, len(params))
	for i, p := range params {
		res[i] = &wantypes.CredentialParameter{
			Type: string(p.Type),
			Alg:  int32(p.Algorithm),
		}
	}
	return res
}

func inputExtensionsToProto(exts AuthenticationExtensions) *wantypes.AuthenticationExtensionsClientInputs {
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

func rpEntityToProto(rp RelyingPartyEntity) *wantypes.RelyingPartyEntity {
	return &wantypes.RelyingPartyEntity{
		Id:   rp.ID,
		Name: rp.Name,
	}
}

func userEntityToProto(user UserEntity) *wantypes.UserEntity {
	return &wantypes.UserEntity{
		Id:          user.ID,
		Name:        user.Name,
		DisplayName: user.DisplayName,
	}
}

func authenticatorAssertionResponseFromProto(resp *wantypes.AuthenticatorAssertionResponse) AuthenticatorAssertionResponse {
	if resp == nil {
		return AuthenticatorAssertionResponse{}
	}
	return AuthenticatorAssertionResponse{
		AuthenticatorResponse: AuthenticatorResponse{
			ClientDataJSON: resp.ClientDataJson,
		},
		AuthenticatorData: resp.AuthenticatorData,
		Signature:         resp.Signature,
		UserHandle:        resp.UserHandle,
	}
}

func authenticatorAttestationResponseFromProto(resp *wantypes.AuthenticatorAttestationResponse) AuthenticatorAttestationResponse {
	if resp == nil {
		return AuthenticatorAttestationResponse{}
	}
	return AuthenticatorAttestationResponse{
		AuthenticatorResponse: AuthenticatorResponse{
			ClientDataJSON: resp.ClientDataJson,
		},
		AttestationObject: resp.AttestationObject,
	}
}

func authenticatorSelectionFromProto(a *wantypes.AuthenticatorSelection) AuthenticatorSelection {
	if a == nil {
		return AuthenticatorSelection{}
	}
	rrk := a.RequireResidentKey
	return AuthenticatorSelection{
		AuthenticatorAttachment: protocol.AuthenticatorAttachment(a.AuthenticatorAttachment),
		RequireResidentKey:      &rrk,
		UserVerification:        protocol.UserVerificationRequirement(a.UserVerification),
	}
}

func credentialDescriptorsFromProto(creds []*wantypes.CredentialDescriptor) []CredentialDescriptor {
	var res []CredentialDescriptor
	for _, cred := range creds {
		if cred == nil {
			continue
		}
		res = append(res, CredentialDescriptor{
			Type:         protocol.CredentialType(cred.Type),
			CredentialID: cred.Id,
		})
	}
	return res
}

func credentialParametersFromProto(params []*wantypes.CredentialParameter) []CredentialParameter {
	var res []CredentialParameter
	for _, p := range params {
		if p == nil {
			continue
		}
		res = append(res, CredentialParameter{
			Type:      protocol.CredentialType(p.Type),
			Algorithm: webauthncose.COSEAlgorithmIdentifier(p.Alg),
		})
	}
	return res
}

func inputExtensionsFromProto(exts *wantypes.AuthenticationExtensionsClientInputs) AuthenticationExtensions {
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

func publicKeyCredentialCreationOptionsFromProto(pubKey *wantypes.PublicKeyCredentialCreationOptions) PublicKeyCredentialCreationOptions {
	if pubKey == nil {
		return PublicKeyCredentialCreationOptions{}
	}
	return PublicKeyCredentialCreationOptions{
		Challenge:              pubKey.Challenge,
		RelyingParty:           rpEntityFromProto(pubKey.Rp),
		User:                   userEntityFromProto(pubKey.User),
		Parameters:             credentialParametersFromProto(pubKey.CredentialParameters),
		Timeout:                int(pubKey.TimeoutMs),
		CredentialExcludeList:  credentialDescriptorsFromProto(pubKey.ExcludeCredentials),
		Extensions:             inputExtensionsFromProto(pubKey.Extensions),
		Attestation:            protocol.ConveyancePreference(pubKey.Attestation),
		AuthenticatorSelection: authenticatorSelectionFromProto(pubKey.AuthenticatorSelection),
	}
}

func publicKeyCredentialRequestOptionsFromProto(pubKey *wantypes.PublicKeyCredentialRequestOptions) PublicKeyCredentialRequestOptions {
	if pubKey == nil {
		return PublicKeyCredentialRequestOptions{}
	}
	return PublicKeyCredentialRequestOptions{
		Challenge:          pubKey.Challenge,
		Timeout:            int(pubKey.TimeoutMs),
		RelyingPartyID:     pubKey.RpId,
		AllowedCredentials: credentialDescriptorsFromProto(pubKey.AllowCredentials),
		Extensions:         inputExtensionsFromProto(pubKey.Extensions),
		UserVerification:   protocol.UserVerificationRequirement(pubKey.UserVerification),
	}
}

func rpEntityFromProto(rp *wantypes.RelyingPartyEntity) RelyingPartyEntity {
	if rp == nil {
		return RelyingPartyEntity{}
	}
	return RelyingPartyEntity{
		CredentialEntity: CredentialEntity{
			Name: rp.Name,
		},
		ID: rp.Id,
	}
}

func userEntityFromProto(user *wantypes.UserEntity) UserEntity {
	if user == nil {
		return UserEntity{}
	}
	return UserEntity{
		CredentialEntity: CredentialEntity{
			Name: user.Name,
		},
		DisplayName: user.DisplayName,
		ID:          user.Id,
	}
}
