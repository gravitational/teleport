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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"encoding/pem"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestParsePrivateKey tests that ParsePrivateKey successfully parses private key PEM.
func TestParsePrivateKey(t *testing.T) {
	for _, tt := range []struct {
		desc        string
		keyPEM      []byte
		assertError require.ErrorAssertionFunc
		assertKey   require.ValueAssertionFunc
	}{
		{
			desc:   "invalid PEM",
			keyPEM: []byte(`non-pem data`),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter, got %T", err)
			},
			assertKey: require.Nil,
		},
		{
			desc:   "invalid key",
			keyPEM: invalidKeyPEM,
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter, got %T", err)
			},
			assertKey: require.Nil,
		},
		{
			desc:        "rsa key",
			keyPEM:      rsaKeyPEM,
			assertError: require.NoError,
			assertKey: func(tt require.TestingT, key interface{}, i2 ...interface{}) {
				privateKey, ok := key.(*PrivateKey)
				require.True(t, ok)
				require.IsType(t, &rsa.PrivateKey{}, privateKey.Signer)
			},
		},
		{
			desc:        "ecdsa key",
			keyPEM:      ecdsaKeyPEM,
			assertError: require.NoError,
			assertKey: func(tt require.TestingT, key interface{}, i2 ...interface{}) {
				privateKey, ok := key.(*PrivateKey)
				require.True(t, ok)
				require.IsType(t, &ecdsa.PrivateKey{}, privateKey.Signer)
			},
		},
		{
			desc:        "ed25519 key",
			keyPEM:      ed25519KeyPEM,
			assertError: require.NoError,
			assertKey: func(tt require.TestingT, key interface{}, i2 ...interface{}) {
				privateKey, ok := key.(*PrivateKey)
				require.True(t, ok)
				require.IsType(t, ed25519.PrivateKey{}, privateKey.Signer)
			},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			priv, err := ParsePrivateKey(tt.keyPEM)
			tt.assertError(t, err)
			tt.assertKey(t, priv)
		})
	}
}

// TestX509KeyPair tests that X509KeyPair returns the same value as tls.X509KeyPair.
func TestX509KeyPair(t *testing.T) {
	for _, tc := range []struct {
		desc    string
		keyPEM  []byte
		certPEM []byte
	}{
		{
			desc:    "rsa cert",
			keyPEM:  rsaKeyPEM,
			certPEM: rsaCertPEM,
		}, {
			desc:   "rsa certs",
			keyPEM: rsaKeyPEM,
			certPEM: func() []byte {
				// encode two certs into certPEM.
				rsaCertPEMDuplicated := new(bytes.Buffer)
				der, _ := pem.Decode(rsaCertPEM)
				pem.Encode(rsaCertPEMDuplicated, der)
				pem.Encode(rsaCertPEMDuplicated, der)
				return rsaCertPEMDuplicated.Bytes()
			}(),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			expectCert, err := tls.X509KeyPair(tc.certPEM, tc.keyPEM)
			require.NoError(t, err)

			tlsCert, err := X509KeyPair(tc.certPEM, tc.keyPEM)
			require.NoError(t, err)

			require.Equal(t, expectCert, tlsCert)
		})
	}
}

