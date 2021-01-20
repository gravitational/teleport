/*
Copyright 2019 Gravitational, Inc.

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

package tlsca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/stretchr/testify/require"

	"github.com/jonboulle/clockwork"
	check "gopkg.in/check.v1"
)

func TestTLSCA(t *testing.T) { check.TestingT(t) }

type TLSCASuite struct {
	clock clockwork.Clock
}

var _ = check.Suite(&TLSCASuite{
	clock: clockwork.NewFakeClock(),
})

// TestPrincipals makes sure that SAN extension of generated x509 cert gets
// correctly set with DNS names and IP addresses based on the provided
// principals.
func (s *TLSCASuite) TestPrincipals(c *check.C) {
	ca, err := New([]byte(fixtures.SigningCertPEM), []byte(fixtures.SigningKeyPEM))
	c.Assert(err, check.IsNil)

	privateKey, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	c.Assert(err, check.IsNil)

	hostnames := []string{"localhost", "example.com"}
	ips := []string{"127.0.0.1", "192.168.1.1"}

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     s.clock,
		PublicKey: privateKey.Public(),
		Subject:   pkix.Name{CommonName: "test"},
		NotAfter:  s.clock.Now().Add(time.Hour),
		DNSNames:  append(hostnames, ips...),
	})
	c.Assert(err, check.IsNil)

	cert, err := ParseCertificatePEM(certBytes)
	c.Assert(err, check.IsNil)
	c.Assert(cert.DNSNames, check.DeepEquals, hostnames)
	var certIPs []string
	for _, ip := range cert.IPAddresses {
		certIPs = append(certIPs, ip.String())
	}
	c.Assert(certIPs, check.DeepEquals, ips)
}

// TestKubeExtensions test ASN1 subject kubernetes extensions
func TestKubeExtensions(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ca, err := New([]byte(fixtures.SigningCertPEM), []byte(fixtures.SigningKeyPEM))
	require.NoError(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	require.NoError(t, err)

	expires := clock.Now().Add(time.Hour)
	identity := Identity{
		Username: "alice@example.com",
		Groups:   []string{"admin"},
		// Generate a certificate restricted for
		// use against a kubernetes endpoint, and not the API server endpoint
		// otherwise proxies can generate certs for any user.
		Usage:             []string{teleport.UsageKubeOnly},
		KubernetesGroups:  []string{"system:masters", "admin"},
		KubernetesUsers:   []string{"IAM#alice@example.com"},
		KubernetesCluster: "kube-cluster",
		TeleportCluster:   "tele-cluster",
		RouteToDatabase: RouteToDatabase{
			ServiceName: "postgres-rds",
			Protocol:    "postgres",
			Username:    "postgres",
		},
		DatabaseNames: []string{"postgres", "main"},
		DatabaseUsers: []string{"postgres", "alice"},
		Expires:       expires,
	}

	subj, err := identity.Subject()
	require.NoError(t, err)

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)
	out, err := FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(out, &identity))
}
