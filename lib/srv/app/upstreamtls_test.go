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

package app

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authcatest"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// TestHandleConnectionTLSUpstream verifies that the app service can establish
// a TLS connection with an upstream HTTPS application using the "tls" options.
//
// Note: We can't test default behavior when TLS is empty because our test
// server uses self-signed certificate, and setting `SSL_CERT_FILE` would only
// work on Linux.
func TestHandleConnectionTLSUpstream(t *testing.T) {
	for name, tc := range map[string]struct {
		tlsOptsFunc  func(*tlsUpstreamSetup) *types.AppTLS
		upstreamOpts []upstreamServerOpt
		insecureMode bool
		expectError  bool
	}{
		"verify-server-name": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:       types.AppTLSModeVerifyServerName,
					AllowedCas: []string{string(setup.serverCACertPEM)},
				}
			},
			expectError: false,
		},
		"verify-server-name custom server name": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:       types.AppTLSModeVerifyServerName,
					ServerName: "example-server.com",
					AllowedCas: []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"example-server.com"}),
			},
			expectError: false,
		},
		"verify-server-name mismatch": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:       types.AppTLSModeVerifyServerName,
					ServerName: "random-server.com",
					AllowedCas: []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"example-server.com"}),
			},
			expectError: true,
		},
		"verify-server-name untrusted CA": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode: types.AppTLSModeVerifyServerName,
				}
			},
			expectError: true,
		},
		"verify-spiffe-id": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: false,
		},
		"verify-spiffe-id skips server name verify": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"example-server.com"}),
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: false,
		},
		"verify-spiffe-id mismatch": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/random",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: true,
		},
		"verify-spiffe-id non-SVID cert": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/random",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			expectError: true,
		},
		"verify-spiffe-id untrusted CA": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: true,
		},
		"verify-spiffe-id workload_identity alias": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{types.AppTLSInternalCAWorkloadIdentity},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
				withUpstreamCustomCA(types.SPIFFECA),
			},
			expectError: false,
		},
		"verify-spiffe-id workload_identity alias with cert from another CA": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{types.AppTLSInternalCAWorkloadIdentity},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: true,
		},
		"verify-spiffe-id workload_identity alias combined with inline CA": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifySpiffeID,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas: []string{
						types.AppTLSInternalCAWorkloadIdentity,
						string(setup.serverCACertPEM),
					},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: false,
		},
		"verify-full": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerName:     "example-server.com",
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"example-server.com"}),
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: false,
		},
		"verify-full unspecified server name": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: false,
		},
		"verify-full untrusted CA": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerName:     "example-server.com",
					ServerSpiffeId: "spiffe://mycluster/svc/example",
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"example-server.com"}),
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: true,
		},
		"verify-full server name mismatch": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerName:     "example-server.com",
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"random-server.com"}),
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
			expectError: true,
		},
		"verify-full mismatch server spiffe id": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerName:     "example-server.com",
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"example-server.com"}),
				withUpstreamSpiffeID("spiffe://mycluster/svc/random"),
			},
			expectError: true,
		},
		"verify-full workload_identity alias": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerName:     "example-server.com",
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{types.AppTLSInternalCAWorkloadIdentity},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"example-server.com"}),
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
				withUpstreamCustomCA(types.SPIFFECA),
			},
			expectError: false,
		},
		"verify-full both mismatch": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerName:     "example-server.com",
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"random-server.com"}),
				withUpstreamSpiffeID("spiffe://mycluster/svc/random"),
			},
			expectError: true,
		},
		"verify-full server name is set to an IP value": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerName:     "127.0.0.1",
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
		},
		"verify-full server name falls back to app URI hostname": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:           types.AppTLSModeVerifyFull,
					ServerSpiffeId: "spiffe://mycluster/svc/example",
					AllowedCas:     []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamSpiffeID("spiffe://mycluster/svc/example"),
			},
		},
		"service level insecure mode takes precedence": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode:       types.AppTLSModeVerifyServerName,
					AllowedCas: []string{string(setup.serverCACertPEM)},
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"random-value.com"}),
			},
			insecureMode: true,
			expectError:  false,
		},
		"insecure": {
			tlsOptsFunc: func(setup *tlsUpstreamSetup) *types.AppTLS {
				return &types.AppTLS{
					Mode: types.AppTLSModeInsecure,
				}
			},
			upstreamOpts: []upstreamServerOpt{
				withUpstreamCertSANs([]string{"random-value.com"}),
			},
			expectError: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			clock := clockwork.NewRealClock()
			setup := newTLSUpstreamSetup(t)

			httpsMessage := uuid.New().String()
			httpsUpstreamAddr := setup.startUpstreamHTTPSServer(
				t,
				clock,
				httpsMessage,
				tc.upstreamOpts...,
			)
			tlsMessage := uuid.New().String()
			tlsUpstreamAddr := setup.startUpstreamTLSServer(
				t,
				clock,
				tlsMessage,
				tc.upstreamOpts...,
			)

			// Since we're configuring the TLS verify function, we must ensure
			// that different handling (IP and DNS) should work the same way.
			httpsApps, httpsPublicAddrs := configureAppIPAndLocalhost(t, setup, httpsUpstreamAddr, tc.tlsOptsFunc(setup))
			tlsApps, tlsPublicAddrs := configureAppIPAndLocalhost(t, setup, tlsUpstreamAddr, tc.tlsOptsFunc(setup))

			s := SetUpSuiteWithConfig(t, suiteConfig{
				Apps:         append(httpsApps, tlsApps...),
				OverrideCAs:  []types.CertAuthority{setup.spiffeCAResource},
				InsecureMode: tc.insecureMode,
			})

			t.Run("HTTPS", func(t *testing.T) {
				for _, publicAddr := range httpsPublicAddrs {
					t.Run(publicAddr, func(t *testing.T) {
						cert := s.generateCertificate(t, s.user, publicAddr, "")

						s.checkHTTPResponse(t, cert, func(resp *http.Response) {
							if tc.expectError {
								require.Equal(t, http.StatusInternalServerError, resp.StatusCode, "expected HTTPS request to fail")
								return
							}

							require.Equal(t, http.StatusOK, resp.StatusCode)
							buf, err := io.ReadAll(resp.Body)
							require.NoError(t, err)
							require.Equal(t, httpsMessage, strings.TrimSpace(string(buf)))
						})
					})
				}
			})

			t.Run("TLS", func(t *testing.T) {
				for _, publicAddr := range tlsPublicAddrs {
					t.Run(publicAddr, func(t *testing.T) {
						serverConn, clientConn := net.Pipe()
						defer serverConn.Close()
						defer clientConn.Close()

						var wg sync.WaitGroup

						wg.Go(func() {
							s.appServer.HandleConnection(clientConn)
						})

						// This is dialing the app server not the upstream
						// target. We should expect no errors in this phase.
						cert := s.generateCertificate(t, s.user, publicAddr, "")
						tlsConn := tls.Client(serverConn, &tls.Config{
							RootCAs:      s.hostCertPool,
							Certificates: []tls.Certificate{cert},
							ServerName:   constants.APIDomain,
							Time:         s.clock.Now,
						})
						t.Cleanup(func() { _ = tlsConn.Close() })
						err := tlsConn.HandshakeContext(t.Context())
						require.NoError(t, err)

						// Upstream errors will appear for clients on the first
						// write.
						_, err = tlsConn.Write([]byte("hello"))
						if tc.expectError {
							require.Error(t, err)
							return
						}
						require.NoError(t, err)

						buf := make([]byte, len(tlsMessage))
						_, err = tlsConn.Read(buf)
						require.NoError(t, err)
						require.Equal(t, tlsMessage, string(buf))
					})
				}
			})
		})
	}
}

