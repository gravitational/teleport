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

package keys

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
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

type YubiKeyPIVService struct {
	prompt HardwareKeyPrompt

	// ctx is provided to signature requests, since `crypto.Sign` does have
	// context support directly.
	ctx context.Context

	// TODO: do we need sign mutex to ensure signature requests are queued through without over-prompting?
	// Should this logic go ino the hardware key prompt itself? Maybe even sync.Cond?
	signMux sync.Mutex
}

func NewYubiKeyPIVService(ctx context.Context, prompt HardwareKeyPrompt) HardwareKeyService {
	if ctx == nil {
		ctx = context.Background()
	}

	if prompt == nil {
		prompt = &CLIPrompt{}
	}

	return &YubiKeyPIVService{
		ctx:    ctx,
		prompt: prompt,
	}
}

func (s *YubiKeyPIVService) NewPrivateKey(ctx context.Context, customSlot PIVSlot, requiredPolicy PrivateKeyPolicy) (*HardwarePrivateKeyRef, error) {
	// Use the first yubiKey we find.
	y, err := s.getYubiKey(0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the requested or default PIV slot.
	var pivSlot piv.Slot
	if customSlot != "" {
		pivSlot, err = customSlot.parse()
	} else {
		pivSlot, err = GetDefaultKeySlot(requiredPolicy)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If PIN is required, check that PIN and PUK are not the defaults.
	if requiredPolicy.IsHardwareKeyPINVerified() {
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
			if err := s.promptOverwriteSlot(ctx, nonTeleportCertificateMessage(pivSlot, cert), KeyInfo{}); err != nil {
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
	keyPolicy := GetPrivateKeyPolicyFromAttestation(att)
	switch {
	case err == nil && !requiredPolicy.IsSatisfiedBy(keyPolicy):
		// Key does not meet the required key policy, prompt the user before we overwrite the slot.
		msg := fmt.Sprintf("private key in YubiKey PIV slot %q does not meet private key policy %q.", pivSlot, requiredPolicy)
		if err := s.promptOverwriteSlot(ctx, msg, KeyInfo{}); err != nil {
			return nil, trace.Wrap(err)
		}
		return y.generatePrivateKey(pivSlot, requiredPolicy)
	case errors.Is(err, piv.ErrNotFound):
		return y.generatePrivateKey(pivSlot, requiredPolicy)
	case err != nil:
		return nil, trace.Wrap(err)
	}

	return &HardwarePrivateKeyRef{
		SerialNumber:     y.serialNumber,
		SlotKey:          pivSlot.Key,
		PublicKey:        slotCert.PublicKey,
		PrivateKeyPolicy: keyPolicy,
		AttestationStatement: &AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        slotCert.Raw,
					AttestationCert: attCert.Raw,
				},
			},
		},
	}, nil
}

func (s *YubiKeyPIVService) promptOverwriteSlot(ctx context.Context, msg string, keyInfo KeyInfo) error {
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
func (s *YubiKeyPIVService) Sign(ctx context.Context, ref *HardwarePrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
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

func (s *YubiKeyPIVService) SetPrompt(prompt HardwareKeyPrompt) {
	s.prompt = prompt
}

// Get the given YubiKey with the serial number. If the provided serialNumber is "0",
// return the first YubiKey found in the smart card list.
func (s *YubiKeyPIVService) getYubiKey(serialNumber uint32) (*YubiKey, error) {
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

// FindYubiKey finds a yubiKey PIV card by serial number. If no serial
// number is provided, the first yubiKey found will be returned.
func FindYubiKey(serialNumber uint32) (*YubiKey, error) {
	yubiKeyCards, err := findYubiKeyCards()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(yubiKeyCards) == 0 {
		if serialNumber != 0 {
			return nil, trace.ConnectionProblem(nil, "no YubiKey device connected with serial number %d", serialNumber)
		}
		return nil, trace.ConnectionProblem(nil, "no YubiKey device connected")
	}

	for _, card := range yubiKeyCards {
		y, err := newYubiKey(card)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if serialNumber == 0 || y.serialNumber == serialNumber {
			return y, nil
		}
	}

	return nil, trace.ConnectionProblem(nil, "no YubiKey device connected with serial number %d", serialNumber)
}

// PIVCardTypeYubiKey is the PIV card type assigned to yubiKeys.
const PIVCardTypeYubiKey = "yubikey"

// findYubiKeyCards returns a list of connected yubiKey PIV card names.
func findYubiKeyCards() ([]string, error) {
	cards, err := piv.Cards()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var yubiKeyCards []string
	for _, card := range cards {
		if strings.Contains(strings.ToLower(card), PIVCardTypeYubiKey) {
			yubiKeyCards = append(yubiKeyCards, card)
		}
	}

	return yubiKeyCards, nil
}

func GetDefaultKeySlot(policy PrivateKeyPolicy) (piv.Slot, error) {
	switch policy {
	case PrivateKeyPolicyHardwareKey:
		// private_key_policy: hardware_key -> 9a
		return piv.SlotAuthentication, nil
	case PrivateKeyPolicyHardwareKeyTouch:
		// private_key_policy: hardware_key_touch -> 9c
		return piv.SlotSignature, nil
	case PrivateKeyPolicyHardwareKeyTouchAndPIN:
		// private_key_policy: hardware_key_touch_and_pin -> 9d
		return piv.SlotKeyManagement, nil
	case PrivateKeyPolicyHardwareKeyPIN:
		// private_key_policy: hardware_key_pin -> 9e
		return piv.SlotCardAuthentication, nil
	default:
		return piv.Slot{}, trace.BadParameter("unexpected private key policy %v", policy)
	}
}

// GetPrivateKeyPolicyFromAttestation returns the PrivateKeyPolicy satisfied by the given hardware key attestation.
func GetPrivateKeyPolicyFromAttestation(att *piv.Attestation) PrivateKeyPolicy {
	if att == nil {
		return PrivateKeyPolicyNone
	}

	isTouchPolicy := att.TouchPolicy == piv.TouchPolicyCached ||
		att.TouchPolicy == piv.TouchPolicyAlways

	isPINPolicy := att.PINPolicy == piv.PINPolicyOnce ||
		att.PINPolicy == piv.PINPolicyAlways

	switch {
	case isPINPolicy && isTouchPolicy:
		return PrivateKeyPolicyHardwareKeyTouchAndPIN
	case isPINPolicy:
		return PrivateKeyPolicyHardwareKeyPIN
	case isTouchPolicy:
		return PrivateKeyPolicyHardwareKeyTouch
	default:
		return PrivateKeyPolicyHardwareKey
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
