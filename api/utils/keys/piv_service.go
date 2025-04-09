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

package keys

import (
	"context"
	"crypto"
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
	var requiredKeyPolicy PrivateKeyPolicy
	switch config.Policy {
	case hardwarekey.PromptPolicyNone:
		requiredKeyPolicy = PrivateKeyPolicyHardwareKey
	case hardwarekey.PromptPolicyTouch:
		requiredKeyPolicy = PrivateKeyPolicyHardwareKeyTouch
	case hardwarekey.PromptPolicyPIN:
		requiredKeyPolicy = PrivateKeyPolicyHardwareKeyPIN
	case hardwarekey.PromptPolicyTouchAndPIN:
		requiredKeyPolicy = PrivateKeyPolicyHardwareKeyTouchAndPIN
	}

	ykPriv, err := getOrGenerateYubiKeyPrivateKey(ctx, requiredKeyPolicy, config.CustomSlot, s.prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref := &hardwarekey.PrivateKeyRef{
		SerialNumber: ykPriv.serialNumber,
		SlotKey:      hardwarekey.PIVSlotKey(ykPriv.pivSlot.Key),
		PublicKey:    ykPriv.Public(),
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: ykPriv.attestation.TouchPolicy != piv.TouchPolicyNever,
			PINRequired:   ykPriv.attestation.PINPolicy != piv.PINPolicyNever,
		},
		AttestationStatement: &hardwarekey.AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        ykPriv.slotCert.Raw,
					AttestationCert: ykPriv.attestationCert.Raw,
				},
			},
		},
	}

	keyRefsMux.Lock()
	defer keyRefsMux.Unlock()
	keyRefs[baseKeyRef{
		serialNumber: ref.SerialNumber,
		slotKey:      ref.SlotKey,
	}] = ref

	return hardwarekey.NewSigner(s, ref), nil
}

// Sign performs a cryptographic signature using the specified hardware
// private key and provided signature parameters.
func (s *YubiKeyService) Sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	y, err := getYubiKey(ref.SerialNumber, s.prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pivSlot, err := parsePIVSlot(ref.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, err := y.getPrivateKey(pivSlot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return priv.Sign(rand, digest, opts)
}

// SetPrompt sets the hardware key prompt used by the hardware key service, if applicable.
// This is used by Teleport Connect which sets the prompt later than the hardware key service,
// due to process initialization constraints.
func (s *YubiKeyService) SetPrompt(prompt hardwarekey.Prompt) {
	s.prompt = prompt
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

	ykPriv, err := y.getPrivateKey(pivSlot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref := &hardwarekey.PrivateKeyRef{
		SerialNumber: serialNumber,
		SlotKey:      slotKey,
		PublicKey:    ykPriv.Public(),
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: ykPriv.attestation.TouchPolicy != piv.TouchPolicyNever,
			PINRequired:   ykPriv.attestation.PINPolicy != piv.PINPolicyNever,
		},
		AttestationStatement: &hardwarekey.AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        ykPriv.slotCert.Raw,
					AttestationCert: ykPriv.attestationCert.Raw,
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
