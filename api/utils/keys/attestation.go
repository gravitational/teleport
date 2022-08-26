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
	"crypto"
	"crypto/x509"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"
)

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

// HardwareKeyAttestation holds data necessary to attest a hardware key of the given type.
type HardwareKeyAttestationResponse struct {
	// PublicKeyDER is the public key of the hardware private key attested in PKIX, ASN.1 DER form.
	PublicKey crypto.PublicKey
	// PrivateKeyPolicy specifies the private key policy supported by the attested hardware key.
	PrivateKeyPolicy PrivateKeyPolicy
}

// AttestHardwareKey performs attestation using the given attestation object, and returns
// a response containing the public key attested and any hardware key policies that it meets.
func AttestHardwareKey(req *AttestationRequest) (*HardwareKeyAttestationResponse, error) {
	switch req.GetAttestationRequest().(type) {
	case *AttestationRequest_YubikeyAttestationRequest:
		return AttestYubikey(req.GetYubikeyAttestationRequest())
	default:
		return nil, trace.BadParameter("attestation request type %T", req.GetAttestationRequest())
	}
}

// AttestYubikey verifies that the given slot certificate chains to the attestation certificate,
// which chains to a Yubico CA.
func AttestYubikey(req *YubiKeyAttestationRequest) (*HardwareKeyAttestationResponse, error) {
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

	return &HardwareKeyAttestationResponse{
		PublicKey:        slotCert.PublicKey,
		PrivateKeyPolicy: policy,
	}, nil
}

// VerifyPrivateKeyPolicy verfies that the provided private key policy passes
// the required private key policy.
func VerifyPrivateKeyPolicy(providedPolicy, requiredPolicy PrivateKeyPolicy) bool {
	switch requiredPolicy {
	case PrivateKeyPolicyNone:
		// No policy enforced, so any policy is valid
		return true
	case PrivateKeyPolicyHardwareKey:
		return providedPolicy == PrivateKeyPolicyHardwareKey || providedPolicy == PrivateKeyPolicyHardwareKeyTouch
	case PrivateKeyPolicyHardwareKeyTouch:
		return providedPolicy == PrivateKeyPolicyHardwareKeyTouch
	default:
		return false
	}
}
