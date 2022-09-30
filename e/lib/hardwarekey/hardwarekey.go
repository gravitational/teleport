//go:build !linux || libpcsclite

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

package hardwarekey

import (
	"crypto/x509"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys"
)

// attestYubikey verifies that the given slot certificate chains to the attestation certificate,
// which chains to a Yubico CA.
func attestYubikey(att *attestation.YubiKeyAttestationStatement) (*keys.AttestationData, error) {
	slotCert, err := x509.ParseCertificate(att.SlotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestationCert, err := x509.ParseCertificate(att.AttestationCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attestation, err := piv.Verify(attestationCert, slotCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	policy := keys.PrivateKeyPolicyHardwareKey
	if attestation.TouchPolicy == piv.TouchPolicyAlways || attestation.TouchPolicy == piv.TouchPolicyCached {
		policy = keys.PrivateKeyPolicyHardwareKeyTouch
	}

	pubDER, err := x509.MarshalPKIXPublicKey(slotCert.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &keys.AttestationData{
		PublicKeyDER:     pubDER,
		PrivateKeyPolicy: policy,
	}, nil
}
