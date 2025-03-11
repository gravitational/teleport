//go:build pivtest

// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"crypto/ed25519"
	"crypto/rand"
	"io"

	"github.com/gravitational/trace"
)

type fakeYubiKeyPIVService struct {
	// TODO(Joerger): TestHardwareKeyLogin fails because the hardware key service is not being
	// reused from login -> use, resulting in the key not being found. Rather than introducing
	// a global key map, ensure that the hardware key service is set from a shared call stack.
	keys map[crypto.PublicKey]crypto.Signer
}

func NewYubiKeyPIVService(ctx context.Context, _ HardwareKeyPrompt) HardwareKeyService {
	return &fakeYubiKeyPIVService{
		keys: map[crypto.PublicKey]crypto.Signer{},
	}
}

func (s *fakeYubiKeyPIVService) NewPrivateKey(ctx context.Context, customSlot PIVSlot, requiredPolicy PrivateKeyPolicy) (*HardwarePrivateKeyRef, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.keys[string(pub)] = priv

	return &HardwarePrivateKeyRef{
		PrivateKeyPolicy: requiredPolicy,
		PublicKey:        pub,
		// Since this is only used in tests, we will ignore the attestation statement in the end.
		// We just need it to be non-nil so that it goes through the test modules implementation
		// of AttestHardwareKey.
		AttestationStatement: &AttestationStatement{},
	}, nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *fakeYubiKeyPIVService) Sign(ctx context.Context, ref HardwarePrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	ed25519Pub, ok := ref.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, trace.BadParameter("expected public key of type %T", ed25519.PublicKey{})
	}
	priv, ok := s.keys[string(ed25519Pub)]
	if !ok {
		return nil, trace.NotFound("key not found")
	}

	return priv.Sign(rand, digest, opts)
}

func (s *fakeYubiKeyPIVService) SetPrompt(prompt HardwareKeyPrompt) {}

func (s PIVSlot) validate() error {
	return trace.Wrap(errPIVUnavailable)
}
