/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package webauthn_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

type attestationTest struct {
	name    string
	obj     protocol.AttestationObject
	wantErr bool
}

func TestVerifyAttestation(t *testing.T) {
	sig := []byte{1, 2, 3} // fake signature

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

func TestVerifyAttestation_windowsHello(t *testing.T) {
	// Attestation object captured from a Windows Hello registration.
	// Holds 2 certificates in its x5c chain, the second of which chains to
	// microsoftTPMRootCA2014
	// - x5c[0]: subject=""
	//   issuer="EUS-NTC-KEYID-667D154665CAC01F70CB40D8DB33594C90B4D911"
	// - x5c[1]: subject="EUS-NTC-KEYID-667D154665CAC01F70CB40D8DB33594C90B4D911"
	//   issuer="Microsoft TPM Root Certificate Authority 2014"
	const rawAttObjB64 = `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`

	// Decode and unmarshal attestation object.
	rawAttObj, err := base64.StdEncoding.DecodeString(rawAttObjB64)
	require.NoError(t, err, "Decode B64 attestation object")
	obj := &protocol.AttestationObject{}
	require.NoError(t,
		webauthncbor.Unmarshal(rawAttObj, obj),
		"Unmarshal CBOR attestation object",
	)

	webConfig := &types.Webauthn{
		RPID: "localhost", // unimportant for the test
		AttestationAllowedCAs: []string{
			microsoftTPMRootCA2014,
		},
	}
	assert.NoError(t,
		wanlib.VerifyAttestation(webConfig, *obj),
		"VerifyAttestation failed unexpectedly",
	)
}

// http://www.microsoft.com/pkiops/certs/Microsoft%20TPM%20Root%20Certificate%20Authority%202014.crt
const microsoftTPMRootCA2014 = `-----BEGIN CERTIFICATE-----
MIIF9TCCA92gAwIBAgIQXbYwTgy/J79JuMhpUB5dyzANBgkqhkiG9w0BAQsFADCB
jDELMAkGA1UEBhMCVVMxEzARBgNVBAgTCldhc2hpbmd0b24xEDAOBgNVBAcTB1Jl
ZG1vbmQxHjAcBgNVBAoTFU1pY3Jvc29mdCBDb3Jwb3JhdGlvbjE2MDQGA1UEAxMt
TWljcm9zb2Z0IFRQTSBSb290IENlcnRpZmljYXRlIEF1dGhvcml0eSAyMDE0MB4X
DTE0MTIxMDIxMzExOVoXDTM5MTIxMDIxMzkyOFowgYwxCzAJBgNVBAYTAlVTMRMw
EQYDVQQIEwpXYXNoaW5ndG9uMRAwDgYDVQQHEwdSZWRtb25kMR4wHAYDVQQKExVN
aWNyb3NvZnQgQ29ycG9yYXRpb24xNjA0BgNVBAMTLU1pY3Jvc29mdCBUUE0gUm9v
dCBDZXJ0aWZpY2F0ZSBBdXRob3JpdHkgMjAxNDCCAiIwDQYJKoZIhvcNAQEBBQAD
ggIPADCCAgoCggIBAJ+n+bnKt/JHIRC/oI/xgkgsYdPzP0gpvduDA2GbRtth+L4W
UyoZKGBw7uz5bjjP8Aql4YExyjR3EZQ4LqnZChMpoCofbeDR4MjCE1TGwWghGpS0
mM3GtWD9XiME4rE2K0VW3pdN0CLzkYbvZbs2wQTFfE62yNQiDjyHFWAZ4BQH4eWa
8wrDMUxIAneUCpU6zCwM+l6Qh4ohX063BHzXlTSTc1fDsiPaKuMMjWjK9vp5UHFP
a+dMAWr6OljQZPFIg3aZ4cUfzS9y+n77Hs1NXPBn6E4Db679z4DThIXyoKeZTv1a
aWOWl/exsDLGt2mTMTyykVV8uD1eRjYriFpmoRDwJKAEMOfaURarzp7hka9TOElG
yD2gOV4Fscr2MxAYCywLmOLzA4VDSYLuKAhPSp7yawET30AvY1HRfMwBxetSqWP2
+yZRNYJlHpor5QTuRDgzR+Zej+aWx6rWNYx43kLthozeVJ3QCsD5iEI/OZlmWn5W
Yf7O8LB/1A7scrYv44FD8ck3Z+hxXpkklAsjJMsHZa9mBqh+VR1AicX4uZG8m16x
65ZU2uUpBa3rn8CTNmw17ZHOiuSWJtS9+PrZVA8ljgf4QgA1g6NPOEiLG2fn8Gm+
r5Ak+9tqv72KDd2FPBJ7Xx4stYj/WjNPtEUhW4rcLK3ktLfcy6ea7Rocw5y5AgMB
AAGjUTBPMAsGA1UdDwQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBR6
jArOL0hiF+KU0a5VwVLscXSkVjAQBgkrBgEEAYI3FQEEAwIBADANBgkqhkiG9w0B
AQsFAAOCAgEAW4ioo1+J9VWC0UntSBXcXRm1ePTVamtsxVy/GpP4EmJd3Ub53JzN
BfYdgfUL51CppS3ZY6BoagB+DqoA2GbSL+7sFGHBl5ka6FNelrwsH6VVw4xV/8kl
IjmqOyfatPYsz0sUdZev+reeiGpKVoXrK6BDnUU27/mgPtem5YKWvHB/soofUrLK
zZV3WfGdx9zBr8V0xW6vO3CKaqkqU9y6EsQw34n7eJCbEVVQ8VdFd9iV1pmXwaBA
fBwkviPTKEP9Cm+zbFIOLr3V3CL9hJj+gkTUuXWlJJ6wVXEG5i4rIbLAV59UrW4L
onP+seqvWMJYUFxu/niF0R3fSGM+NU11DtBVkhRZt1u0kFhZqjDz1dWyfT/N7Hke
3WsDqUFsBi+8SEw90rWx2aUkLvKo83oU4Mx4na+2I3l9F2a2VNGk4K7l3a00g51m
iPiq0Da0jqw30PaLluTMTGY5+RnZVh50JD6nk+Ea3wRkU8aiYFnpIxfKBZ72whmY
Ya/egj9IKeqpR0vuLebbU0fJBf880K1jWD3Z5SFyJXo057Mv0OPw5mttytE585ZI
y5JsaRXlsOoWGRXE3kUT/MKR1UoAgR54c8Bsh+9Dq2wqIK9mRn15zvBDeyHG6+cz
urLopziOUeWokxZN1syrEdKlhFoPYavm6t+PzIcpdxZwHA+V3jLJPfI=
-----END CERTIFICATE-----`
