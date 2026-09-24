// Copyright 2026 Gravitational, Inc.
// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This source file was initially vendored from github.com/go-piv/piv-go

package yubiattest

import (
	"crypto/x509"
	_ "embed"
	"encoding/asn1"
	"fmt"
	"sync"
)

var (
	yubicoCAsOnce sync.Once
	yubicoCAsErr  error

	yubicoCAPool            *x509.CertPool
	yubicoIntermediatesPool *x509.CertPool
)

// Name: Yubico Attestation Root 1
// Issued: 2024-12-01
// Address:
// - https://developers.yubico.com/PKI/yubico-ca-1.pem
// - https://developers.yubico.com/PKI/yubico-ca-1.pem.sig
//
//go:embed yubico-ca-1.der
var yubico_ca_1_der string

// Name: Yubico U2F Root CA Serial 457200631
// Issued: 2014-08-01
// Address:
// - https://developers.yubico.com/PKI/yubico-fido-ca-1.pem
// - https://developers.yubico.com/PKI/yubico-fido-ca-1.pem.sig
//
//go:embed yubico-fido-ca-1.der
var yubico_fido_ca_1_der string

// Name: Yubico PIV Root CA Serial 263751
// Issued: 2016-03-14
// Address:
// - https://developers.yubico.com/PKI/yubico-piv-ca-1.pem
// - https://developers.yubico.com/PKI/yubico-piv-ca-1.pem.sig
//
//go:embed yubico-piv-ca-1.der
var yubico_piv_ca_1_der string

// Last Update: 2025-02-03
// Address:
// - https://developers.yubico.com/PKI/yubico-intermediate.pem
//
//go:embed yubico-intermediate.der
var yubico_intermediate_der string

func parseYubicoCAs() (roots []*x509.Certificate, intermediates []*x509.Certificate, _ error) {
	certU2F, err := x509.ParseCertificate([]byte(yubico_fido_ca_1_der))
	if err != nil {
		yubicoCAsErr = err
		return
	}

	// The U2F root cert has pathlen x509 basic constraint set to 0.
	// As per RFC 5280 this means that no intermediate cert is allowed
	// in the validation path. This isn't really helpful since we do
	// want to use the device attestation cert as intermediate cert in
	// the chain. To make this work, set pathlen of the U2F root to 1.
	//
	// See https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.9
	certU2F.MaxPathLen = 1

	certPIV, err := x509.ParseCertificate([]byte(yubico_piv_ca_1_der))
	if err != nil {
		return nil, nil, err
	}

	certAtt, err := x509.ParseCertificate([]byte(yubico_ca_1_der))
	if err != nil {
		return nil, nil, err
	}

	i, err := x509.ParseCertificates([]byte(yubico_intermediate_der))
	if err != nil {
		return nil, nil, err
	}

	return []*x509.Certificate{certU2F, certPIV, certAtt}, i, nil
}

func ensureYubicoCAs() {
	yubicoCAsOnce.Do(func() {
		roots, intermediates, err := parseYubicoCAs()
		if err != nil {
			yubicoCAsErr = err
			return
		}
		yubicoCAPool = x509.NewCertPool()
		for _, c := range roots {
			yubicoCAPool.AddCert(c)
		}
		yubicoIntermediatesPool = x509.NewCertPool()
		for _, c := range intermediates {
			yubicoIntermediatesPool.AddCert(c)
		}
	})
}

type Attestation struct {
	SerialNumber uint32

	TouchRequired bool
	PINRequired   bool
}

// Verify proves that a key was generated on a YubiKey. It ensures the slot and
// YubiKey certificate chains up to the Yubico CA, parsing additional
// information out of the slot certificate, such as the touch and PIN policies
// of a key. The certificates passed to this function might be modified and thus
// should not be reused.
func Verify(attestationCert, slotCert *x509.Certificate) (Attestation, error) {
	ensureYubicoCAs()
	if yubicoCAsErr != nil {
		return Attestation{}, fmt.Errorf("failed to load Yubico CAs (this is a bug): %w", yubicoCAsErr)
	}

	intermediates := yubicoIntermediatesPool.Clone()

	// The attestation cert in some yubikey 4 does not encode X509v3 Basic Constraints.
	// This isn't valid as per https://datatracker.ietf.org/doc/html/rfc5280#section-4.2.1.9
	// (fourth paragraph) and thus makes x509.go validation fail.
	// Work around this by setting this constraint here.
	if !attestationCert.BasicConstraintsValid {
		attestationCert.BasicConstraintsValid = true
		attestationCert.IsCA = true
	}
	intermediates.AddCert(attestationCert)

	o := x509.VerifyOptions{
		KeyUsages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
		},
		Roots:         yubicoCAPool,
		Intermediates: intermediates,
	}

	if _, err := slotCert.Verify(o); err != nil {
		return Attestation{}, fmt.Errorf("verifying attestation certificate: %w", err)
	}

	a, err := parseAttestation(slotCert)
	if err != nil {
		return Attestation{}, fmt.Errorf("parsing attestation certificate: %w", err)
	}

	return a, nil
}

func parseAttestation(slotCert *x509.Certificate) (Attestation, error) {
	var a Attestation
	for _, ext := range slotCert.Extensions {
		// see https://docs.yubico.com/hardware/oid/webdocs.pdf
		switch {
		case asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 41482, 3, 7}.Equal(ext.Id):
			var serial int64
			if _, err := asn1.Unmarshal(ext.Value, &serial); err != nil {
				return Attestation{}, fmt.Errorf("parsing serial number: %w", err)
			}
			if serial < 0 {
				return Attestation{}, fmt.Errorf("serial number was negative: %d", serial)
			}
			a.SerialNumber = uint32(serial)

		case asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 41482, 3, 8}.Equal(ext.Id):
			if len(ext.Value) != 2 {
				return Attestation{}, fmt.Errorf("expected 2 bytes from key policy, got: %d", len(ext.Value))
			}

			const (
				pinPolicyNever       = 1
				pinPolicyOnce        = 2
				pinPolicyAlways      = 3
				pinPolicyMatchOnce   = 4
				pinPolicyMatchAlways = 5
			)
			switch ext.Value[0] {
			case pinPolicyNever:
				a.PINRequired = false
			case pinPolicyOnce:
				a.PINRequired = true
			case pinPolicyAlways:
				a.PINRequired = true
			case pinPolicyMatchOnce:
				a.PINRequired = false
			case pinPolicyMatchAlways:
				a.PINRequired = false
			default:
				return Attestation{}, fmt.Errorf("unrecognized pin policy: %#x", ext.Value[0])
			}

			const (
				touchPolicyNever  = 1
				touchPolicyAlways = 2
				touchPolicyCached = 3
			)
			switch ext.Value[1] {
			case touchPolicyNever:
				a.TouchRequired = false
			case touchPolicyAlways:
				a.TouchRequired = true
			case touchPolicyCached:
				a.TouchRequired = true
			default:
				return Attestation{}, fmt.Errorf("unrecognized touch policy: %#x", ext.Value[1])
			}
		}
	}

	return a, nil
}
