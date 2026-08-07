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
	"crypto/x509"
	"crypto/x509/pkix"
	"os"
	"slices"
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
	apiworkloadidentity "github.com/gravitational/teleport/api/workloadidentity"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestBundleSet_Clone(t *testing.T) {
	t.Parallel()

	// We don't need to test too thoroughly since most of the Clone
	// implementation comes from spiffe-go. We just want to ensure both the
	// local and federated bundles are cloned.

	localTD := spiffeid.RequireTrustDomainFromString("example.com")
	localBundle := spiffebundle.New(localTD)
	appClientTD := spiffeid.RequireTrustDomainFromString(apiworkloadidentity.NewInternalAppTrustDomain("example.com"))
	appClientBundle := spiffebundle.New(appClientTD)

	federatedTD := spiffeid.RequireTrustDomainFromString("federated.example.com")
	federatedBundle := spiffebundle.New(federatedTD)

	bundleSet := BundleSet{
		Local:     localBundle,
		AppClient: appClientBundle,
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": federatedBundle,
		},
	}

	clonedBundleSet := bundleSet.Clone()
	require.True(t, bundleSet.Equal(clonedBundleSet))

	bundleSet.Local.SetSequenceNumber(12)
	bundleSet.AppClient.SetSequenceNumber(21)
	bundleSet.Federated["federated.example.com"].SetSequenceNumber(13)
	require.False(t, bundleSet.Equal(clonedBundleSet))
}

func TestBundleSet_Equal(t *testing.T) {
	t.Parallel()

	bundleSet1 := &BundleSet{
		Local:     spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		AppClient: spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
		},
	}
	bundleSet2 := &BundleSet{
		Local:     spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		AppClient: spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com":  spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
			"federated2.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated2.example.com")),
		},
	}
	bundleSet3 := &BundleSet{
		Local:     spiffebundle.New(spiffeid.RequireTrustDomainFromString("2.example.com")),
		AppClient: spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
		},
	}
	bundleSet4 := &BundleSet{
		Local:     spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		AppClient: spiffebundle.New(spiffeid.RequireTrustDomainFromString("2.example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
		},
	}
	bundleSet5 := &BundleSet{
		Local:     spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		AppClient: nil,
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
		{
			name: "bundle set 1 and 4 not equal",
			a:    bundleSet1,
			b:    bundleSet4,
		},
		{
			name:  "bundle set 5 equal",
			a:     bundleSet5,
			b:     bundleSet5,
			equal: true,
		},
		{
			name: "bundle set 4 and 5 not equal",
			a:    bundleSet4,
			b:    bundleSet5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.equal, tt.a.Equal(tt.b))
		})
	}
}

