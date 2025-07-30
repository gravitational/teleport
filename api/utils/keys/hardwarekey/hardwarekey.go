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
	"crypto/x509"
	"encoding/json"
	"io"
	"time"

	"github.com/gravitational/trace"
)

// Service for interfacing with hardware private keys.
type Service interface {
	// NewPrivateKey creates a hardware private key that satisfies the provided [config],
	// if one does not already exist, and returns a corresponding [hardwarekey.Signer].
	NewPrivateKey(ctx context.Context, config PrivateKeyConfig) (*Signer, error)
	// Sign performs a cryptographic signature using the specified hardware
	// private key and provided signature parameters.
	Sign(ctx context.Context, ref *PrivateKeyRef, keyInfo ContextualKeyInfo, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
	// TODO(Joerger): DELETE IN v19.0.0
	// GetFullKeyRef gets the full [PrivateKeyRef] for an existing hardware private
	// key in the given slot of the hardware key with the given serial number.
	GetFullKeyRef(serialNumber uint32, slotKey PIVSlotKey) (*PrivateKeyRef, error)
}

// Signer is a hardware key implementation of [crypto.Signer].
type Signer struct {
	service Service
	Ref     *PrivateKeyRef
	KeyInfo ContextualKeyInfo
}

// NewSigner returns a [Signer] for the given service and ref.
// keyInfo is an optional argument to supply additional contextual info
// used to add additional context to prompts, e.g. ProxyHost.
func NewSigner(s Service, ref *PrivateKeyRef, keyInfo ContextualKeyInfo) *Signer {
	return &Signer{
		service: s,
		Ref:     ref,
		KeyInfo: keyInfo,
	}
}

// Public implements [crypto.Signer].
func (h *Signer) Public() crypto.PublicKey {
	return h.Ref.PublicKey
}

// Sign implements [crypto.Signer].
func (h *Signer) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return h.service.Sign(context.TODO(), h.Ref, h.KeyInfo, rand, digest, opts)
}

// GetAttestation returns the hardware private key attestation details.
func (h *Signer) GetAttestationStatement() *AttestationStatement {
	return h.Ref.AttestationStatement
}

// GetPrivateKeyPolicy returns the PrivateKeyPolicy satisfied by this key.
func (h *Signer) GetPromptPolicy() PromptPolicy {
	return h.Ref.Policy
}

// WarmupHardwareKey performs a bogus sign() call to prompt the user for PIN/touch (if needed).
func (h *Signer) WarmupHardwareKey(ctx context.Context) error {
	if !h.Ref.Policy.PINRequired && !h.Ref.Policy.TouchRequired {
		return nil
	}

	// ed25519 keys only support sha512 hashing, or no hashing. Currently we don't support
	// ed25519 hardware keys outside of the mocked PIV service, but we may extend support in
	// the future as newer keys are being made with ed25519 support (YubiKey 5.7.x, SoloKey).
	hash := crypto.SHA512

	// We don't actually need to hash the digest, just make it match the hash size.
	digest := make([]byte, hash.Size())

	_, err := h.service.Sign(ctx, h.Ref, h.KeyInfo, rand.Reader, digest, hash)
	return trace.Wrap(err, "failed to perform warmup signature with hardware private key")
}

// EncodeSigner encodes the hardware key signer a format understood by other Teleport clients.
func EncodeSigner(p *Signer) ([]byte, error) {
	return p.Ref.encode()
}

