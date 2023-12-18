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

	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func init() {
	// Make tests silent.
	PromptWriter = io.Discard
}

func TestRegister(t *testing.T) {
	resetNativeAfterTests(t)

	const origin = "https://example.com"
	okCC := &wantypes.CredentialCreation{
		Response: wantypes.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: wantypes.RelyingPartyEntity{
				ID: "example.com",
				CredentialEntity: wantypes.CredentialEntity{
					Name: "Teleport",
				},
			},
			User: wantypes.UserEntity{
				ID:          []byte{1, 2, 3, 4},
				DisplayName: "display name",
				CredentialEntity: wantypes.CredentialEntity{
					Name: "user name",
				},
			},
			Parameters: []wantypes.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgRS256},
			},
			AuthenticatorSelection: wantypes.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	tests := []struct {
		name     string
		origin   string
		createCC func() *wantypes.CredentialCreation
		assertFn func(t *testing.T, ccr *wanpb.CredentialCreationResponse, req *makeCredentialRequest)
	}{
		{
			name:     "flow with auto attachment and discouraged UV",
			origin:   origin,
			createCC: func() *wantypes.CredentialCreation { return okCC },
			assertFn: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnAttachmentAny, req.opts.dwAuthenticatorAttachment)

				assert.Equal(t, webauthnUserVerificationDiscouraged, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, uint32(0), req.opts.bRequireResidentKey)
			},
		},
		{
			name:   "with UV required and cross-platform and RRK",
			origin: origin,
			createCC: func() *wantypes.CredentialCreation {
				cc := *okCC
				cc.Response.User.DisplayName = "display name"
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired
				cc.Response.AuthenticatorSelection.AuthenticatorAttachment = protocol.CrossPlatform
				cc.Response.AuthenticatorSelection.ResidentKey = protocol.ResidentKeyRequirementRequired
				return &cc
			},
			assertFn: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnUserVerificationRequired, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentCrossPlatform, req.opts.dwAuthenticatorAttachment)

				assert.Equal(t, uint32(1), req.opts.bRequireResidentKey)
			},
		},
		{
			name:   "with UV preferred and platform",
			origin: origin,
			createCC: func() *wantypes.CredentialCreation {
				cc := *okCC
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationPreferred
				cc.Response.AuthenticatorSelection.AuthenticatorAttachment = protocol.Platform
				return &cc
			},
			assertFn: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnUserVerificationPreferred, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentPlatform, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "with UV discouraged and platform",
			origin: origin,
			createCC: func() *wantypes.CredentialCreation {
				cc := *okCC
				cc.Response.AuthenticatorSelection.UserVerification = protocol.VerificationDiscouraged
				return &cc
			},
			assertFn: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, req *makeCredentialRequest) {
				assert.Equal(t, webauthnUserVerificationDiscouraged, req.opts.dwUserVerificationRequirement)
			},
		},
		{
			name:   "RRK from RequireResidentKey if is empty ResidentKey",
			origin: origin,
			createCC: func() *wantypes.CredentialCreation {
				cc := *okCC
				cc.Response.AuthenticatorSelection.RequireResidentKey = protocol.ResidentKeyRequired()
				return &cc
			},
			assertFn: func(t *testing.T, ccr *wanpb.CredentialCreationResponse, req *makeCredentialRequest) {
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
	okAssertion := &wantypes.CredentialAssertion{
		Response: wantypes.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []wantypes.CredentialDescriptor{
				{Type: protocol.PublicKeyCredentialType, CredentialID: []byte{1, 2, 3, 4, 5}},
			},
			UserVerification: protocol.VerificationDiscouraged,
		},
	}

	tests := []struct {
		name        string
		origin      string
		assertionIn func() *wantypes.CredentialAssertion
		opts        LoginOpts
		wantErr     string
		assertFn    func(t *testing.T, car *wanpb.CredentialAssertionResponse, req *getAssertionRequest)
	}{
		{
			name:        "uv discouraged, attachment auto",
			origin:      origin,
			assertionIn: func() *wantypes.CredentialAssertion { return okAssertion },
			assertFn: func(t *testing.T, car *wanpb.CredentialAssertionResponse, req *getAssertionRequest) {
				assert.Equal(t, uint32(6), req.opts.dwVersion)

				assert.Equal(t, webauthnUserVerificationDiscouraged, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentAny, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "uv required, attachment platform",
			origin: origin,
			assertionIn: func() *wantypes.CredentialAssertion {
				out := *okAssertion
				out.Response.UserVerification = protocol.VerificationRequired
				return &out
			},
			opts: LoginOpts{AuthenticatorAttachment: AttachmentPlatform},
			assertFn: func(t *testing.T, car *wanpb.CredentialAssertionResponse, req *getAssertionRequest) {
				assert.Equal(t, uint32(6), req.opts.dwVersion)

				assert.Equal(t, webauthnUserVerificationRequired, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentPlatform, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "uv preferred, attachment cross-platform",
			origin: origin,
			assertionIn: func() *wantypes.CredentialAssertion {
				out := *okAssertion
				out.Response.UserVerification = protocol.VerificationPreferred
				return &out
			},
			opts: LoginOpts{AuthenticatorAttachment: AttachmentCrossPlatform},
			assertFn: func(t *testing.T, car *wanpb.CredentialAssertionResponse, req *getAssertionRequest) {
				assert.Equal(t, uint32(6), req.opts.dwVersion)

				assert.Equal(t, webauthnUserVerificationPreferred, req.opts.dwUserVerificationRequirement)

				assert.Equal(t, webauthnAttachmentCrossPlatform, req.opts.dwAuthenticatorAttachment)
			},
		},
		{
			name:   "uv discouraged",
			origin: origin,
			assertionIn: func() *wantypes.CredentialAssertion {
				out := *okAssertion
				out.Response.UserVerification = protocol.VerificationDiscouraged
				return &out
			},
			opts: LoginOpts{AuthenticatorAttachment: AttachmentCrossPlatform},
			assertFn: func(t *testing.T, car *wanpb.CredentialAssertionResponse, req *getAssertionRequest) {
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

func (m *mockNative) GetAssertion(origin string, in *getAssertionRequest) (*wantypes.CredentialAssertionResponse, error) {
	m.getAssersionReq = in
	return &wantypes.CredentialAssertionResponse{}, nil
}

func (m *mockNative) MakeCredential(origin string, in *makeCredentialRequest) (*wantypes.CredentialCreationResponse, error) {
	m.makeCredentialReq = in
	return &wantypes.CredentialCreationResponse{}, nil
}
