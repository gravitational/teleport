// Copyright 2026 Gravitational, Inc.
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

// Package identityapi defines types used by the tbot identity-api signer flow.
package identityapi

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
)

const PrivateKeyPEMType = "TBOT IDENTITY API PRIVATE KEY"

// Service performs cryptographic signatures using a referenced private key.
type Service interface {
	Sign(ctx context.Context, ref *PrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error)
}

// Signer implements [crypto.Signer] using an [identityapi.Service].
type Signer struct {
	service Service
	Ref     *PrivateKeyRef
}

// NewSigner returns a signer for the given service and private key reference.
func NewSigner(service Service, ref *PrivateKeyRef) *Signer {
	return &Signer{
		service: service,
		Ref:     ref,
	}
}

// Public implements [crypto.Signer].
func (s *Signer) Public() crypto.PublicKey {
	return s.Ref.PublicKey
}

// Sign implements [crypto.Signer].
func (s *Signer) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return s.service.Sign(context.TODO(), s.Ref, rand, digest, opts)
}

// EncodeSigner encodes the signer reference into a portable wire format.
func EncodeSigner(s *Signer) ([]byte, error) {
	return s.Ref.encode()
}

// EncodeRef encodes a private key reference into a portable wire format.
func EncodeRef(ref *PrivateKeyRef) ([]byte, error) {
	return ref.encode()
}

// DecodeSigner decodes the signer reference using the provided service.
func DecodeSigner(encoded []byte, service Service) (*Signer, error) {
	ref, err := decodeKeyRef(encoded)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewSigner(service, ref), nil
}

// PrivateKeyRef identifies the public half of an identity-api backed key.
type PrivateKeyRef struct {
	// PublicKey is the public key paired with the signer-held private key.
	PublicKey crypto.PublicKey `json:"-"`
}

func (r *PrivateKeyRef) encode() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := json.Marshal(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func decodeKeyRef(encoded []byte) (*PrivateKeyRef, error) {
	ref := &PrivateKeyRef{}
	if err := json.Unmarshal(encoded, ref); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ref.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	return ref, nil
}

// Validate returns an error if the key reference is incomplete.
func (r *PrivateKeyRef) Validate() error {
	if r.PublicKey == nil {
		return trace.BadParameter("private key ref missing PublicKey")
	}
	return nil
}

type rawPrivateKeyRef PrivateKeyRef

type privateKeyRefJSON struct {
	rawPrivateKeyRef
	PublicKeyDER []byte `json:"public_key,omitempty"`
}

// MarshalJSON marshals [PrivateKeyRef] with custom logic for the public key.
func (r PrivateKeyRef) MarshalJSON() ([]byte, error) {
	var publicKeyDER []byte
	if r.PublicKey != nil {
		var err error
		publicKeyDER, err = x509.MarshalPKIXPublicKey(r.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return json.Marshal(&privateKeyRefJSON{
		rawPrivateKeyRef: rawPrivateKeyRef(r),
		PublicKeyDER:     publicKeyDER,
	})
}

// UnmarshalJSON unmarshals [PrivateKeyRef] with custom logic for the public key.
func (r *PrivateKeyRef) UnmarshalJSON(data []byte) error {
	var ref privateKeyRefJSON
	if err := json.Unmarshal(data, &ref); err != nil {
		return trace.Wrap(err)
	}

	if len(ref.PublicKeyDER) != 0 {
		pub, err := x509.ParsePKIXPublicKey(ref.PublicKeyDER)
		if err != nil {
			return trace.Wrap(err)
		}
		ref.rawPrivateKeyRef.PublicKey = pub
	}

	*r = PrivateKeyRef(ref.rawPrivateKeyRef)
	return nil
}