// DecodeSigner decodes an encoded hardware key signer for the given service.
func DecodeSigner(encodedKey []byte, s Service, keyInfo ContextualKeyInfo) (*Signer, error) {
	ref, err := decodeKeyRef(encodedKey, s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewSigner(s, ref, keyInfo), nil
}

// PrivateKeyRef references a specific hardware private key.
type PrivateKeyRef struct {
	// SerialNumber is the hardware key's serial number.
	SerialNumber uint32 `json:"serial_number"`
	// SlotKey is the key name for the hardware key PIV slot, e.g. "9a".
	SlotKey PIVSlotKey `json:"slot_key"`
	// PublicKey is the public key paired with the hardware private key.
	PublicKey crypto.PublicKey `json:"-"` // uses custom JSON marshaling in PKIX, ASN.1 DER form
	// Policy specifies the hardware private key's PIN/touch prompt policies.
	Policy PromptPolicy `json:"policy"`
	// AttestationStatement contains the hardware private key's attestation statement, which is
	// to attest the touch and pin requirements for this hardware private key during login.
	AttestationStatement *AttestationStatement `json:"attestation_statement"`
	// PINCacheTTL is how long hardware key prompts should cache the PIN for this key, if at all.
	PINCacheTTL time.Duration `json:"pin_cache_ttl"`
}

// encode encodes a [PrivateKeyRef] to JSON.
func (r *PrivateKeyRef) encode() ([]byte, error) {
	// Ensure that all required fields are provided to encode.
	if err := r.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	keyRefBytes, err := json.Marshal(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyRefBytes, nil
}

// decodeKeyRef decodes a [PrivateKeyRef] from JSON.
func decodeKeyRef(encodedKeyRef []byte, s Service) (*PrivateKeyRef, error) {
	ref := &PrivateKeyRef{}
	if err := json.Unmarshal(encodedKeyRef, ref); err != nil {
		return nil, trace.Wrap(err)
	}

	// Ensure that all required fields are decoded.
	if err := ref.Validate(); err != nil {
		// If some fields are missing, this is likely an old login key with only
		// the serial number and slot. Fetch missing data from the hardware key.
		// This data will be saved to the login key on next login
		// TODO(Joerger): DELETE IN v19.0.0
		if ref.SerialNumber != 0 && ref.SlotKey != 0 {
			return s.GetFullKeyRef(ref.SerialNumber, ref.SlotKey)
		}

		return nil, trace.Wrap(err)
	}

	return ref, nil
}

func (r *PrivateKeyRef) Validate() error {
	if r.SerialNumber == 0 {
		return trace.BadParameter("private key ref missing SerialNumber")
	}
	if r.SlotKey == 0 {
		return trace.BadParameter("private key ref missing SlotKey")
	}
	if r.PublicKey == nil {
		return trace.BadParameter("private key ref missing PublicKey")
	}
	if r.AttestationStatement == nil {
		return trace.BadParameter("private key ref missing AttestationStatement")
	}
	return nil
}

// These types are used for custom marshaling of the crypto.PublicKey field in [PrivateKeyRef].
type rawPrivateKeyRef PrivateKeyRef
type hardwarePrivateKeyRefJSON struct {
	// embedding an alias type instead of [HardwarePrivateKeyRef] prevents the custom marshaling
	// from recursively applying, which would result in a stack overflow.
	rawPrivateKeyRef
	PublicKeyDER []byte `json:"public_key,omitempty"`
}

// MarshalJSON marshals [PrivateKeyRef] with custom logic for the public key.
func (r PrivateKeyRef) MarshalJSON() ([]byte, error) {
	var pubDER []byte
	if r.PublicKey != nil {
		var err error
		if pubDER, err = x509.MarshalPKIXPublicKey(r.PublicKey); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return json.Marshal(&hardwarePrivateKeyRefJSON{
		rawPrivateKeyRef: rawPrivateKeyRef(r),
		PublicKeyDER:     pubDER,
	})
}

// UnmarshalJSON unmarshals [PrivateKeyRef] with custom logic for the public key.
func (r *PrivateKeyRef) UnmarshalJSON(b []byte) error {
	var ref hardwarePrivateKeyRefJSON
	err := json.Unmarshal(b, &ref)
	if err != nil {
		return trace.Wrap(err)
	}

	if ref.PublicKeyDER != nil {
		ref.rawPrivateKeyRef.PublicKey, err = x509.ParsePKIXPublicKey(ref.PublicKeyDER)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	*r = PrivateKeyRef(ref.rawPrivateKeyRef)
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
	CustomSlot PIVSlotKeyString
	// Algorithm is the key algorithm to use. Defaults to [AlgorithmEC256].
	// [AlgorithmEd25519] is not supported by all hardware keys.
	Algorithm SignatureAlgorithm
	// ContextualKeyInfo contains additional info to associate with the key.
	ContextualKeyInfo ContextualKeyInfo
	// PINCacheTTL is an option to enable PIN caching for this key with the specified TTL.
	PINCacheTTL time.Duration
}

// ContextualKeyInfo contains contextual information associated with a hardware [PrivateKey].
type ContextualKeyInfo struct {
	// ProxyHost is the root proxy hostname that the key is associated with.
	ProxyHost string
	// Username is a Teleport username that the key is associated with.
	Username string
	// ClusterName is a Teleport cluster name that the key is associated with.
	ClusterName string
	// AgentKeyInfo contains info associated with an hardware key agent signature request.
	AgentKeyInfo AgentKeyInfo
}

// AgentKeyInfo contains info associated with an hardware key agent signature request.
type AgentKeyInfo struct {
	// UnknownAgentKey indicates whether this hardware private key is known to the hardware key agent
	// process, usually based on whether a matching key is found in the process's client key store.
	//
	// For unknown agent keys, the hardware key service will check that the certificate in the same
	// slot as the key matches a Teleport client metadata certificate in order to ensure the agent
	// doesn't provide access to non teleport client PIV keys.
	UnknownAgentKey bool
	// Command is the command reported by the agent client which this agent key is being utilized to
	// complete, e.g. `tsh ssh server01`.
	Command string
}

// SignatureAlgorithm is a signature key algorithm option.
type SignatureAlgorithm int

const (
	SignatureAlgorithmEC256 SignatureAlgorithm = iota + 1
	SignatureAlgorithmEd25519
	SignatureAlgorithmRSA2048
)
