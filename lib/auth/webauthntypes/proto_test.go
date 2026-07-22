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
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	webauthnv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/webauthn/v2"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func TestConversionFromProto_nils(t *testing.T) {
	// The objective of this test is not to check for correct conversions; those
	// are already checked elsewhere by the various flows that require them.
	// What we want here is to make sure that malformed protos won't make us
	// panic. If a malformed message makes it through, validation will catch it
	// downstream.

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "CredentialAssertion nil",
			fn: func() {
				wantypes.CredentialAssertionFromProto(nil)
			},
		},
		{
			name: "CredentialAssertion empty",
			fn: func() {
				wantypes.CredentialAssertionFromProto(&wanpb.CredentialAssertion{})
			},
		},
		{
			name: "CredentialAssertion.PublicKey empty",
			fn: func() {
				wantypes.CredentialAssertionFromProto(&wanpb.CredentialAssertion{
					PublicKey: &wanpb.PublicKeyCredentialRequestOptions{},
				})
			},
		},
		{
			name: "CredentialAssertion.PublicKey slice elements nil",
			fn: func() {
				wantypes.CredentialAssertionFromProto(&wanpb.CredentialAssertion{
					PublicKey: &wanpb.PublicKeyCredentialRequestOptions{
						AllowCredentials: []*wanpb.CredentialDescriptor{
							{}, nil, {},
						},
					},
				})
			},
		},
		{
			name: "CredentialAssertionResponse nil",
			fn: func() {
				wantypes.CredentialAssertionResponseFromProto(nil)
			},
		},
		{
			name: "CredentialAssertionResponse empty",
			fn: func() {
				wantypes.CredentialAssertionResponseFromProto(&wanpb.CredentialAssertionResponse{})
			},
		},
		{
			name: "CredentialAssertionResponse.Response empty",
			fn: func() {
				wantypes.CredentialAssertionResponseFromProto(&wanpb.CredentialAssertionResponse{
					Response: &wanpb.AuthenticatorAssertionResponse{},
				})
			},
		},
		{
			name: "CredentialCreation nil",
			fn: func() {
				wantypes.CredentialCreationFromProto(nil)
			},
		},
		{
			name: "CredentialCreation empty",
			fn: func() {
				wantypes.CredentialCreationFromProto(&wanpb.CredentialCreation{})
			},
		},
		{
			name: "CredentialCreation.PublicKey empty",
			fn: func() {
				wantypes.CredentialCreationFromProto(&wanpb.CredentialCreation{
					PublicKey: &wanpb.PublicKeyCredentialCreationOptions{},
				})
			},
		},
		{
			name: "CredentialCreation.PublicKey slice elements nil",
			fn: func() {
				wantypes.CredentialCreationFromProto(&wanpb.CredentialCreation{
					PublicKey: &wanpb.PublicKeyCredentialCreationOptions{
						CredentialParameters: []*wanpb.CredentialParameter{
							{}, nil, {},
						},
						ExcludeCredentials: []*wanpb.CredentialDescriptor{
							{}, nil, {},
						},
					},
				})
			},
		},
		{
			name: "CredentialCreationResponse nil",
			fn: func() {
				wantypes.CredentialCreationResponseFromProto(nil)
			},
		},
		{
			name: "CredentialCreationResponse empty",
			fn: func() {
				wantypes.CredentialCreationResponseFromProto(&wanpb.CredentialCreationResponse{})
			},
		},
		{
			name: "CredentialCreationResponse.Response empty",
			fn: func() {
				wantypes.CredentialCreationResponseFromProto(&wanpb.CredentialCreationResponse{
					Response: &wanpb.AuthenticatorAttestationResponse{},
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Absence of panics is good enough for us.
			require.NotPanics(t, test.fn)
		})
	}
}

