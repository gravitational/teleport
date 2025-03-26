// Copyright 2025 Gravitational, Inc.
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

// Package hardwarekey defines types and interfaces for hardware private keys.

package hardwarekey_test

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// TestEncodeDecodePrivateKey tests encoding and decoding a hardware private key.
// In particular, this tests that the public key is properly encoded and that the
// contextual key info and missing key info (old client logins) is handled correctly.
func TestEncodeDecodePrivateKey(t *testing.T) {
	s := &mockHardwareKeyService{}

	pub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	fullRef := &hardwarekey.PrivateKeyRef{
		SerialNumber: 12345678,
		SlotKey:      hardwarekey.PivSlotKeyTouch,
		PublicKey:    pub,
		Policy:       hardwarekey.PromptPolicyTouch,
		AttestationStatement: &hardwarekey.AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        []byte{1},
					AttestationCert: []byte{2},
				},
			},
		},
	}

	contextualKeyInfo := hardwarekey.ContextualKeyInfo{
		ProxyHost:   "billy.io",
		Username:    "Billy@billy.io",
		ClusterName: "billy.io",
	}
	priv := hardwarekey.NewPrivateKey(s, fullRef, contextualKeyInfo)

	for _, tt := range []struct {
		name         string
		ref          *hardwarekey.PrivateKeyRef
		updateKeyRef func(*hardwarekey.PrivateKeyRef) error
		expectPriv   *hardwarekey.PrivateKey
	}{
		{
			name:       "new client encoding",
			ref:        fullRef,
			expectPriv: priv,
		},
		{
			// Old client logins would only have encoded the serial number and slot key.
			// TODO(Joerger): DELETE IN v19.0.0
			name: "old client encoding",
			ref: &hardwarekey.PrivateKeyRef{
				SerialNumber: 12345678,
				SlotKey:      hardwarekey.PivSlotKeyTouch,
			},
			updateKeyRef: func(ref *hardwarekey.PrivateKeyRef) error {
				ref.PublicKey = pub
				ref.Policy = hardwarekey.PromptPolicyTouch
				ref.AttestationStatement = &hardwarekey.AttestationStatement{
					AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
						YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
							SlotCert:        []byte{1},
							AttestationCert: []byte{2},
						},
					},
				}
				return nil
			},
			expectPriv: priv,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			priv := hardwarekey.NewPrivateKey(s, tt.ref, hardwarekey.ContextualKeyInfo{})
			encoded, err := priv.Encode()
			require.NoError(t, err)

			decodedPriv, err := hardwarekey.DecodePrivateKey(s, encoded, contextualKeyInfo, tt.updateKeyRef)
			require.NoError(t, err)
			require.Equal(t, tt.expectPriv, decodedPriv)
		})
	}

}

type mockHardwareKeyService struct{}

func (s *mockHardwareKeyService) NewPrivateKey(_ context.Context, _ hardwarekey.PrivateKeyConfig) (*hardwarekey.PrivateKey, error) {
	return nil, nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *mockHardwareKeyService) Sign(_ context.Context, _ *hardwarekey.PrivateKeyRef, _ hardwarekey.ContextualKeyInfo, _ io.Reader, _ []byte, _ crypto.SignerOpts) ([]byte, error) {
	return nil, nil
}

func (s *mockHardwareKeyService) SetPrompt(_ hardwarekey.Prompt) {}
