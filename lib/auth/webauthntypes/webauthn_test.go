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

package webauthntypes_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func TestCredentialAssertionResponse_json(t *testing.T) {
	resp := &wantypes.CredentialAssertionResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString([]byte("credentialid")),
				Type: "public-key",
			},
			RawID: []byte("credentialid"),
			Extensions: &wantypes.AuthenticationExtensionsClientOutputs{
				AppID: true,
			},
		},
		AssertionResponse: wantypes.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: []byte("clientdatajson"),
			},
			AuthenticatorData: []byte("authdata"),
			Signature:         []byte("signature"),
			UserHandle:        []byte("userhandle"),
		},
	}

	respJSON, err := json.Marshal(resp)
	require.NoError(t, err)

	got := &wantypes.CredentialAssertionResponse{}
	require.NoError(t, json.Unmarshal(respJSON, got))
	if diff := cmp.Diff(resp, got); diff != "" {
		t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
	}
}

func TestCredentialCreation_Validate(t *testing.T) {
	okCC := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: "example.com",
				CredentialEntity: wantypes.CredentialEntity{
					Name: "Teleport",
				},
			},
			Parameters: []wantypes.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
			User: wantypes.UserEntity{
				CredentialEntity: wantypes.CredentialEntity{
					Name: "llama",
				},
				DisplayName: "Llama",
				ID:          []byte{1, 2, 3, 4, 5}, // arbitrary
			},
		},
	}

	tests := []struct {
		name           string
		createCC       func() *wantypes.CredentialCreation
		alwaysCreateRK bool
		wantErr        string
	}{
		{
			name:     "ok", // check that good params are good
			createCC: func() *wantypes.CredentialCreation { return okCC },
			wantErr:  "",
		},
		{
			name:     "nil cc",
			createCC: func() *wantypes.CredentialCreation { return nil },
			wantErr:  "credential creation required",
		},
		{
			name: "nil challenge",
			createCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.Challenge = nil
				return &cp
			},
			wantErr: "challenge",
		},
		{
			name: "empty RPID",
			createCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.RelyingParty.ID = ""
				return &cp
			},
			wantErr: "relying party ID",
		},
		{
			name: "empty RP name",
			createCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.RelyingParty.Name = ""
				return &cp
			},
			wantErr: "relying party name",
		},
		{
			name: "empty user name",
			createCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.User.Name = ""
				return &cp
			},
			wantErr: "user name",
		},
		{
			name: "empty user display name",
			createCC: func() *wantypes.CredentialCreation {
				cp := *okCC
				cp.Response.User.DisplayName = ""
				return &cp
			},
			wantErr: "user display name",
		},
		{
			name: "nil user ID",
			createCC: func() *wantypes.CredentialCreation {
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
	okAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []wantypes.CredentialDescriptor{
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
		assertion *wantypes.CredentialAssertion
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
		in      wantypes.AuthenticatorSelection
		want    bool
		wantErr string
	}{
		{
			name: "nothing set",
			in:   wantypes.AuthenticatorSelection{},
			want: false,
		},
		{
			name: "discouraged and rrk=true",
			in: wantypes.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementDiscouraged,
				RequireResidentKey: protocol.ResidentKeyRequired(),
			},
			wantErr: "invalid combination of ResidentKey",
		},
		{
			name: "required and rrk=false",
			in: wantypes.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementRequired,
				RequireResidentKey: protocol.ResidentKeyNotRequired(),
			},
			wantErr: "invalid combination of ResidentKey",
		},
		{
			name: "support nil RequireResidentKey",
			in: wantypes.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: nil,
			},
			want: false,
		},
		{
			name: "ResidentKey preferred result in false",
			in: wantypes.AuthenticatorSelection{
				ResidentKey:        protocol.ResidentKeyRequirementPreferred,
				RequireResidentKey: nil,
			},
			want: false,
		},
		{
			name: "ResidentKey required",
			in: wantypes.AuthenticatorSelection{
				ResidentKey: protocol.ResidentKeyRequirementRequired,
			},
			want: true,
		},
		{
			name: "ResidentKey discouraged",
			in: wantypes.AuthenticatorSelection{
				ResidentKey: protocol.ResidentKeyRequirementDiscouraged,
			},
			want: false,
		},
		{
			name: "use RequireResidentKey required if ResidentKey empty",
			in: wantypes.AuthenticatorSelection{
				ResidentKey:        "",
				RequireResidentKey: protocol.ResidentKeyRequired(),
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &wantypes.CredentialCreation{
				Response: wantypes.PublicKeyCredentialCreationOptions{
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