func TestCredPropsConversions(t *testing.T) {
	t.Parallel()

	ccExtensions := protocol.AuthenticationExtensions{
		wantypes.CredPropsExtension: true,
	}

	t.Run("CredentialCreation", func(t *testing.T) {
		t.Parallel()

		// CC -> proto -> CC.
		cc := wantypes.CredentialCreationFromProto(
			wantypes.CredentialCreationToProto(
				&wantypes.CredentialCreation{
					Response: wantypes.PublicKeyCredentialCreationOptions{
						Extensions: ccExtensions,
					},
				},
			),
		)
		if diff := cmp.Diff(ccExtensions, cc.Response.Extensions); diff != "" {
			t.Errorf("CredentialCreation.Response.Extensions mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("CredentialCreation from protocol", func(t *testing.T) {
		t.Parallel()

		// protocol -> CC.
		cc := wantypes.CredentialCreationFromProtocol(&protocol.CredentialCreation{
			Response: protocol.PublicKeyCredentialCreationOptions{
				Extensions: map[string]any{
					wantypes.CredPropsExtension: true,
				},
			},
		})
		if diff := cmp.Diff(ccExtensions, cc.Response.Extensions); diff != "" {
			t.Errorf("CredentialCreation.Response.Extensions mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("CredentialCreationResponse", func(t *testing.T) {
		t.Parallel()

		// CCR -> proto -> CCR.
		ccr := wantypes.CredentialCreationResponseFromProto(
			wantypes.CredentialCreationResponseToProto(
				&wantypes.CredentialCreationResponse{
					PublicKeyCredential: wantypes.PublicKeyCredential{
						Extensions: &wantypes.AuthenticationExtensionsClientOutputs{
							CredProps: &wantypes.CredentialPropertiesOutput{
								RK: true,
							},
						},
					},
				},
			),
		)
		assert.True(t, ccr.Extensions.CredProps.RK, "ccr.Extensions.CredProps.RK mismatch")
	})
}

func TestCredentialAssertionToProtoV2(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		got := wantypes.CredentialAssertionToProtoV2(nil)
		assert.Nil(t, got)
	})

	t.Run("required fields only", func(t *testing.T) {
		got := wantypes.CredentialAssertionToProtoV2(
			&wantypes.CredentialAssertion{
				Response: wantypes.PublicKeyCredentialRequestOptions{
					Challenge:      wantypes.Challenge([]byte("challenge")),
					RelyingPartyID: "example.com",
				},
			},
		)

		want := webauthnv2.CredentialAssertion_builder{
			PublicKey: webauthnv2.PublicKeyCredentialRequestOptions_builder{
				Challenge: []byte("challenge"),
				RpId:      "example.com",
			}.Build(),
		}.Build()

		require.Empty(
			t,
			cmp.Diff(want, got, protocmp.Transform()),
			"CredentialAssertionToProtoV2 mismatch (-want +got)",
		)
	})

	t.Run("with credentials and extensions", func(t *testing.T) {
		got := wantypes.CredentialAssertionToProtoV2(&wantypes.CredentialAssertion{
			Response: wantypes.PublicKeyCredentialRequestOptions{
				Challenge:      wantypes.Challenge([]byte("challenge")),
				Timeout:        60000,
				RelyingPartyID: "example.com",
				AllowedCredentials: []wantypes.CredentialDescriptor{
					{Type: protocol.PublicKeyCredentialType, CredentialID: []byte("cred1")},
					{Type: protocol.PublicKeyCredentialType, CredentialID: []byte("cred2")},
				},
				UserVerification: protocol.VerificationDiscouraged,
				Extensions: map[string]any{
					wantypes.AppIDExtension:     "https://example.com",
					wantypes.CredPropsExtension: true,
				},
			},
		})

		want := webauthnv2.CredentialAssertion_builder{
			PublicKey: webauthnv2.PublicKeyCredentialRequestOptions_builder{
				Challenge: []byte("challenge"),
				TimeoutMs: 60000,
				RpId:      "example.com",
				AllowCredentials: []*webauthnv2.CredentialDescriptor{
					webauthnv2.CredentialDescriptor_builder{
						Type: string(protocol.PublicKeyCredentialType),
						Id:   []byte("cred1"),
					}.Build(),
					webauthnv2.CredentialDescriptor_builder{
						Type: string(protocol.PublicKeyCredentialType),
						Id:   []byte("cred2"),
					}.Build(),
				},
				UserVerification: string(protocol.VerificationDiscouraged),
				Extensions: webauthnv2.AuthenticationExtensionsClientInputs_builder{
					AppId:     "https://example.com",
					CredProps: true,
				}.Build(),
			}.Build(),
		}.Build()

		require.Empty(
			t,
			cmp.Diff(want, got, protocmp.Transform()),
			"CredentialAssertionToProtoV2 mismatch (-want +got)",
		)
	})
}

func TestCredentialAssertionResponseFromProtoV2(t *testing.T) {
	t.Parallel()

	t.Run("nil response", func(t *testing.T) {
		got := wantypes.CredentialAssertionResponseFromProtoV2(nil)
		assert.Nil(t, got)
	})

	t.Run("required fields only", func(t *testing.T) {
		proto := webauthnv2.CredentialAssertionResponse_builder{
			Type:  string(protocol.PublicKeyCredentialType),
			RawId: []byte("rawid"),
			Response: webauthnv2.AuthenticatorAssertionResponse_builder{
				ClientDataJson:    []byte("client-data"),
				AuthenticatorData: []byte("auth-data"),
			}.Build(),
		}.Build()

		got := wantypes.CredentialAssertionResponseFromProtoV2(proto)

		want := &wantypes.CredentialAssertionResponse{
			PublicKeyCredential: wantypes.PublicKeyCredential{
				Credential: wantypes.Credential{
					ID:   base64.RawURLEncoding.EncodeToString([]byte("rawid")),
					Type: string(protocol.PublicKeyCredentialType),
				},
				RawID: []byte("rawid"),
			},
			AssertionResponse: wantypes.AuthenticatorAssertionResponse{
				AuthenticatorResponse: wantypes.AuthenticatorResponse{
					ClientDataJSON: []byte("client-data"),
				},
				AuthenticatorData: []byte("auth-data"),
			},
		}

		require.Empty(
			t,
			cmp.Diff(want, got),
			"CredentialAssertionResponseFromProtoV2 mismatch (-want +got)",
		)
	})

	t.Run("with signature and extensions", func(t *testing.T) {
		proto := webauthnv2.CredentialAssertionResponse_builder{
			Type:  string(protocol.PublicKeyCredentialType),
			RawId: []byte("rawid"),
			Response: webauthnv2.AuthenticatorAssertionResponse_builder{
				ClientDataJson:    []byte("client-data"),
				AuthenticatorData: []byte("auth-data"),
				Signature:         []byte("signature"),
				UserHandle:        []byte("user-handle"),
			}.Build(),
			Extensions: webauthnv2.AuthenticationExtensionsClientOutputs_builder{
				AppId: true,
				CredProps: webauthnv2.CredentialPropertiesOutput_builder{
					Rk: true,
				}.Build(),
			}.Build(),
		}.Build()

		got := wantypes.CredentialAssertionResponseFromProtoV2(proto)

		want := &wantypes.CredentialAssertionResponse{
			PublicKeyCredential: wantypes.PublicKeyCredential{
				Credential: wantypes.Credential{
					ID:   base64.RawURLEncoding.EncodeToString([]byte("rawid")),
					Type: string(protocol.PublicKeyCredentialType),
				},
				RawID: []byte("rawid"),
				Extensions: &wantypes.AuthenticationExtensionsClientOutputs{
					AppID: true,
					CredProps: &wantypes.CredentialPropertiesOutput{
						RK: true,
					},
				},
			},
			AssertionResponse: wantypes.AuthenticatorAssertionResponse{
				AuthenticatorResponse: wantypes.AuthenticatorResponse{
					ClientDataJSON: []byte("client-data"),
				},
				AuthenticatorData: []byte("auth-data"),
				Signature:         []byte("signature"),
				UserHandle:        []byte("user-handle"),
			},
		}

		require.Empty(
			t,
			cmp.Diff(want, got),
			"CredentialAssertionResponseFromProtoV2 mismatch (-want +got)",
		)
	})
}
