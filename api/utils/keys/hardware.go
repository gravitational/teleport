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
	"crypto/x509"

	"github.com/go-piv/piv-go/piv"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"

	proto "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
)

// HardwareSigner is a crypto.Signer which can be attested to support enforced private key policies.
type HardwareSigner interface {
	crypto.Signer

	// GetAttestationRequest returns an AttestationRequest for this private key.
	GetAttestationRequest() (*AttestationRequest, error)

	// GetPrivateKeyPolicy returns the PrivateKeyPolicy supported by this private key.
	GetPrivateKeyPolicy() PrivateKeyPolicy
}

// GetAttestationRequest returns an AttestationRequest for the given private key.
// If the given private key is not a YubiKeyPrivateKey, then a nil request will be returned.
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

// AttestationRequest is an alias for proto.AttestationRequest which supports
// json marshaling and unmarshaling.
type AttestationRequest proto.AttestationRequest

// ToProto converts this AttestationRequest to its protobuf form.
func (ar *AttestationRequest) ToProto() *proto.AttestationRequest {
	return (*proto.AttestationRequest)(ar)
}

// AttestationRequestFromProto converts an AttestationRequest from its protobuf form.
func AttestationRequestFromProto(req *proto.AttestationRequest) *AttestationRequest {
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

// AttestationResponse is veriried attestation data for a public key.
type AttestationResponse struct {
	// PublicKeyDER is the public key in PKIX, ASN.1 DER form.
	PublicKeyDER []byte `json:"public_key"`
	// PrivateKeyPolicy specifies the private key policy supported by the attested hardware key.
	PrivateKeyPolicy PrivateKeyPolicy `json:"private_key_policy"`
}

// AttestHardwareKey performs attestation using the given attestation object, and returns
// a response containing the public key attested and any hardware key policies that it meets.
func AttestHardwareKey(req *AttestationRequest) (*AttestationResponse, error) {
	protoReq := req.ToProto()
	switch protoReq.GetAttestationRequest().(type) {
	case *proto.AttestationRequest_YubikeyAttestationRequest:
		return attestYubikey(protoReq.GetYubikeyAttestationRequest())
	default:
		return nil, trace.BadParameter("attestation request type %T", protoReq.GetAttestationRequest())
	}
}

// attestYubikey verifies that the given slot certificate chains to the attestation certificate,
// which chains to a Yubico CA.
func attestYubikey(req *proto.YubiKeyAttestationRequest) (*AttestationResponse, error) {
	slotCert, err := x509.ParseCertificate(req.SlotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestationCert, err := x509.ParseCertificate(req.AttestationCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestation, err := piv.Verify(attestationCert, slotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	policy := PrivateKeyPolicyHardwareKey
	if attestation.TouchPolicy == piv.TouchPolicyAlways || attestation.TouchPolicy == piv.TouchPolicyCached {
		policy = PrivateKeyPolicyHardwareKeyTouch
	}

	pubDER, err := x509.MarshalPKIXPublicKey(slotCert.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AttestationResponse{
		PublicKeyDER:     pubDER,
		PrivateKeyPolicy: policy,
	}, nil
}