// tlsUpstreamSetup holds the CAs, and helpers needed to configure an upstream
// TLS application for testing.
type tlsUpstreamSetup struct {
	serverCA         *tlsca.CertAuthority
	serverCACertPEM  []byte
	spiffeCA         *tlsca.CertAuthority
	spiffeCAResource types.CertAuthority
}

func newTLSUpstreamSetup(t *testing.T) *tlsUpstreamSetup {
	t.Helper()

	serverCAKey, serverCACertPEM, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{Organization: []string{"test-upstream-server-ca"}},
		nil,
		1*time.Hour,
	)
	require.NoError(t, err)

	serverCA, err := tlsca.FromKeys(serverCACertPEM, serverCAKey)
	require.NoError(t, err)

	// Build a SPIFFE CA whose private key is owned by the test.
	spiffeCAResource, err := authcatest.NewCA(types.SPIFFECA, "root.example.com")
	require.NoError(t, err)
	spiffeKP := spiffeCAResource.GetActiveKeys().TLS[0]
	spiffeCA, err := tlsca.FromKeys(spiffeKP.Cert, spiffeKP.Key)
	require.NoError(t, err)

	return &tlsUpstreamSetup{
		serverCA:         serverCA,
		serverCACertPEM:  serverCACertPEM,
		spiffeCA:         spiffeCA,
		spiffeCAResource: spiffeCAResource,
	}
}

