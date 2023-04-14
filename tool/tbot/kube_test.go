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

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetKubeCredentialData(t *testing.T) {
	// Generate a dummy cert.
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()
	notAfter := clock.Now().Add(time.Hour)
	certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   pkix.Name{CommonName: "test"},
		NotAfter:  notAfter,
	})
	require.NoError(t, err)

	privateKeyBytes := tlsca.MarshalPrivateKeyPEM(privateKey)
	idFile := &identityfile.IdentityFile{
		PrivateKey: privateKeyBytes,
		Certs: identityfile.Certs{
			SSH: []byte(ssh.CertAlgoRSAv01), // dummy value
			TLS: certBytes,
		},
		CACerts: identityfile.CACerts{
			SSH: [][]byte{[]byte(fixtures.SSHCAPublicKey)},
			TLS: [][]byte{[]byte(fixtures.TLSCACertPEM)},
		},
	}

	data, err := getCredentialData(idFile, clock.Now())
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &parsed))
	status := parsed["status"].(map[string]interface{})
	require.NotNil(t, status)

	require.Equal(t, string(certBytes), status["clientCertificateData"])
	require.Equal(t, string(privateKeyBytes), status["clientKeyData"])

	// Note: We usually subtract a minute from the expiration time in
	// getCredentialData to avoid the cert expiring mid-request.
	ts, err := time.Parse(time.RFC3339, status["expirationTimestamp"].(string))
	require.NoError(t, err)
	require.WithinDuration(t, notAfter.Add(-1*time.Minute), ts, time.Second)
}
