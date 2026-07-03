/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestKubeClientCertFingerprintDerivation proves both sides of the in-band MFA
// exchange derive the same session fingerprint: the local proxy from the leaf of
// the tls.Certificate it presents, and the kube forwarder from the parsed mTLS
// peer certificate.
func TestKubeClientCertFingerprintDerivation(t *testing.T) {
	t.Parallel()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "user-a"},
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)

	// Client side: the leaf of the tls.Certificate presented for mTLS.
	clientCert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	clientLeaf, err := x509.ParseCertificate(clientCert.Certificate[0])
	require.NoError(t, err)
	clientFingerprint := KubeClientCertFingerprint(clientLeaf)

	// Server side: the peer certificate from the TLS connection state.
	peerCert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	serverFingerprint := KubeClientCertFingerprint(peerCert)

	require.Equal(t, clientFingerprint, serverFingerprint)
	wantSum := sha256.Sum256(der)
	require.Equal(t, wantSum[:], serverFingerprint)
}