func TestBundleSet_TrustDomainsBundles(t *testing.T) {
	t.Parallel()

	appClientBundle := makeSPIFFEBundle(t, apiworkloadidentity.NewInternalAppTrustDomain("example.com"))

	expectBundle := func(want *spiffebundle.Bundle) require.ValueAssertionFunc {
		return func(tt require.TestingT, got any, _ ...any) {
			require.Same(tt, want, got)
		}
	}
	expectErrContains := func(msg string) require.ErrorAssertionFunc {
		return func(tt require.TestingT, err error, _ ...any) {
			require.ErrorContains(tt, err, msg)
		}
	}

	type entry struct {
		expectBundle require.ValueAssertionFunc
		expectErr    require.ErrorAssertionFunc
	}

	for name, tc := range map[string]struct {
		bundleSet       *BundleSet
		tds             bot.TrustDomainsSelector
		expectedEntries []entry
	}{
		"nil selector yields nothing": {
			bundleSet:       &BundleSet{AppClient: appClientBundle},
			tds:             nil,
			expectedEntries: nil,
		},
		"empty selector yields nothing": {
			bundleSet:       &BundleSet{AppClient: appClientBundle},
			tds:             bot.TrustDomainsSelector{},
			expectedEntries: nil,
		},
		"app_client yields the app client bundle": {
			bundleSet: &BundleSet{AppClient: appClientBundle},
			tds:       bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expectedEntries: []entry{
				{
					expectBundle: expectBundle(appClientBundle),
					expectErr:    require.NoError,
				},
			},
		},
		"app_client with nil app client yields a NotImplemented error": {
			bundleSet: &BundleSet{AppClient: nil},
			tds:       bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expectedEntries: []entry{
				{
					expectBundle: require.Nil,
					expectErr:    expectErrContains("app client trust domain is not available"),
				},
			},
		},
		"unknown trust domain yields a BadParameter error": {
			bundleSet: &BundleSet{AppClient: appClientBundle},
			tds:       bot.TrustDomainsSelector{bot.TrustDomain("random")},
			expectedEntries: []entry{
				{
					expectBundle: require.Nil,
					expectErr:    expectErrContains(`invalid trust domain selector "random"`),
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			type yielded struct {
				bundle *spiffebundle.Bundle
				err    error
			}
			var got []yielded
			for bundle, err := range tc.bundleSet.InternalTrustDomainsBundles(tc.tds) {
				got = append(got, yielded{bundle: bundle, err: err})
			}

			require.Len(t, got, len(tc.expectedEntries))
			for i, want := range tc.expectedEntries {
				want.expectBundle(t, got[i].bundle, "bundle at index %d", i)
				want.expectErr(t, got[i].err, "error at index %d", i)
			}
		})
	}
}

func TestBundleSet_FederatedAndTrustedDomains(t *testing.T) {
	t.Parallel()

	appClientBundle := makeSPIFFEBundle(t, apiworkloadidentity.NewInternalAppTrustDomain("example.com"))
	federatedA := makeSPIFFEBundle(t, "a.federated.example.com")
	federatedB := makeSPIFFEBundle(t, "b.federated.example.com")

	expectBundles := func(want ...*spiffebundle.Bundle) require.ValueAssertionFunc {
		return func(tt require.TestingT, got any, _ ...any) {
			require.ElementsMatch(tt, want, got)
		}
	}

	for name, tc := range map[string]struct {
		bundleSet *BundleSet
		tds       bot.TrustDomainsSelector
		expect    require.ValueAssertionFunc
	}{
		"no federated and empty selector returns nothing": {
			bundleSet: &BundleSet{
				Local:     makeSPIFFEBundle(t, "example.com"),
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{},
			},
			tds:    bot.TrustDomainsSelector{},
			expect: require.Empty,
		},
		"empty selector returns only federated": {
			bundleSet: &BundleSet{
				Local:     makeSPIFFEBundle(t, "example.com"),
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedA.TrustDomain().Name(): federatedA,
					federatedB.TrustDomain().Name(): federatedB,
				},
			},
			tds:    nil,
			expect: expectBundles(federatedA, federatedB),
		},
		"app_client selector with no federated returns only app client": {
			bundleSet: &BundleSet{
				Local:     makeSPIFFEBundle(t, "example.com"),
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{},
			},
			tds:    bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expect: expectBundles(appClientBundle),
		},
		"app_client selector with federated returns both": {
			bundleSet: &BundleSet{
				Local:     makeSPIFFEBundle(t, "example.com"),
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedA.TrustDomain().Name(): federatedA,
					federatedB.TrustDomain().Name(): federatedB,
				},
			},
			tds:    bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expect: expectBundles(appClientBundle, federatedA, federatedB),
		},
		"unknown trust domain only returns federated": {
			bundleSet: &BundleSet{
				Local:     makeSPIFFEBundle(t, "example.com"),
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedA.TrustDomain().Name(): federatedA,
				},
			},
			tds:    bot.TrustDomainsSelector{bot.TrustDomain("random")},
			expect: expectBundles(federatedA),
		},
		"nil app client is silently skipped, returning federated only": {
			bundleSet: &BundleSet{
				Local:     makeSPIFFEBundle(t, "example.com"),
				AppClient: nil,
				Federated: map[string]*spiffebundle.Bundle{
					federatedA.TrustDomain().Name(): federatedA,
				},
			},
			tds:    bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expect: expectBundles(federatedA),
		},
		"nil app client with no federated returns nothing": {
			bundleSet: &BundleSet{
				Local:     makeSPIFFEBundle(t, "example.com"),
				AppClient: nil,
				Federated: map[string]*spiffebundle.Bundle{},
			},
			tds:    bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expect: require.Empty,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.expect(t, tc.bundleSet.FederatedAndInternalTrustDomains(tc.tds))
		})
	}

	// Given a bundle set with trust domains and federated bundles, trust-domain
	// bundles must return before federated bundles.
	//
	// Some callers (e.g. the SDS handler) rely on iteration order when picking
	// the first matching bundle by trust domain.
	t.Run("trust domains bundles first", func(t *testing.T) {
		appClientBundle := makeSPIFFEBundle(t, apiworkloadidentity.NewInternalAppTrustDomain("example.com"))
		federatedBundle := makeSPIFFEBundle(t, "federated.example.com")
		bs := &BundleSet{
			Local:     makeSPIFFEBundle(t, "example.com"),
			AppClient: appClientBundle,
			Federated: map[string]*spiffebundle.Bundle{
				federatedBundle.TrustDomain().Name(): federatedBundle,
			},
		}

		got := bs.FederatedAndInternalTrustDomains(bot.TrustDomainsSelector{bot.TrustDomainAppClient})
		require.NotEmpty(t, got)
		require.Same(t, appClientBundle, got[0])
	})
}

