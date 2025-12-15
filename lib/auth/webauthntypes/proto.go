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

package webauthntypes

import (
	"encoding/base64"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"

	wanpb "github.com/gravitational/teleport/api/types/webauthn"
)

// CredentialAssertionToProto converts a CredentialAssertion to its proto
// counterpart.
func CredentialAssertionToProto(assertion *CredentialAssertion) *wanpb.CredentialAssertion {
	if assertion == nil {
		return nil
	}
	return &wanpb.CredentialAssertion{
		PublicKey: &wanpb.PublicKeyCredentialRequestOptions{
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
func CredentialAssertionResponseToProto(car *CredentialAssertionResponse) *wanpb.CredentialAssertionResponse {
	if car == nil {
		return nil
	}
	return &wanpb.CredentialAssertionResponse{
		Type:  car.Type,
		RawId: car.RawID,
		Response: &wanpb.AuthenticatorAssertionResponse{
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
func CredentialCreationToProto(cc *CredentialCreation) *wanpb.CredentialCreation {
	if cc == nil {
		return nil
	}
	return &wanpb.CredentialCreation{
		PublicKey: &wanpb.PublicKeyCredentialCreationOptions{
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
func CredentialCreationResponseToProto(ccr *CredentialCreationResponse) *wanpb.CredentialCreationResponse {
	if ccr == nil {
		return nil
	}
	return &wanpb.CredentialCreationResponse{
		Type:  ccr.Type,
		RawId: ccr.RawID,
		Response: &wanpb.AuthenticatorAttestationResponse{
			ClientDataJson:    ccr.AttestationResponse.ClientDataJSON,
			AttestationObject: ccr.AttestationResponse.AttestationObject,
		},
		Extensions: outputExtensionsToProto(ccr.Extensions),
	}
}

// CredentialAssertionFromProto converts a CredentialAssertion proto to its lib
// counterpart.
func CredentialAssertionFromProto(assertion *wanpb.CredentialAssertion) *CredentialAssertion {
	if assertion == nil {
		return nil
	}
	return &CredentialAssertion{
		Response: publicKeyCredentialRequestOptionsFromProto(assertion.PublicKey),
	}
}

// CredentialAssertionResponseFromProto converts a CredentialAssertionResponse
// proto to its lib counterpart.
func CredentialAssertionResponseFromProto(car *wanpb.CredentialAssertionResponse) *CredentialAssertionResponse {
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
func CredentialCreationFromProto(cc *wanpb.CredentialCreation) *CredentialCreation {
	if cc == nil {
		return nil
	}
	return &CredentialCreation{
		Response: publicKeyCredentialCreationOptionsFromProto(cc.PublicKey),
	}
}

// CredentialCreationResponseFromProto converts a CredentialCreationResponse
// proto to its lib counterpart.
func CredentialCreationResponseFromProto(ccr *wanpb.CredentialCreationResponse) *CredentialCreationResponse {
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

func authenticatorSelectionToProto(a AuthenticatorSelection) *wanpb.AuthenticatorSelection {
	return &wanpb.AuthenticatorSelection{
		AuthenticatorAttachment: string(a.AuthenticatorAttachment),
		RequireResidentKey:      a.RequireResidentKey != nil && *a.RequireResidentKey,
		UserVerification:        string(a.UserVerification),
	}
}

func credentialDescriptorsToProto(creds []CredentialDescriptor) []*wanpb.CredentialDescriptor {
	res := make([]*wanpb.CredentialDescriptor, len(creds))
	for i, cred := range creds {
		res[i] = &wanpb.CredentialDescriptor{
			Type: string(cred.Type),
			Id:   cred.CredentialID,
		}
	}
	return res
}

func credentialParametersToProto(params []CredentialParameter) []*wanpb.CredentialParameter {
	res := make([]*wanpb.CredentialParameter, len(params))
	for i, p := range params {
		res[i] = &wanpb.CredentialParameter{
			Type: string(p.Type),
			Alg:  int32(p.Algorithm),
		}
	}
	return res
}

func inputExtensionsToProto(exts AuthenticationExtensions) *wanpb.AuthenticationExtensionsClientInputs {
	if len(exts) == 0 {
		return nil
	}

	res := &wanpb.AuthenticationExtensionsClientInputs{}

	// appid (string).
	if value, ok := exts[AppIDExtension]; ok {
		// Type should always be string, since we are the ones setting it, but let's
		// play it safe and check anyway.
		if appID, ok := value.(string); ok {
			res.AppId = appID
		}
	}

	// credProps (bool).
	if val, ok := exts[CredPropsExtension]; ok {
		b, ok := val.(bool)
		res.CredProps = ok && b
	}

	return res
}

func outputExtensionsToProto(exts *AuthenticationExtensionsClientOutputs) *wanpb.AuthenticationExtensionsClientOutputs {
	if exts == nil {
		return nil
	}

	res := &wanpb.AuthenticationExtensionsClientOutputs{
		AppId: exts.AppID,
	}

	// credProps.
	if credProps := exts.CredProps; credProps != nil {
		res.CredProps = &wanpb.CredentialPropertiesOutput{
			Rk: credProps.RK,
		}
	}

	return res
}

func rpEntityToProto(rp RelyingPartyEntity) *wanpb.RelyingPartyEntity {
	return &wanpb.RelyingPartyEntity{
		Id:   rp.ID,
		Name: rp.Name,
	}
}

func userEntityToProto(user UserEntity) *wanpb.UserEntity {
	return &wanpb.UserEntity{
		Id:          user.ID,
		Name:        user.Name,
		DisplayName: user.DisplayName,
	}
}

func authenticatorAssertionResponseFromProto(resp *wanpb.AuthenticatorAssertionResponse) AuthenticatorAssertionResponse {
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

func authenticatorAttestationResponseFromProto(resp *wanpb.AuthenticatorAttestationResponse) AuthenticatorAttestationResponse {
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

func authenticatorSelectionFromProto(a *wanpb.AuthenticatorSelection) AuthenticatorSelection {
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

func credentialDescriptorsFromProto(creds []*wanpb.CredentialDescriptor) []CredentialDescriptor {
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

func credentialParametersFromProto(params []*wanpb.CredentialParameter) []CredentialParameter {
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

func inputExtensionsFromProto(exts *wanpb.AuthenticationExtensionsClientInputs) AuthenticationExtensions {
	if exts == nil {
		return nil
	}
	res := make(map[string]any)
	if exts.AppId != "" {
		res[AppIDExtension] = exts.AppId
	}
	if exts.CredProps {
		res[CredPropsExtension] = true
	}
	return res
}

func outputExtensionsFromProto(exts *wanpb.AuthenticationExtensionsClientOutputs) *AuthenticationExtensionsClientOutputs {
	if exts == nil {
		return nil
	}

	res := &AuthenticationExtensionsClientOutputs{
		AppID: exts.AppId,
	}

	// credProps.
	if credProps := exts.CredProps; credProps != nil {
		res.CredProps = &CredentialPropertiesOutput{
			RK: credProps.Rk,
		}
	}

	return res
}

func publicKeyCredentialCreationOptionsFromProto(pubKey *wanpb.PublicKeyCredentialCreationOptions) PublicKeyCredentialCreationOptions {
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

func publicKeyCredentialRequestOptionsFromProto(pubKey *wanpb.PublicKeyCredentialRequestOptions) PublicKeyCredentialRequestOptions {
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

func rpEntityFromProto(rp *wanpb.RelyingPartyEntity) RelyingPartyEntity {
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

func userEntityFromProto(user *wanpb.UserEntity) UserEntity {
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
