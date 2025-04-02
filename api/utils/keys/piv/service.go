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

	generatePrivateKey := func() (*hardwarekey.PrivateKey, error) {
		ref, err := y.generatePrivateKey(pivSlot, config.Policy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return hardwarekey.NewPrivateKey(s, ref), nil
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

	return hardwarekey.NewPrivateKey(s, &hardwarekey.PrivateKeyRef{
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