var (
	invalidKeyPEM = []byte(`-----BEGIN INVALID KEY-----
-----END INVALID KEY-----
`)
	// generated with `openssl req -x509 -out rsa.crt -keyout rsa.key -newkey rsa:2048 -nodes -sha256`
	rsaKeyPEM = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCudYRUc0u2xdQi
wzEckPP9lVnXmC2b8vkstKhwZPwoffUDZ6tbS/IjaMAIPaL5Vh6B1oyN8M0qYtQ8
L6IVTN8f3MnTrqHulGWfx6PnSOjgLQ640Z/SY9KMNnZvs66Ag7ka+2v7BDPYv3Ik
eUyPPQbrxDYs37vfa+iFKU/5CgYKQFbFmeiP6C/jaExSz+up+ImwUyaVJWZjTWlh
9z4dp7Z3C+avY4HzOEu/DlaPDAOSKnlHRRaeX3Fyv41cva0CHxaJsbdeF/UrkTef
ClhOxvl+ZEFGqgbBvU/5nAUKk/1Ai/iPQ7Rfw/lKMc0/aLE3wx4WVxy2cVPlmxmQ
2u3RwwRFAgMBAAECggEAb7XmV2FAkTeZ/+x3DTCwW6d/0PKr+dkavwqrdNTlNlR5
SIXgjuRRl2Ti2iQFsJz5ifBFLjqMVWDVP/jMU9FWaoOpZPfEzw2NCUP/6wCfxbR0
Ydow+bpbvta8/gfTbI1sQR/PY/ur61WjlEFryaitPtj0S8Wz+nuRd3sdr31AotzD
HV/oxjZffZrVkq3gKvu9v9KX96ExXitZQ4zk9bh5As8pwbdOcOni6kFjr3OXZ0nC
agPsLwGvL+t+Nq6md/MwvU8t0GdCoBX4IuS/gC9BAuCE0S1F5nJUZ2W4iqsCUbQA
/BCIkRv30DSgHgLSxKp6KZt+VVgNIlV5URrJ1A+h3QKBgQDbXkMNdSfowI6UusMr
xoG8J1KoHFp2QhT5gyMNK/sNYPHpMvQJQWSaEaqzGaeNAvuzfAoDbEs2S5i0BhU0
UzNpZ/PkREBaBaIk0lNMoiVv7yk6CQIz12CVVgd7iD9xPDX6BiTrtrpNfod1zySF
zzqV0qJ8RD7ipB/n5/1fpuwJDwKBgQDLl3CvCPe+anXMhWFNC3PFy+h9lHA7eo4n
9FAwducgq1IHxy6qspf0Y7nZPv6CY3kQbTRWyaFP4M4HCPJpmYEkZWmxvzcjDI2L
1kTSHkNgr8EXP/w+6tMO0zkU3MtvqhX2CybLuY9u7O2Cnmvze9PAE2fDV0YjLngK
0Lr8N9MVawKBgCPiwrNT5Ah2X5zDBKSHn7eI80OfB8lqvAWpRzWjaTliD5DnjfZp
pSxzEWqlGry9rTFKbFTtBUzHhx6EFDnwFmv63nIMHD7dxw2g/pF9wQQTqrncuWiD
pkAnx6eUvVQn1milUqrgxI9i0IQcM8xT/zB9Oal8fJEU6kdEszVPmDNPAoGBAL4d
kfVxq1+eLJiq6Py4OAk568XxKojwXfVDeOp47kYclYJ75sEx+yIVSkRrReFeoHvN
bnWo3cEozVvWaABify0MopGAXS2WmEs/8I5CAms0VFywvI3IXQTYC9LGiBajPtS+
/yB5DE7qYrR52ZbKSCdyN5A7XFyYFTMMTcAfJTc3AoGADyQ5MTQVcQHKtTULy5/6
RCqu3NBv4fj237N7FPiBJv/aAhz/nNSi98CPUESJ++5KtIrbLmm02Gm2Bi+WGU92
gn3QD885jR7bH2kvUg1NSrjoAYqb3AwnGduILus/MbsoizSIgEJZeTUQFJ/sr5Q1
k4M8rcOBNRgCFpwDm9DC+fI=
-----END PRIVATE KEY-----`)
	rsaCertPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUWKKpMWB8DhGCOtOKV41eBwhLo60wDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMjA4MjIxOTAxMDFaFw0yMjA5
MjExOTAxMDFaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCudYRUc0u2xdQiwzEckPP9lVnXmC2b8vkstKhwZPwo
ffUDZ6tbS/IjaMAIPaL5Vh6B1oyN8M0qYtQ8L6IVTN8f3MnTrqHulGWfx6PnSOjg
LQ640Z/SY9KMNnZvs66Ag7ka+2v7BDPYv3IkeUyPPQbrxDYs37vfa+iFKU/5CgYK
QFbFmeiP6C/jaExSz+up+ImwUyaVJWZjTWlh9z4dp7Z3C+avY4HzOEu/DlaPDAOS
KnlHRRaeX3Fyv41cva0CHxaJsbdeF/UrkTefClhOxvl+ZEFGqgbBvU/5nAUKk/1A
i/iPQ7Rfw/lKMc0/aLE3wx4WVxy2cVPlmxmQ2u3RwwRFAgMBAAGjUzBRMB0GA1Ud
DgQWBBTqyM9oMkpwxREibsYlOhq3gs+3yTAfBgNVHSMEGDAWgBTqyM9oMkpwxREi
bsYlOhq3gs+3yTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQCf
mdUw76V5pyMt+2wIurGDdItl6OZDmNOh7HGR6Nh7Y9pRe1cjzdRweIbH5CA+NLuv
J1rQB1pdt1Jk6fnH2hk8U8rpGFoZgHFHEVaIo5sge4HCL2qlnBPU5skDH7D891HK
qEzAKNJRsJTqzmItzBDQzjZ185BijcM/X3NZjTfiOGJwcMehH/F85syXQLODrXgp
mg0exCUFW40aXpfm0z0dNNwoN+FPSefKMYMQ1LV87I6zGnmVTYH9Nix3REiuliIQ
7XXnJc7A6tsc6yXdVG6IpGnKXuTvl/r4iIbH+JDv3MDSvZSCE5kzAPFjgB3zMAZ8
Z0+424ERgom0Zdy75Y8I
-----END CERTIFICATE-----`)
	// generated with `openssl ecparam -genkey -name prime256v1 -noout -out ecdsa.key`
	ecdsaKeyPEM = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIMDaz87Hngva0Wm+QkhCJ0Nz5o958+dsyH0DzsCe6Fl6oAoGCCqGSM49
AwEHoUQDQgAEI06FHb4RKoYKcj+51w6WcN7kNI9OVSTp6H8BlljYYs2zxuIh6LQ3
hXIC6UT+IOGQBnvq86SAbnPEWMLowtQc/Q==
-----END EC PRIVATE KEY-----
`)
	// generated with `openssl genpkey -algorithm ed25519 -out ed25519.key`
	ed25519KeyPEM = []byte(`-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIGf81V4UAiKXFehNALvwlSlB8ZYb/RbRUMSdTG3mSZLN
-----END PRIVATE KEY-----
`)
)
