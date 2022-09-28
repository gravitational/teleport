// Copyright 2022 Gravitational, Inc
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

package webauthnwin_test

import (
	"context"
	"testing"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	resetNativeAfterTests(t)

	const origin = "https://example.com"
	okCC := &wanlib.CredentialCreation{
		Response: protocol.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: protocol.RelyingPartyEntity{
				ID: "example.com",
				CredentialEntity: protocol.CredentialEntity{
					Name: "Teleport",
				},
			},
			User: protocol.UserEntity{
				ID: []byte{1, 2, 3, 4},
				CredentialEntity: protocol.CredentialEntity{
					Name: "user name",
				},
			},
			Parameters: []protocol.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgRS256},
			},
			AuthenticatorSelection: protocol.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	tests := []struct {
		name     string
		origin   string
		createCC func() *wanlib.CredentialCreation
		wantErr  string
		assertFn func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *webauthnwin.MakeCredentialRequest)
	}{
		{
			name:     "ok",
			origin:   origin,
			createCC: func() *wanlib.CredentialCreation { return okCC },
			assertFn: func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *webauthnwin.MakeCredentialRequest) {
				assert.Equal(t, uint32(0), req.Opts.DwAuthenticatorAttachment)
				assert.Equal(t, uint32(3), req.Opts.DwUserVerificationRequirement)
			},
		},
		{
			name:   "with UV and cross-platform",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cc := *okCC
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired
				cc.Response.AuthenticatorSelection.AuthenticatorAttachment = protocol.CrossPlatform
				return &cc
			},
			assertFn: func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *webauthnwin.MakeCredentialRequest) {
				assert.Equal(t, uint32(1), req.Opts.DwUserVerificationRequirement)
				assert.Equal(t, uint32(2), req.Opts.DwAuthenticatorAttachment)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeNative{}
			*webauthnwin.Native = fake

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			resp, err := webauthnwin.Register(ctx, test.origin, test.createCC())
			switch {
			case test.wantErr != "" && err == nil:
				t.Fatalf("Register returned err = nil, wantErr %q", test.wantErr)
			case test.wantErr != "":
				require.Contains(t, err.Error(), test.wantErr, "FIDO2Login returned err = %q, wantErr %q", err, test.wantErr)
				return
			default:
				require.NoError(t, err, "FIDO2Login failed")
				require.NotNil(t, resp, "resp nil")
			}

			if test.assertFn != nil {
				test.assertFn(t, resp.GetWebauthn(), fake.makeCredentialReq)
			}

		})
	}
}

func TestRegister_errors(t *testing.T) {
	resetNativeAfterTests(t)

	*webauthnwin.Native = &fakeNative{}

	const origin = "https://example.com"
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
		name     string
		origin   string
		createCC func() *wanlib.CredentialCreation
		wantErr  string
	}{
		{
			// right now there is not need to provide anything in fakeNative
			name:     "ok",
			origin:   origin,
			createCC: func() *wanlib.CredentialCreation { return okCC },
			wantErr:  "not implemented in fakeNative",
		},
		{
			name:     "nil origin",
			createCC: func() *wanlib.CredentialCreation { return okCC },
			wantErr:  "origin",
		},
		{
			name:     "nil cc",
			origin:   origin,
			createCC: func() *wanlib.CredentialCreation { return nil },
			wantErr:  "credential creation required",
		},
		{
			name:   "cc without challenge",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.Challenge = nil
				return &cp
			},
			wantErr: "challenge",
		},
		{
			name:   "cc without RPID",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.RelyingParty.ID = ""
				return &cp
			},
			wantErr: "relying party ID",
		},
		{
			name:   "rrk empty RP name",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.RelyingParty.Name = ""
				return &cp
			},
			wantErr: "relying party name",
		},
		{
			name:   "rrk empty user name",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.Name = ""
				return &cp
			},
			wantErr: "user name",
		},
		{
			name:   "rrk empty user display name",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.DisplayName = ""
				return &cp
			},
			wantErr: "user display name",
		},
		{
			name:   "rrk nil user ID",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.ID = nil
				return &cp
			},
			wantErr: "user ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, err := webauthnwin.Register(ctx, test.origin, test.createCC())
			require.Error(t, err, "Register returned err = nil, want %q", test.wantErr)
			assert.Contains(t, err.Error(), test.wantErr, "Register returned err = %q, want %q", err, test.wantErr)
		})
	}
}

func TestLogin_errors(t *testing.T) {
	resetNativeAfterTests(t)

	*webauthnwin.Native = &fakeNative{}

	const origin = "https://example.com"
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
		origin    string
		assertion *wanlib.CredentialAssertion
		wantErr   string
	}{
		{
			// right now there is not need to provide anything in fakeNative
			name:      "ok",
			origin:    origin,
			assertion: okAssertion,
			wantErr:   "not implemented in fakeNative",
		},
		{
			name:      "nil origin",
			assertion: okAssertion,
			wantErr:   "origin",
		},
		{
			name:    "nil assertion",
			origin:  origin,
			wantErr: "assertion required",
		},
		{
			name:      "assertion without challenge",
			origin:    origin,
			assertion: &nilChallengeAssertion,
			wantErr:   "challenge",
		},
		{
			name:      "assertion without RPID",
			origin:    origin,
			assertion: &emptyRPIDAssertion,
			wantErr:   "relying party ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, _, err := webauthnwin.Login(ctx, test.origin, test.assertion, nil /* opts */)
			require.Error(t, err, "Login returned err = nil, want %q", test.wantErr)
			assert.Contains(t, err.Error(), test.wantErr, "Login returned err = %q, want %q", err, test.wantErr)
		})
	}
}

func resetNativeAfterTests(t *testing.T) {
	n := *webauthnwin.Native
	t.Cleanup(func() {
		*webauthnwin.Native = n
	})
}

type fakeNative struct {
	getAssersionReq   *webauthnwin.GetAssertionRequest
	makeCredentialReq *webauthnwin.MakeCredentialRequest
}

func (f *fakeNative) CheckSupport() webauthnwin.CheckSupportResult {
	return webauthnwin.CheckSupportResult{
		HasCompileSupport: true,
		IsAvailable:       true,
	}
}

func (f *fakeNative) GetAssertion(origin string, in *webauthnwin.GetAssertionRequest) (*wanlib.CredentialAssertionResponse, error) {
	f.getAssersionReq = in
	return &wanlib.CredentialAssertionResponse{}, nil
}

func (f *fakeNative) MakeCredential(origin string, in *webauthnwin.MakeCredentialRequest) (*wanlib.CredentialCreationResponse, error) {
	f.makeCredentialReq = in
	return &wanlib.CredentialCreationResponse{}, nil
}
