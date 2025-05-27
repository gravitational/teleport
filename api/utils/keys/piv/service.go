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

// Package piv provides a PIV implementation of [hardwarekey.Service].
package piv

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// yubiKeyService is a global YubiKeyService used to share yubikey connections
// and prompt mutex logic across the process in cases where [NewYubiKeyService]
// is called multiple times.
//
// TODO(Joerger): Ensure all clients initialize [NewYubiKeyService] only once so we can
// remove this global variable.
var yubiKeyService *YubiKeyService
var yubiKeyServiceMu sync.Mutex

// YubiKeyService is a YubiKey PIV implementation of [hardwarekey.Service].
type YubiKeyService struct {
	prompt hardwarekey.Prompt
	// TODO(Joerger): Remove prompt mutex once there is no longer a shared global service
	// that needs its protection.
	promptMu sync.Mutex

	// signMu prevents prompting for PIN/touch repeatedly for concurrent signatures.
	// TODO(Joerger): Rather than preventing concurrent signatures, we can make the
	// PIN and touch prompts durable to concurrent signatures.
	signMu sync.Mutex

	// yubiKeys is a shared, thread-safe [YubiKey] cache by serial number. It allows for
	// separate goroutines to share a YubiKey connection to work around the single PC/SC
	// transaction (connection) per-yubikey limit.
	yubiKeys   map[uint32]*YubiKey
	yubiKeysMu sync.Mutex
}

// Returns a new [YubiKeyService]. If [customPrompt] is nil, the default CLI prompt will be used.
//
// Only a single service should be created for each process to ensure the cached connections
// are shared and multiple services don't compete for PIV resources.
func NewYubiKeyService(customPrompt hardwarekey.Prompt) *YubiKeyService {
	yubiKeyServiceMu.Lock()
	defer yubiKeyServiceMu.Unlock()

	if yubiKeyService != nil {
		// If a prompt is provided, prioritize it over the existing prompt value.
		if customPrompt != nil {
			yubiKeyService.setPrompt(customPrompt)
		}
		return yubiKeyService
	}

	if customPrompt == nil {
		customPrompt = hardwarekey.NewStdCLIPrompt()
	}

	yubiKeyService = &YubiKeyService{
		prompt:   customPrompt,
		yubiKeys: map[uint32]*YubiKey{},
	}
	return yubiKeyService
}

