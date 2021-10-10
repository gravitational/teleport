// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webauthn_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

type attestationTest struct {
	name    string
	obj     protocol.AttestationObject
	wantErr bool
}

func TestVerifyAttestation(t *testing.T) {
	var sig = []byte{1, 2, 3} // fake signature

	// secureKeyCA stands for a security key manufacturer CA.
	// In practice, attestation certs are likely to derive directly from this one,
	// but for testing we include a couple intermediates as well (called
	// "series 1" and "series 2").
	secureKeyCACert, secureKeyCAKey, err := makeSelfSigned(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Secure Key Root CA",
		},
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:     true,
	})
	require.NoError(t, err)
	series1CACert, series1CAKey, err := makeCertificate(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Secure Key Series 1 CA",
		},
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:     true,
	}, secureKeyCACert, secureKeyCAKey)
	require.NoError(t, err)
	series2CACert, series2CAKey, err := makeCertificate(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Secure Key Series 2 CA",
		},
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:     true,
	}, secureKeyCACert, secureKeyCAKey)
	require.NoError(t, err)

	// secureKeyDevCert is a typical secure key attestation cert.
	secureKeyDevCert, _, err := makeCertificate(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Secure Key Device #123",
		},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
	}, secureKeyCACert, secureKeyCAKey)
	require.NoError(t, err)
	// series1 and series2DevCert are secure key attestation certs for specific
	// device batches/series.
	series1DevCert, _, err := makeCertificate(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Secure Key Series 1 Device #124",
		},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
	}, series1CACert, series1CAKey)
	require.NoError(t, err)
	series2DevCert, _, err := makeCertificate(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Secure Key Series 2 Device #125",
		},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
	}, series2CACert, series2CAKey)
	require.NoError(t, err)

	// platformRootCA stands for a platform device attestation CA.
	// It simulates a chain where the device cert always comes from an "unknown"
	// intermediate (like Touch ID).
	platformRootCACert, platformRootCAKey, err := makeSelfSigned(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Platform WebAuthn Root CA",
		},
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:     true,
	})
	require.NoError(t, err)
	platformIntermediateCACert, platformIntermediateCAKey, err := makeCertificate(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Platform WebAuthn CA 1",
		},
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:     true,
	}, platformRootCACert, platformRootCAKey)
	require.NoError(t, err)

	// platformDevCert is a typical platform device attestation certificate.
	platformDevCert, _, err := makeCertificate(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Llama Platform Device SN=1231231231",
		},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
	}, platformIntermediateCACert, platformIntermediateCAKey)
	require.NoError(t, err)

	// unknownDevCert stands for a self-signed/unknown attestation certificate.
	unknownDevCert, _, err := makeSelfSigned(&x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Totally Secure Device",
		},
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:     true,
	})
	require.NoError(t, err)

	cfg := &types.Webauthn{
		RPID: "localhost",
		AttestationAllowedCAs: derToPEMs([][]byte{
			secureKeyCACert.Raw,    // trust secure keys
			platformRootCACert.Raw, // trust platform authenticators
		}),
		AttestationDeniedCAs: derToPEMs([][]byte{
			series1CACert.Raw, // exclude "bad" secure key series 1
		}),
	}

	// Do a simple check of supported formats before we move on to other
	// scenarios.
	// Scenarios below are intended as a format-parsing-check, thus may be a bit
	// unrealistic (ie, "apple" format with a Yubikey-looking cert).
	var tests []attestationTest
	for _, format := range []string{
		"packed",
		"tpm",
		"android-key",
		"fido-u2f",
		"apple",
	} {
		tests = append(tests, attestationTest{
			name: fmt.Sprintf("OK format=%v simple check", format),
			obj: protocol.AttestationObject{
				Format: format,
				AttStatement: map[string]interface{}{
					"alg": webauthncose.AlgES256,
					"sig": sig,
					"x5c": []interface{}{
						secureKeyDevCert.Raw,
					},
				},
			},
		})
	}

	// All scenarios are based on responses where "direct" attestation was
	// requested.
	// YMMV if using other conveyance preferences.
	// https://www.w3.org/TR/webauthn/#enum-attestation-convey.
	tests = append(tests, []attestationTest{
		{
			// eg: Brave, Chrome and Safari using Yubikey.
			name: "OK format=packed root-signed secure key",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					"alg": webauthncose.AlgES256,
					"sig": sig,
					"x5c": []interface{}{secureKeyDevCert.Raw},
				},
			},
		},
		{
			// eg: Firefox using Yubikey, tsh.
			name: "OK format=fido-u2f root-signed secure key",
			obj: protocol.AttestationObject{
				Format: "fido-u2f",
				AttStatement: map[string]interface{}{
					"sig": sig,
					"x5c": []interface{}{secureKeyDevCert.Raw},
				},
			},
		},
		{
			// eg: Touch ID on Safari.
			name: "OK format=apple Touch ID attestation",
			obj: protocol.AttestationObject{
				Format: "apple",
				AttStatement: map[string]interface{}{
					"x5c": []interface{}{
						platformDevCert.Raw,
						platformIntermediateCACert.Raw,
					},
				},
			},
		},
		{
			// eg: Brave or Chrome using Touch ID.
			name: "NOK format=packed self-attestation",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					"sig": sig,
					// "x5c" not present.
				},
			},
			wantErr: true,
		},
		{
			// eg: Firefox using anonymized attestation.
			// They aren't joking about the anonymized part.
			name: "NOK format=none",
			obj: protocol.AttestationObject{
				Format:       "none",
				AttStatement: map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "OK allowed device series",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					"alg": webauthncose.AlgES256,
					"sig": sig,
					"x5c": []interface{}{
						series2DevCert.Raw,
						series2CACert.Raw,
					},
				},
			},
		},
		{
			name: "NOK denied device series",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					"alg": webauthncose.AlgES256,
					"sig": sig,
					"x5c": []interface{}{
						series1DevCert.Raw,
						series1CACert.Raw, // series1CA prohibited by config
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NOK unknown device",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					"alg": webauthncose.AlgES256,
					"sig": sig,
					"x5c": []interface{}{
						unknownDevCert.Raw,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NOK format not supported",
			obj: protocol.AttestationObject{
				Format:       "notsupported",
				AttStatement: map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "NOK x5c empty",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					"x5c": []interface{}{},
				},
			},
			wantErr: true,
		},
		{
			name: "NOK x5c with wrong type",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					// want []interface{} instead of [][]byte.
					"x5c": [][]byte{secureKeyDevCert.Raw},
				},
			},
			wantErr: true,
		},
		{
			name: "NOK x5c cert with wrong type",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					// want []byte instead of string.
					"x5c": []interface{}{string(secureKeyDevCert.Raw)},
				},
			},
			wantErr: true,
		},
		{
			name: "NOK x5c invalid cert",
			obj: protocol.AttestationObject{
				Format: "packed",
				AttStatement: map[string]interface{}{
					"x5c": []interface{}{[]byte("not a certificate")},
				},
			},
			wantErr: true,
		},
	}...)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := wanlib.VerifyAttestation(cfg, test.obj)
			if gotErr := err != nil; gotErr != test.wantErr {
				t.Errorf("VerifyAttestation returned err = %v, wantErr = %v", err, test.wantErr)
			}
		})
	}
}

func makeSelfSigned(template *x509.Certificate) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return makeCertificate(template, template, privKey)
}

func makeCertificate(template, parent *x509.Certificate, signingKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	var certKey *ecdsa.PrivateKey
	if template == parent {
		certKey = signingKey // aka self-signed
	} else {
		var err error
		if certKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	sn, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	template.SerialNumber = sn
	template.NotBefore = time.Now().Add(-1 * time.Minute)
	template.NotAfter = time.Now().Add(60 * time.Minute)
	template.BasicConstraintsValid = true

	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &certKey.PublicKey, signingKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	cert, err := x509.ParseCertificate(certBytes)
	return cert, certKey, trace.Wrap(err)
}
