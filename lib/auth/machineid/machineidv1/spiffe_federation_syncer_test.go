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

package machineidv1

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/federation"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func makeTrustDomain(t *testing.T, name string) (spiffeid.TrustDomain, *spiffebundle.Bundle) {
	td := spiffeid.RequireTrustDomainFromString(name)
	bundle := spiffebundle.New(td)
	b, _ := pem.Decode([]byte(fixtures.TLSCACertPEM))
	cert, err := x509.ParseCertificate(b.Bytes)
	require.NoError(t, err)
	bundle.AddX509Authority(cert)
	bundle.SetRefreshHint(time.Minute * 12)
	return td, bundle
}

func TestSPIFFEFederationSyncer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := utils.NewSlogLoggerForTests()
	clock := clockwork.NewRealClock()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	store, err := local.NewSPIFFEFederationService(backend)
	require.NoError(t, err)
	eventsSvc := local.NewEventsService(backend)

	td1, bundle1 := makeTrustDomain(t, "1.example.com")
	marshaledBundle1, err := bundle1.Marshal()
	require.NoError(t, err)
	td2, bundle2 := makeTrustDomain(t, "2.example.com")
	marshaledBundle2, err := bundle1.Marshal()
	require.NoError(t, err)

	// Implement a fake SPIFFE Federation bundle endpoint
	testSrv1 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := federation.NewHandler(td1, bundle1)
		if !assert.NoError(t, err) {
			return
		}
		h.ServeHTTP(w, r)
	}))
	testSrv2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := federation.NewHandler(td2, bundle2)
		if !assert.NoError(t, err) {
			return
		}
		h.ServeHTTP(w, r)
	}))

	caPool := x509.NewCertPool()
	caPool.AddCert(testSrv1.Certificate())
	caPool.AddCert(testSrv2.Certificate())

	// Create one trust domain prior to startng the syncer
	created1, err := store.CreateSPIFFEFederation(ctx, &machineidv1pb.SPIFFEFederation{
		Kind:    types.KindSPIFFEFederation,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "1.example.com",
		},
		Spec: &machineidv1pb.SPIFFEFederationSpec{
			BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
				HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
					BundleEndpointUrl: testSrv1.URL,
				},
			},
		},
	})
	require.NoError(t, err)

	syncer, err := NewSPIFFEFederationSyncer(SPIFFEFederationSyncerConfig{
		Backend:       backend,
		Store:         store,
		EventsWatcher: eventsSvc,
		Clock:         clock,
		Logger:        logger,
		SPIFFEFetchOptions: []federation.FetchOption{
			federation.WithWebPKIRoots(caPool),
		},
	})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		err := syncer.Run(ctx)
		assert.NoError(t, err)
		errCh <- err
	}()

	// Wait for the initially created SPIFFEFederation to be synced
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		got, err := store.GetSPIFFEFederation(ctx, created1.Metadata.Name)
		if !assert.NoError(t, err) {
			return
		}
		// Check that some update as occurred (as indicated by the revision)
		if !assert.NotEqual(t, got.Metadata.Revision, created1.Metadata.Revision) {
			return
		}
		// Check that the expected status fields have been set...
		if !assert.NotNil(t, got.Status) {
			return
		}
		assert.Equal(t, string(marshaledBundle1), got.Status.CurrentBundle)
	}, time.Second*10, time.Millisecond*200)

	// Create a second SPIFFEFederation and wait for it to be synced
	created2, err := store.CreateSPIFFEFederation(ctx, &machineidv1pb.SPIFFEFederation{
		Kind:    types.KindSPIFFEFederation,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "2.example.com",
		},
		Spec: &machineidv1pb.SPIFFEFederationSpec{
			BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
				HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
					BundleEndpointUrl: testSrv2.URL,
				},
			},
		},
	})
	require.NoError(t, err)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		got, err := store.GetSPIFFEFederation(ctx, created2.Metadata.Name)
		if !assert.NoError(t, err) {
			return
		}
		// Check that some update as occurred (as indicated by the revision)
		if !assert.NotEqual(t, got.Metadata.Revision, created2.Metadata.Revision) {
			return
		}
		// Check that the expected status fields have been set...
		if !assert.NotNil(t, got.Status) {
			return
		}
		assert.Equal(t, string(marshaledBundle2), got.Status.CurrentBundle)
	}, time.Second*10, time.Millisecond*200)

	cancel()
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for syncer to stop")
	}
}

