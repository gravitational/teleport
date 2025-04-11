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
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// TODO(Joerger): Rather than using a global cache and mutexes, clients should be updated
// to create a single YubiKeyService and ensure it is reused across the program execution.
var (
	// yubiKeys is a shared, thread-safe [YubiKey] cache by serial number. It allows for
	// separate goroutines to share a YubiKey connection to work around the single PC/SC
	// transaction (connection) per-yubikey limit.
	yubiKeys    map[uint32]*YubiKey = map[uint32]*YubiKey{}
	yubiKeysMux sync.Mutex

	// promptMux is used to prevent over-prompting, especially for back-to-back sign requests
	// since touch/PIN from the first signature should be cached for following signatures.
	promptMux sync.Mutex
)

// YubiKeyService is a YubiKey PIV implementation of [hardwarekey.Service].
type YubiKeyService struct {
	prompt hardwarekey.Prompt
}

// Returns a new [YubiKeyService]. If [customPrompt] is nil, the default CLI prompt will be used.
//
// Only a single service should be created for each process to ensure the cached connections
// are shared and multiple services don't compete for PIV resources.
func NewYubiKeyService(customPrompt hardwarekey.Prompt) *YubiKeyService {
	if customPrompt == nil {
		customPrompt = hardwarekey.NewStdCLIPrompt()
	}

	return &YubiKeyService{
		prompt: customPrompt,
	}
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
	if config.Policy.PINRequired {
		if err := s.checkOrSetPIN(ctx, y, config.ContextualKeyInfo); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	generatePrivateKey := func() (*hardwarekey.Signer, error) {
		ref, err := y.generatePrivateKey(pivSlot, config.Policy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return hardwarekey.NewSigner(s, ref, config.ContextualKeyInfo), nil
	}

	// If a custom slot was not specified, check for a key in the
	// default slot for the given policy and generate a new one if needed.
	if config.CustomSlot == "" {
		switch cert, err := y.getCertificate(pivSlot); {
		case errors.Is(err, piv.ErrNotFound):
			return generatePrivateKey()

		case err != nil:
			return nil, trace.Wrap(err)

		// Unknown cert found, this slot could be in use by a non-teleport client.
		// Prompt the user before we overwrite the slot.
		case len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != certOrgName:
			if err := s.promptOverwriteSlot(ctx, nonTeleportCertificateMessage(pivSlot, cert), config.ContextualKeyInfo); err != nil {
				return nil, trace.Wrap(err)
			}
			return generatePrivateKey()
		}
	}

	// Check for an existing key in the slot that satisfies the required
	// prompt policy, or generate a new one if needed.
	keyRef, err := y.getKeyRef(pivSlot)
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

	return hardwarekey.NewSigner(s, keyRef, config.ContextualKeyInfo), nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *YubiKeyService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	y, err := s.getYubiKey(ref.SerialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	promptMux.Lock()
	defer promptMux.Unlock()

	return y.sign(ctx, ref, keyInfo, s.prompt, rand, digest, opts)
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

	ref, err := y.getKeyRef(pivSlot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyRefs[baseRef] = ref
	return ref, nil
}

// Get the given YubiKey with the serial number. If the provided serialNumber is "0",
// return the first YubiKey found in the smart card list.
func (s *YubiKeyService) getYubiKey(serialNumber uint32) (*YubiKey, error) {
	yubiKeysMux.Lock()
	defer yubiKeysMux.Unlock()

	if y, ok := yubiKeys[serialNumber]; ok {
		return y, nil
	}

	y, err := FindYubiKey(serialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	yubiKeys[y.serialNumber] = y
	return y, nil
}

// checkOrSetPIN prompts the user for PIN and verifies it with the YubiKey.
// If the user provides the default PIN, they will be prompted to set a
// non-default PIN and PUK before continuing.
func (s *YubiKeyService) checkOrSetPIN(ctx context.Context, y *YubiKey, keyInfo hardwarekey.ContextualKeyInfo) error {
	promptMux.Lock()
	defer promptMux.Unlock()

	pin, err := s.prompt.AskPIN(ctx, hardwarekey.PINOptional, keyInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	switch pin {
	case piv.DefaultPIN:
		fmt.Fprintf(os.Stderr, "The default PIN %q is not supported.\n", piv.DefaultPIN)
		fallthrough
	case "":
		pin, err = y.setPINAndPUKFromDefault(ctx, s.prompt, keyInfo)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(y.verifyPIN(pin))
}

func (s *YubiKeyService) promptOverwriteSlot(ctx context.Context, msg string, keyInfo hardwarekey.ContextualKeyInfo) error {
	promptMux.Lock()
	defer promptMux.Unlock()

	promptQuestion := fmt.Sprintf("%v\nWould you like to overwrite this slot's private key and certificate?", msg)
	if confirmed, confirmErr := s.prompt.ConfirmSlotOverwrite(ctx, promptQuestion, keyInfo); confirmErr != nil {
		return trace.Wrap(confirmErr)
	} else if !confirmed {
		return trace.Wrap(trace.CompareFailed(msg), "user declined to overwrite slot")
	}
	return nil
}

func nonTeleportCertificateMessage(slot piv.Slot, cert *x509.Certificate) string {
	// Gather a small list of user-readable x509 certificate fields to display to the user.
	sum := sha256.Sum256(cert.Raw)
	fingerPrint := hex.EncodeToString(sum[:])
	return fmt.Sprintf(`Certificate in YubiKey PIV slot %q is not a Teleport client cert:
Slot %s:
	Algorithm:		%v
	Subject DN:		%v
	Issuer DN:		%v
	Serial:			%v
	Fingerprint:	%v
	Not before:		%v
	Not after:		%v
`,
		slot, slot,
		cert.SignatureAlgorithm,
		cert.Subject,
		cert.Issuer,
		cert.SerialNumber,
		fingerPrint,
		cert.NotBefore,
		cert.NotAfter,
	)
}