func TestBundleSet_EncodedX509Bundles(t *testing.T) {
	t.Parallel()

	localBundle := makeSPIFFEBundle(t, "example.com")
	appClientBundle := makeSPIFFEBundle(t, apiworkloadidentity.NewInternalAppTrustDomain("example.com"))
	federatedBundle := makeSPIFFEBundle(t, "federated.example.com")

	marshaledBundles := map[string][]byte{
		localBundle.TrustDomain().IDString():     MarshalX509Bundle(localBundle.X509Bundle()),
		appClientBundle.TrustDomain().IDString(): MarshalX509Bundle(appClientBundle.X509Bundle()),
		federatedBundle.TrustDomain().IDString(): MarshalX509Bundle(federatedBundle.X509Bundle()),
	}

	for name, tc := range map[string]struct {
		bs           *BundleSet
		includeLocal bool
		tds          bot.TrustDomainsSelector
		expectedKeys []string
	}{
		"include local, no trust domains": {

			bs: &BundleSet{
				Local:     localBundle,
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedBundle.TrustDomain().Name(): federatedBundle,
				},
			},
			includeLocal: true,
			tds:          nil,
			expectedKeys: []string{
				localBundle.TrustDomain().IDString(),
				federatedBundle.TrustDomain().IDString(),
			},
		},
		"exclude local, no trust domains": {
			bs: &BundleSet{
				Local:     localBundle,
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedBundle.TrustDomain().Name(): federatedBundle,
				},
			},
			includeLocal: false,
			tds:          nil,
			expectedKeys: []string{federatedBundle.TrustDomain().IDString()},
		},
		"exclude local, app_client only": {
			bs: &BundleSet{
				Local:     localBundle,
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedBundle.TrustDomain().Name(): federatedBundle,
				},
			},
			includeLocal: false,
			tds:          bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expectedKeys: []string{
				appClientBundle.TrustDomain().IDString(),
				federatedBundle.TrustDomain().IDString(),
			},
		},
		"include local and app_client": {
			bs: &BundleSet{
				Local:     localBundle,
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedBundle.TrustDomain().Name(): federatedBundle,
				},
			},
			includeLocal: true,
			tds:          bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expectedKeys: []string{
				localBundle.TrustDomain().IDString(),
				appClientBundle.TrustDomain().IDString(),
				federatedBundle.TrustDomain().IDString(),
			},
		},
		"unknown trust domain entry is skipped": {
			bs: &BundleSet{
				Local:     localBundle,
				AppClient: appClientBundle,
				Federated: map[string]*spiffebundle.Bundle{
					federatedBundle.TrustDomain().Name(): federatedBundle,
				},
			},
			includeLocal: false,
			tds:          bot.TrustDomainsSelector{bot.TrustDomain("random")},
			expectedKeys: []string{federatedBundle.TrustDomain().IDString()},
		},
		"nil app client is silently skipped": {
			bs: &BundleSet{
				Local:     localBundle,
				AppClient: nil,
				Federated: map[string]*spiffebundle.Bundle{
					federatedBundle.TrustDomain().Name(): federatedBundle,
				},
			},
			includeLocal: true,
			tds:          bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expectedKeys: []string{
				localBundle.TrustDomain().IDString(),
				federatedBundle.TrustDomain().IDString(),
			},
		},
		"nil app client with no federated returns only local": {
			bs: &BundleSet{
				Local:     localBundle,
				AppClient: nil,
				Federated: map[string]*spiffebundle.Bundle{},
			},
			includeLocal: true,
			tds:          bot.TrustDomainsSelector{bot.TrustDomainAppClient},
			expectedKeys: []string{localBundle.TrustDomain().IDString()},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.bs.EncodedX509Bundles(tc.includeLocal, tc.tds)

			gotKeys := make([]string, 0, len(got))
			for k := range got {
				gotKeys = append(gotKeys, k)
			}
			require.ElementsMatch(t, tc.expectedKeys, gotKeys)

			for _, k := range tc.expectedKeys {
				require.Equal(t, marshaledBundles[k], got[k], "encoded bytes for %q", k)
			}
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

	logger := logtest.NewLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockWatcher := newMockWatcher()
	mockEventsClient := &mockEventsClient{
		watcher:      &mockWatcher,
		supportedCAs: []string{string(types.SPIFFECA), string(types.AppClientCA)},
	}

	// Initialize CA prior to cache start
	caKey, caCertPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{}, []string{}, time.Hour)
	require.NoError(t, err)
	caCert, err := tlsca.ParseCertificatePEM(caCertPEM)
	require.NoError(t, err)
	jwtCAPublic, jwtCAPrivate, err := testauthority.GenerateJWT()
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

	appClientCAKey, appClientCACertPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{}, []string{}, time.Hour)
	require.NoError(t, err)
	appClientCACert, err := tlsca.ParseCertificatePEM(appClientCACertPEM)
	require.NoError(t, err)
	appClientCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.AppClientCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: appClientCACertPEM,
					Key:  appClientCAKey,
				},
			},
		},
	})
	require.NoError(t, err)

	mockTrustClient := &mockTrustClient{
		t:           t,
		ca:          ca.(*types.CertAuthorityV2),
		appClientCA: appClientCA.(*types.CertAuthorityV2),
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
	// Check the app client bundle
	require.NotNil(t, gotBundleSet.AppClient)
	require.Equal(t, "_teleport_app.example.com", gotBundleSet.AppClient.TrustDomain().Name())
	require.Len(t, gotBundleSet.AppClient.X509Authorities(), 1)
	require.True(t, gotBundleSet.AppClient.X509Authorities()[0].Equal(appClientCACert))
	require.Empty(t, gotBundleSet.AppClient.JWTAuthorities())
	// Check the federated bundle
	gotFederatedBundle, ok := gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))

	// Update each local and app client bundles with a new additional cert. Do
	// assertions of state after each update to avoid bundle set being updated
	// all at once.
	additionalCACert := addCACert(t, ca, mockWatcher.events)
	gotBundleSet = waitUpdatedBundleSet(t, gotBundleSet, cache)
	appClientAdditionalCACert := addCACert(t, appClientCA, mockWatcher.events)
	gotBundleSet = waitUpdatedBundleSet(t, gotBundleSet, cache)

	// Check the local bundle
	require.NotNil(t, gotBundleSet.Local)
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	// Check the app client bundle
	require.NotNil(t, gotBundleSet.AppClient)
	require.Equal(t, "_teleport_app.example.com", gotBundleSet.AppClient.TrustDomain().Name())
	require.Len(t, gotBundleSet.AppClient.X509Authorities(), 2)
	require.True(t, gotBundleSet.AppClient.X509Authorities()[0].Equal(appClientCACert))
	require.True(t, gotBundleSet.AppClient.X509Authorities()[1].Equal(appClientAdditionalCACert))
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
	// Check the app client bundle
	require.NotNil(t, gotBundleSet.AppClient)
	require.Equal(t, "_teleport_app.example.com", gotBundleSet.AppClient.TrustDomain().Name())
	require.Len(t, gotBundleSet.AppClient.X509Authorities(), 2)
	require.True(t, gotBundleSet.AppClient.X509Authorities()[0].Equal(appClientCACert))
	require.True(t, gotBundleSet.AppClient.X509Authorities()[1].Equal(appClientAdditionalCACert))
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
	gotBundleSet = waitUpdatedBundleSet(t, gotBundleSet, cache)
	// Check the local bundle
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	// Check the app client bundle
	require.NotNil(t, gotBundleSet.AppClient)
	require.Equal(t, "_teleport_app.example.com", gotBundleSet.AppClient.TrustDomain().Name())
	require.Len(t, gotBundleSet.AppClient.X509Authorities(), 2)
	require.True(t, gotBundleSet.AppClient.X509Authorities()[0].Equal(appClientCACert))
	require.True(t, gotBundleSet.AppClient.X509Authorities()[1].Equal(appClientAdditionalCACert))
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
	gotBundleSet = waitUpdatedBundleSet(t, gotBundleSet, cache)
	// Check the local bundle
	require.NotNil(t, gotBundleSet.Local)
	require.Equal(t, "example.com", gotBundleSet.Local.TrustDomain().Name())
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[0].Equal(caCert))
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	// Check the app client bundle
	require.NotNil(t, gotBundleSet.AppClient)
	require.Equal(t, "_teleport_app.example.com", gotBundleSet.AppClient.TrustDomain().Name())
	require.Len(t, gotBundleSet.AppClient.X509Authorities(), 2)
	require.True(t, gotBundleSet.AppClient.X509Authorities()[0].Equal(appClientCACert))
	require.True(t, gotBundleSet.AppClient.X509Authorities()[1].Equal(appClientAdditionalCACert))
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