type upstreamConfig struct {
	sans     []string
	spiffeID string
	customCA types.CertAuthType
}

type upstreamServerOpt func(c *upstreamConfig)

func withUpstreamCertSANs(sans []string) upstreamServerOpt {
	return func(c *upstreamConfig) {
		c.sans = sans
	}
}

func withUpstreamSpiffeID(s string) upstreamServerOpt {
	return func(c *upstreamConfig) {
		c.spiffeID = s
	}
}

func withUpstreamCustomCA(caType types.CertAuthType) upstreamServerOpt {
	return func(c *upstreamConfig) {
		c.customCA = caType
	}
}

// startUpstreamHTTPSServer starts an httptest HTTPS server using self-signed
// certificate.
func (m *tlsUpstreamSetup) startUpstreamHTTPSServer(t *testing.T, clock clockwork.Clock, message string, opts ...upstreamServerOpt) string {
	t.Helper()

	var cfg upstreamConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, message)
	}))

	// Disable keep alive so we force new connections every time, forcing
	// certificate validation.
	upstream.Config.SetKeepAlivesEnabled(false)
	upstream.TLS = m.generateUpstreamTLSConfig(t, cfg, clock)
	upstream.StartTLS()
	t.Cleanup(upstream.Close)

	return upstream.URL
}

// startUpstreamTLSServer starts an httptest HTTPS server using self-signed
// certificate.
func (m *tlsUpstreamSetup) startUpstreamTLSServer(t *testing.T, clock clockwork.Clock, message string, opts ...upstreamServerOpt) string {
	t.Helper()

	var cfg upstreamConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", m.generateUpstreamTLSConfig(t, cfg, clock))
	require.NoError(t, err)

	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = ln.Close()
		wg.Wait()
	})

	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			wg.Go(func() {
				defer conn.Close()

				buf := make([]byte, 32)
				for {
					_, err := conn.Read(buf)
					if err != nil {
						return
					}
					fmt.Fprintln(conn, message)
				}
			})
		}
	})

	return types.SchemeTLS + "://" + ln.Addr().String()
}

// generateUpstreamTLSConfig generates a server certificate signed by the server
// CA (using real time for NotAfter so the app service transport can validate it).
func (m *tlsUpstreamSetup) generateUpstreamTLSConfig(t *testing.T, cfg upstreamConfig, clock clockwork.Clock) *tls.Config {
	t.Helper()

	serverKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	serverKeyPEM, err := keys.MarshalPrivateKey(serverKey)
	require.NoError(t, err)

	csr := tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: serverKey.Public(),
		Subject:   pkix.Name{CommonName: "127.0.0.1"},
		NotAfter:  time.Now().Add(1 * time.Hour),
		DNSNames:  []string{"127.0.0.1", "localhost"},
	}
	if len(cfg.sans) > 0 {
		csr.DNSNames = cfg.sans
	}

	signingCA := m.serverCA
	if cfg.customCA == types.SPIFFECA {
		signingCA = m.spiffeCA
	}

	var serverCertPEM []byte
	if cfg.spiffeID != "" {
		spiffeID, err := spiffeid.FromString(cfg.spiffeID)
		require.NoError(t, err)
		serverCertPEM = generateSVIDCertificate(t, *signingCA, csr, spiffeID)
	} else {
		serverCertPEM, err = signingCA.GenerateCertificate(csr)
		require.NoError(t, err)
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	require.NoError(t, err)

	// Disable keep alive so we force new connections every time, forcing
	// certificate validation.
	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		Time:         clock.Now,
	}
}

