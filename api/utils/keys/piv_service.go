//go:build piv && !pivtest

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

package keys

import (
	"context"
	"crypto"
	"io"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// YubiKeyService is a YubiKey PIV implementation of [hardwarekey.Service].
type YubiKeyService struct {
	prompt hardwarekey.Prompt
}

// Returns a new [YubiKeyService]. If [prompt] is nil, the default CLI prompt will be used.
//
// Only a single service should be created for each process to ensure the cached connections
// are shared and multiple services don't compete for PIV resources.
func NewYubiKeyService(prompt hardwarekey.Prompt) *YubiKeyService {
	return &YubiKeyService{
		prompt: prompt,
	}
}

// NewPrivateKey creates or retrieves a hardware private key from the given PIV slot matching
// the given policy and returns the details required to perform signatures with that key.
//
// If a customSlot is not provided, the service uses the default slot for the given policy:
//   - !touch & !pin -> 9a
//   - !touch & pin  -> 9c
//   - touch  & pin  -> 9d
//   - touch  & !pin -> 9e
func (s *YubiKeyService) NewPrivateKey(ctx context.Context, config hardwarekey.PrivateKeyConfig) (*hardwarekey.PrivateKey, error) {
	return newPrivateKey(ctx, config, s.prompt, s)
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *YubiKeyService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	y, err := FindYubiKey(ref.SerialNumber, s.prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return y.sign(ctx, ref, s.prompt, rand, digest, opts)
}

// GetMissingKeyRefDetails updates the key ref with missing information by querying the hardware key.
func (s *YubiKeyService) GetMissingKeyRefDetails(ref *hardwarekey.PrivateKeyRef) error {
	y, err := FindYubiKey(ref.SerialNumber, s.prompt)
	if err != nil {
		return trace.Wrap(err)
	}

	pivSlot, err := parsePIVSlot(ref.SlotKey)
	if err != nil {
		return trace.Wrap(err)
	}

	slotCert, attCert, att, err := y.attestKey(pivSlot)
	if err != nil {
		return trace.Wrap(err)
	}

	ref.PublicKey = slotCert.PublicKey
	ref.Policy = hardwarekey.PromptPolicy{
		TouchRequired: att.TouchPolicy != piv.TouchPolicyNever,
		PINRequired:   att.PINPolicy != piv.PINPolicyNever,
	}
	ref.AttestationStatement = &hardwarekey.AttestationStatement{
		AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
			YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
				SlotCert:        slotCert.Raw,
				AttestationCert: attCert.Raw,
			},
		},
	}
	return nil
}
