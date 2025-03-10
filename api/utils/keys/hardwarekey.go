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
	"crypto/x509"
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
)

// TODO: replace with PIV implementation
func NewHardwareKeyService(prompt HardwareKeyPrompt) HardwareKeyService {
	return nil
}

// HardwareKeyService is an interface for interacting with hardware private keys.
type HardwareKeyService interface {
	// NewPrivateKey creates or retrieves a hardware private key from the given PIV slot matching
	// the given private key policy and returns the details required to perform signatures with
	// that key.
	NewPrivateKey(ctx context.Context, customSlot PIVSlot, requiredPolicy PrivateKeyPolicy) (*HardwarePrivateKeyRef, error)
	// Sign performs a cryptographic signature using the specified hardware
	// private key and provided signature parameters.
	Sign(ref *HardwarePrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
	// SetPrompt sets the hardware key prompt used by the hardware key service, if applicable.
	// This is used by Teleport Connect which sets the prompt later than the hardware key service,
	// due to process initialization constraints.
	SetPrompt(prompt HardwareKeyPrompt)
}

// HardwarePrivateKey is a hardware private key.
type HardwarePrivateKey struct {
	service HardwareKeyService
	ref     *HardwarePrivateKeyRef
}

// Public implements [crypto.Signer].
func (h *HardwarePrivateKey) Public() crypto.PublicKey {
	return h.ref.PublicKey
}

// Sign implements [crypto.Signer].
func (h *HardwarePrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return h.service.Sign(h.ref, rand, digest, opts)
}

// GetAttestation returns the hardware private key attestation details.
func (h *HardwarePrivateKey) GetAttestationStatement() *AttestationStatement {
	return h.ref.AttestationStatement
}

// GetPrivateKeyPolicy returns the PrivateKeyPolicy satisfied by this key.
func (h *HardwarePrivateKey) GetPrivateKeyPolicy() PrivateKeyPolicy {
	return h.ref.PrivateKeyPolicy
}

// HardwarePrivateKeyRef references a specific hardware private key.
type HardwarePrivateKeyRef struct {
	// SerialNumber is the hardware key's serial number.
	SerialNumber uint32 `json:"serial_number"`
	// SlotKey is the key name for the hardware key PIV slot, e.g. "9a".
	SlotKey uint32 `json:"slot_key"`
	// PublicKey is the public key paired with the hardware private key.
	PublicKey crypto.PublicKey `json:"-"` // uses custom JSON marshaling in PKIX, ASN.1 DER form
	// PrivateKeyPolicy is the private key policy satisfied by the hardware private key.
	PrivateKeyPolicy PrivateKeyPolicy `json:"private_key_policy"`
	// AttestationStatement contains the hardware private key's attestation statement, which is
	// to attest the touch and pin requirements for this hardware private key during login.
	AttestationStatement *AttestationStatement `json:"attestation_statement"`
}

// encodeHardwarePrivateKeyRef encodes a [HardwarePrivateKeyRef] to JSON.
func encodeHardwarePrivateKeyRef(ref *HardwarePrivateKeyRef) ([]byte, error) {
	keyRefBytes, err := json.Marshal(ref)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRefBytes, nil
}

// decodeHardwarePrivateKeyRef decodes a [HardwarePrivateKeyRef] from JSON.
func decodeHardwarePrivateKeyRef(encodedKeyRef []byte) (*HardwarePrivateKeyRef, error) {
	// TODO: old clients would only have SerialNumber and SlotKey, gather missing information directly for backwards compatibility.
	keyRef := &HardwarePrivateKeyRef{}
	if err := json.Unmarshal(encodedKeyRef, keyRef); err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRef, nil
}

// These types are used for custom marshaling of the crypto.PublicKey field in [HardwarePrivateKeyRef].
type refAlias HardwarePrivateKeyRef
type hardwarePrivateKeyRefJSON struct {
	// embedding an alias type instead of [HardwarePrivateKeyRef] prevents the custom marshaling
	// from recursively applying, which would result in a stack overflow.
	refAlias
	PublicKeyDER []byte `json:"public_key"`
}

// UnmarshalJSON marshals [HardwarePrivateKeyRef] with custom logic for the public key.
func (r HardwarePrivateKeyRef) MarshalJSON() ([]byte, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(r.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return json.Marshal(&hardwarePrivateKeyRefJSON{
		refAlias:     refAlias(r),
		PublicKeyDER: pubDER,
	})
}

// UnmarshalJSON unmarshals [HardwarePrivateKeyRef] with custom logic for the public key.
func (r *HardwarePrivateKeyRef) UnmarshalJSON(b []byte) error {
	ref := hardwarePrivateKeyRefJSON{}
	err := json.Unmarshal(b, &ref)
	if err != nil {
		return trace.Wrap(err)
	}

	ref.refAlias.PublicKey, err = x509.ParsePKIXPublicKey(ref.PublicKeyDER)
	if err != nil {
		return trace.Wrap(err)
	}

	*r = HardwarePrivateKeyRef(ref.refAlias)
	return nil
}
