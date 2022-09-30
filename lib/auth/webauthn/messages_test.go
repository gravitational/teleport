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

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/google/go-cmp/cmp"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Response: protocol.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: protocol.RelyingPartyEntity{
				ID: "example.com",
			},
			Parameters: []protocol.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: protocol.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	pwdlessOK := *okCC
	pwdlessOK.Response.RelyingParty.Name = "Teleport"
	pwdlessOK.Response.User = protocol.UserEntity{
		CredentialEntity: protocol.CredentialEntity{
			Name: "llama",
		},
		DisplayName: "Llama",
		ID:          []byte{1, 2, 3, 4, 5}, // arbitrary
	}
	rrk := true
	pwdlessOK.Response.AuthenticatorSelection.RequireResidentKey = &rrk
	pwdlessOK.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired

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
			name: "cc without challenge",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.Challenge = nil
				return &cp
			},
			wantErr: "challenge",
		},
		{
			name: "cc without RPID",
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.RelyingParty.ID = ""
				return &cp
			},
			wantErr: "relying party ID",
		},
		{
			name: "rrk empty RP name",
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.RelyingParty.Name = ""
				return &cp
			},
			wantErr: "relying party name",
		},
		{
			name: "rrk empty user name",

			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.Name = ""
				return &cp
			},
			wantErr: "user name",
		},
		{
			name: "rrk empty user display name",

			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.DisplayName = ""
				return &cp
			},
			wantErr: "user display name",
		},
		{
			name: "rrk nil user ID",
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.ID = nil
				return &cp
			},
			wantErr: "user ID",
		},
		{
			name:           "cc without rrk but pass alwaysCreateRK",
			createCC:       func() *wanlib.CredentialCreation { return okCC },
			alwaysCreateRK: true,
			wantErr:        "relying party name required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.createCC().Validate(test.alwaysCreateRK)
			if test.wantErr != "" {
				require.Error(t, err, "Validate returned err = nil, want %q", test.wantErr)
				assert.Contains(t, err.Error(), test.wantErr, "Validate returned err = %q, want %q", err, test.wantErr)
			} else {
				require.NoError(t, err, "Validate returned err %v, want nil", err)
			}
		})
	}
}

func TestCredentialAssertion_Validate(t *testing.T) {
	okAssertion := &wanlib.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []protocol.CredentialDescriptor{
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
				require.Error(t, err, "Validate returned err = nil, want %q", test.wantErr)
				assert.Contains(t, err.Error(), test.wantErr, "Validate returned err = %q, want %q", err, test.wantErr)
			} else {
				require.NoError(t, err, "Validate returned err %v, want nil", err)
			}
		})
	}
}

func TestRequireResidentKey(t *testing.T) {
	tests := []struct {
		name string
		in   protocol.AuthenticatorSelection
		want bool
	}{
		{
			name: "nothing set",
			in:   protocol.AuthenticatorSelection{},
			want: false,
		},
		{
			name: "prefer ResidentKey over RequireResidentKey",
			in: protocol.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementDiscouraged,
				RequireResidentKey: protocol.ResidentKeyRequired(),
			},
			want: false,
		},
		{
			name: "support nil RequireResidentKey",
			in: protocol.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: nil,
			},
			want: false,
		},
		{
			name: "ResidentKey preferred result in false",
			in: protocol.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementPreferred,
				RequireResidentKey: nil,
			},
			want: false,
		},
		{
			name: "use RequireResidentKey required if ResidentKey empty",
			in: protocol.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: protocol.ResidentKeyRequired(),
			},
			want: true,
		},
		{
			name: "use RequireResidentKey unrequired if ResidentKey empty",
			in: protocol.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: protocol.ResidentKeyUnrequired(),
			},
			want: false,
		},
		{
			name: "support nil RequireResidentKey with ResidentKey empty",
			in: protocol.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: nil,
			},
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := wanlib.RequireResidentKey(&wanlib.CredentialCreation{
				Response: protocol.PublicKeyCredentialCreationOptions{
					AuthenticatorSelection: test.in,
				},
			}); got != test.want {
				t.Errorf("RequireResidentKey() = %v, want %v", got, test.want)
			}
		})
	}
}
