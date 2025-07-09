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

package main

import (
	"crypto/x509/pkix"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetKubeCredentialData(t *testing.T) {
	// Generate a dummy cert.
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
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

	privateKeyBytes, err := keys.MarshalPrivateKey(privateKey)
	require.NoError(t, err)
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
