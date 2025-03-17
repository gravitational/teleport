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
	// NewPrivateKey creates or retrieves a hardware private key for the given config.
	NewPrivateKey(ctx context.Context, config PrivateKeyConfig) (*PrivateKey, error)
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
	// ContextualKeyInfo contains contextual key info which may be used to add context to prompts,
	// such as the name of the Teleport user using the key. This information is not saved encoded
	// in JSON as this info may depend on the context in which the client is using the key.
	ContextualKeyInfo ContextualKeyInfo
}

// ContextualKeyInfo contains contextual information associated with a hardware [PrivateKey].
// TODO(Joerger): This is not hardware key specific, so it may be better placed in a more general package
// if it is used more broadly, though moving this to the keys package would cause an import cycle.
type ContextualKeyInfo struct {
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
// keyInfo is an optional argument to supply additional contextual info.
func NewPrivateKey(s Service, ref *PrivateKeyRef) *PrivateKey {
	return &PrivateKey{
		service: s,
		ref:     ref,
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

// Encode encodes [h]'s [PrivateKeyRef] to JSON.
func (h *PrivateKey) EncodeKeyRef() ([]byte, error) {
	keyRefBytes, err := json.Marshal(h.ref)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRefBytes, nil
}

// DecodeKeyRef decodes a [PrivateKeyRef] from JSON.
func DecodeKeyRef(encodedKeyRef []byte) (*PrivateKeyRef, error) {
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
	CustomSlot PIVSlot
	// ContextualKeyInfo contains additional info to associate with the key.
	ContextualKeyInfo ContextualKeyInfo
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
