// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestUnifiedResourceWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	type client struct {
		services.Presence
		services.WindowsDesktops
		services.SAMLIdPServiceProviders
		types.Events
		services.AccessListsGetter
	}

	samlService, err := local.NewSAMLIdPServiceProviderService(bk)
	require.NoError(t, err)

	clt := &client{
		Presence:                local.NewPresenceService(bk),
		WindowsDesktops:         local.NewWindowsDesktopService(bk),
		SAMLIdPServiceProviders: samlService,
		Events:                  local.NewEventsService(bk),
	}
	// Add node to the backend.
	node := newNodeServer(t, "node1", "127.0.0.1:22", false /*tunnel*/)
	_, err = clt.UpsertNode(ctx, node)
	require.NoError(t, err)

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "db1",
	}, types.DatabaseSpecV3{
		Protocol: "test-protocol",
		URI:      "test-uri",
	})
	require.NoError(t, err)
	dbServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "db1-server",
	}, types.DatabaseServerSpecV3{
		Hostname: "db-hostname",
		HostID:   uuid.NewString(),
		Database: db,
	})
	require.NoError(t, err)
	_, err = clt.UpsertDatabaseServer(ctx, dbServer)
	require.NoError(t, err)

	w, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentUnifiedResource,
			Client:    clt,
		},
		ResourceGetter: clt,
	})
	require.NoError(t, err)
	// node and db expected initially
	res, err := w.GetUnifiedResources(ctx)
	require.NoError(t, err)
	require.Len(t, res, 2)

	assert.Eventually(t, func() bool {
		return w.IsInitialized()
	}, 5*time.Second, 10*time.Millisecond, "unified resource watcher never initialized")

	// Add app to the backend.
	app, err := types.NewAppServerV3(
		types.Metadata{Name: "app1"},
		types.AppServerSpecV3{
			HostID: "app1-host-id",
			App:    newApp(t, "app1"),
		},
	)
	require.NoError(t, err)
	_, err = clt.UpsertApplicationServer(ctx, app)
	require.NoError(t, err)

	// Add saml idp service provider to the backend.
	samlapp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newTestEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	)
	require.NoError(t, err)
	err = clt.CreateSAMLIdPServiceProvider(ctx, samlapp)
	require.NoError(t, err)

	win, err := types.NewWindowsDesktopV3(
		"win1",
		nil,
		types.WindowsDesktopSpecV3{Addr: "localhost", HostID: "win1-host-id"},
	)
	require.NoError(t, err)
	err = clt.UpsertWindowsDesktop(ctx, win)
	require.NoError(t, err)

	// we expect each of the resources above to exist
	expectedRes := []types.ResourceWithLabels{node, app, samlapp, dbServer, win}
	assert.Eventually(t, func() bool {
		res, err = w.GetUnifiedResources(ctx)
		return len(res) == len(expectedRes)
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be added")
	assert.Empty(t, cmp.Diff(
		expectedRes,
		res,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))

	// // Update and remove some resources.
	nodeUpdated := newNodeServer(t, "node1", "192.168.0.1:22", false /*tunnel*/)
	_, err = clt.UpsertNode(ctx, nodeUpdated)
	require.NoError(t, err)
	err = clt.DeleteApplicationServer(ctx, defaults.Namespace, "app1-host-id", "app1")
	require.NoError(t, err)

	// this should include the updated node, and shouldn't have any apps included
	expectedRes = []types.ResourceWithLabels{nodeUpdated, samlapp, dbServer, win}
	assert.Eventually(t, func() bool {
		res, err = w.GetUnifiedResources(ctx)
		require.NoError(t, err)
		serverUpdated := slices.ContainsFunc(res, func(r types.ResourceWithLabels) bool {
			node, ok := r.(types.Server)
			return ok && node.GetAddr() == "192.168.0.1:22"
		})
		return len(res) == len(expectedRes) && serverUpdated
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be updated")
	assert.Empty(t, cmp.Diff(
		expectedRes,
		res,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))
}

func newTestEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`
