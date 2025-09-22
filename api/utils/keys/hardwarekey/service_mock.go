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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"sync"
	"time"

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

type MockHardwareKeyService struct {
	prompt    Prompt
	promptMu  sync.Mutex
	mockTouch chan struct{}

	fakeHardwarePrivateKeys    map[hardwareKeySlot]*fakeHardwarePrivateKey
	fakeHardwarePrivateKeysMux sync.Mutex

	// mock a PIV slot with a key but no teleport metadata cert.
	unknownAgentKey map[hardwareKeySlot]bool
}

// NewMockHardwareKeyService returns a [mockHardwareKeyService] for use in tests.
// If [prompt] is provided, the service will also mock PIN and touch prompts.
func NewMockHardwareKeyService(prompt Prompt) *MockHardwareKeyService {
	return &MockHardwareKeyService{
		prompt:                  prompt,
		mockTouch:               make(chan struct{}),
		fakeHardwarePrivateKeys: map[hardwareKeySlot]*fakeHardwarePrivateKey{},
		unknownAgentKey:         map[hardwareKeySlot]bool{},
	}
}

func (s *MockHardwareKeyService) NewPrivateKey(ctx context.Context, config PrivateKeyConfig) (*Signer, error) {
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
		return NewSigner(s, priv.ref, config.ContextualKeyInfo), nil
	}

	// generating a new key with PIN/touch requirements requires the corresponding prompt.
	if err := s.tryPrompt(ctx, config.Policy, config.ContextualKeyInfo); err != nil {
		return nil, trace.Wrap(err)
	}

	var priv crypto.Signer
	switch config.Algorithm {
	// Use ECDSA key by default.
	case SignatureAlgorithmEC256, 0:
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case SignatureAlgorithmEd25519:
		_, priv, err = ed25519.GenerateKey(rand.Reader)
	case SignatureAlgorithmRSA2048:
		//nolint:forbidigo // Allow /api to generate RSA key without importing Teleport.
		priv, err = rsa.GenerateKey(rand.Reader, 2048)
	default:
		return nil, trace.BadParameter("unknown algorithm option %v", config.Algorithm)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref := &PrivateKeyRef{
		SerialNumber: serialNumber,
		SlotKey:      slotKey,
		PublicKey:    priv.Public(),
		Policy:       config.Policy,
		// Since this is only used in tests, we will ignore the attestation statement in the end.
		// We just need it to be non-nil so that it goes through the test modules implementation
		// of Attest
		AttestationStatement: &AttestationStatement{},
		PINCacheTTL:          config.PINCacheTTL,
	}

	if err := ref.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	s.fakeHardwarePrivateKeys[keySlot] = &fakeHardwarePrivateKey{
		Signer: priv,
		ref:    ref,
	}

	return NewSigner(s, ref, config.ContextualKeyInfo), nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *MockHardwareKeyService) Sign(ctx context.Context, ref *PrivateKeyRef, keyInfo ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	s.fakeHardwarePrivateKeysMux.Lock()
	defer s.fakeHardwarePrivateKeysMux.Unlock()

	slot := hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         ref.SlotKey,
	}

	if keyInfo.AgentKeyInfo.UnknownAgentKey && s.unknownAgentKey[slot] {
		return nil, trace.BadParameter("unknown agent key")
	}

	priv, ok := s.fakeHardwarePrivateKeys[slot]
	if !ok {
		return nil, trace.NotFound("key not found in slot 0x%x", ref.SlotKey)
	}

	if err := s.tryPrompt(ctx, ref.Policy, keyInfo); err != nil {
		return nil, trace.Wrap(err)
	}

	return priv.Sign(rand, digest, opts)
}

func (s *MockHardwareKeyService) tryPrompt(ctx context.Context, policy PromptPolicy, keyInfo ContextualKeyInfo) error {
	s.promptMu.Lock()
	defer s.promptMu.Unlock()

	if s.prompt == nil || (!policy.PINRequired && !policy.TouchRequired) {
		return nil
	}

	if policy.PINRequired {
		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		if _, err := s.prompt.AskPIN(ctx, PINRequired, keyInfo); err != nil {
			return trace.Wrap(err, "failed to handle pin prompt")
		}
		// We don't actually check the PIN for the current tests, any input is sufficient to unblock the prompt.
	}

	if policy.TouchRequired {
		if err := s.prompt.Touch(ctx, keyInfo); err != nil {
			return trace.Wrap(err)
		}
		select {
		case <-s.mockTouch:
		case <-time.After(100 * time.Millisecond):
			return trace.Wrap(context.DeadlineExceeded, "failed to handle touch prompt")
		}
	}

	return nil
}

func (s *MockHardwareKeyService) SetPrompt(prompt Prompt) {
	s.promptMu.Lock()
	defer s.promptMu.Unlock()
	s.prompt = prompt
}

func (s *MockHardwareKeyService) AddUnknownAgentKey(ref *PrivateKeyRef) {
	s.unknownAgentKey[hardwareKeySlot{
		serialNumber: ref.SerialNumber,
		slot:         ref.SlotKey,
	}] = true
}

// TODO(Joerger): DELETE IN v19.0.0
func (s *MockHardwareKeyService) GetFullKeyRef(serialNumber uint32, slotKey PIVSlotKey) (*PrivateKeyRef, error) {
	s.fakeHardwarePrivateKeysMux.Lock()
	defer s.fakeHardwarePrivateKeysMux.Unlock()

	priv, ok := s.fakeHardwarePrivateKeys[hardwareKeySlot{
		serialNumber: serialNumber,
		slot:         slotKey,
	}]
	if !ok {
		return nil, trace.NotFound("key not found in slot 0x%x", slotKey)
	}

	return priv.ref, nil
}

func (s *MockHardwareKeyService) MockTouch() {
	s.mockTouch <- struct{}{}
}

func (s *MockHardwareKeyService) Reset() {
	s.fakeHardwarePrivateKeysMux.Lock()
	defer s.fakeHardwarePrivateKeysMux.Unlock()
	s.fakeHardwarePrivateKeys = map[hardwareKeySlot]*fakeHardwarePrivateKey{}
}
