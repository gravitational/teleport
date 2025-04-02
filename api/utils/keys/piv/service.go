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
	"sync"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// TODO(Joerger): Rather than using a global cache and mutexes, clients should be updated
// to create a single YubiKeyService and ensure it is reused across the program execution.
var (
	// yubiKeys is a shared, thread-safe [YubiKey] cache by serial number. It allows for
	// separate goroutines to share a YubiKey connection to work around the single PC/SC
	// transaction (connection) limit.
	//
	// TODO(Joerger): This will replace the key cache in yubikey.go
	yubiKeys    map[uint32]*YubiKey = map[uint32]*YubiKey{}
	yubiKeysMux sync.Mutex
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
	if prompt == nil {
		prompt = hardwarekey.NewStdCLIPrompt()
	}
	return &YubiKeyService{
		prompt: prompt,
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

	pivSlot, err := parsePIVSlot(slotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the program has already retrieved and cached this key, return it.
	cachedKeysMu.Lock()
	defer cachedKeysMu.Unlock()

	if key, ok := cachedKeys[pivSlot]; ok && key.GetPromptPolicy() == config.Policy {
		return key, nil
	}

	// Use the first yubiKey we find.
	y, err := FindYubiKey(0, s.prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If PIN is required, check that PIN and PUK are not the defaults.
	if config.Policy.PINRequired {
		if err := y.checkOrSetPIN(ctx); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	generatePrivateKey := func() (*hardwarekey.Signer, error) {
		ref, err := y.generatePrivateKey(pivSlot, config.Policy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		hwPriv := hardwarekey.NewSigner(s, ref)
		cachedKeys[pivSlot] = hwPriv
		return hwPriv, nil
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
			if err := s.promptOverwriteSlot(ctx, nonTeleportCertificateMessage(pivSlot, cert)); err != nil {
				return nil, trace.Wrap(err)
			}
			return generatePrivateKey()
		}
	}

	// Check for an existing key in the slot that satisfies the required
	// prompt policy, or generate a new one if needed.
	slotCert, attCert, att, err := y.attestKey(pivSlot)
	switch {
	case errors.Is(err, piv.ErrNotFound):
		return generatePrivateKey()

	case err != nil:
		return nil, trace.Wrap(err)

	case config.Policy.TouchRequired && att.TouchPolicy == piv.TouchPolicyNever:
		msg := fmt.Sprintf("private key in YubiKey PIV slot %q does not require touch.", pivSlot)
		if err := s.promptOverwriteSlot(ctx, msg); err != nil {
			return nil, trace.Wrap(err)
		}
		return generatePrivateKey()

	case config.Policy.PINRequired && att.PINPolicy == piv.PINPolicyNever:
		msg := fmt.Sprintf("private key in YubiKey PIV slot %q does not require PIN", pivSlot)
		if err := s.promptOverwriteSlot(ctx, msg); err != nil {
			return nil, trace.Wrap(err)
		}
		return generatePrivateKey()
	}

	return hardwarekey.NewSigner(s, &hardwarekey.PrivateKeyRef{
		SerialNumber: y.serialNumber,
		SlotKey:      slotKey,
		PublicKey:    slotCert.PublicKey,
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: att.TouchPolicy != piv.TouchPolicyNever,
			PINRequired:   att.PINPolicy != piv.PINPolicyNever,
		},
		AttestationStatement: &hardwarekey.AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        slotCert.Raw,
					AttestationCert: attCert.Raw,
				},
			},
		},
	}), nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *YubiKeyService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	y, err := getYubiKey(ref.SerialNumber, s.prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return y.sign(ctx, ref, s.prompt, rand, digest, opts)
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
func (s *YubiKeyService) GetFullKeyRef(serialNumber uint32, slotKey hardwarekey.PIVSlotKey) (*hardwarekey.PrivateKeyRef, error) {
	keyRefsMux.Lock()
	defer keyRefsMux.Unlock()

	baseRef := baseKeyRef{serialNumber: serialNumber, slotKey: slotKey}
	if ref, ok := keyRefs[baseRef]; ok && ref != nil {
		return ref, nil
	}

	y, err := getYubiKey(serialNumber, s.prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pivSlot, err := parsePIVSlot(slotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	slotCert, attCert, att, err := y.attestKey(pivSlot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref := &hardwarekey.PrivateKeyRef{
		SerialNumber: serialNumber,
		SlotKey:      slotKey,
		PublicKey:    slotCert.PublicKey,
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: att.TouchPolicy != piv.TouchPolicyNever,
			PINRequired:   att.PINPolicy != piv.PINPolicyNever,
		},
		AttestationStatement: &hardwarekey.AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        slotCert.Raw,
					AttestationCert: attCert.Raw,
				},
			},
		},
	}

	keyRefs[baseRef] = ref
	return ref, nil
}

// Get the given YubiKey with the serial number. If the provided serialNumber is "0",
// return the first YubiKey found in the smart card list.
func getYubiKey(serialNumber uint32, prompt hardwarekey.Prompt) (*YubiKey, error) {
	yubiKeysMux.Lock()
	defer yubiKeysMux.Unlock()

	if y, ok := yubiKeys[serialNumber]; ok {
		return y, nil
	}

	y, err := FindYubiKey(serialNumber, prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	yubiKeys[y.serialNumber] = y
	return y, nil
}

func (s *YubiKeyService) promptOverwriteSlot(ctx context.Context, msg string) error {
	promptQuestion := fmt.Sprintf("%v\nWould you like to overwrite this slot's private key and certificate?", msg)
	if confirmed, confirmErr := s.prompt.ConfirmSlotOverwrite(ctx, promptQuestion); confirmErr != nil {
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