// NewPrivateKey creates a hardware private key that satisfies the provided [config],
// if one does not already exist, and returns a corresponding [hardwarekey.Signer].
//
// If a customSlot is not provided in [config], the service uses the default slot for the given policy:
//   - !touch & !pin -> 9a
//   - !touch & pin  -> 9c
//   - touch  & pin  -> 9d
//   - touch  & !pin -> 9e
func (s *YubiKeyService) NewPrivateKey(ctx context.Context, config hardwarekey.PrivateKeyConfig) (*hardwarekey.Signer, error) {
	// Use the first yubiKey we find.
	y, err := s.getYubiKey(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the requested or default PIV slot.
	var slotKey hardwarekey.PIVSlotKey
	if config.CustomSlot != "" {
		slotKey, err = config.CustomSlot.Parse()
	} else {
		slotKey, err = hardwarekey.GetDefaultSlotKey(config.Policy)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pivSlot, err := parsePIVSlot(slotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If PIN is required, check that PIN and PUK are not the defaults.
	// This also caches the PIN in the PIV connection, similar to a call
	// to [hardwarekey.Signer.WarmupHardwareKey].
	if config.Policy.PINRequired {
		if err := y.checkOrSetPIN(ctx, s.getPrompt(), config.ContextualKeyInfo, config.PINCacheTTL); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	generatePrivateKey := func() (*hardwarekey.Signer, error) {
		ref, err := y.generatePrivateKey(pivSlot, config.Policy, config.Algorithm, config.PINCacheTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		signer := hardwarekey.NewSigner(s, ref, config.ContextualKeyInfo)
		if config.Policy.TouchRequired {
			// Warmup the hardware key with a touch prompt now rather than later. This is intended
			// to avoid prompting for PIV touch directly after a WebAuthn touch prompt, which
			// can delay the PIV signature and beyond the expected [sigsignTouchPromptDelay].
			if err := signer.WarmupHardwareKey(ctx); err != nil {
				return nil, trace.Wrap(err)
			}
		}

		return signer, nil
	}

	// If a custom slot was not specified, check for a key in the
	// default slot for the given policy and generate a new one if needed.
	if config.CustomSlot == "" {
		switch err := y.checkCertificate(pivSlot); {
		case trace.IsNotFound(err):
			return generatePrivateKey()

		// Unknown cert found, this slot could be in use by a non-teleport client.
		// Prompt the user before we overwrite the slot.
		case errors.As(err, &nonTeleportCertError{}):
			if err := s.promptOverwriteSlot(ctx, err.Error(), config.ContextualKeyInfo); err != nil {
				return nil, trace.Wrap(err)
			}
			return generatePrivateKey()

		case err != nil:
			return nil, trace.Wrap(err)
		}
	}

	// Check for an existing key in the slot that satisfies the required
	// prompt policy, or generate a new one if needed.
	keyRef, err := y.getKeyRef(pivSlot, config.PINCacheTTL)
	switch {
	case errors.Is(err, piv.ErrNotFound):
		return generatePrivateKey()

	case err != nil:
		return nil, trace.Wrap(err)

	case config.Policy.TouchRequired && !keyRef.Policy.TouchRequired:
		msg := fmt.Sprintf("private key in YubiKey PIV slot %q does not require touch.", pivSlot)
		if err := s.promptOverwriteSlot(ctx, msg, config.ContextualKeyInfo); err != nil {
			return nil, trace.Wrap(err)
		}
		return generatePrivateKey()

	case config.Policy.PINRequired && !keyRef.Policy.PINRequired:
		msg := fmt.Sprintf("private key in YubiKey PIV slot %q does not require PIN", pivSlot)
		if err := s.promptOverwriteSlot(ctx, msg, config.ContextualKeyInfo); err != nil {
			return nil, trace.Wrap(err)
		}
		return generatePrivateKey()
	}

	signer := hardwarekey.NewSigner(s, keyRef, config.ContextualKeyInfo)
	if config.Policy.TouchRequired {
		// Warmup the hardware key with a touch prompt now rather than later. This is intended
		// to avoid prompting for PIV touch directly after a WebAuthn touch prompt, which
		// can delay the PIV signature and beyond the expected [sigsignTouchPromptDelay].
		if err := signer.WarmupHardwareKey(ctx); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return signer, nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *YubiKeyService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	y, err := s.getYubiKey(ref.SerialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pivSlot, err := parsePIVSlot(ref.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check that the public key in the slot matches our record.
	publicKey, err := y.getPublicKey(pivSlot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !publicKey.Equal(ref.PublicKey) {
		return nil, trace.CompareFailed("public key mismatch on PIV slot 0x%x", pivSlot.Key)
	}

	// If the sign request is for an unknown agent key, ensure that the requested PIV slot was
	// configured with a self-signed Teleport metadata certificate.
	if keyInfo.AgentKeyInfo.UnknownAgentKey {
		switch err := y.checkCertificate(pivSlot); {
		case trace.IsNotFound(err), errors.As(err, &nonTeleportCertError{}):
			return nil, trace.Wrap(err, agentRequiresTeleportCertMessage)
		case err != nil:
			return nil, trace.Wrap(err)
		}
	}

	return y.sign(ctx, ref, keyInfo, s.getPrompt(), rand, digest, opts)
}

// TODO(Joerger): Re-attesting the key every time we decode a hardware key signer is very resource
// intensive. This cache is a stand-in solution for the problem, which was previously handled within
// the YubiKeyPrivateKey cache that is being phased out with this change. In a follow up, the attested
// information will be saved to the key file at login time so each client will not need to re-attest
// the hardware key at all.
var (
	keyRefs    = map[baseKeyRef]*hardwarekey.PrivateKeyRef{}
	keyRefsMux sync.Mutex
)

type baseKeyRef struct {
	serialNumber uint32
	slotKey      hardwarekey.PIVSlotKey
}

// GetFullKeyRef gets the full [PrivateKeyRef] for an existing hardware private
// key in the given slot of the hardware key with the given serial number.
//
// Used for backwards compatibility with old logins.
// TODO(Joerger): DELETE IN v19.0.0
func (s *YubiKeyService) GetFullKeyRef(serialNumber uint32, slotKey hardwarekey.PIVSlotKey) (*hardwarekey.PrivateKeyRef, error) {
	keyRefsMux.Lock()
	defer keyRefsMux.Unlock()

	baseRef := baseKeyRef{serialNumber: serialNumber, slotKey: slotKey}
	if ref, ok := keyRefs[baseRef]; ok && ref != nil {
		return ref, nil
	}

	y, err := s.getYubiKey(serialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pivSlot, err := parsePIVSlot(slotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref, err := y.getKeyRef(pivSlot, 0 /*PIN is not cached for out-of-date client keys*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyRefs[baseRef] = ref
	return ref, nil
}

// Get the given YubiKey with the serial number. If the provided serialNumber is "0",
// return the first YubiKey found in the smart card list.
func (s *YubiKeyService) getYubiKey(serialNumber uint32) (*YubiKey, error) {
	s.yubiKeysMu.Lock()
	defer s.yubiKeysMu.Unlock()

	if y, ok := s.yubiKeys[serialNumber]; ok {
		return y, nil
	}

	y, err := FindYubiKey(serialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.yubiKeys[y.serialNumber] = y
	return y, nil
}

func (s *YubiKeyService) promptOverwriteSlot(ctx context.Context, msg string, keyInfo hardwarekey.ContextualKeyInfo) error {
	promptQuestion := fmt.Sprintf("%v\nWould you like to overwrite this slot's private key and certificate?", msg)
	if confirmed, confirmErr := s.getPrompt().ConfirmSlotOverwrite(ctx, promptQuestion, keyInfo); confirmErr != nil {
		return trace.Wrap(confirmErr)
	} else if !confirmed {
		return trace.Wrap(trace.CompareFailed(msg), "user declined to overwrite slot")
	}
	return nil
}

func (s *YubiKeyService) setPrompt(prompt hardwarekey.Prompt) {
	s.promptMu.Lock()
	defer s.promptMu.Unlock()
	s.prompt = prompt
}

func (s *YubiKeyService) getPrompt() hardwarekey.Prompt {
	s.promptMu.Lock()
	defer s.promptMu.Unlock()
	return s.prompt
}
