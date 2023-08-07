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

package webauthnwin

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

func init() {
	// Make tests silent.
	PromptWriter = io.Discard
}

func TestRegister(t *testing.T) {
	resetNativeAfterTests(t)

	const origin = "https://example.com"
	okCC := &wanlib.CredentialCreation{
		Response: wanlib.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wanlib.RelyingPartyEntity{
				ID: "example.com",
				CredentialEntity: wanlib.CredentialEntity{
					Name: "Teleport",
				},
			},
			User: wanlib.UserEntity{
				ID:          []byte{1, 2, 3, 4},
				DisplayName: "display name",
				CredentialEntity: wanlib.CredentialEntity{
					Name: "user name",
				},
			},
			Parameters: []wanlib.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgRS256},
			},
			AuthenticatorSelection: wanlib.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	tests := []struct {
		name     string
		origin   string
		createCC func() *wanlib.CredentialCreation
		assertFn func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *makeCredentialRequest)
	}{
		{
			name:     "flow with auto attachment and discouraged UV",
			origin:   origin,
			createCC: func() *wanlib.CredentialCreation { return okCC },
			assertFn: func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnAttachmentAny, req.opts.dwAuthenticatorAttachment)

				assert.Equal(t, webauthnUserVerificationDiscouraged, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, uint32(0), req.opts.bRequireResidentKey)
			},
		},
		{
			name:   "with UV required and cross-platform and RRK",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cc := *okCC
				cc.Response.User.DisplayName = "display name"
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired
				cc.Response.AuthenticatorSelection.AuthenticatorAttachment = protocol.CrossPlatform
				cc.Response.AuthenticatorSelection.ResidentKey = protocol.ResidentKeyRequirementRequired
				return &cc
			},
			assertFn: func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnUserVerificationRequired, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentCrossPlatform, req.opts.dwAuthenticatorAttachment)

				assert.Equal(t, uint32(1), req.opts.bRequireResidentKey)
			},
		},
		{
			name:   "with UV preferred and platform",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cc := *okCC
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationPreferred
				cc.Response.AuthenticatorSelection.AuthenticatorAttachment = protocol.Platform
				return &cc
			},
			assertFn: func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnUserVerificationPreferred, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentPlatform, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "with UV discouraged and platform",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cc := *okCC
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationDiscouraged
				return &cc
			},
			assertFn: func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnUserVerificationDiscouraged, req.opts.dwUserVerificationRequirement)
			},
		},
		{
			name:   "RRK from RequireResidentKey if is empty ResidentKey",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cc := *okCC
				cc.Response.AuthenticatorSelection.RequireResidentKey = protocol.ResidentKeyRequired()
				return &cc
			},
			assertFn: func(t *testing.T, ccr *webauthn.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, uint32(1), req.opts.bRequireResidentKey)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := &mockNative{}
			*Native = mock

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			resp, err := Register(ctx, test.origin, test.createCC())
			require.NoError(t, err, "Register failed")
			if test.assertFn != nil {
				test.assertFn(t, resp.GetWebauthn(), mock.makeCredentialReq)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	resetNativeAfterTests(t)

	const origin = "https://example.com"
	okAssertion := &wanlib.CredentialAssertion{
		Response: wanlib.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []wanlib.CredentialDescriptor{
				{Type: protocol.PublicKeyCredentialType, CredentialID: []byte{1, 2, 3, 4, 5}},
			},
			UserVerification: protocol.VerificationDiscouraged,
		},
	}

	tests := []struct {
		name        string
		origin      string
		assertionIn func() *wanlib.CredentialAssertion
		opts        LoginOpts
		wantErr     string
		assertFn    func(t *testing.T, car *webauthn.CredentialAssertionResponse, req *getAssertionRequest)
	}{
		{
			name:        "uv discouraged, attachment auto",
			origin:      origin,
			assertionIn: func() *wanlib.CredentialAssertion { return okAssertion },
			assertFn: func(t *testing.T, car *webauthn.CredentialAssertionResponse, req *getAssertionRequest) {
				assert.Equal(t, uint32(6), req.opts.dwVersion)

				assert.Equal(t, webauthnUserVerificationDiscouraged, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentAny, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "uv required, attachment platform",
			origin: origin,
			assertionIn: func() *wanlib.CredentialAssertion {
				out := *okAssertion
				out.Response.UserVerification = protocol.VerificationRequired
				return &out
			},
			opts: LoginOpts{AuthenticatorAttachment: AttachmentPlatform},
			assertFn: func(t *testing.T, car *webauthn.CredentialAssertionResponse, req *getAssertionRequest) {
				assert.Equal(t, uint32(6), req.opts.dwVersion)

				assert.Equal(t, webauthnUserVerificationRequired, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentPlatform, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "uv preferred, attachment cross-platform",
			origin: origin,
			assertionIn: func() *wanlib.CredentialAssertion {
				out := *okAssertion
				out.Response.UserVerification = protocol.VerificationPreferred
				return &out
			},
			opts: LoginOpts{AuthenticatorAttachment: AttachmentCrossPlatform},
			assertFn: func(t *testing.T, car *webauthn.CredentialAssertionResponse, req *getAssertionRequest) {
				assert.Equal(t, uint32(6), req.opts.dwVersion)

				assert.Equal(t, webauthnUserVerificationPreferred, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentCrossPlatform, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "uv discouraged",
			origin: origin,
			assertionIn: func() *wanlib.CredentialAssertion {
				out := *okAssertion
				out.Response.UserVerification = protocol.VerificationDiscouraged
				return &out
			},
			opts: LoginOpts{AuthenticatorAttachment: AttachmentCrossPlatform},
			assertFn: func(t *testing.T, car *webauthn.CredentialAssertionResponse, req *getAssertionRequest) {
				assert.Equal(t, uint32(6), req.opts.dwVersion)

				assert.Equal(t, webauthnUserVerificationDiscouraged, req.opts.dwUserVerificationRequirement)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := &mockNative{}
			*Native = mock

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			resp, _, err := Login(ctx, test.origin, test.assertionIn(), &test.opts)
			require.NoError(t, err, "Login failed")
			if test.assertFn != nil {
				test.assertFn(t, resp.GetWebauthn(), mock.getAssersionReq)
			}
		})
	}
}

func resetNativeAfterTests(t *testing.T) {
	n := *Native
	t.Cleanup(func() {
		*Native = n
	})
}

type mockNative struct {
	getAssersionReq   *getAssertionRequest
	makeCredentialReq *makeCredentialRequest
}

func (m *mockNative) CheckSupport() CheckSupportResult {
	return CheckSupportResult{
		HasCompileSupport: true,
		IsAvailable:       true,
	}
}

func (m *mockNative) GetAssertion(origin string, in *getAssertionRequest) (*wanlib.CredentialAssertionResponse, error) {
	m.getAssersionReq = in
	return &wanlib.CredentialAssertionResponse{}, nil
}

func (m *mockNative) MakeCredential(origin string, in *makeCredentialRequest) (*wanlib.CredentialCreationResponse, error) {
	m.makeCredentialReq = in
	return &wanlib.CredentialCreationResponse{}, nil
}
