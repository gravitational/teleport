// Copyright 2026 Gravitational, Inc.
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

package tlsutils_test

import (
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/tlsutils"
)

func TestParseCertificatePEMStrict(t *testing.T) {
	t.Parallel()

	// Generated with openssl.
	const certPEM = `-----BEGIN CERTIFICATE-----
MIIBfjCCASOgAwIBAgIUcgtowC2aiqtoaaqg8Wz9IQsUV5cwCgYIKoZIzj0EAwIw
FDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTI2MDIyNjE5MzAzOVoXDTI3MDIyNjE5
MzAzOVowFDESMBAGA1UEAwwJbG9jYWxob3N0MFkwEwYHKoZIzj0CAQYIKoZIzj0D
AQcDQgAExP70cLQNy03OwKKr5DadftNYQyLEe6POP0ncvRxOV4PwlTSjPzetJJvV
cvD8osxLRHxoUIO6XHP15NjcMo3gpKNTMFEwHQYDVR0OBBYEFL9zTzq0IkOOQysJ
4oHUm5wv7cSdMB8GA1UdIwQYMBaAFL9zTzq0IkOOQysJ4oHUm5wv7cSdMA8GA1Ud
EwEB/wQFMAMBAf8wCgYIKoZIzj0EAwIDSQAwRgIhAPiuMeGa9LOZzAb1QzRvS3hW
1CnGa5we8zUbh+L7g8/kAiEAt4OjLC0bXoq0pLYoMcPhFP3QOBSA3LPd+vH939ym
uQM=
-----END CERTIFICATE-----`

	// Start with a successful parse.
	cert, err := tlsutils.ParseCertificatePEMStrict([]byte(certPEM))
	require.NoError(t, err)

	// Test a few errors.
	tests := []struct {
		name    string
		certPEM []byte
		wantErr string
	}{
		{
			name:    "empty PEM",
			wantErr: "expected PEM",
		},
		{
			name:    "invalid PEM",
			certPEM: []byte("not a PEM"),
			wantErr: "expected PEM",
		},
		{
			name: "invalid PEM data",
			certPEM: pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: []byte("ceci n'est pas a certificate"),
			}),
			wantErr: "malformed certificate",
		},
		{
			name: "invalid PEM type",
			certPEM: pem.EncodeToMemory(&pem.Block{
				Type:  "NOTCERTIFICATE",
				Bytes: cert.Raw,
			}),
			wantErr: "unexpected block type",
		},
		{
			name: "trailing PEM data",
			certPEM: []byte(
				string(pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: cert.Raw,
				})) + "  "),
			wantErr: "trailing data",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := tlsutils.ParseCertificatePEMStrict(test.certPEM)
			assert.ErrorContains(t, err, test.wantErr, "Error mismatch")
		})
	}
}
