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
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSPIFFEFederationSyncer_syncFederation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := utils.NewSlogLoggerForTests()
	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	store, err := local.NewSPIFFEFederationService(backend)
	require.NoError(t, err)
	eventsSvc := local.NewEventsService(backend)

	syncer, err := NewSPIFFEFederationSyncer(SPIFFEFederationSyncerConfig{
		Backend:       backend,
		Store:         store,
		EventsWatcher: eventsSvc,
		Clock:         clock,
		Logger:        logger,
	})
	require.NoError(t, err)

	testSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	}))

	in := &machineidv1pb.SPIFFEFederation{
		Kind:    types.KindSPIFFEFederation,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "foo",
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

	// TODO: Add test for static bundle, change and no change
	// TODO: Add test for already synced recently (e.g sync not necessary)

	out, err := syncer.syncFederation(ctx, "foo")
	require.NoError(t, err)
}
