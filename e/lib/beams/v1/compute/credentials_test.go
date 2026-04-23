package compute_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/e/lib/beams/v1/compute"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
)

func TestTransportCredentials(t *testing.T) {
	t.Parallel()

	clusterName := "example.com"
	activeCA := newTestCAKeys(t, "active", signerFromFixture(t, fixtures.PEMBytes["rsa"]))
	rotatedCA := newTestCAKeys(t, "rotated", signerFromFixture(t, fixtures.PEMBytes["rsa-db-client"]))
	ca := newTestSPIFFECA(clusterName, activeCA, rotatedCA)

	clientCreds, err := compute.TransportCredentials(t.Context(), compute.TransportCredentialsConfig{
		ClusterName:          clusterName,
		AuthPreferenceGetter: testAuthPreferenceGetter{},
		CertAuthorityGetter:  testCertAuthorityGetter{ca: ca},
		Keystore:             testKeystore{certPEM: activeCA.certPEM, signer: activeCA.signer},
		Emitter:              &eventstest.MockRecorderEmitter{},
	})
	require.NoError(t, err)

	trustDomain := spiffeid.RequireTrustDomainFromString(clusterName)
	clientID := spiffeid.RequireFromString("spiffe://example.com/_teleport-cloud/beams/auth-server")
	serverID := spiffeid.RequireFromString("spiffe://example.com/_teleport-cloud/beams/orchestrator")
	serverSVID := newTestSVID(t, rotatedCA, serverID, signerFromFixture(t, fixtures.LocalhostKey))

	serverBundle := x509bundle.New(trustDomain)
	serverBundle.AddX509Authority(activeCA.cert)
	serverBundle.AddX509Authority(rotatedCA.cert)

	clientPeerIDs := make(chan spiffeid.ID, 1)
	server := grpc.NewServer(
		grpc.Creds(
			grpccredentials.MTLSServerCredentials(
				staticSVIDSource{svid: serverSVID},
				staticBundleSource{trustDomain: trustDomain, bundle: serverBundle},
				tlsconfig.AuthorizeID(clientID),
			),
		),
		grpc.UnaryInterceptor(func(
			ctx context.Context,
			req any,
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (any, error) {
			if id, ok := grpccredentials.PeerIDFromContext(ctx); ok {
				select {
				case clientPeerIDs <- id:
				default:
				}
			}
			return handler(ctx, req)
		}),
	)
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthSrv)

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		_ = lis.Close()
		server.Stop()
	})

	go func() { _ = server.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(clientCreds),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	var serverPeer peer.Peer
	_, err = healthpb.NewHealthClient(conn).Check(
		t.Context(),
		&healthpb.HealthCheckRequest{},
		grpc.Peer(&serverPeer),
	)
	require.NoError(t, err)

	select {
	case got := <-clientPeerIDs:
		require.Equal(t, clientID, got)
	default:
		t.Fatal("server did not observe client SPIFFE ID")
	}

	gotServerID, ok := grpccredentials.PeerIDFromPeer(&serverPeer)
	require.True(t, ok)
	require.Equal(t, serverID, gotServerID)
}

type testAuthPreferenceGetter struct{}

func (testAuthPreferenceGetter) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}

type testCertAuthorityGetter struct {
	ca types.CertAuthority
}

func (g testCertAuthorityGetter) GetCertAuthority(context.Context, types.CertAuthID, bool) (types.CertAuthority, error) {
	return g.ca, nil
}

type testKeystore struct {
	certPEM []byte
	signer  crypto.Signer
}

func (k testKeystore) GetTLSCertAndSigner(context.Context, types.CertAuthority) ([]byte, crypto.Signer, error) {
	return k.certPEM, k.signer, nil
}

type staticSVIDSource struct {
	svid *x509svid.SVID
}

func (s staticSVIDSource) GetX509SVID() (*x509svid.SVID, error) {
	return s.svid, nil
}

type staticBundleSource struct {
	trustDomain spiffeid.TrustDomain
	bundle      *x509bundle.Bundle
}

func (s staticBundleSource) GetX509BundleForTrustDomain(trustDomain spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	if trustDomain.String() != s.trustDomain.String() {
		return nil, trace.NotFound("unknown trust domain: %s", trustDomain)
	}
	return s.bundle, nil
}

func signerFromFixture(t *testing.T, keyPEM []byte) crypto.Signer {
	t.Helper()

	key, err := keys.ParsePrivateKey(keyPEM)
	require.NoError(t, err)
	return key.Signer
}

type testCAKeys struct {
	cert    *x509.Certificate
	certPEM []byte
	signer  crypto.Signer
}

func newTestCAKeys(t *testing.T, commonName string, signer crypto.Signer) testCAKeys {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, signer.Public(), signer)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return testCAKeys{
		cert: cert,
		certPEM: pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDER,
		}),
		signer: signer,
	}
}

func newTestSPIFFECA(clusterName string, active, additional testCAKeys) types.CertAuthority {
	return &types.CertAuthorityV2{
		Kind:    types.KindCertAuthority,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: clusterName,
		},
		Spec: types.CertAuthoritySpecV2{
			Type:        types.SPIFFECA,
			ClusterName: clusterName,
			ActiveKeys: types.CAKeySet{
				TLS: []*types.TLSKeyPair{
					{Cert: active.certPEM},
				},
			},
			AdditionalTrustedKeys: types.CAKeySet{
				TLS: []*types.TLSKeyPair{
					{Cert: additional.certPEM},
				},
			},
		},
	}
}

func newTestSVID(t *testing.T, ca testCAKeys, spiffeID spiffeid.ID, key crypto.Signer) *x509svid.SVID {
	t.Helper()

	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			NotBefore:    time.Now().Add(-time.Minute),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature |
				x509.KeyUsageKeyEncipherment |
				x509.KeyUsageKeyAgreement,
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
				x509.ExtKeyUsageClientAuth,
			},
			BasicConstraintsValid: true,
			URIs:                  []*url.URL{spiffeID.URL()},
		},
		ca.cert,
		key.Public(),
		ca.signer,
	)
	require.NoError(t, err)

	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	svid, err := x509svid.Parse(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		keyPEM,
	)
	require.NoError(t, err)

	return svid
}
