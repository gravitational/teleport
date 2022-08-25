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
	"crypto/x509"
	"strings"

	"github.com/go-piv/piv-go/piv"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"
)

// HardwarePrivateKey is a private key which can be attested to support enforced private key policies.
type HardwarePrivateKey interface {
	// GetAttestationRequest returns an AttestationRequest for this private key.
	GetAttestationRequest() (*AttestationRequest, error)

	// GetPrivateKeyPolicy returns the PrivateKeyPolicy supported by this private key.
	GetPrivateKeyPolicy() PrivateKeyPolicy
}

// GetAttestationRequest returns an AttestationRequest for the given private key.
// If the given private key is not a YubiKeyPrivateKey, then a nil request will be returned.
func GetAttestationRequest(priv *PrivateKey) (*AttestationRequest, error) {
	if attestedPriv, ok := priv.Signer.(HardwarePrivateKey); ok {
		return attestedPriv.GetAttestationRequest()
	}
	// Just return a nil attestation request and let this key fail any attestation checks.
	return nil, nil
}

func GetPrivateKeyPolicy(priv *PrivateKey) PrivateKeyPolicy {
	if attestedPriv, ok := priv.Signer.(HardwarePrivateKey); ok {
		return attestedPriv.GetPrivateKeyPolicy()
	}
	return PrivateKeyPolicyNone
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
	switch req.GetAttestationRequest().(type) {
	case *AttestationRequest_YubikeyAttestationRequest:
		return AttestYubikey(req.GetYubikeyAttestationRequest())
	default:
		return nil, trace.BadParameter("attestation request type %T", req.GetAttestationRequest())
	}
}

// AttestYubikey verifies that the given slot certificate chains to the attestation certificate,
// which chains to a Yubico CA.
func AttestYubikey(req *YubiKeyAttestationRequest) (*AttestationResponse, error) {
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

// PrivateKeyPolicy is the mode required for client private key storage.
type PrivateKeyPolicy string

const (
	// PrivateKeyPolicyNone means that the client can store their private keys
	// anywhere (usually on disk).
	PrivateKeyPolicyNone PrivateKeyPolicy = "none"
	// PrivateKeyPolicyHardwareKey means that the client must use a valid
	// hardware key to generate and store their private keys securely.
	PrivateKeyPolicyHardwareKey PrivateKeyPolicy = "hardware_key"
	// PrivateKeyPolicyHardwareKeyTouch means that the client must use a valid
	// hardware key to generate and store their private keys securely, and
	// this key must require touch to be accessed and used.
	PrivateKeyPolicyHardwareKeyTouch PrivateKeyPolicy = "hardware_key_touch"
)

var privateKeyPolicyErrMsg = "private key policy not met: "

// VerifyPolicy verifies that the given policy meets the requirements of this policy.
func (p PrivateKeyPolicy) VerifyPolicy(policy PrivateKeyPolicy) error {
	switch p {
	case PrivateKeyPolicyNone:
		return nil
	case PrivateKeyPolicyHardwareKey:
		if policy == PrivateKeyPolicyHardwareKey || policy == PrivateKeyPolicyHardwareKeyTouch {
			return nil
		}
	case PrivateKeyPolicyHardwareKeyTouch:
		if policy == PrivateKeyPolicyHardwareKeyTouch {
			return nil
		}
	}
	return trace.BadParameter(privateKeyPolicyErrMsg + string(p))
}

func IsPrivateKeyPolicyError(err error) bool {
	if trace.IsBadParameter(err) {
		return strings.Contains(err.Error(), privateKeyPolicyErrMsg)
	}
	return false
}

// ParsePrivateKeyPolicyError checks if the given error matches one from VerifyPolicy,
// and returns the contained PrivateKeyPolicy.
func ParsePrivateKeyPolicyError(err error) (PrivateKeyPolicy, error) {
	if trace.IsBadParameter(err) {
		policyStr := strings.ReplaceAll(err.Error(), privateKeyPolicyErrMsg, "")
		policy := PrivateKeyPolicy(policyStr)
		if err := policy.validate(); err != nil {
			return "", trace.Wrap(err)
		}
		return policy, nil
	}
	return "", trace.BadParameter("provided error is not a key policy error")
}

func (p PrivateKeyPolicy) validate() error {
	switch p {
	case PrivateKeyPolicyNone, PrivateKeyPolicyHardwareKey, PrivateKeyPolicyHardwareKeyTouch:
		return nil
	}
	return trace.BadParameter("%q is not a valid key policy", p)
}

func (ar *AttestationRequest) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := (&jsonpb.Marshaler{}).Marshal(buf, ar)
	return buf.Bytes(), trace.Wrap(err)
}

func (ar *AttestationRequest) UnmarshalJSON(buf []byte) error {
	return jsonpb.Unmarshal(bytes.NewReader(buf), ar)
}
