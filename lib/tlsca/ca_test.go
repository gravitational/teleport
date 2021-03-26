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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/fixtures"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestPrincipals makes sure that SAN extension of generated x509 cert gets
// correctly set with DNS names and IP addresses based on the provided
// principals.
func TestPrincipals(t *testing.T) {
	ca, err := FromKeys([]byte(fixtures.SigningCertPEM), []byte(fixtures.SigningKeyPEM))
	require.NoError(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	require.NoError(t, err)

	hostnames := []string{"localhost", "example.com"}
	ips := []string{"127.0.0.1", "192.168.1.1"}

	clock := clockwork.NewFakeClock()

	certBytes, err := ca.GenerateCertificate(CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   pkix.Name{CommonName: "test"},
		NotAfter:  clock.Now().Add(time.Hour),
		DNSNames:  append(hostnames, ips...),
	})
	require.NoError(t, err)

	cert, err := ParseCertificatePEM(certBytes)
	require.NoError(t, err)
	require.ElementsMatch(t, cert.DNSNames, hostnames)
	var certIPs []string
	for _, ip := range cert.IPAddresses {
		certIPs = append(certIPs, ip.String())
	}
	require.ElementsMatch(t, certIPs, ips)
}

// TestKubeExtensions test ASN1 subject kubernetes extensions
func TestKubeExtensions(t *testing.T) {
	clock := clockwork.NewFakeClock()
	ca, err := FromKeys([]byte(fixtures.SigningCertPEM), []byte(fixtures.SigningKeyPEM))
	require.NoError(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	require.NoError(t, err)

	expires := clock.Now().Add(time.Hour)
	identity := Identity{
		Username:     "alice@example.com",
		Groups:       []string{"admin"},
		Impersonator: "bob@example.com",
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