func TestSPIFFEFederationSyncer_syncFederation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := utils.NewSlogLoggerForTests()
	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	store, err := local.NewSPIFFEFederationService(backend)
	require.NoError(t, err)
	eventsSvc := local.NewEventsService(backend)

	td, bundle := makeTrustDomain(t, "example.com")
	marshaledBundle, err := bundle.Marshal()
	require.NoError(t, err)

	// Implement a fake SPIFFE Federation bundle endpoint
	testSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, err := federation.NewHandler(td, bundle)
		if !assert.NoError(t, err) {
			return
		}
		h.ServeHTTP(w, r)
	}))
	caPool := x509.NewCertPool()
	caPool.AddCert(testSrv.Certificate())

	syncer, err := NewSPIFFEFederationSyncer(SPIFFEFederationSyncerConfig{
		Backend:       backend,
		Store:         store,
		EventsWatcher: eventsSvc,
		Clock:         clock,
		Logger:        logger,
		SPIFFEFetchOptions: []federation.FetchOption{
			federation.WithWebPKIRoots(caPool),
		},
	})
	require.NoError(t, err)

	t.Run("https-web", func(t *testing.T) {
		in := &machineidv1pb.SPIFFEFederation{
			Kind:    types.KindSPIFFEFederation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "example.com",
			},
			Spec: &machineidv1pb.SPIFFEFederationSpec{
				BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
					HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
						BundleEndpointUrl: testSrv.URL,
					},
				},
			},
		}
		created, err := store.CreateSPIFFEFederation(ctx, in)
		require.NoError(t, err)

		firstSync, err := syncer.syncTrustDomain(ctx, "example.com")
		require.NoError(t, err)
		got, err := store.GetSPIFFEFederation(ctx, "example.com")
		require.NoError(t, err)
		// Require that the persisted resource equals the resource output by syncTrustDomain
		require.Empty(t, cmp.Diff(got, firstSync, protocmp.Transform()))
		// Check that some update as occurred (as indicated by the revision)
		require.NotEqual(t, created.Metadata.Revision, firstSync.Metadata.Revision)
		// Check that the expected status fields have been set...
		require.NotNil(t, firstSync.Status)
		wantStatus := &machineidv1pb.SPIFFEFederationStatus{
			CurrentBundleSyncedAt:   timestamppb.New(clock.Now().UTC()),
			CurrentBundleSyncedFrom: proto.Clone(created).(*machineidv1pb.SPIFFEFederation).Spec.BundleSource,
			NextSyncAt:              timestamppb.New(clock.Now().UTC().Add(time.Minute * 12)),
			CurrentBundle:           string(marshaledBundle),
		}
		require.Empty(t, cmp.Diff(firstSync.Status, wantStatus, protocmp.Transform()))

		// Check that syncing again does nothing.
		secondSync, err := syncer.syncTrustDomain(ctx, "example.com")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(secondSync, firstSync, protocmp.Transform()))

		// Advance the clock and check that the sync is triggered again.
		clock.Advance(time.Minute * 15)
		thirdSync, err := syncer.syncTrustDomain(ctx, "example.com")
		require.NoError(t, err)
		require.NotEqual(t, firstSync.Metadata.Revision, thirdSync.Metadata.Revision)
		wantStatus = &machineidv1pb.SPIFFEFederationStatus{
			CurrentBundleSyncedAt:   timestamppb.New(clock.Now().UTC()),
			CurrentBundleSyncedFrom: proto.Clone(created).(*machineidv1pb.SPIFFEFederation).Spec.BundleSource,
			NextSyncAt:              timestamppb.New(clock.Now().UTC().Add(time.Minute * 12)),
			CurrentBundle:           string(marshaledBundle),
		}
		require.Empty(t, cmp.Diff(thirdSync.Status, wantStatus, protocmp.Transform()))

		// Check that syncing again does nothing.
		fourthSync, err := syncer.syncTrustDomain(ctx, "example.com")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(fourthSync, thirdSync, protocmp.Transform()))

		// Check that modifying the bundle source triggers a sync.
		fourthSync.Spec.BundleSource.HttpsWeb.BundleEndpointUrl = fmt.Sprintf("%s/modified", testSrv.URL)
		updated, err := store.UpdateSPIFFEFederation(ctx, fourthSync)
		require.NoError(t, err)
		fifthSync, err := syncer.syncTrustDomain(ctx, "example.com")
		require.NoError(t, err)
		require.NotEqual(t, updated.Metadata.Revision, fifthSync.Metadata.Revision)
		wantStatus = &machineidv1pb.SPIFFEFederationStatus{
			CurrentBundleSyncedAt:   timestamppb.New(clock.Now().UTC()),
			CurrentBundleSyncedFrom: proto.Clone(updated).(*machineidv1pb.SPIFFEFederation).Spec.BundleSource,
			NextSyncAt:              timestamppb.New(clock.Now().UTC().Add(time.Minute * 12)),
			CurrentBundle:           string(marshaledBundle),
		}
		require.Empty(t, cmp.Diff(fifthSync.Status, wantStatus, protocmp.Transform()))
	})
}
