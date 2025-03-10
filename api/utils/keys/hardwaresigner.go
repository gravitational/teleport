/*
Copyright 2022 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keys

import (
	"bytes"
	"crypto"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
)

// HardwareSigner is a crypto.Signer which can be attested as being backed by a hardware key.
// This enables the ability to enforce hardware key private key policies.
//
// TODO: Delete in favor of HardwarePrivateKey
type HardwareSigner interface {
	crypto.Signer

	// GetAttestationStatement returns an AttestationStatement for this private key.
	GetAttestationStatement() *AttestationStatement

	// GetPrivateKeyPolicy returns the PrivateKeyPolicy supported by this private key.
	GetPrivateKeyPolicy() PrivateKeyPolicy
}

// GetAttestationStatement returns this key's AttestationStatement. If the key is
// not a hardware-backed key, this method returns nil.
func (k *PrivateKey) GetAttestationStatement() *AttestationStatement {
	if attestedPriv, ok := k.Signer.(HardwareSigner); ok {
		return attestedPriv.GetAttestationStatement()
	}
	// Just return a nil attestation statement and let this key fail any attestation checks.
	return nil
}

// GetPrivateKeyPolicy returns this key's PrivateKeyPolicy.
func (k *PrivateKey) GetPrivateKeyPolicy() PrivateKeyPolicy {
	if attestedPriv, ok := k.Signer.(HardwareSigner); ok {
		return attestedPriv.GetPrivateKeyPolicy()
	}
	return PrivateKeyPolicyNone
}

// AttestationStatement is an attestation statement for a hardware private key
// that supports json marshaling through the standard json/encoding package.
type AttestationStatement attestation.AttestationStatement

// ToProto converts this AttestationStatement to its protobuf form.
func (ar *AttestationStatement) ToProto() *attestation.AttestationStatement {
	return (*attestation.AttestationStatement)(ar)
}

// AttestationStatementFromProto converts an AttestationStatement from its protobuf form.
func AttestationStatementFromProto(att *attestation.AttestationStatement) *AttestationStatement {
	return (*AttestationStatement)(att)
}

// MarshalJSON implements custom protobuf json marshaling.
func (ar *AttestationStatement) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := (&jsonpb.Marshaler{}).Marshal(buf, ar.ToProto())
	return buf.Bytes(), trace.Wrap(err)
}

// UnmarshalJSON implements custom protobuf json unmarshaling.
func (ar *AttestationStatement) UnmarshalJSON(buf []byte) error {
	return jsonpb.Unmarshal(bytes.NewReader(buf), ar.ToProto())
}

// AttestationData is verified attestation data for a public key.
type AttestationData struct {
	// PublicKeyDER is the public key in PKIX, ASN.1 DER form.
	PublicKeyDER []byte `json:"public_key"`
	// PrivateKeyPolicy specifies the private key policy supported by the associated private key.
	PrivateKeyPolicy PrivateKeyPolicy `json:"private_key_policy"`
	// SerialNumber is the serial number of the Attested hardware key.
	SerialNumber uint32 `json:"serial_number"`
}
