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

package workloadidentity

import (
	"context"
	"crypto"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	trustv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestBundleSet_Clone(t *testing.T) {
	t.Parallel()

	// We don't need to test too thoroughly since most of the Clone
	// implementation comes from spiffe-go. We just want to ensure both the
	// local and federated bundles are cloned.

	localTD := spiffeid.RequireTrustDomainFromString("example.com")
	localBundle := spiffebundle.New(localTD)

	federatedTD := spiffeid.RequireTrustDomainFromString("federated.example.com")
	federatedBundle := spiffebundle.New(federatedTD)

	bundleSet := BundleSet{
		Local: localBundle,
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": federatedBundle,
		},
	}

	clonedBundleSet := bundleSet.Clone()
	require.True(t, bundleSet.Equal(clonedBundleSet))

	bundleSet.Local.SetSequenceNumber(12)
	bundleSet.Federated["federated.example.com"].SetSequenceNumber(13)
	require.False(t, bundleSet.Equal(clonedBundleSet))
}

func TestBundleSet_Equal(t *testing.T) {
	t.Parallel()

	bundleSet1 := &BundleSet{
		Local: spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
		},
	}
	bundleSet2 := &BundleSet{
		Local: spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com":  spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
			"federated2.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated2.example.com")),
		},
	}
	bundleSet3 := &BundleSet{
		Local: spiffebundle.New(spiffeid.RequireTrustDomainFromString("2.example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
		},
	}

	// We don't need to test too thoroughly since most of the Equals
	// implementation comes from spiffe-go.
	tests := []struct {
		name  string
		a     *BundleSet
		b     *BundleSet
		equal bool
	}{
		{
			name:  "bundle set 1 equal",
			a:     bundleSet1,
			b:     bundleSet1,
			equal: true,
		},
		{
			name:  "bundle set 2 equal",
			a:     bundleSet2,
			b:     bundleSet2,
			equal: true,
		},
		{
			name:  "bundle set 3 equal",
			a:     bundleSet3,
			b:     bundleSet3,
			equal: true,
		},
		{
			name: "bundle set 1 and 2 not equal",
			a:    bundleSet1,
			b:    bundleSet2,
		},
		{
			name: "bundle set 1 and 3 not equal",
			a:    bundleSet1,
			b:    bundleSet3,
		},
		{
			name: "bundle set 2 and 3 not equal",
			a:    bundleSet2,
			b:    bundleSet3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.equal, tt.a.Equal(tt.b))
		})
	}
}

func makeSPIFFEBundle(t *testing.T, td string) *spiffebundle.Bundle {
	bundle := spiffebundle.New(spiffeid.RequireTrustDomainFromString(td))

	_, certPem, err := tlsca.GenerateSelfSignedCA(pkix.Name{}, []string{}, time.Hour)
	require.NoError(t, err)

	ca, err := tlsca.ParseCertificatePEM(certPem)
	require.NoError(t, err)
	bundle.AddX509Authority(ca)
	return bundle
}