func waitUpdatedBundleSet(t *testing.T, old *BundleSet, cache *TrustBundleCache) *BundleSet {
	t.Helper()

	select {
	case <-old.Stale():
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for bundle set update")
	case <-t.Context().Done():
		t.Fatalf("context canceled waiting for bundle set update")
	}

	// Return the "new" bundle set.
	gotBundleSet, err := cache.GetBundleSet(t.Context())
	require.NoError(t, err)
	require.NotNil(t, gotBundleSet)
	return gotBundleSet
}

func addCACert(t *testing.T, ca types.CertAuthority, events chan types.Event) *x509.Certificate {
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
	select {
	case events <- types.Event{
		Type:     types.OpPut,
		Resource: ca,
	}:
	default:
		t.Fatalf("expected the watcher to consume the events immediately")
	}
	return additionalCACert
}

func TestFetchInitialBundleSet_AppClientNotFound(t *testing.T) {
	t.Parallel()

	caKey, caCertPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{}, []string{}, time.Hour)
	require.NoError(t, err)
	caCert, err := tlsca.ParseCertificatePEM(caCertPEM)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.SPIFFECA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{Cert: caCertPEM, Key: caKey},
			},
		},
	})
	require.NoError(t, err)

	trustClient := &mockTrustClient{
		t:           t,
		ca:          ca.(*types.CertAuthorityV2),
		appClientCA: nil, // triggers a NotFound from the mock
	}
	federationClient := &mockFederationClient{t: t}

	bs, err := FetchInitialBundleSet(
		t.Context(),
		logtest.NewLogger(),
		federationClient,
		trustClient,
		false, // fetchFederatedBundles
		"example.com",
	)
	require.NoError(t, err)
	require.NotNil(t, bs)

	require.NotNil(t, bs.Local)
	require.Equal(t, "example.com", bs.Local.TrustDomain().Name())
	require.Len(t, bs.Local.X509Authorities(), 1)
	require.True(t, bs.Local.X509Authorities()[0].Equal(caCert))

	require.Nil(t, bs.AppClient, "AppClient must be left nil when the auth server has no AppClient CA")

	for _, err := range bs.InternalTrustDomainsBundles(bot.TrustDomainsSelector{bot.TrustDomainAppClient}) {
		require.ErrorContains(t, err, "app client trust domain is not available")
	}
	require.Empty(t, bs.FederatedAndInternalTrustDomains(bot.TrustDomainsSelector{bot.TrustDomainAppClient}))
}

