// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package auth_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestSPIFFEAuth_ListUnifiedResources end-to-end exercises:
//   - SPIFFECA included in the Auth server's ClientCA pool.
//   - Middleware identification of an SVID as a SPIFFEIdentity.
//   - Scoped authorizer building a Pin on the fly via the new
//     PopulatePinnedAssignmentsForSPIFFEID path.
//   - scopedListUnifiedResources filtering nodes by the resulting CheckerContext.
//
// The SVID is signed directly using the SPIFFECA's signing key (extracted via
// the auth server's keystore) rather than going through the full Workload
// Identity issuance RPC, which would require a Bot/ProvisionToken/WorkloadIdentity
// setup unrelated to what this test is exercising. The SVID's content (URI SAN,
// SPIFFECA issuer, key usage flags) matches what the issuance flow produces.
func TestSPIFFEAuth_ListUnifiedResources(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	ctx := t.Context()
	srv := newSPIFFEAuthTestServer(t)
	clusterName := srv.ClusterName()

	assignedScope := "/poc"
	scopedNode := registerNode(t, srv, "scoped-node", assignedScope)

	// Create a ScopedRole that grants list/read on nodes and SSH access.
	const roleName = "node-reader"
	const spiffePath = "/poc/test"
	spiffeID := spiffeid.RequireFromString("spiffe://" + clusterName + spiffePath)

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminClient.Close() })
	scopedSvc := adminClient.ScopedAccessServiceClient()

	_, err = scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    access.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: roleName,
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{assignedScope},
				// node:list/read aren't valid rules for ScopedRoles (see
				// lib/scopes/access/verbs.go). Node access is gated entirely
				// by the Ssh block + CheckerContext.SSH().CanAccessSSHServer
				// in scopedListUnifiedResources.
				Ssh: &scopedaccessv1.ScopedRoleSSH{
					Logins: []string{"root"},
					Labels: []*labelv1.Label{{
						Name:   types.Wildcard,
						Values: []string{types.Wildcard},
					}},
				},
			},
		},
	})
	require.NoError(t, err)

	// Create the SPIFFE-targeted ScopedRoleAssignment.
	createResp, err := scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    access.KindScopedRoleAssignment,
			SubKind: access.SubKindDynamic,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			Scope: "/",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				SpiffeId: spiffeID.String(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: roleName, Scope: assignedScope},
				},
			},
		},
	})
	require.NoError(t, err)

	// Wait for cache propagation so the authorizer sees the new assignment.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := srv.Auth().ScopedAccessCache.GetScopedRoleAssignment(ctx, &scopedaccessv1.GetScopedRoleAssignmentRequest{
			Name:    createResp.GetAssignment().GetMetadata().GetName(),
			SubKind: createResp.GetAssignment().GetSubKind(),
		})
		require.NoError(t, err)
	}, 10*time.Second, 100*time.Millisecond)

	// Mint an SVID against this cluster's SPIFFECA.
	svidCert, svidKey := mintSVID(t, srv, spiffeID)
	trustPool := spiffeTrustPool(t, srv)

	t.Run("positive — node at assigned scope is visible", func(t *testing.T) {
		client := dialAuthWithSVID(t, srv, svidCert, svidKey, trustPool, clusterName)
		resp, err := client.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
			Kinds: []string{types.KindNode},
			Limit: 100,
		})
		require.NoError(t, err)

		names := make([]string, 0, len(resp.Resources))
		for _, r := range resp.Resources {
			if n := r.GetNode(); n != nil {
				names = append(names, n.GetName())
			}
		}
		// scoped-node is at /poc; the role applies at /poc.
		require.Contains(t, names, scopedNode.GetName(), "expected scoped-node to be returned")
	})

	t.Run("negative — foreign-issuer SVID fails TLS handshake", func(t *testing.T) {
		// Mint an SVID signed by a different (test-generated) CA. The TLS
		// handshake must reject it before any authz happens, since the
		// foreign CA isn't in the auth server's ClientCAs.
		_, foreignCert, foreignKey := mintForeignSVID(t, clusterName, spiffePath)
		client := dialAuthWithSVID(t, srv, foreignCert, foreignKey, trustPool, clusterName)
		_, err := client.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
			Kinds: []string{types.KindNode},
			Limit: 100,
		})
		require.Error(t, err)
	})
}

