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
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/prompt"
)

// TestPrivateKey_EncodeDecode tests encoding and decoding a hardware private key.
// In particular, this tests that the public key is properly encoded and that the
// contextual key info and missing key info (old client logins) is handled correctly.
func TestPrivateKey_EncodeDecode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := hardwarekey.NewMockHardwareKeyService()
	hwPriv, err := s.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{
		Policy: hardwarekey.PromptPolicyTouch,
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name         string
		ref          *hardwarekey.PrivateKeyRef
		updateKeyRef func(*hardwarekey.PrivateKeyRef) error
		expectPriv   *hardwarekey.PrivateKey
	}{
		{
			name:       "new client encoding",
			ref:        hwPriv.Ref,
			expectPriv: hwPriv,
		},
		{
			// Old client logins would only have encoded the serial number and slot key.
			// TODO(Joerger): DELETE IN v19.0.0
			name: "old client encoding",
			ref: &hardwarekey.PrivateKeyRef{
				SerialNumber: hwPriv.Ref.SerialNumber,
				SlotKey:      hwPriv.Ref.SlotKey,
			},
			expectPriv: hwPriv,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			priv := hardwarekey.NewPrivateKey(s, tt.ref)
			encoded, err := priv.Encode()
			require.NoError(t, err)

			decodedPriv, err := hardwarekey.DecodePrivateKey(s, encoded)
			require.NoError(t, err)
			require.Equal(t, tt.expectPriv, decodedPriv)
		})
	}
}

// TestPrivateKey_Prompt tests hardware key service PIN/Touch logic with a mocked service.
func TestPrivateKey_Prompt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := hardwarekey.NewMockHardwareKeyService()

	for _, policy := range []hardwarekey.PromptPolicy{
		hardwarekey.PromptPolicyNone,
		hardwarekey.PromptPolicyTouch,
		hardwarekey.PromptPolicyPIN,
		hardwarekey.PromptPolicyTouchAndPIN,
	} {
		t.Run(fmt.Sprintf("policy:%+v", policy), func(t *testing.T) {
			type newPrivateKeyRet struct {
				priv *hardwarekey.PrivateKey
				err  error
			}

			// Creating a new hardware key requires PIN/touch.
			newPrivateKeyReturn := doWithPrompt(t, s, policy, func() newPrivateKeyRet {
				hwPriv, err := s.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{
					Policy: policy,
				})
				return newPrivateKeyRet{
					priv: hwPriv,
					err:  err,
				}
			})
			require.NoError(t, newPrivateKeyReturn.err)
			hwPriv := newPrivateKeyReturn.priv
			require.NotNil(t, hwPriv)

			// Signatures requires PIN/touch. Do a bogus signature.
			err := doWithPrompt(t, s, policy, func() error {
				hash := sha512.Sum512(make([]byte, 512))
				_, err := hwPriv.Sign(rand.Reader, hash[:], crypto.SHA512)
				return err
			})
			require.NoError(t, err)
		})
	}
}

func doWithPrompt[T any](t *testing.T, s *hardwarekey.MockHardwareKeyService, policy hardwarekey.PromptPolicy, fn func() T) T {
	// Mock a CLI prompt.
	promptWriter := bytes.NewBuffer([]byte{})
	promptReader := prompt.NewFakeReader()
	s.SetPrompt(hardwarekey.NewCLIPrompt(promptWriter, promptReader))

	out := make(chan T)
	go func() {
		out <- fn()
	}()

	if policy.PINRequired {
		require.Eventually(t, func() bool {
			return strings.Contains(promptWriter.String(), "Enter your YubiKey PIV PIN")
		}, 100*time.Millisecond, 10*time.Millisecond)
		// mock service doesn't actually check the pin, it just waits for input.
		promptReader.AddString("")
	}

	if policy.TouchRequired {
		require.Eventually(t, func() bool {
			return strings.Contains(promptWriter.String(), "Tap your YubiKey")
		}, 100*time.Millisecond, 10*time.Millisecond)
		// mock touch.
		s.MockTouch()
	}

	select {
	case out := <-out:
		return out
	case <-time.After(100 * time.Millisecond):
		t.Error("failed to complete fn after prompts")
		return *new(T)
	}
}
