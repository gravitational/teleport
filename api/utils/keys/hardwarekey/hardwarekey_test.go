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
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/prompt"
)

// TestPrivateKey_EncodeDecode tests encoding and decoding a hardware key signer.
// In particular, this tests that the public key is properly encoded and that the
// contextual key info and missing key info (old client logins) is handled correctly.
func TestPrivateKey_EncodeDecode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/)
	hwSigner, err := s.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{
		Policy: hardwarekey.PromptPolicyNone,
	})
	require.NoError(t, err)

	priv := hardwarekey.NewSigner(s, hwSigner.Ref)
	encoded, err := hardwarekey.EncodeSigner(priv)
	require.NoError(t, err)

	decodedPriv, err := hardwarekey.DecodeSigner(s, encoded)
	require.NoError(t, err)
	require.Equal(t, hwSigner, decodedPriv)
}

// TestPrivateKey_Prompt tests hardware key service PIN/Touch logic with a mocked service.
func TestPrivateKey_Prompt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := hardwarekey.NewMockHardwareKeyService(nil /*prompt*/) // a new prompt is set for each [doWithPrompt] call.

	for _, policy := range []hardwarekey.PromptPolicy{
		hardwarekey.PromptPolicyNone,
		hardwarekey.PromptPolicyTouch,
		hardwarekey.PromptPolicyPIN,
		hardwarekey.PromptPolicyTouchAndPIN,
	} {
		t.Run(fmt.Sprintf("policy:%+v", policy), func(t *testing.T) {
			type newPrivateKeyRet struct {
				priv *hardwarekey.Signer
				err  error
			}

			// Creating a new hardware key requires PIN/touch.
			newPrivateKeyReturn := doWithPrompt(t, s, policy, func() newPrivateKeyRet {
				hwSigner, err := s.NewPrivateKey(ctx, hardwarekey.PrivateKeyConfig{
					Policy: policy,
				})
				return newPrivateKeyRet{
					priv: hwSigner,
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
	t.Helper()
	// Mock a CLI prompt.
	pipeReader, pipeWriter := io.Pipe()
	promptReader := prompt.NewFakeReader()
	s.SetPrompt(hardwarekey.NewCLIPrompt(pipeWriter, promptReader))

	out := make(chan T)
	go func() {
		out <- fn()
	}()

	if policy.PINRequired {
		out := make([]byte, 100)
		_, err := pipeReader.Read(out)
		assert.NoError(t, err)
		assert.Contains(t, string(out), "Enter your YubiKey PIV PIN")
		// mock service doesn't actually check the pin, it just waits for input.
		promptReader.AddString("")
	}

	if policy.TouchRequired {
		out := make([]byte, 100)
		_, err := pipeReader.Read(out)
		assert.NoError(t, err)
		assert.Contains(t, string(out), "Tap your YubiKey")
		// mock touch.
		s.MockTouch()
	}

	select {
	case out := <-out:
		return out
	case <-time.After(time.Second):
		t.Error("failed to complete fn after prompts")
		return *new(T)
	}
}
