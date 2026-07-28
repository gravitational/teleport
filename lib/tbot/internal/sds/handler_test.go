/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package sds

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net/url"
	"os"
	"testing"
	"time"

	tlsv3pb "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discoveryv3pb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	workloadpb "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/utils/keys"
	apiworkloadidentity "github.com/gravitational/teleport/api/workloadidentity"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

type mockTrustBundleCache struct {
	currentBundle *workloadidentity.BundleSet
}

func (m *mockTrustBundleCache) GetBundleSet(ctx context.Context) (*workloadidentity.BundleSet, error) {
	return m.currentBundle, nil
}

func newTestSVID(t *testing.T, ca *tlsca.CertAuthority, spiffeID string) (certDER []byte, key crypto.Signer) {
	t.Helper()

	id, err := spiffeid.FromString(spiffeID)
	require.NoError(t, err)

	key, err = cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	certDER, err = x509.CreateCertificate(
		rand.Reader,
		&x509.Certificate{
			SerialNumber: big.NewInt(time.Now().UnixNano()),
			NotBefore:    time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
			NotAfter:     time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			KeyUsage: x509.KeyUsageDigitalSignature |
				x509.KeyUsageKeyEncipherment |
				x509.KeyUsageKeyAgreement,
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
				x509.ExtKeyUsageClientAuth,
			},
			BasicConstraintsValid: true,
			URIs:                  []*url.URL{id.URL()},
		},
		ca.Cert,
		key.Public(),
		ca.Signer,
	)
	require.NoError(t, err)

	return
}

func newTestSVIDPEM(t *testing.T, ca *tlsca.CertAuthority, spiffeID string) ([]byte, []byte) {
	t.Helper()

	certDER, key := newTestSVID(t, ca, spiffeID)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  internal.PEMBlockTypeCertificate,
		Bytes: certDER,
	})
	require.NotNil(t, certPEM)

	keyPEM, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err)

	return certPEM, keyPEM
}

func newSVIDWithChain(t *testing.T, spiffeID string, intermediates int) (chainBytes []byte, keyBytes []byte, root *x509.Certificate) {
	t.Helper()

	chain, err := testenv.MakeCAChain(intermediates+1, nil)
	require.NoError(t, err)

	// The last CA in the chain signs the leaf
	signer := chain[len(chain)-1]

	leafDER, leafKey := newTestSVID(t, &tlsca.CertAuthority{
		Cert:   signer.Cert,
		Signer: signer.Key,
	}, spiffeID)

	// Build the DER chain, starting with the leaf and following the chain in
	// reverse order.
	chainBuf := bytes.Buffer{}
	_, _ = chainBuf.Write(leafDER)
	for i := len(chain) - 1; i >= 1; i-- {
		_, _ = chainBuf.Write(chain[i].Cert.Raw)
	}

	keyBytes, err = x509.MarshalPKCS8PrivateKey(leafKey)
	require.NoError(t, err)

	return chainBuf.Bytes(), keyBytes, chain[0].Cert
}

func loadOrGenerateSVID(t *testing.T, ca *tlsca.CertAuthority, name, spiffeID string) ([]byte, []byte) {
	t.Helper()

	svidName := name + ".pem"
	keyName := name + "-key.pem"

	if golden.ShouldSet() {
		cert, key := newTestSVIDPEM(t, ca, spiffeID)

		golden.SetNamed(t, svidName, cert)
		golden.SetNamed(t, keyName, key)
	}

	// Parse the PEM through x509svid and roundtrip back to DER.
	svid, err := x509svid.Parse(
		golden.GetNamed(t, svidName),
		golden.GetNamed(t, keyName),
	)
	require.NoError(t, err)

	certDER, keyDER, err := svid.MarshalRaw()
	require.NoError(t, err)

	return certDER, keyDER
}

