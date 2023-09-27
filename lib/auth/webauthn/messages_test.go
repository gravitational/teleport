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

package webauthn_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

func TestCredentialAssertionResponse_json(t *testing.T) {
	resp := &wanlib.CredentialAssertionResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   base64.RawURLEncoding.EncodeToString([]byte("credentialid")),
				Type: "public-key",
			},
			RawID: []byte("credentialid"),
			Extensions: &wanlib.AuthenticationExtensionsClientOutputs{
				AppID: true,
			},
		},
		AssertionResponse: wanlib.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: []byte("clientdatajson"),
			},
			AuthenticatorData: []byte("authdata"),
			Signature:         []byte("signature"),
			UserHandle:        []byte("userhandle"),
		},
	}

	respJSON, err := json.Marshal(resp)
	require.NoError(t, err)

	got := &wanlib.CredentialAssertionResponse{}
	require.NoError(t, json.Unmarshal(respJSON, got))
	if diff := cmp.Diff(resp, got); diff != "" {
		t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
	}
}

func TestCredentialCreation_Validate(t *testing.T) {
	okCC := &wanlib.CredentialCreation{
		Response: wanlib.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wanlib.RelyingPartyEntity{
				ID: "example.com",
				CredentialEntity: wanlib.CredentialEntity{
					Name: "Teleport",
				},
			},
			Parameters: []wanlib.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: wanlib.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
			User: wanlib.UserEntity{
				CredentialEntity: wanlib.CredentialEntity{
					Name: "llama",
				},
				DisplayName: "Llama",
				ID:          []byte{1, 2, 3, 4, 5}, // arbitrary
			},
		},
	}

	tests := []struct {
		name           string
		createCC       func() *wanlib.CredentialCreation
		alwaysCreateRK bool
		wantErr        string
	}{
		{
			name:     "ok", // check that good params are good
			createCC: func() *wanlib.CredentialCreation { return okCC },
			wantErr:  "",
		},
		{
			name:     "nil cc",
			createCC: func() *wanlib.CredentialCreation { return nil },
			wantErr:  "credential creation required",
		},
		{
			name: "nil challenge",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.Challenge = nil
				return &cp
			},
			wantErr: "challenge",
		},
		{
			name: "empty RPID",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.RelyingParty.ID = ""
				return &cp
			},
			wantErr: "relying party ID",
		},
		{
			name: "empty RP name",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.RelyingParty.Name = ""
				return &cp
			},
			wantErr: "relying party name",
		},
		{
			name: "empty user name",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.User.Name = ""
				return &cp
			},
			wantErr: "user name",
		},
		{
			name: "empty user display name",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.User.DisplayName = ""
				return &cp
			},
			wantErr: "user display name",
		},
		{
			name: "nil user ID",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.User.ID = nil
				return &cp
			},
			wantErr: "user ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.createCC().Validate()
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr, "Validate returned unexpected error")
				return
			}
			require.NoError(t, err, "Validate errored")
		})
	}
}

func TestCredentialAssertion_Validate(t *testing.T) {
	okAssertion := &wanlib.CredentialAssertion{
		Response: wanlib.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []wanlib.CredentialDescriptor{
				{Type: protocol.PublicKeyCredentialType, CredentialID: []byte{1, 2, 3, 4, 5}},
			},
		},
	}

	nilChallengeAssertion := *okAssertion
	nilChallengeAssertion.Response.Challenge = nil

	emptyRPIDAssertion := *okAssertion
	emptyRPIDAssertion.Response.RelyingPartyID = ""
	tests := []struct {
		name      string
		assertion *wanlib.CredentialAssertion
		wantErr   string
	}{
		{
			name:      "ok", // check that good params are good
			assertion: okAssertion,
			wantErr:   "",
		},
		{
			name:    "nil assertion",
			wantErr: "assertion required",
		},
		{
			name:      "assertion without challenge",
			assertion: &nilChallengeAssertion,
			wantErr:   "challenge",
		},
		{
			name:      "assertion without RPID",
			assertion: &emptyRPIDAssertion,
			wantErr:   "relying party ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.assertion.Validate()
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr, "Validate returned unexpected error")
				return
			}
			require.NoError(t, err, "Validate errored")
		})
	}
}

func TestRequireResidentKey(t *testing.T) {
	tests := []struct {
		name    string
		in      wanlib.AuthenticatorSelection
		want    bool
		wantErr string
	}{
		{
			name: "nothing set",
			in:   wanlib.AuthenticatorSelection{},
			want: false,
		},
		{
			name: "discouraged and rrk=true",
			in: wanlib.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementDiscouraged,
				RequireResidentKey: protocol.ResidentKeyRequired(),
			},
			wantErr: "invalid combination of ResidentKey",
		},
		{
			name: "required and rrk=false",
			in: wanlib.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementRequired,
				RequireResidentKey: protocol.ResidentKeyNotRequired(),
			},
			wantErr: "invalid combination of ResidentKey",
		},
		{
			name: "support nil RequireResidentKey",
			in: wanlib.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: nil,
			},
			want: false,
		},
		{
			name: "ResidentKey preferred result in false",
			in: wanlib.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementPreferred,
				RequireResidentKey: nil,
			},
			want: false,
		},
		{
			name: "ResidentKey required",
			in: wanlib.AuthenticatorSelection{
				ResidentKey: protocol.ResidentKeyRequirementRequired,
			},
			want: true,
		},
		{
			name: "ResidentKey discouraged",
			in: wanlib.AuthenticatorSelection{
				ResidentKey: protocol.ResidentKeyRequirementDiscouraged,
			},
			want: false,
		},
		{
			name: "use RequireResidentKey required if ResidentKey empty",
			in: wanlib.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: protocol.ResidentKeyRequired(),
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &wanlib.CredentialCreation{
				Response: wanlib.PublicKeyCredentialCreationOptions{
					AuthenticatorSelection: test.in,
				},
			}
			got, err := req.RequireResidentKey()
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr, "RequireResidentKey returned unexpected error")
				return
			}
			require.NoError(t, err, "RequireResidentKey errored")
			if got != test.want {
				assert.Equal(t, test.want, got, "RequireResidentKey mismatch")
			}
		})
	}
}
