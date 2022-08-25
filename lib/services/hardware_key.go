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

package services

import (
	"crypto"
	"crypto/x509"
	"encoding/json"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// HardwareKeyType is a specific type of hardware key.
type HardwareKeyType string

const (
	HardwareKeyTypeYubikeyPIV HardwareKeyType = "yubikey_piv"
)

// HardwareKeyAttestation holds data necessary to attest a hardware key of the given type.
type HardwareKeyAttestationRequest struct {
	// Type is the type of hardware key this attestation is for.
	Type HardwareKeyType `json:"type"`

	// AttestationObject is a json encoded attestation data object. This object's
	// format is dependent on its hardware key type.
	AttestationObject []byte `json:"attestation_object"`
}

// HardwareKeyAttestation holds data necessary to attest a hardware key of the given type.
type HardwareKeyAttestationResponse struct {
	// PublicKeyDER is the public key of the hardware private key attested in PKIX, ASN.1 DER form.
	PublicKey crypto.PublicKey

	// PrivateKeyPolicy specifies the private key policy supported by the attested hardware key.
	PrivateKeyPolicy constants.PrivateKeyPolicy
}

// AttestHardwareKey performs attestation using the given attestation object, and returns
// a response containing the public key attested and any hardware key policies that it meets.
func AttestHardwareKey(req *HardwareKeyAttestationRequest) (*HardwareKeyAttestationResponse, error) {
	switch req.Type {
	case HardwareKeyTypeYubikeyPIV:
		var yubikeyReq YubikeyAttestationRequest
		if err := json.Unmarshal(req.AttestationObject, &yubikeyReq); err != nil {
			return nil, trace.Wrap(err)
		}
		return AttestYubikey(&yubikeyReq)
	default:
		return nil, trace.BadParameter("unsupported hardware key type %T", req.Type)
	}
}

// YubikeyAttestationRequest is a request object used to attest a yubikey PIV slot.
type YubikeyAttestationRequest struct {
	// SlotCertificate is an attestation certificate generated from a yubikey PIV
	// slot's public key and signed by the yubikey's attestation certificate.
	SlotCertificate []byte `json:"slot_certificate"`

	// AttesationCertificate is a yubikey's attestation certificate which only signs
	// public keys generated in the same yubikey and chains to a trusted Yubico CA.
	AttestationCertificate []byte `json:"attestation_certificate"`
}

// AttestYubikey verifies that the given slot certificate chains to the attestation certificate,
// which chains to a Yubico CA.
func AttestYubikey(req *YubikeyAttestationRequest) (*HardwareKeyAttestationResponse, error) {
	slotCert, err := x509.ParseCertificate(req.SlotCertificate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestationCert, err := x509.ParseCertificate(req.AttestationCertificate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestation, err := piv.Verify(attestationCert, slotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	policy := constants.PrivateKeyPolicyHardwareKey
	if attestation.TouchPolicy == piv.TouchPolicyAlways || attestation.TouchPolicy == piv.TouchPolicyCached {
		policy = constants.PrivateKeyPolicyHardwareKeyTouch
	}

	return &HardwareKeyAttestationResponse{
		PublicKey:        slotCert.PublicKey,
		PrivateKeyPolicy: policy,
	}, nil
}

// VerifyPrivateKeyPolicy verfies that the provided private key policy passes
// the required private key policy.
func VerifyPrivateKeyPolicy(providedPolicy, requiredPolicy constants.PrivateKeyPolicy) bool {
	switch requiredPolicy {
	case constants.PrivateKeyPolicyNone:
		// No policy enforced, so any policy is valid
		return true
	case constants.PrivateKeyPolicyHardwareKey:
		return providedPolicy == constants.PrivateKeyPolicyHardwareKey || providedPolicy == constants.PrivateKeyPolicyHardwareKeyTouch
	case constants.PrivateKeyPolicyHardwareKeyTouch:
		return providedPolicy == constants.PrivateKeyPolicyHardwareKeyTouch
	default:
		return false
	}
}
