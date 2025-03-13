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
	"crypto/x509"
	"encoding/json"
	"io"
	"strconv"

	"github.com/gravitational/trace"
)

// Service for interfacing with hardware private keys.
type Service interface {
	// NewPrivateKey creates or retrieves a hardware private key from the given PIV slot matching
	// the given policy and returns the details required to perform signatures with that key.
	//
	// If a customSlot is not provided, the service uses the default slot for the given policy:
	//   - !touch & !pin -> 9a
	//   - !touch & pin  -> 9c
	//   - touch & pin   -> 9d
	//   - touch & !pin  -> 9e
	NewPrivateKey(ctx context.Context, customSlot PIVSlot, policy PromptPolicy) (*PrivateKeyRef, error)
	// Sign performs a cryptographic signature using the specified hardware
	// private key and provided signature parameters.
	Sign(ctx context.Context, ref *PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
	// SetPrompt sets the hardware key prompt used by the hardware key service, if applicable.
	// This is used by Teleport Connect which sets the prompt later than the hardware key service,
	// due to process initialization constraints.
	SetPrompt(prompt Prompt)
}

// PrivateKey is a hardware private key.
type PrivateKey struct {
	service Service
	ref     *PrivateKeyRef
	// keyInfo contains additional key info which may be used to add context to prompts,
	// such as the name of the Teleport user using the key.
	keyInfo PrivateKeyInfo
}

// PrivateKeyRef references a specific hardware private key.
type PrivateKeyRef struct {
	// SerialNumber is the hardware key's serial number.
	SerialNumber uint32 `json:"serial_number"`
	// SlotKey is the key name for the hardware key PIV slot, e.g. "9a".
	SlotKey uint32 `json:"slot_key"`
	// PublicKey is the public key paired with the hardware private key.
	PublicKey crypto.PublicKey `json:"-"` // uses custom JSON marshaling in PKIX, ASN.1 DER form
	// Policy specifies the hardware private key's PIN/touch prompt policies.
	Policy PromptPolicy `json:"policy"`
	// AttestationStatement contains the hardware private key's attestation statement, which is
	// to attest the touch and pin requirements for this hardware private key during login.
	AttestationStatement *AttestationStatement `json:"attestation_statement"`
}

// PrivateKeyInfo includes info relevant to the key being parsed. Useful for adding context
// to hardware key pin/touch prompts when performing signatures.
type PrivateKeyInfo struct {
	// ProxyHost is the root proxy hostname that a key is associated with.
	ProxyHost string
}

// PromptPolicy specifies a hardware private key's PIN/touch policies.
type PromptPolicy struct {
	// TouchRequired means that touch is required for signatures.
	TouchRequired bool
	// PINRequired means that PIN is required for signatures.
	PINRequired bool
}

// NewPrivateKey returns a [PrivateKey] for the given service and ref.
func NewPrivateKey(s Service, ref *PrivateKeyRef, keyInfo PrivateKeyInfo) *PrivateKey {
	return &PrivateKey{
		service: s,
		ref:     ref,
		keyInfo: keyInfo,
	}
}

// Public implements [crypto.Signer].
func (h *PrivateKey) Public() crypto.PublicKey {
	return h.ref.PublicKey
}

// Sign implements [crypto.Signer].
func (h *PrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	// When context.TODO() is passed, the service should replace this with its own parent context.
	return h.service.Sign(context.TODO(), h.ref, rand, digest, opts)
}

// GetAttestation returns the hardware private key attestation details.
func (h *PrivateKey) GetAttestationStatement() *AttestationStatement {
	return h.ref.AttestationStatement
}

// GetPrivateKeyPolicy returns the PrivateKeyPolicy satisfied by this key.
func (h *PrivateKey) GetPromptPolicy() PromptPolicy {
	return h.ref.Policy
}

// WarmupHardwareKey performs a bogus sign() call to prompt the user for PIN/touch (if needed).
func (h *PrivateKey) WarmupHardwareKey(ctx context.Context) error {
	if !h.ref.Policy.PINRequired && !h.ref.Policy.TouchRequired {
		return nil
	}

	// ed25519 keys only support sha512 hashing, or no hashing. Currently we don't support
	// ed25519 hardware keys outside of the fake "pivtest" service, but we may extend support in
	// the future as newer keys are being made with ed25519 support (YubiKey 5.7.x, SoloKey).
	hash := sha512.Sum512(make([]byte, 512))
	_, err := h.service.Sign(ctx, h.ref, rand.Reader, hash[:], crypto.SHA512)
	return trace.Wrap(err, "failed to perform warmup signature with hardware private key")
}

// encodeHardwarePrivateKeyRef encodes a [PrivateKeyRef] to JSON.
func EncodeHardwarePrivateKeyRef(ref *PrivateKeyRef) ([]byte, error) {
	keyRefBytes, err := json.Marshal(ref)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRefBytes, nil
}

// decodeHardwarePrivateKeyRef decodes a [PrivateKeyRef] from JSON.
func DecodeHardwarePrivateKeyRef(encodedKeyRef []byte) (*PrivateKeyRef, error) {
	// TODO: old clients would only have SerialNumber and SlotKey, gather missing information directly for backwards compatibility.
	keyRef := &PrivateKeyRef{}
	if err := json.Unmarshal(encodedKeyRef, keyRef); err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRef, nil
}

// These types are used for custom marshaling of the crypto.PublicKey field in [PrivateKeyRef].
type refAlias PrivateKeyRef
type hardwarePrivateKeyRefJSON struct {
	// embedding an alias type instead of [HardwarePrivateKeyRef] prevents the custom marshaling
	// from recursively applying, which would result in a stack overflow.
	refAlias
	PublicKeyDER []byte `json:"public_key"`
}

// UnmarshalJSON marshals [PrivateKeyRef] with custom logic for the public key.
func (r PrivateKeyRef) MarshalJSON() ([]byte, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(r.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return json.Marshal(&hardwarePrivateKeyRefJSON{
		refAlias:     refAlias(r),
		PublicKeyDER: pubDER,
	})
}

// UnmarshalJSON unmarshals [PrivateKeyRef] with custom logic for the public key.
func (r *PrivateKeyRef) UnmarshalJSON(b []byte) error {
	ref := hardwarePrivateKeyRefJSON{}
	err := json.Unmarshal(b, &ref)
	if err != nil {
		return trace.Wrap(err)
	}

	ref.refAlias.PublicKey, err = x509.ParsePKIXPublicKey(ref.PublicKeyDER)
	if err != nil {
		return trace.Wrap(err)
	}

	*r = PrivateKeyRef(ref.refAlias)
	return nil
}

// PIVSlot is the string representation of a PIV slot. e.g. "9a".
type PIVSlot string

// Validate that the PIV slot is a valid value.
func (s PIVSlot) Validate() error {
	slotKey, err := strconv.ParseUint(string(s), 16, 32)
	if err != nil {
		return trace.Wrap(err)
	}

	switch slotKey {
	case 0x9a, 0x9c, 0x9d, 0x9e:
		return nil
	default:
		return trace.BadParameter("invalid PIV slot %q", s)
	}
}