// Given a bundle cache running with a server that doesn't support a newly
// introduced CA, ensure that it will still keep working for supported ones.
func TestTrustBundleCache_Run_WithUnsupportedCAs(t *testing.T) {
	logger := logtest.NewLogger()
	ctx, cancel := context.WithCancel(t.Context())

	mockWatcher := newMockWatcher()
	mockEventsClient := &mockEventsClient{
		watcher: &mockWatcher,
		// Only support SPIFFECA for now.
		supportedCAs: []string{string(types.SPIFFECA)},
	}

	// Initialize CA prior to cache start
	caKey, caCertPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{}, []string{}, time.Hour)
	require.NoError(t, err)
	jwtCAPublic, jwtCAPrivate, err := testauthority.GenerateJWT()
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
		t:           t,
		ca:          ca.(*types.CertAuthorityV2),
		appClientCA: nil,
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

	// Here we don't require the full assertion, just ensure we got values
	// initialized.
	gotBundleSet, err := cache.GetBundleSet(ctx)
	require.NoError(t, err)
	require.NotNil(t, gotBundleSet)
	require.NotNil(t, gotBundleSet.Local)
	gotFederatedBundle, ok := gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))

	// AppClient isn't supported in this test, so the bundle must reflect this.
	require.Nil(t, gotBundleSet.AppClient)

	// Update a supported CA
	additionalCACert := addCACert(t, ca, mockWatcher.events)
	gotBundleSet = waitUpdatedBundleSet(t, gotBundleSet, cache)

	// Asserts only one CA was correctly updated.
	require.NotNil(t, gotBundleSet.Local)
	require.Len(t, gotBundleSet.Local.X509Authorities(), 2)
	require.True(t, gotBundleSet.Local.X509Authorities()[1].Equal(additionalCACert))
	gotFederatedBundle, ok = gotBundleSet.Federated["pre-init-federated.example.com"]
	require.True(t, ok)
	require.True(t, gotFederatedBundle.Equal(preInitFed))

	require.Nil(t, gotBundleSet.AppClient)

	// Wait for the cache to exit.
	cancel()
	<-errCh
}

