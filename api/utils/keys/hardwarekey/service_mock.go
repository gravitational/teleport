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

package hardwarekey

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"sync"

	"github.com/gravitational/trace"
)

// Currently Teleport does not provide a way to choose a specific hardware key,
// so we just hard code a serial number for all tests.
const serialNumber uint32 = 12345678

type fakeHardwarePrivateKey struct {
	crypto.Signer
	ref *PrivateKeyRef
}

// hardwareKeySlot references a specific hardware key slot on a specific hardware key.
type hardwareKeySlot struct {
	serialNumber uint32
	slot         PIVSlotKey
}

type mockHardwareKeyService struct {
	fakeHardwarePrivateKeys    map[hardwareKeySlot]*fakeHardwarePrivateKey
	fakeHardwarePrivateKeysMux *sync.Mutex
}

// NewMockHardwareKeyService returns a [mockHardwareKeyService] for use in tests.
func NewMockHardwareKeyService() *mockHardwareKeyService {
	return &mockHardwareKeyService{
		fakeHardwarePrivateKeys:    map[hardwareKeySlot]*fakeHardwarePrivateKey{},
		fakeHardwarePrivateKeysMux: &sync.Mutex{},
	}
}

func (s *mockHardwareKeyService) NewPrivateKey(ctx context.Context, config PrivateKeyConfig) (*PrivateKey, error) {
	s.fakeHardwarePrivateKeysMux.Lock()
	defer s.fakeHardwarePrivateKeysMux.Unlock()

	// Get the requested or default PIV slot.
	var slotKey PIVSlotKey
	var err error
	if config.CustomSlot != "" {
		slotKey, err = config.CustomSlot.Parse()
	} else {
		slotKey, err = GetDefaultSlotKey(config.Policy)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keySlot := hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         slotKey,
	}

	if priv, ok := s.fakeHardwarePrivateKeys[keySlot]; ok {
		return NewPrivateKey(s, priv.ref), nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref := &PrivateKeyRef{
		SerialNumber: serialNumber,
		SlotKey:      slotKey,
		PublicKey:    pub,
		Policy:       config.Policy,
		// Since this is only used in tests, we will ignore the attestation statement in the end.
		// We just need it to be non-nil so that it goes through the test modules implementation
		// of Attest
		AttestationStatement: &AttestationStatement{},
	}

	s.fakeHardwarePrivateKeys[keySlot] = &fakeHardwarePrivateKey{
		Signer: priv,
		ref:    ref,
	}

	return NewPrivateKey(s, ref), nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *mockHardwareKeyService) Sign(ctx context.Context, ref *PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	s.fakeHardwarePrivateKeysMux.Lock()
	defer s.fakeHardwarePrivateKeysMux.Unlock()

	priv, ok := s.fakeHardwarePrivateKeys[hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         ref.SlotKey,
	}]
	if !ok {
		return nil, trace.NotFound("key not found in slot %d", ref.SlotKey)
	}

	return priv.Sign(rand, digest, opts)
}

func (s *mockHardwareKeyService) SetPrompt(prompt Prompt) {}

// TODO(Joerger): DELETE IN v19.0.0
func (s *mockHardwareKeyService) GetMissingKeyRefDetails(ref *PrivateKeyRef) error {
	s.fakeHardwarePrivateKeysMux.Lock()
	defer s.fakeHardwarePrivateKeysMux.Unlock()

	priv, ok := s.fakeHardwarePrivateKeys[hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         ref.SlotKey,
	}]
	if !ok {
		return trace.NotFound("key not found in slot %d", ref.SlotKey)
	}

	*ref = *priv.ref
	return nil
}
