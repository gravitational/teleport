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
	"context"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"testing"
	"time"

	discoveryv3pb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	workloadpb "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

type mockTrustBundleCache struct {
	currentBundle *workloadidentity.BundleSet
}

func (m *mockTrustBundleCache) GetBundleSet(ctx context.Context) (*workloadidentity.BundleSet, error) {
	return m.currentBundle, nil
}

// TestSDS_FetchSecrets performs a unit-test over the FetchSecrets method.
// It tests the generation of the DiscoveryResponses and that authentication
// is enforced.
func TestSDS_FetchSecrets(t *testing.T) {
	log := logtest.NewLogger()
	ctx := context.Background()

	td, err := spiffeid.TrustDomainFromString("example.com")
	require.NoError(t, err)

	b, _ := pem.Decode([]byte(fixtures.TLSCACertPEM))
	require.NotNil(t, b, "Decode failed")
	ca, err := x509.ParseCertificate(b.Bytes)
	require.NoError(t, err)

	clientAuthenticator := func(ctx context.Context) (*slog.Logger, SVIDFetcher, error) {
		return log, func(ctx context.Context, localBundle *spiffebundle.Bundle) ([]*workloadpb.X509SVID, error) {
			return []*workloadpb.X509SVID{
				{
					SpiffeId:    "spiffe://example.com/default",
					X509Svid:    []byte("CERT-spiffe://example.com/default"),
					X509SvidKey: []byte("KEY-spiffe://example.com/default"),
					Bundle:      workloadidentity.MarshalX509Bundle(localBundle.X509Bundle()),
				},
				{
					SpiffeId:    "spiffe://example.com/second",
					X509Svid:    []byte("CERT-spiffe://example.com/second"),
					X509SvidKey: []byte("KEY-spiffe://example.com/second"),
					Bundle:      workloadidentity.MarshalX509Bundle(localBundle.X509Bundle()),
				},
			}, nil
		}, nil
	}

	bundle := spiffebundle.New(td)
	bundle.AddX509Authority(ca)

	federatedBundle := spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com"))
	federatedBundle.AddX509Authority(ca)

	mockBundleCache := &mockTrustBundleCache{
		currentBundle: &workloadidentity.BundleSet{
			Local: bundle,
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
