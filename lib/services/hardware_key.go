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
	"crypto/x509"
	"encoding/json"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

type HardwareKeyType string

const (
	HardwareKeyTypeYubikeyPIV HardwareKeyType = "yubikey_piv"
)

type HardwareKeyAttestation struct {
	// PublicKeyDER is the public key of the hardware private key attested in PKIX, ASN.1 DER form.
	PublicKeyDER []byte `json:"public_key"`

	// Type is the type of hardware key this attestation is for.
	Type HardwareKeyType `json:"type"`

	// PrivateKeyPolicy specifies the private key policy supported by the attested hardware key.
	PrivateKeyPolicy constants.PrivateKeyPolicy `json:"private_key_policy"`

	// AttestationObject is a json encoded attestation data object. This object's
	// format is dependent on its hardware key type.
	AttestationObject []byte `json:"attestation_object"`
}

// AttestYubikey verifies that the given slot certificate chains to the attestation certificate,
// which chains to a Yubico CA.
func AttestYubikey(slotCertDER, attestationCertDER []byte) (*HardwareKeyAttestation, error) {
	slotCert, err := x509.ParseCertificate(slotCertDER)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestationCert, err := x509.ParseCertificate(attestationCertDER)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestation, err := piv.Verify(attestationCert, slotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(slotCert.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	policy := constants.PrivateKeyPolicyHardwareKey
	if attestation.TouchPolicy == piv.TouchPolicyAlways || attestation.TouchPolicy == piv.TouchPolicyCached {
		policy = constants.PrivateKeyPolicyHardwareKeyTouch
	}
	attestationObject, err := json.Marshal(attestation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &HardwareKeyAttestation{
		PublicKeyDER:      pubDER,
		Type:              HardwareKeyTypeYubikeyPIV,
		PrivateKeyPolicy:  policy,
		AttestationObject: attestationObject,
	}, nil
}

// GetPrivateKeyPolicy gets a user's private key policy by checking the auth preference and role option policy.
func GetPrivateKeyPolicy(authPref types.AuthPreference, accessChecker AccessChecker) constants.PrivateKeyPolicy {
	// Default to the auth preference's private key policy
	authPolicy := authPref.GetPrivateKeyPolicy()
	rolePolicy := accessChecker.PrivateKeyPolicy()
	if authPolicy == constants.PrivateKeyPolicyHardwareKeyTouch || rolePolicy == constants.PrivateKeyPolicyHardwareKeyTouch {
		return constants.PrivateKeyPolicyHardwareKeyTouch
	} else if authPolicy == constants.PrivateKeyPolicyHardwareKey || rolePolicy == constants.PrivateKeyPolicyHardwareKey {
		return constants.PrivateKeyPolicyHardwareKey
	}
	return constants.PrivateKeyPolicyNone
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