// TestSDS_FetchSecrets performs a unit-test over the FetchSecrets method.
// It tests the generation of the DiscoveryResponses and that authentication
// is enforced.
func TestSDS_FetchSecrets(t *testing.T) {
	log := logtest.NewLogger()
	ctx := context.Background()

	td, err := spiffeid.TrustDomainFromString("example.com")
	require.NoError(t, err)

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	firstCert, firstKey := loadOrGenerateSVID(t, ca, "default", "spiffe://example.com/default")
	secondCert, secondKey := loadOrGenerateSVID(t, ca, "second", "spiffe://example.com/second")

	clientAuthenticator := func(ctx context.Context) (*slog.Logger, SVIDFetcher, error) {
		return log, func(ctx context.Context, localBundle *spiffebundle.Bundle) ([]*workloadpb.X509SVID, error) {
			return []*workloadpb.X509SVID{
				{
					SpiffeId:    "spiffe://example.com/default",
					X509Svid:    firstCert,
					X509SvidKey: firstKey,
					Bundle:      workloadidentity.MarshalX509Bundle(localBundle.X509Bundle()),
				},
				{
					SpiffeId:    "spiffe://example.com/second",
					X509Svid:    secondCert,
					X509SvidKey: secondKey,
					Bundle:      workloadidentity.MarshalX509Bundle(localBundle.X509Bundle()),
				},
			}, nil
		}, nil
	}

	bundle := spiffebundle.New(td)
	bundle.AddX509Authority(ca.Cert)

	federatedBundle := spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com"))
	federatedBundle.AddX509Authority(ca.Cert)

	appClientBundle := spiffebundle.New(spiffeid.RequireTrustDomainFromString(
		apiworkloadidentity.NewInternalAppTrustDomain("example.com"),
	))
	appClientBundle.AddX509Authority(ca.Cert)

	mockBundleCache := &mockTrustBundleCache{
		currentBundle: &workloadidentity.BundleSet{
			Local:     bundle,
			AppClient: appClientBundle,
			Federated: map[string]*spiffebundle.Bundle{
				"federated.example.com": federatedBundle,
			},
		},
	}

	tests := []struct {
		name string

		resourceNames []string

		wantErr string
	}{
		{
			name:          "all",
			resourceNames: []string{},
		},
		{
			name: "specific svid",
			resourceNames: []string{
				"spiffe://example.com/second",
			},
		},
		{
			name: "specific ca",
			resourceNames: []string{
				"spiffe://example.com",
			},
		},
		{
			name: "specific ca federated",
			resourceNames: []string{
				"spiffe://federated.example.com",
			},
		},
		{
			name: "special default",
			resourceNames: []string{
				EnvoyDefaultSVIDName,
			},
		},
		{
			name: "special ROOTCA",
			resourceNames: []string{
				EnvoyDefaultBundleName,
			},
		},
		{
			name: "special ALL",
			resourceNames: []string{
				EnvoyAllBundlesName,
			},
		},
		{
			name:          "no results",
			resourceNames: []string{"spiffe://example.com/not-matching"},
			wantErr:       "unknown resource names: [spiffe://example.com/not-matching]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdsHandler, err := NewHandler(HandlerConfig{
				Logger:              log,
				RenewalInterval:     time.Minute,
				TrustBundleCache:    mockBundleCache,
				ClientAuthenticator: clientAuthenticator,
			})
			require.NoError(t, err)

			req := &discoveryv3pb.DiscoveryRequest{
				TypeUrl:       "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret",
				ResourceNames: tt.resourceNames,
			}

			res, err := sdsHandler.FetchSecrets(ctx, req)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if golden.ShouldSet() {
				resBytes, err := protojson.MarshalOptions{
					Multiline: true,
				}.Marshal(res)
				require.NoError(t, err)
				golden.Set(t, resBytes)
			}

			want := &discoveryv3pb.DiscoveryResponse{}
			require.NoError(t, protojson.UnmarshalOptions{}.Unmarshal(golden.Get(t), want))
			require.Empty(t, cmp.Diff(res, want, protocmp.Transform()))
		})
	}
}

