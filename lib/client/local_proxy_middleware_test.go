// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package client_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestCertChecker(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	certIssuer := newMockCertIssuer(t, clock)
	certChecker := client.NewCertChecker(certIssuer, clock)

	// certChecker should issue a new cert on first request.
	cert, err := certChecker.GetOrIssueCert(ctx)
	require.NoError(t, err)
	require.NoError(t, certChecker.RetrieveError())

	// subsequent calls should return the same cert.
	sameCert, err := certChecker.GetOrIssueCert(ctx)
	require.NoError(t, err)
	require.NoError(t, certChecker.RetrieveError())
	require.Equal(t, cert, sameCert)

	// If the current cert expires it should be reissued.
	clock.Advance(2 * time.Minute)
	expiredCert := cert

	cert, err = certChecker.GetOrIssueCert(ctx)
	require.NoError(t, err)
	require.NoError(t, certChecker.RetrieveError())
	require.NotEqual(t, cert, expiredCert)

	// If the current cert fails certIssuer checks, a new one should be issued.
	certIssuer.checkErr = trace.BadParameter("bad cert")
	badCert := cert

	cert, err = certChecker.GetOrIssueCert(ctx)
	require.NoError(t, err)
	require.NoError(t, certChecker.RetrieveError())
	require.NotEqual(t, cert, badCert)

	// If issuing a new cert fails, an error is returned.
	certIssuer.issueErr = trace.BadParameter("failed to issue cert")
	_, err = certChecker.GetOrIssueCert(ctx)
	require.ErrorIs(t, err, certIssuer.issueErr, "expected error %v but got %v", certIssuer.issueErr, err)
	require.ErrorIs(t, certChecker.RetrieveError(), err, "expected retrieve error to be the same get error but got: %v", certChecker.RetrieveError())

	// If the problem is solved, the error is clean up.
	certIssuer.issueErr = nil
	_, err = certChecker.GetOrIssueCert(ctx)
	require.NoError(t, err)
	require.NoError(t, certChecker.RetrieveError())
}

func TestLocalCertGenerator(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	certIssuer := newMockCertIssuer(t, clock)
	certChecker := client.NewCertChecker(certIssuer, clock)
	caPath := filepath.Join(t.TempDir(), "localca.pem")

	localCertGenerator, err := client.NewLocalCertGenerator(ctx, certChecker, caPath)
	require.NoError(t, err)

	// The cert generator should return the local CA cert for SNIs "localhost" or empty (plain ip).
	caCert, err := localCertGenerator.GetCertificate(&tls.ClientHelloInfo{
		ServerName: "localhost",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"localhost"}, caCert.Leaf.DNSNames)

	cert, err := localCertGenerator.GetCertificate(&tls.ClientHelloInfo{
		ServerName: "",
	})
	require.NoError(t, err)
	require.Equal(t, caCert, cert)

	// The cert generator should issue new certs from the local CA for other SNIs.
	exampleCert, err := localCertGenerator.GetCertificate(&tls.ClientHelloInfo{
		ServerName: "example.com",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"example.com"}, exampleCert.Leaf.DNSNames)
}

type mockCertIssuer struct {
	ca       *tlsca.CertAuthority
	clock    clockwork.Clock
	checkErr error
	issueErr error
}

func newMockCertIssuer(t *testing.T, clock clockwork.Clock) *mockCertIssuer {
	certIssuer := &mockCertIssuer{
		clock: clock,
	}

	certIssuer.initCA(t)
	return certIssuer
}

func (c *mockCertIssuer) initCA(t *testing.T) {
	priv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	cert, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Signer: priv,
		Entity: pkix.Name{
			CommonName:   "root",
			Organization: []string{"teleport"},
		},
		TTL:   defaults.CATTL,
		Clock: c.clock,
	})
	require.NoError(t, err)

	c.ca, err = tlsca.FromCertAndSigner(cert, priv)
	require.NoError(t, err)
}

func (c *mockCertIssuer) CheckCert(cert *x509.Certificate) error {
	return trace.Wrap(c.checkErr)
}

func (c *mockCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	if c.issueErr != nil {
		return tls.Certificate{}, trace.Wrap(c.issueErr)
	}

	priv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	certPem, err := c.ca.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: priv.Public(),
		Subject: pkix.Name{
			CommonName:   "user",
			Organization: []string{"teleport"},
		},
		NotAfter: c.clock.Now().Add(time.Minute),
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	tlsCert, err := tls.X509KeyPair(certPem, priv.PrivateKeyPEM())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return tlsCert, nil
}