func TestTrustBundleCache_Run(t *testing.T) {
	t.Parallel()

	logger := utils.NewSlogLoggerForTests()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockWatcher := newMockWatcher([]types.WatchKind{
		{
			Kind: types.KindCertAuthority,
		},
		{
			Kind: types.KindSPIFFEFederation,
		},
	})
	mockEventsClient := &mockEventsClient{
		watcher: &mockWatcher,
	}

	// Initialize CA prior to cache start
	caKey, caCertPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{}, []string{}, time.Hour)
	require.NoError(t, err)
	caCert, err := tlsca.ParseCertificatePEM(caCertPEM)
	require.NoError(t, err)
	jwtCAPublic, jwtCAPrivate, err := testauthority.New().GenerateJWT()
	require.NoError(t, err)
	jwtCA, err := keys.ParsePublicKey(jwtCAPublic)
	require.NoError(t, err)
	jwtCAKID, err := jwt.KeyID(jwtCA)
	require.NoError(t, err)
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.SPIFFECA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: caCertPEM,
					Key:  caKey,
				},
			},
			JWT: []*types.JWTKeyPair{
				{
					PublicKey:  jwtCAPublic,
					PrivateKey: jwtCAPrivate,
				},
			},
		},
	})
	require.NoError(t, err)
	mockTrustClient := &mockTrustClient{
		t:  t,
		ca: ca.(*types.CertAuthorityV2),
	}
	// Initialize one SPIFFEFederation prior to cache start
	preInitFed := makeSPIFFEBundle(t, "pre-init-federated.example.com")
	marshaledPreInitFed, err := preInitFed.Marshal()
	require.NoError(t, err)
	mockFederationClient := &mockFederationClient{
		t: t,
		spiffeFed: &machineidv1pb.SPIFFEFederation{
			Metadata: &v1.Metadata{
				Name: preInitFed.TrustDomain().Name(),
			},
			Status: &machineidv1pb.SPIFFEFederationStatus{
				CurrentBundle: string(marshaledPreInitFed),
			},
		},
	}

	cache, err := NewTrustBundleCache(TrustBundleCacheConfig{
		EventsClient:     mockEventsClient,
		TrustClient:      mockTrustClient,
		FederationClient: mockFederationClient,
		Logger:           logger,
		ClusterName:      "example.com",
	})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		err := cache.Run(ctx)
		assert.NoError(t, err)
		errCh <- err
	}()

	gotBundleSet, err := cache.GetBundleSet(ctx)
	require.NoError(t, err)
	require.NotNil(t, gotBundleSet)
	// Check the local bundle
	require.NotNil(t, gotBundleSet.Local)
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 1)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.Len(t, gotBundleSet.Local.JWTAuthorities(), 1)
	gotBundleJWTKey, ok := gotBundleSet.Local.FindJWTAuthority(jwtCAKID)
	require.True(t, ok, "public key not found in bundle")
	require.True(t, gotBundleJWTKey.(interface{ Equal(x crypto.PublicKey) bool }).Equal(jwtCA), "public keys do not match")
	// Check the federated bundle
	gotFederatedBundle, ok := gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))

	// Update the local bundle with a new additional cert
	ca = ca.Clone()
	additionalCAKey, additionalCACertPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{}, []string{}, time.Hour)
	require.NoError(t, err)
	additionalCACert, err := tlsca.ParseCertificatePEM(additionalCACertPEM)
	require.NoError(t, err)
	err = ca.SetAdditionalTrustedKeys(types.CAKeySet{
		TLS: []*types.TLSKeyPair{
			{
				Cert: additionalCACertPEM,
				Key:  additionalCAKey,
			},
		},
	})
	require.NoError(t, err)
	mockWatcher.events <- types.Event{
		Type:     types.OpPut,
		Resource: ca,
	}
	// Check we receive a bundle with the updated local CA
	select {
	case <-gotBundleSet.Stale():
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for bundle set update")
	case <-ctx.Done():
		t.Fatalf("context canceled waiting for bundle set update")
	}
	gotBundleSet, err = cache.GetBundleSet(ctx)
	require.NoError(t, err)
	require.NotNil(t, gotBundleSet)
	// Check the local bundle
	require.NotNil(t, gotBundleSet.Local)
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	// Check the federated bundle
	gotFederatedBundle, ok = gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))

	// Update the federated bundle with a new cert
	preInitFed = makeSPIFFEBundle(t, "pre-init-federated.example.com")
	marshaledPreInitFed, err = preInitFed.Marshal()
	require.NoError(t, err)
	mockWatcher.events <- types.Event{
		Type: types.OpPut,
		Resource: types.Resource153ToLegacy(&machineidv1pb.SPIFFEFederation{
			Kind: types.KindSPIFFEFederation,
			Metadata: &v1.Metadata{
				Name: preInitFed.TrustDomain().Name(),
			},
			Status: &machineidv1pb.SPIFFEFederationStatus{
				CurrentBundle: string(marshaledPreInitFed),
			},
		}),
	}
	// Check we receive a bundle with the updated federated bundle
	select {
	case <-gotBundleSet.Stale():
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for bundle set update")
	case <-ctx.Done():
		t.Fatalf("context canceled waiting for bundle set update")
	}
	gotBundleSet, err = cache.GetBundleSet(ctx)
	require.NoError(t, err)
	require.NotNil(t, gotBundleSet)
	// Check the local bundle
	require.NotNil(t, gotBundleSet.Local)
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	// Check the federated bundle
	gotFederatedBundle, ok = gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))

	// Add a new federated bundle
	afterInitFed := makeSPIFFEBundle(t, "after-init-federated.example.com")
	marshaledAfterInitFed, err := afterInitFed.Marshal()
	require.NoError(t, err)
	mockWatcher.events <- types.Event{
		Type: types.OpPut,
		Resource: types.Resource153ToLegacy(&machineidv1pb.SPIFFEFederation{
			Kind: types.KindSPIFFEFederation,
			Metadata: &v1.Metadata{
				Name: afterInitFed.TrustDomain().Name(),
			},
			Status: &machineidv1pb.SPIFFEFederationStatus{
				CurrentBundle: string(marshaledAfterInitFed),
			},
		}),
	}
	// Check we receive a bundle with the new federated bundle
	select {
	case <-gotBundleSet.Stale():
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for bundle set update")
	case <-ctx.Done():
		t.Fatalf("context canceled waiting for bundle set update")
	}
	gotBundleSet, err = cache.GetBundleSet(ctx)
	require.NoError(t, err)
	require.NotNil(t, gotBundleSet)
	// Check the local bundle
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	// Check the pre-init federated bundle
	gotFederatedBundle, ok = gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))
	// Check the after-init federated bundle
	gotFederatedBundle, ok = gotBundleSet.Federated["after-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(afterInitFed))

	// Delete the newly added federated bundle
	mockWatcher.events <- types.Event{
		Type: types.OpDelete,
		Resource: types.Resource153ToLegacy(&machineidv1pb.SPIFFEFederation{
			Kind: types.KindSPIFFEFederation,
			Metadata: &v1.Metadata{
				Name: afterInitFed.TrustDomain().Name(),
			},
		}),
	}
	// Check we receive a bundle with the new federated bundle deleted
	select {
	case <-gotBundleSet.Stale():
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for bundle set update")
	case <-ctx.Done():
		t.Fatalf("context canceled waiting for bundle set update")
	}
	gotBundleSet, err = cache.GetBundleSet(ctx)
	require.NoError(t, err)
	require.NotNil(t, gotBundleSet)
	// Check the local bundle
	require.NotNil(t, gotBundleSet.Local)
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	// Check the pre-init federated bundle
	gotFederatedBundle, ok = gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))
	// Check the after-init federated bundle is gone
	_, ok = gotBundleSet.Federated["after-init-federated.example.com"]
	require.False(t, ok)

	// Wait for the cache to exit.
	cancel()
	<-errCh
}