func TestSDS_FetchSecrets_TrustDomainSelector(t *testing.T) {
	log := logtest.NewLogger()
	ctx := context.Background()

	td, err := spiffeid.TrustDomainFromString("example.com")
	require.NoError(t, err)

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	defaultCert, defaultKey := loadOrGenerateSVID(t, ca, "default", "spiffe://example.com/default")

	clientAuthenticator := func(ctx context.Context) (*slog.Logger, SVIDFetcher, error) {
		return log, func(ctx context.Context, localBundle *spiffebundle.Bundle) ([]*workloadpb.X509SVID, error) {
			return []*workloadpb.X509SVID{
				{
					SpiffeId:    "spiffe://example.com/default",
					X509Svid:    defaultCert,
					X509SvidKey: defaultKey,
					Bundle:      workloadidentity.MarshalX509Bundle(localBundle.X509Bundle()),
				},
			}, nil
		}, nil
	}

	bundle := spiffebundle.New(td)
	bundle.AddX509Authority(ca.Cert)

	federatedBundle := spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com"))
	federatedBundle.AddX509Authority(ca.Cert)

	appClientBundle := spiffebundle.New(spiffeid.RequireTrustDomainFromString(apiworkloadidentity.NewInternalAppTrustDomain("example.com")))
	appClientBundle.AddX509Authority(ca.Cert)

	defaultBundleSet := &workloadidentity.BundleSet{
		Local:     bundle,
		AppClient: appClientBundle,
		Federated: map[string]*spiffebundle.Bundle{
			federatedBundle.TrustDomain().Name(): federatedBundle,
		},
	}
	noAppClientBundleSet := &workloadidentity.BundleSet{
		Local:     bundle,
		AppClient: nil,
		Federated: map[string]*spiffebundle.Bundle{
			federatedBundle.TrustDomain().Name(): federatedBundle,
		},
	}

	expectErrContains := func(msg string) require.ErrorAssertionFunc {
		return func(tt require.TestingT, err error, i ...any) {
			require.ErrorContains(tt, err, msg, i...)
		}
	}

	for name, tc := range map[string]struct {
		bundleSet     *workloadidentity.BundleSet
		trustDomains  bot.TrustDomainsSelector
		resourceNames []string
		expectedErr   require.ErrorAssertionFunc
	}{
		"specific ca app_client without selector errors": {
			resourceNames: []string{appClientBundle.TrustDomain().IDString()},
			expectedErr:   expectErrContains("unknown resource names: [" + appClientBundle.TrustDomain().IDString() + "]"),
		},
		"specific ca app_client with selector": {
			trustDomains:  bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			resourceNames: []string{appClientBundle.TrustDomain().IDString()},
			expectedErr:   require.NoError,
		},
		"special ALL with app_client selector": {
			trustDomains:  bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			resourceNames: []string{EnvoyAllBundlesName},
			expectedErr:   require.NoError,
		},
		"all with app_client selector": {
			trustDomains:  bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			resourceNames: []string{},
			expectedErr:   require.NoError,
		},
		"specific ca app_client with selector but nil app client errors": {
			bundleSet:     noAppClientBundleSet,
			trustDomains:  bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			resourceNames: []string{appClientBundle.TrustDomain().IDString()},
			expectedErr:   expectErrContains("unknown resource names: [" + appClientBundle.TrustDomain().IDString() + "]"),
		},
		"special ALL with selector but nil app client": {
			bundleSet:     noAppClientBundleSet,
			trustDomains:  bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			resourceNames: []string{EnvoyAllBundlesName},
			expectedErr:   require.NoError,
		},
	} {
		t.Run(name, func(t *testing.T) {
			bundleSet := tc.bundleSet
			if bundleSet == nil {
				bundleSet = defaultBundleSet
			}

			sdsHandler, err := NewHandler(HandlerConfig{
				Logger:              log,
				RenewalInterval:     time.Minute,
				TrustBundleCache:    &mockTrustBundleCache{currentBundle: bundleSet},
				ClientAuthenticator: clientAuthenticator,
				TrustDomainSelector: tc.trustDomains,
			})
			require.NoError(t, err)

			req := &discoveryv3pb.DiscoveryRequest{
				TypeUrl:       "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret",
				ResourceNames: tc.resourceNames,
			}

			res, err := sdsHandler.FetchSecrets(ctx, req)
			tc.expectedErr(t, err)
			if err != nil {
				return
			}

			if golden.ShouldSet() {
				resBytes, err := protojson.MarshalOptions{
					Multiline: true,
				}.Marshal(res)
				require.NoError(t, err)
				golden.Set(t, resBytes)
			}

			want := &discoveryv3pb.DiscoveryResponse{}
			require.NoError(t, protojson.UnmarshalOptions{}.Unmarshal(golden.Get(t), want))
			require.Empty(t, cmp.Diff(res, want, protocmp.Transform()))
		})
	}
}

func TestNewTLSV3Certificate_PreservesCertChain(t *testing.T) {
	const spiffeID = "spiffe://example.com/default"

	for intermediateCount := range 3 {
		t.Run(fmt.Sprint(intermediateCount), func(t *testing.T) {
			chainDER, keyDER, root := newSVIDWithChain(t, spiffeID, intermediateCount)

			rootPool := x509.NewCertPool()
			rootPool.AddCert(root)

			rawSecret, err := newTLSV3Certificate(&workloadpb.X509SVID{
				SpiffeId:    spiffeID,
				X509Svid:    chainDER,
				X509SvidKey: keyDER,
			}, "")
			require.NoError(t, err)

			secret := &tlsv3pb.Secret{}
			require.NoError(t, rawSecret.UnmarshalTo(secret))
			require.Equal(t, spiffeID, secret.Name)

			tlsCert := secret.GetTlsCertificate()
			require.NotNil(t, tlsCert)

			intermediatePool := x509.NewCertPool()
			var certs []*x509.Certificate
			chainBytes := tlsCert.GetCertificateChain().GetInlineBytes()
			i := 0
			for {
				var block *pem.Block
				block, chainBytes = pem.Decode(chainBytes)
				if block == nil {
					break
				}

				require.Equal(t, internal.PEMBlockTypeCertificate, block.Type)

				cert, err := x509.ParseCertificate(block.Bytes)
				require.NoError(t, err)

				// Add the cert to the pool, but only after the first (the leaf)
				if i > 0 {
					intermediatePool.AddCert(cert)
				}

				certs = append(certs, cert)
				i++
			}

			// Should be count+1 (the leaf)
			require.Len(t, certs, intermediateCount+1, "full chain should be parsed")
			require.Empty(t, chainBytes, "all chain bytes should be consumed")

			leaf := certs[0]
			require.Equal(t, spiffeID, leaf.URIs[0].String(), "svid leaf should be first")

			_, err = leaf.Verify(x509.VerifyOptions{
				Roots:         rootPool,
				Intermediates: intermediatePool,
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
			})
			require.NoError(t, err, "leaf must verify against full cert chain")
		})
	}
}
