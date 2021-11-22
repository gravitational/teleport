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

import "github.com/duo-labs/webauthn/protocol"

// CredentialAssertion is the payload sent to authenticators to initiate login.
type CredentialAssertion protocol.CredentialAssertion

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
