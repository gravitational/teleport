//go:build piv && !pivtest

package hardwarekey

import (
	"crypto/x509"

	"github.com/go-piv/piv-go/v2/piv"
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

	pubDER, err := x509.MarshalPKIXPublicKey(slotCert.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &keys.AttestationData{
		PublicKeyDER:     pubDER,
		PrivateKeyPolicy: getPrivateKeyPolicyFromAttestation(attestation),
		SerialNumber:     attestation.Serial,
	}, nil
}

// getPrivateKeyPolicyFromAttestation returns the PrivateKeyPolicy satisfied by the given hardware key attestation.
func getPrivateKeyPolicyFromAttestation(att *piv.Attestation) keys.PrivateKeyPolicy {
	if att == nil {
		return keys.PrivateKeyPolicyNone
	}

	isTouchPolicy := att.TouchPolicy == piv.TouchPolicyCached ||
		att.TouchPolicy == piv.TouchPolicyAlways

	isPINPolicy := att.PINPolicy == piv.PINPolicyOnce ||
		att.PINPolicy == piv.PINPolicyAlways

	switch {
	case isPINPolicy && isTouchPolicy:
		return keys.PrivateKeyPolicyHardwareKeyTouchAndPIN
	case isPINPolicy:
		return keys.PrivateKeyPolicyHardwareKeyPIN
	case isTouchPolicy:
		return keys.PrivateKeyPolicyHardwareKeyTouch
	default:
		return keys.PrivateKeyPolicyHardwareKey
	}
}
