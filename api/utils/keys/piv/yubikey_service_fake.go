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

package piv

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// TODO(Joerger): Rather than using a global cache, clients should be updated to
// create a single YubiKeyService and ensure it is reused across the program
// execution.
var (
	keys    = map[string]crypto.Signer{}
	keysMux sync.Mutex
)

type fakeYubiKeyPIVService struct{}

func NewYubiKeyService(ctx context.Context, _ hardwarekey.Prompt) *fakeYubiKeyPIVService {
	return &fakeYubiKeyPIVService{}
}

func (s *fakeYubiKeyPIVService) NewPrivateKey(ctx context.Context, config hardwarekey.PrivateKeyConfig) (*hardwarekey.PrivateKey, error) {
	keysMux.Lock()
	defer keysMux.Unlock()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keys[string(pub)] = priv

	ref := &hardwarekey.PrivateKeyRef{
		Policy:    config.Policy,
		PublicKey: pub,
		// Since this is only used in tests, we will ignore the attestation statement in the end.
		// We just need it to be non-nil so that it goes through the test modules implementation
		// of AttestHardwareKey.
		AttestationStatement: &hardwarekey.AttestationStatement{},
		ContextualKeyInfo:    config.ContextualKeyInfo,
	}

	return hardwarekey.NewPrivateKey(s, ref), nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *fakeYubiKeyPIVService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	keysMux.Lock()
	defer keysMux.Unlock()

	ed25519Pub, ok := ref.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, trace.BadParameter("expected public key of type %T", ed25519.PublicKey{})
	}
	priv, ok := keys[string(ed25519Pub)]
	if !ok {
		return nil, trace.NotFound("key not found")
	}

	return priv.Sign(rand, digest, opts)
}

func (s *fakeYubiKeyPIVService) SetPrompt(prompt hardwarekey.Prompt) {}

// TODO(Joerger): DELETE IN v19.0.0
func UpdateKeyRef(ref *hardwarekey.PrivateKeyRef) error {
	return nil
}