// newApp creates a types.AppV3 configured with TLS options.
func (m *tlsUpstreamSetup) newApp(t *testing.T, upstreamURL string, publicAddr string, appTLS *types.AppTLS) *types.AppV3 {
	t.Helper()

	name := uuid.New().String()
	app, err := types.NewAppV3(types.Metadata{
		Name:   name,
		Labels: staticLabels,
	}, types.AppSpecV3{
		URI:        upstreamURL,
		PublicAddr: publicAddr,
		TLS:        appTLS,
	})
	require.NoError(t, err)
	return app
}

func generateSVIDCertificate(t *testing.T, ca tlsca.CertAuthority, req tlsca.CertificateRequest, spiffeID spiffeid.ID) []byte {
	require.NoError(t, req.CheckAndSetDefaults())
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	// Based on Workload idenity issuance service.
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      req.Subject,
		NotBefore:    req.Clock.Now().UTC().Add(-1 * time.Minute),
		NotAfter:     req.NotAfter,
		// SPEC(X509-SVID) 4.3. Key Usage:
		// - Leaf SVIDs MUST NOT set keyCertSign or cRLSign.
		// - Leaf SVIDs MUST set digitalSignature
		// - They MAY set keyEncipherment and/or keyAgreement;
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment |
			x509.KeyUsageKeyAgreement,
		// SPEC(X509-SVID) 4.4. Extended Key Usage:
		// - Leaf SVIDs SHOULD include this extension, and it MAY be marked as critical.
		// - When included, fields id-kp-serverAuth and id-kp-clientAuth MUST be set.
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
		},
		// SPEC(X509-SVID) 4.1. Basic Constraints:
		// - leaf certificates MUST set the cA field to false
		BasicConstraintsValid: true,
		IsCA:                  false,
		// SPEC(X509-SVID) 2. SPIFFE ID:
		// - The corresponding SPIFFE ID is set as a URI type in the Subject Alternative Name extension
		// - An X.509 SVID MUST contain exactly one URI SAN, and by extension, exactly one SPIFFE ID.
		// - An X.509 SVID MAY contain any number of other SAN field types, including DNS SANs.
		URIs: []*url.URL{spiffeID.URL()},
	}

	for i := range req.DNSNames {
		if ip := net.ParseIP(req.DNSNames[i]); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, req.DNSNames[i])
		}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, req.PublicKey, ca.Signer)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
}

func configureAppIPAndLocalhost(t *testing.T, setup *tlsUpstreamSetup, url string, tlsOptions *types.AppTLS) ([]types.Application, []string) {
	randomHex, err := utils.CryptoRandomHex(5)
	require.NoError(t, err)
	ipPublicAddr := "ip." + randomHex + ".example.com"
	localhostPublicAddr := "localhost." + randomHex + ".example.com"

	appUsingIP := setup.newApp(t, url, ipPublicAddr, tlsOptions)
	appUsingLocalhost := setup.newApp(t, convertToLocalhost(t, url), localhostPublicAddr, tlsOptions)

	return []types.Application{appUsingIP, appUsingLocalhost}, []string{ipPublicAddr, localhostPublicAddr}
}

// convertToLocalhost converts a upstream IP address to localhost.
func convertToLocalhost(t *testing.T, rawURL string) string {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	u.Host = net.JoinHostPort("localhost", port)
	return u.String()
}
