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
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"
)

// CredentialAssertion is the payload sent to authenticators to initiate login.
type CredentialAssertion protocol.CredentialAssertion

// Validate performs client-side validation of CredentialAssertion.
// It makes sure that data are valid and can be sent to authenticator.
// This is general purpose validation and authenticator should add its own
// on top of it, if necessary.
func (ca *CredentialAssertion) Validate() error {
	switch {
	case ca == nil:
		return trace.BadParameter("credential assertion required")
	case len(ca.Response.Challenge) == 0:
		return trace.BadParameter("credential assertion challenge required")
	case ca.Response.RelyingPartyID == "":
		return trace.BadParameter("credential assertion relying party ID required")
	}
	return nil
}

// CredentialAssertionResponse is the reply from authenticators to complete
// login.
type CredentialAssertionResponse struct {
	// CredentialAssertionResponse is redefined because, unlike
	// CredentialAssertion, it is likely to be manually created by package users.
	// Redefining allows us to 1) make sure it can be properly JSON-marshaled
	// (protocol.CredentialAssertionResponse.Extensions can't) and 2) we avoid
	// leaking the duo-labs/webauthn dependency.
	// The nesting of types is identical to protocol.CredentialAssertionResponse.

	PublicKeyCredential
	AssertionResponse AuthenticatorAssertionResponse `json:"response"`
}

// CredentialCreation is the payload sent to authenticators to initiate
// registration.
type CredentialCreation protocol.CredentialCreation

// RequireResidentKey returns information whether resident key is required or
// not. It checks ResidentKey and fallbacks to RequireResidentKey.
func (cc *CredentialCreation) RequireResidentKey() (bool, error) {
	as := cc.Response.AuthenticatorSelection
	switch as.ResidentKey {
	case protocol.ResidentKeyRequirementRequired:
		if as.RequireResidentKey != nil && !*as.RequireResidentKey {
			return false, trace.BadParameter("invalid combination of ResidentKey: %v and RequireResidentKey: %v", as.ResidentKey, *as.RequireResidentKey)
		}
		return true, nil
	case protocol.ResidentKeyRequirementDiscouraged:
		if as.RequireResidentKey != nil && *as.RequireResidentKey {
			return false, trace.BadParameter("invalid combination of ResidentKey: %v and RequireResidentKey: %v", as.ResidentKey, *as.RequireResidentKey)
		}
		return false, nil
	case protocol.ResidentKeyRequirementPreferred:
		return false, nil
	}
	// If ResidentKey is not set, then fallback to the legacy RequireResidentKey
	// field.
	return as.RequireResidentKey != nil && *as.RequireResidentKey, nil
}

// Validate performs client-side validation of CredentialCreation.
// It makes sure that data are valid and can be sent to authenticator.
// This is general purpose validation and authenticator should add its own
// on top of it, if necessary.
func (cc *CredentialCreation) Validate() error {
	switch {
	case cc == nil:
		return trace.BadParameter("credential creation required")
	case len(cc.Response.Challenge) == 0:
		return trace.BadParameter("credential creation challenge required")
	case cc.Response.RelyingParty.ID == "":
		return trace.BadParameter("credential creation relying party ID required")
	case len(cc.Response.RelyingParty.Name) == 0:
		return trace.BadParameter("relying party name required")
	case len(cc.Response.User.Name) == 0:
		return trace.BadParameter("user name required")
	case len(cc.Response.User.DisplayName) == 0:
		return trace.BadParameter("user display name required")
	case len(cc.Response.User.ID) == 0:
		return trace.BadParameter("user ID required")
	default:
		return nil
	}
}

// CredentialCreationResponse is the reply from authenticators to complete
// registration.
type CredentialCreationResponse struct {
	// CredentialCreationResponse is manually redefined, instead of directly based
	// in protocol.CredentialCreationResponse, for the same reasoning that
	// CredentialAssertionResponse is - in short, we want a clean package.
	// The nesting of types is identical to protocol.CredentialCreationResponse.

	PublicKeyCredential
	AttestationResponse AuthenticatorAttestationResponse `json:"response"`
}

type PublicKeyCredential struct {
	Credential
	RawID      protocol.URLEncodedBase64              `json:"rawId"`
	Extensions *AuthenticationExtensionsClientOutputs `json:"extensions,omitempty"`
}

type Credential protocol.Credential

type AuthenticationExtensionsClientOutputs struct {
	AppID bool `json:"appid,omitempty"`
}

type AuthenticatorAssertionResponse struct {
	AuthenticatorResponse
	AuthenticatorData protocol.URLEncodedBase64 `json:"authenticatorData"`
	Signature         protocol.URLEncodedBase64 `json:"signature"`
	UserHandle        protocol.URLEncodedBase64 `json:"userHandle,omitempty"`
}

type AuthenticatorResponse protocol.AuthenticatorResponse

type AuthenticatorAttestationResponse struct {
	AuthenticatorResponse
	AttestationObject protocol.URLEncodedBase64 `json:"attestationObject"`
}
