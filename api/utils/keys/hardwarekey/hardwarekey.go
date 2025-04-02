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

// Package hardwarekey defines types and interfaces for hardware private keys.
package hardwarekey

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha512"
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
)

// Service for interfacing with hardware private keys.
type Service interface {
	// NewPrivateKey creates or retrieves a hardware private key for the given config.
	NewPrivateKey(ctx context.Context, config PrivateKeyConfig) (*PrivateKey, error)
	// Sign performs a cryptographic signature using the specified hardware
	// private key and provided signature parameters.
	Sign(ctx context.Context, ref *PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
	// GetMissingKeyRefDetails updates the key ref with missing details from the hardware key.
	GetFullKeyRef(serialNumber uint32, slotKey PIVSlotKey) (*PrivateKeyRef, error)
}

// PrivateKey is a hardware private key implementation of [crypto.Signer].
type PrivateKey struct {
	service Service
	Ref     *PrivateKeyRef
}

// NewPrivateKey returns a [PrivateKey] for the given service and ref.
// keyInfo is an optional argument to supply additional contextual info.
func NewPrivateKey(s Service, ref *PrivateKeyRef) *PrivateKey {
	return &PrivateKey{
		service: s,
		Ref:     ref,
	}
}

// Public implements [crypto.Signer].
func (h *PrivateKey) Public() crypto.PublicKey {
	return h.Ref.PublicKey
}

// Sign implements [crypto.Signer].
func (h *PrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return h.service.Sign(context.TODO(), h.Ref, rand, digest, opts)
}

// GetAttestation returns the hardware private key attestation details.
func (h *PrivateKey) GetAttestationStatement() *AttestationStatement {
	return h.Ref.AttestationStatement
}

// GetPrivateKeyPolicy returns the PrivateKeyPolicy satisfied by this key.
func (h *PrivateKey) GetPromptPolicy() PromptPolicy {
	return h.Ref.Policy
}

// WarmupHardwareKey performs a bogus sign() call to prompt the user for PIN/touch (if needed).
func (h *PrivateKey) WarmupHardwareKey(ctx context.Context) error {
	if !h.Ref.Policy.PINRequired && !h.Ref.Policy.TouchRequired {
		return nil
	}

	// ed25519 keys only support sha512 hashing, or no hashing. Currently we don't support
	// ed25519 hardware keys outside of the mocked PIV service, but we may extend support in
	// the future as newer keys are being made with ed25519 support (YubiKey 5.7.x, SoloKey).
	hash := sha512.Sum512(make([]byte, 512))
	_, err := h.service.Sign(ctx, h.Ref, rand.Reader, hash[:], crypto.SHA512)
	return trace.Wrap(err, "failed to perform warmup signature with hardware private key")
}

// EncodePrivateKey encodes the hardware private key a format understood by other Teleport clients.
func EncodePrivateKey(p *PrivateKey) ([]byte, error) {
	return p.Ref.encode()
}

// DecodePrivateKey decodes an encoded hardware private key for the given service.
func DecodePrivateKey(s Service, encodedKey []byte) (*PrivateKey, error) {
	partialRef, err := decodeKeyRef(encodedKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(Joerger): all fields should be encoded to the key's PEM file
	// rather than being retrieved from the hardware key each time. This
	// will result in a massive performance boost by avoiding re-attesting
	// the key for every client call.
	ref, err := s.GetFullKeyRef(partialRef.SerialNumber, partialRef.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewPrivateKey(s, ref), nil
}

// PrivateKeyRef references a specific hardware private key.
type PrivateKeyRef struct {
	// SerialNumber is the hardware key's serial number.
	SerialNumber uint32 `json:"serial_number"`
	// SlotKey is the key name for the hardware key PIV slot, e.g. "9a".
	SlotKey PIVSlotKey `json:"slot_key"`
	// PublicKey is the public key paired with the hardware private key.
	PublicKey crypto.PublicKey `json:"-"`
	// Policy specifies the hardware private key's PIN/touch prompt policies.
	Policy PromptPolicy `json:"-"`
	// AttestationStatement contains the hardware private key's attestation statement, which is
	// to attest the touch and pin requirements for this hardware private key during login.
	AttestationStatement *AttestationStatement `json:"-"`
}

// encode encodes a [PrivateKeyRef] to JSON.
func (r *PrivateKeyRef) encode() ([]byte, error) {
	keyRefBytes, err := json.Marshal(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRefBytes, nil
}

// decodeKeyRef decodes a [PrivateKeyRef] from JSON.
func decodeKeyRef(encodedKeyRef []byte) (*PrivateKeyRef, error) {
	keyRef := &PrivateKeyRef{}
	if err := json.Unmarshal(encodedKeyRef, keyRef); err != nil {
		return nil, trace.Wrap(err)
	}

	return keyRef, nil
}

// PrivateKeyConfig contains config for creating a new hardware private key.
type PrivateKeyConfig struct {
	// Policy is a prompt policy to require for the hardware private key.
	Policy PromptPolicy
	// CustomSlot is a specific PIV slot to generate the hardware private key in.
	// If unset, the default slot for the given policy will be used.
	//   - !touch & !pin -> 9a
	//   - !touch & pin  -> 9c
	//   - touch & pin   -> 9d
	//   - touch & !pin  -> 9e
	CustomSlot PIVSlotKeyString
}
