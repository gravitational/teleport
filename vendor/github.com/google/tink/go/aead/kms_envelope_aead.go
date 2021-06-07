// Copyright 2019 Google LLC
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
//
////////////////////////////////////////////////////////////////////////////////

package aead

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/google/tink/go/core/registry"
	"github.com/google/tink/go/tink"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

const (
	lenDEK = 4
)

// KMSEnvelopeAEAD represents an instance of Envelope AEAD.
type KMSEnvelopeAEAD struct {
	dekTemplate *tinkpb.KeyTemplate
	remote      tink.AEAD
}

// NewKMSEnvelopeAEAD creates an new instance of KMSEnvelopeAEAD.
// Deprecated: use NewKMSEnvelopeAEAD2 which takes a pointer to a KeyTemplate proto rather than a value.
func NewKMSEnvelopeAEAD(kt tinkpb.KeyTemplate, remote tink.AEAD) *KMSEnvelopeAEAD {
	return &KMSEnvelopeAEAD{
		remote:      remote,
		dekTemplate: &kt,
	}
}

// NewKMSEnvelopeAEAD2 creates an new instance of KMSEnvelopeAEAD.
func NewKMSEnvelopeAEAD2(kt *tinkpb.KeyTemplate, remote tink.AEAD) *KMSEnvelopeAEAD {
	return &KMSEnvelopeAEAD{
		remote:      remote,
		dekTemplate: kt,
	}
}

// Encrypt implements the tink.AEAD interface for encryption.
func (a *KMSEnvelopeAEAD) Encrypt(pt, aad []byte) ([]byte, error) {
	dekM, err := registry.NewKey(a.dekTemplate)
	if err != nil {
		return nil, err
	}
	dek, err := proto.Marshal(dekM)
	if err != nil {
		return nil, err
	}
	encryptedDEK, err := a.remote.Encrypt(dek, []byte{})
	if err != nil {
		return nil, err
	}
	p, err := registry.Primitive(a.dekTemplate.TypeUrl, dek)
	if err != nil {
		return nil, err
	}
	primitive, ok := p.(tink.AEAD)
	if !ok {
		return nil, errors.New("kms_envelope_aead: failed to convert AEAD primitive")
	}

	payload, err := primitive.Encrypt(pt, aad)
	if err != nil {
		return nil, err
	}
	return buildCipherText(encryptedDEK, payload)

}

// Decrypt implements the tink.AEAD interface for decryption.
func (a *KMSEnvelopeAEAD) Decrypt(ct, aad []byte) ([]byte, error) {
	// Verify we have enough bytes for the length of the encrypted DEK.
	if len(ct) <= lenDEK {
		return nil, errors.New("kms_envelope_aead: invalid ciphertext")
	}

	// Extract length of encrypted DEK and advance past that length.
	ed := int(binary.BigEndian.Uint32(ct[:lenDEK]))
	ct = ct[lenDEK:]

	// Verify we have enough bytes for the encrypted DEK.
	if ed <= 0 || len(ct) < ed {
		return nil, errors.New("kms_envelope_aead: invalid ciphertext")
	}

	// Extract the encrypted DEK and the payload.
	encryptedDEK := ct[:ed]
	payload := ct[ed:]
	ct = nil

	// Decrypt the DEK.
	dek, err := a.remote.Decrypt(encryptedDEK, []byte{})
	if err != nil {
		return nil, err
	}

	// Get an AEAD primitive corresponding to the DEK.
	p, err := registry.Primitive(a.dekTemplate.TypeUrl, dek)
	if err != nil {
		return nil, fmt.Errorf("kms_envelope_aead: %s", err)
	}
	primitive, ok := p.(tink.AEAD)
	if !ok {
		return nil, errors.New("kms_envelope_aead: failed to convert AEAD primitive")
	}

	// Decrypt the payload.
	return primitive.Decrypt(payload, aad)
}

// buildCipherText builds the cipher text by appending the length DEK, encrypted DEK
// and the encrypted payload.
func buildCipherText(encryptedDEK, payload []byte) ([]byte, error) {
	var b bytes.Buffer

	// Write the length of the encrypted DEK.
	lenDEKbuf := make([]byte, lenDEK)
	binary.BigEndian.PutUint32(lenDEKbuf, uint32(len(encryptedDEK)))
	_, err := b.Write(lenDEKbuf)
	if err != nil {
		return nil, err
	}

	_, err = b.Write(encryptedDEK)
	if err != nil {
		return nil, err
	}

	_, err = b.Write(payload)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
