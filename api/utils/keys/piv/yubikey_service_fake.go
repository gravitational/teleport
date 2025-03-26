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

// TODO(Joerger): Instead of using a distinct build tag for tests, tests should inject
// a mock hardware key service, e.g. into the CLI conf.

// TODO(Joerger): Rather than using a global cache, clients should be updated to
// create a single YubiKeyService and ensure it is reused across the program
// execution.
var (
	hardwarePrivateKeys    = map[hardwareKeySlot]*fakeHardwarePrivateKey{}
	hardwarePrivateKeysMux sync.Mutex
)

// Currently Teleport does not provide a way to choose a specific hardware key,
// so we just hard code a serial number for all tests.
const serialNumber uint32 = 12345678

type fakeHardwarePrivateKey struct {
	crypto.Signer
	ref *hardwarekey.PrivateKeyRef
}

// hardwareKeySlot references a specific hardware key slot on a specific hardware key.
type hardwareKeySlot struct {
	serialNumber uint32
	slot         hardwarekey.PIVSlotKey
}

type fakeYubiKeyPIVService struct{}

func NewYubiKeyService(_ context.Context, _ hardwarekey.Prompt) *fakeYubiKeyPIVService {
	return &fakeYubiKeyPIVService{}
}

func (s *fakeYubiKeyPIVService) NewPrivateKey(ctx context.Context, config hardwarekey.PrivateKeyConfig) (*hardwarekey.PrivateKey, error) {
	hardwarePrivateKeysMux.Lock()
	defer hardwarePrivateKeysMux.Unlock()

	// Get the requested or default PIV slot.
	var slotKey hardwarekey.PIVSlotKey
	var err error
	if config.CustomSlot != "" {
		slotKey, err = config.CustomSlot.Parse()
	} else {
		slotKey, err = hardwarekey.GetDefaultSlotKey(config.Policy)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keySlot := hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         slotKey,
	}

	if priv, ok := hardwarePrivateKeys[keySlot]; ok {
		return hardwarekey.NewPrivateKey(s, priv.ref), nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref := &hardwarekey.PrivateKeyRef{
		SerialNumber: serialNumber,
		SlotKey:      slotKey,
		PublicKey:    pub,
		Policy:       config.Policy,
		// Since this is only used in tests, we will ignore the attestation statement in the end.
		// We just need it to be non-nil so that it goes through the test modules implementation
		// of AttestHardwareKey.
		AttestationStatement: &hardwarekey.AttestationStatement{},
		ContextualKeyInfo:    config.ContextualKeyInfo,
	}

	hardwarePrivateKeys[keySlot] = &fakeHardwarePrivateKey{
		Signer: priv,
		ref:    ref,
	}

	return hardwarekey.NewPrivateKey(s, ref), nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *fakeYubiKeyPIVService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, _ hardwarekey.ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	hardwarePrivateKeysMux.Lock()
	defer hardwarePrivateKeysMux.Unlock()

	priv, ok := hardwarePrivateKeys[hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         ref.SlotKey,
	}]
	if !ok {
		return nil, trace.NotFound("key not found in slot %d", ref.SlotKey)
	}

	return priv.Sign(rand, digest, opts)
}

func (s *fakeYubiKeyPIVService) SetPrompt(prompt hardwarekey.Prompt) {}

// TODO(Joerger): DELETE IN v19.0.0
func UpdateKeyRef(ref *hardwarekey.PrivateKeyRef) error {
	hardwarePrivateKeysMux.Lock()
	defer hardwarePrivateKeysMux.Unlock()

	priv, ok := hardwarePrivateKeys[hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         ref.SlotKey,
	}]
	if !ok {
		return trace.NotFound("key not found in slot %d", ref.SlotKey)
	}

	*ref = *priv.ref
	return nil
}
