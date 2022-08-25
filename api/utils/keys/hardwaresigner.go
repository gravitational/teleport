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
// This enables the ability to enforced hardware key private key policies.
type HardwareSigner interface {
	crypto.Signer

	// GetAttestationRequest returns an AttestationRequest for this private key.
	GetAttestationRequest() (*AttestationRequest, error)

	// GetPrivateKeyPolicy returns the PrivateKeyPolicy supported by this private key.
	GetPrivateKeyPolicy() PrivateKeyPolicy
}

// GetAttestationRequest returns an AttestationRequest for the given private key.
// If the given private key does not have a HardwareSigner, then a nil request
// and error will be returned.
func GetAttestationRequest(priv *PrivateKey) (*AttestationRequest, error) {
	if attestedPriv, ok := priv.Signer.(HardwareSigner); ok {
		return attestedPriv.GetAttestationRequest()
	}
	// Just return a nil attestation request and let this key fail any attestation checks.
	return nil, nil
}

// GetPrivateKeyPolicy returns the PrivateKeyPolicy that applies to the given private key.
func GetPrivateKeyPolicy(priv *PrivateKey) PrivateKeyPolicy {
	if attestedPriv, ok := priv.Signer.(HardwareSigner); ok {
		return attestedPriv.GetPrivateKeyPolicy()
	}
	return PrivateKeyPolicyNone
}

// AttestationRequest is an alias for proto.AttestationRequest that supports
// json marshaling and unmarshaling.
type AttestationRequest attestation.AttestationRequest

// ToProto converts this AttestationRequest to its protobuf form.
func (ar *AttestationRequest) ToProto() *attestation.AttestationRequest {
	return (*attestation.AttestationRequest)(ar)
}

// AttestationRequestFromProto converts an AttestationRequest from its protobuf form.
func AttestationRequestFromProto(req *attestation.AttestationRequest) *AttestationRequest {
	return (*AttestationRequest)(req)
}

// MarshalJSON implements custom protobuf json marshaling.
func (ar *AttestationRequest) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := (&jsonpb.Marshaler{}).Marshal(buf, ar.ToProto())
	return buf.Bytes(), trace.Wrap(err)
}

// UnmarshalJSON implements custom protobuf json unmarshaling.
func (ar *AttestationRequest) UnmarshalJSON(buf []byte) error {
	return jsonpb.Unmarshal(bytes.NewReader(buf), ar.ToProto())
}

// AttestationResponse is verified attestation data for a public key.
type AttestationResponse struct {
	// PublicKeyDER is the public key in PKIX, ASN.1 DER form.
	PublicKeyDER []byte `json:"public_key"`
	// PrivateKeyPolicy specifies the private key policy supported by the associated private key.
	PrivateKeyPolicy PrivateKeyPolicy `json:"private_key_policy"`
}