type mockTrustClient struct {
	trustv1.TrustServiceClient
	ca          *types.CertAuthorityV2
	appClientCA *types.CertAuthorityV2
	t           *testing.T
}

func (m *mockTrustClient) GetCertAuthority(
	ctx context.Context,
	in *trustv1.GetCertAuthorityRequest,
	opts ...grpc.CallOption,
) (*types.CertAuthorityV2, error) {
	if in.IncludeKey {
		return nil, trace.BadParameter("unexpected include key")
	}
	if in.Domain != "example.com" {
		return nil, trace.BadParameter("unexpected domain")
	}
	switch in.Type {
	case string(types.SPIFFECA):
		return m.ca, nil
	case string(types.AppClientCA):
		// A nil appClientCA simulates an older auth server that does not
		// have the AppClient CA registered.
		if m.appClientCA == nil {
			// This error must match the condition on [types.IsUnsupportedAuthorityErr]
			return nil, trace.BadParameter("authority type is not supported")
		}
		return m.appClientCA, nil
	default:
		return nil, trace.BadParameter("unexpected type")
	}
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
	watcher      *mockWatcher
	supportedCAs []string
}

func (c *mockEventsClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	var successfulKinds []types.WatchKind
	for _, kind := range watch.Kinds {
		if kind.Kind != types.KindCertAuthority {
			successfulKinds = append(successfulKinds, kind)
			continue
		}

		for caType := range kind.Filter {
			if !slices.Contains(c.supportedCAs, caType) {
				return nil, trace.BadParameter("ca %q is not supported", caType)
			}
		}

		successfulKinds = append(successfulKinds, kind)
	}

	// This helper expects the watchers has room to receive the event and
	// won't block here.
	c.watcher.events <- types.Event{
		Type:     types.OpInit,
		Resource: types.NewWatchStatus(successfulKinds),
	}

	return c.watcher, nil
}

func newMockWatcher() mockWatcher {
	ch := make(chan types.Event, 1)
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