// TestSPIFFEAuth_NoAssignmentAccessDenied verifies that an SVID with no
// matching ScopedRoleAssignment is rejected with access denied (the
// PopulatePinnedAssignmentsForSPIFFEID call returns NotFound, which the
// authorizer surfaces as access denied).
func TestSPIFFEAuth_NoAssignmentAccessDenied(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	ctx := t.Context()
	srv := newSPIFFEAuthTestServer(t)
	clusterName := srv.ClusterName()

	spiffeID := spiffeid.RequireFromString("spiffe://" + clusterName + "/poc/unbound")
	svidCert, svidKey := mintSVID(t, srv, spiffeID)
	trustPool := spiffeTrustPool(t, srv)

	client := dialAuthWithSVID(t, srv, svidCert, svidKey, trustPool, clusterName)
	_, err := client.ListUnifiedResources(ctx, &proto.ListUnifiedResourcesRequest{
		Kinds: []string{types.KindNode},
		Limit: 100,
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error, got %v", err)
	require.Contains(t, []codes.Code{codes.PermissionDenied, codes.Unauthenticated, codes.NotFound}, st.Code(),
		"expected access-denied-ish code, got %v: %v", st.Code(), err)
}

// newSPIFFEAuthTestServer spins up a TestTLSServer with the scopes feature
// enabled. The SPIFFECA is bootstrapped by the auth server during init.
func newSPIFFEAuthTestServer(t *testing.T) *authtest.TLSServer {
	t.Helper()
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })
	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

// registerNode upserts a node (optionally scoped) and returns it.
func registerNode(t *testing.T, srv *authtest.TLSServer, name, scope string) types.Server {
	t.Helper()
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: name,
		},
		Spec: types.ServerSpecV2{
			Hostname: name,
		},
	}
	if scope != "" {
		node.Scope = scope
	}
	_, err := srv.Auth().UpsertNode(t.Context(), node)
	require.NoError(t, err)
	return node
}

// mintSVID signs an X.509 SVID using the test cluster's SPIFFECA. The cert
// shape mirrors what lib/auth/machineid/workloadidentityv1/issuer_service.go
// produces (URI SAN, no Teleport identity OIDs, X509-SVID-compliant key usage).
func mintSVID(t *testing.T, srv *authtest.TLSServer, id spiffeid.ID) (tls.Certificate, crypto.Signer) {
	t.Helper()
	ctx := t.Context()
	ca, err := srv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: srv.ClusterName(),
	}, true /* loadKeys */)
	require.NoError(t, err)

	caTLSCert, caSigner, err := srv.Auth().GetKeyStore().GetTLSCertAndSigner(ctx, ca)
	require.NoError(t, err)
	caCert, err := tlsca.ParseCertificatePEM(caTLSCert)
	require.NoError(t, err)

	workloadKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment |
			x509.KeyUsageKeyAgreement,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  false,
		URIs:                  []*url.URL{id.URL()},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, workloadKey.Public(), caSigner)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  workloadKey,
		Leaf:        mustParseCert(t, der),
	}, workloadKey
}

// mintForeignSVID generates a self-signed CA and signs an SVID-shaped cert
// with it. Used to verify TLS-handshake rejection of certs not chained to the
// cluster's SPIFFECA. Trust domain in the URI matches the local cluster to
// rule out the trust-domain check being the rejection cause — we want the
// chain verification to fail first.
func mintForeignSVID(t *testing.T, clusterName, spiffePath string) (caCert *x509.Certificate, leaf tls.Certificate, leafKey crypto.Signer) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	caTpl := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: "foreign-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, caKey.Public(), caKey)
	require.NoError(t, err)
	caCert = mustParseCert(t, caDER)

	leafKeyECDSA, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	id := spiffeid.RequireFromString("spiffe://" + clusterName + spiffePath)
	leafTpl := &x509.Certificate{
		SerialNumber: leafSerial,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment |
			x509.KeyUsageKeyAgreement,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  false,
		URIs:                  []*url.URL{id.URL()},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTpl, caCert, leafKeyECDSA.Public(), caKey)
	require.NoError(t, err)

	leaf = tls.Certificate{
		Certificate: [][]byte{leafDER},
		PrivateKey:  leafKeyECDSA,
		Leaf:        mustParseCert(t, leafDER),
	}
	leafKey = leafKeyECDSA
	return caCert, leaf, leafKey
}

// spiffeTrustPool returns a cert pool used as RootCAs on the SVID client. It
// includes the cluster's HostCA (used to verify the auth server's serving
// cert) — the SPIFFECA isn't needed here since it's used for the *server* to
// verify the SVID, not the other way around.
func spiffeTrustPool(t *testing.T, srv *authtest.TLSServer) *x509.CertPool {
	t.Helper()
	ctx := t.Context()
	hostCA, err := srv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: srv.ClusterName(),
	}, false)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	for _, kp := range hostCA.GetTrustedTLSKeyPairs() {
		cert, err := tlsca.ParseCertificatePEM(kp.Cert)
		require.NoError(t, err)
		pool.AddCert(cert)
	}
	return pool
}

// dialAuthWithSVID builds a gRPC client to the auth server using the SVID as
// the client cert.
func dialAuthWithSVID(t *testing.T, srv *authtest.TLSServer, svid tls.Certificate, _ crypto.Signer, roots *x509.CertPool, clusterName string) proto.AuthServiceClient {
	t.Helper()
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{svid},
		RootCAs:      roots,
		ServerName:   apiutils.EncodeClusterName(clusterName),
		MinVersion:   tls.VersionTLS12,
	}
	conn, err := grpc.NewClient(
		srv.Addr().String(),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return proto.NewAuthServiceClient(conn)
}

func mustParseCert(t *testing.T, der []byte) *x509.Certificate {
	t.Helper()
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err, "parsing cert")
	return cert
}

