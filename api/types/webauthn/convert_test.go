// Copyright 2026 Gravitational, Inc
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

package webauthnpb_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	webauthnv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/webauthn/v2"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
)

const publicKeyCredentialType = "public-key"

func TestCredentialAssertionV2ToV1(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		got := webauthnpb.CredentialAssertionV2ToV1(nil)
		assert.Nil(t, got)
	})

	t.Run("required fields only", func(t *testing.T) {
		got := webauthnpb.CredentialAssertionV2ToV1(
			webauthnv2.CredentialAssertion_builder{
				PublicKey: webauthnv2.PublicKeyCredentialRequestOptions_builder{
					Challenge: []byte("challenge"),
					RpId:      "example.com",
				}.Build(),
			}.Build(),
		)

		want := &webauthnpb.CredentialAssertion{
			PublicKey: &webauthnpb.PublicKeyCredentialRequestOptions{
				Challenge: []byte("challenge"),
				RpId:      "example.com",
			},
		}

		require.Empty(
			t,
			cmp.Diff(want, got),
			"CredentialAssertionV2ToV1 mismatch (-want +got)",
		)
	})

	t.Run("with credentials and extensions", func(t *testing.T) {
		got := webauthnpb.CredentialAssertionV2ToV1(
			webauthnv2.CredentialAssertion_builder{
				PublicKey: webauthnv2.PublicKeyCredentialRequestOptions_builder{
					Challenge: []byte("challenge"),
					TimeoutMs: 60000,
					RpId:      "example.com",
					AllowCredentials: []*webauthnv2.CredentialDescriptor{
						webauthnv2.CredentialDescriptor_builder{
							Type: publicKeyCredentialType,
							Id:   []byte("cred1"),
						}.Build(),
						webauthnv2.CredentialDescriptor_builder{
							Type: publicKeyCredentialType,
							Id:   []byte("cred2"),
						}.Build(),
					},
					UserVerification: "required",
					Extensions: webauthnv2.AuthenticationExtensionsClientInputs_builder{
						AppId:     "https://example.com",
						CredProps: true,
					}.Build(),
				}.Build(),
			}.Build(),
		)

		want := &webauthnpb.CredentialAssertion{
			PublicKey: &webauthnpb.PublicKeyCredentialRequestOptions{
				Challenge: []byte("challenge"),
				TimeoutMs: 60000,
				RpId:      "example.com",
				AllowCredentials: []*webauthnpb.CredentialDescriptor{
					{Type: publicKeyCredentialType, Id: []byte("cred1")},
					{Type: publicKeyCredentialType, Id: []byte("cred2")},
				},
				UserVerification: "required",
				Extensions: &webauthnpb.AuthenticationExtensionsClientInputs{
					AppId:     "https://example.com",
					CredProps: true,
				},
			},
		}

		require.Empty(
			t,
			cmp.Diff(want, got),
			"CredentialAssertionV2ToV1 mismatch (-want +got)",
		)
	})
}

func TestCredentialAssertionResponseV1ToV2(t *testing.T) {
	t.Parallel()

	t.Run("nil response", func(t *testing.T) {
		got := webauthnpb.CredentialAssertionResponseV1ToV2(nil)
		assert.Nil(t, got)
	})

	t.Run("required fields only", func(t *testing.T) {
		got := webauthnpb.CredentialAssertionResponseV1ToV2(
			&webauthnpb.CredentialAssertionResponse{
				Type:  publicKeyCredentialType,
				RawId: []byte("rawid"),
				Response: &webauthnpb.AuthenticatorAssertionResponse{
					ClientDataJson:    []byte("client-data"),
					AuthenticatorData: []byte("auth-data"),
				},
			},
		)

		want := webauthnv2.CredentialAssertionResponse_builder{
			Type:  publicKeyCredentialType,
			RawId: []byte("rawid"),
			Response: webauthnv2.AuthenticatorAssertionResponse_builder{
				ClientDataJson:    []byte("client-data"),
				AuthenticatorData: []byte("auth-data"),
			}.Build(),
		}.Build()

		require.Empty(
			t,
			cmp.Diff(want, got, protocmp.Transform()),
			"CredentialAssertionResponseV1ToV2 mismatch (-want +got)",
		)
	})

	t.Run("with signature and extensions", func(t *testing.T) {
		got := webauthnpb.CredentialAssertionResponseV1ToV2(
			&webauthnpb.CredentialAssertionResponse{
				Type:  publicKeyCredentialType,
				RawId: []byte("rawid"),
				Response: &webauthnpb.AuthenticatorAssertionResponse{
					ClientDataJson:    []byte("client-data"),
					AuthenticatorData: []byte("auth-data"),
					Signature:         []byte("signature"),
					UserHandle:        []byte("user-handle"),
				},
				Extensions: &webauthnpb.AuthenticationExtensionsClientOutputs{
					AppId: true,
					CredProps: &webauthnpb.CredentialPropertiesOutput{
						Rk: true,
					},
				},
			},
		)

		want := webauthnv2.CredentialAssertionResponse_builder{
			Type:  publicKeyCredentialType,
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

		require.Empty(
			t,
			cmp.Diff(want, got, protocmp.Transform()),
			"CredentialAssertionResponseV1ToV2 mismatch (-want +got)",
		)
	})
}
