//go:build piv && !pivtest

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
	"strconv"
	"sync"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// The PIV daemon only allows a single PC/SC transaction (connection) at a time,
// so we cache the YubiKey connection for re-use across the process.
//
// TODO(Joerger): Rather than using a global cache, clients should be updated to
// create a single YubiKeyService and ensure it is reused across the program
// execution.
var (
	yubiKeys    map[uint32]*YubiKey = map[uint32]*YubiKey{}
	yubiKeysMux sync.Mutex
)

// YubiKeyService is a YubiKey PIV implementation of [hardwarekey.Service].
type YubiKeyService struct {
	prompt hardwarekey.Prompt

	// ctx is provided to signature requests, since `crypto.Sign` does have
	// context support directly.
	ctx context.Context

	// TODO: do we need sign mutex to ensure signature requests are queued through without over-prompting?
	// Should this logic go ino the hardware key prompt itself? Maybe even sync.Cond?
	signMux sync.Mutex
}

// Returns a new [YubiKeyService].
//
// Only a single service should be created for each process to ensure the cached connections
// are shared and multiple services don't compete for PIV resources.
func NewYubiKeyService(ctx context.Context, prompt hardwarekey.Prompt) *YubiKeyService {
	if ctx == nil {
		ctx = context.Background()
	}

	if prompt == nil {
		prompt = &hardwarekey.CLIPrompt{}
	}

	return &YubiKeyService{
		ctx:    ctx,
		prompt: prompt,
	}
}

// NewPrivateKey creates or retrieves a hardware private key from the given PIV slot matching
// the given policy and returns the details required to perform signatures with that key.
//
// If a customSlot is not provided, the service uses the default slot for the given policy:
//   - !touch & !pin -> 9a
//   - !touch & pin  -> 9c
//   - touch & pin   -> 9d
//   - touch & !pin  -> 9e
func (s *YubiKeyService) NewPrivateKey(ctx context.Context, customSlot hardwarekey.PIVSlot, requiredPolicy hardwarekey.PromptPolicy) (*hardwarekey.PrivateKeyRef, error) {
	// Use the first yubiKey we find.
	y, err := s.getYubiKey(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the requested or default PIV slot.
	var pivSlot piv.Slot
	if customSlot != "" {
		slotKey, err := strconv.ParseUint(string(customSlot), 16, 32)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pivSlot, err = parsePIVSlot(uint32(slotKey))
	} else {
		pivSlot, err = getDefaultKeySlot(requiredPolicy)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If PIN is required, check that PIN and PUK are not the defaults.
	if requiredPolicy.PINRequired {
		if err := y.checkOrSetPIN(ctx, s.prompt); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// If a custom slot was not specified, check for a key in the
	// default slot for the given policy and generate a new one if needed.
	if customSlot == "" {
		// Check the client certificate in the slot.
		switch cert, err := y.getCertificate(pivSlot); {
		case err == nil && (len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != certOrgName):
			// Unknown cert found, prompt the user before we overwrite the slot.
			if err := s.promptOverwriteSlot(ctx, nonTeleportCertificateMessage(pivSlot, cert), hardwarekey.PrivateKeyInfo{}); err != nil {
				return nil, trace.Wrap(err)
			}
			return y.generatePrivateKey(pivSlot, requiredPolicy)
		case errors.Is(err, piv.ErrNotFound):
			return y.generatePrivateKey(pivSlot, requiredPolicy)
		case err != nil:
			return nil, trace.Wrap(err)
		}
	}

	// Check for an existing key in the slot that satisfies the required private
	// key policy, or generate a new one if needed.
	slotCert, attCert, att, err := y.attestKey(pivSlot)
	keyPolicy := hardwarekey.PromptPolicy{
		TouchRequired: att.TouchPolicy != piv.TouchPolicyNever,
		PINRequired:   att.PINPolicy != piv.PINPolicyNever,
	}
	switch {
	case err == nil && (requiredPolicy.TouchRequired && !keyPolicy.TouchRequired) || (requiredPolicy.PINRequired && !keyPolicy.PINRequired):
		// Key does not meet the required key policy, prompt the user before we overwrite the slot.
		msg := fmt.Sprintf("private key in YubiKey PIV slot %q does not meet prompt policy %v.", pivSlot, requiredPolicy)
		if err := s.promptOverwriteSlot(ctx, msg, hardwarekey.PrivateKeyInfo{}); err != nil {
			return nil, trace.Wrap(err)
		}
		return y.generatePrivateKey(pivSlot, requiredPolicy)
	case errors.Is(err, piv.ErrNotFound):
		return y.generatePrivateKey(pivSlot, requiredPolicy)
	case err != nil:
		return nil, trace.Wrap(err)
	}

	return &hardwarekey.PrivateKeyRef{
		SerialNumber: y.serialNumber,
		SlotKey:      pivSlot.Key,
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
	}, nil
}

func getDefaultKeySlot(policy hardwarekey.PromptPolicy) (piv.Slot, error) {
	switch policy {
	case hardwarekey.PromptPolicy{TouchRequired: false, PINRequired: false}:
		return piv.SlotAuthentication, nil
	case hardwarekey.PromptPolicy{TouchRequired: true, PINRequired: false}:
		return piv.SlotSignature, nil
	case hardwarekey.PromptPolicy{TouchRequired: true, PINRequired: true}:
		return piv.SlotKeyManagement, nil
	case hardwarekey.PromptPolicy{TouchRequired: false, PINRequired: false}:
		return piv.SlotCardAuthentication, nil
	default:
		return piv.Slot{}, trace.BadParameter("unexpected private key policy %v", policy)
	}
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

func (s *YubiKeyService) promptOverwriteSlot(ctx context.Context, msg string, keyInfo hardwarekey.PrivateKeyInfo) error {
	promptQuestion := fmt.Sprintf("%v\nWould you like to overwrite this slot's private key and certificate?", msg)
	if confirmed, confirmErr := s.prompt.ConfirmSlotOverwrite(ctx, promptQuestion, keyInfo); confirmErr != nil {
		return trace.Wrap(confirmErr)
	} else if !confirmed {
		return trace.Wrap(trace.CompareFailed(msg), "user declined to overwrite slot")
	}
	return nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *YubiKeyService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	// Usually, Sign will be called without context through the [crypto.Signer] interface,
	// so we opportunistically set the context.
	if ctx == context.TODO() {
		ctx = s.ctx
	}

	// To prevent concurrent calls to sign from failing due to PIV only handling a
	// single connection, use a lock to queue through signature requests one at a time.
	s.signMux.Lock()
	defer s.signMux.Unlock()

	y, err := s.getYubiKey(ref.SerialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return y.sign(ctx, ref, s.prompt, rand, digest, opts)
}

// SetPrompt sets the hardware key prompt used by the hardware key service, if applicable.
// This is used by Teleport Connect which sets the prompt later than the hardware key service,
// due to process initialization constraints.
func (s *YubiKeyService) SetPrompt(prompt hardwarekey.Prompt) {
	s.prompt = prompt
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