type mockTrustClient struct {
	trustv1.TrustServiceClient
	ca *types.CertAuthorityV2
	t  *testing.T
}

func (m *mockTrustClient) GetCertAuthority(
	ctx context.Context,
	in *trustv1.GetCertAuthorityRequest,
	opts ...grpc.CallOption,
) (*types.CertAuthorityV2, error) {
	if in.IncludeKey {
		return nil, trace.BadParameter("unexpected include key")
	}
	if in.Type != string(types.SPIFFECA) {
		return nil, trace.BadParameter("unexpected type")
	}
	if in.Domain != "example.com" {
		return nil, trace.BadParameter("unexpected domain")
	}
	return m.ca, nil
}

type mockFederationClient struct {
	machineidv1pb.SPIFFEFederationServiceClient
	t         *testing.T
	spiffeFed *machineidv1pb.SPIFFEFederation
}

func (m *mockFederationClient) ListSPIFFEFederations(
	ctx context.Context,
	in *machineidv1pb.ListSPIFFEFederationsRequest,
	opts ...grpc.CallOption,
) (*machineidv1pb.ListSPIFFEFederationsResponse, error) {
	return &machineidv1pb.ListSPIFFEFederationsResponse{
		SpiffeFederations: []*machineidv1pb.SPIFFEFederation{
			m.spiffeFed,
		},
	}, nil
}

type mockEventsClient struct {
	watcher *mockWatcher
}

func (c *mockEventsClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return c.watcher, nil
}

func newMockWatcher(kinds []types.WatchKind) mockWatcher {
	ch := make(chan types.Event, 1)

	ch <- types.Event{
		Type:     types.OpInit,
		Resource: types.NewWatchStatus(kinds),
	}

	return mockWatcher{events: ch}
}

type mockWatcher struct {
	events chan types.Event
}

// Events returns a stream of events.
func (w mockWatcher) Events() <-chan types.Event {
	return w.events
}

// Done returns a completion channel.
func (w mockWatcher) Done() <-chan struct{} {
	return nil
}

// Close sends a termination signal to watcher.
func (w mockWatcher) Close() error {
	return nil
}

// Error returns a watcher error.
func (w mockWatcher) Error() error {
	return nil
}
