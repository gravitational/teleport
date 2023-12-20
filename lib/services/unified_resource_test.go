/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
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
	node := newNodeServer(t, "node1", "hostname1", "127.0.0.1:22", false /*tunnel*/)
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))

	// // Update and remove some resources.
	nodeUpdated := newNodeServer(t, "node1", "hostname1", "192.168.0.1:22", false /*tunnel*/)
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))
}

func TestUnifiedResourceWatcher_PreventDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	type client struct {
		services.Presence
		services.WindowsDesktops
		services.SAMLIdPServiceProviders
		types.Events
	}

	samlService, err := local.NewSAMLIdPServiceProviderService(bk)
	require.NoError(t, err)

	clt := &client{
		Presence:                local.NewPresenceService(bk),
		WindowsDesktops:         local.NewWindowsDesktopService(bk),
		SAMLIdPServiceProviders: samlService,
		Events:                  local.NewEventsService(bk),
	}
	w, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentUnifiedResource,
			Client:    clt,
		},
		ResourceGetter: clt,
	})
	require.NoError(t, err)

	// add a node
	node := newNodeServer(t, "node1", "hostname1", "127.0.0.1:22", false /*tunnel*/)
	_, err = clt.UpsertNode(ctx, node)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		res, _ := w.GetUnifiedResources(ctx)
		return len(res) == 1
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be added")

	// update a node
	updatedNode := newNodeServer(t, "node1", "hostname2", "127.0.0.1:22", false /*tunnel*/)
	_, err = clt.UpsertNode(ctx, updatedNode)
	require.NoError(t, err)

	// only one resource should still exists with the name "node1" (with hostname updated)
	assert.Eventually(t, func() bool {
		res, _ := w.GetUnifiedResources(ctx)
		return len(res) == 1
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be added")

}

func TestUnifiedResourceWatcher_DeleteEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	type client struct {
		services.Presence
		services.WindowsDesktops
		services.SAMLIdPServiceProviders
		types.Events
	}

	samlService, err := local.NewSAMLIdPServiceProviderService(bk)
	require.NoError(t, err)

	clt := &client{
		Presence:                local.NewPresenceService(bk),
		WindowsDesktops:         local.NewWindowsDesktopService(bk),
		SAMLIdPServiceProviders: samlService,
		Events:                  local.NewEventsService(bk),
	}
	w, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentUnifiedResource,
			Client:    clt,
		},
		ResourceGetter: clt,
	})
	require.NoError(t, err)

	// add a node
	node := newNodeServer(t, "node1", "hostname1", "127.0.0.1:22", false /*tunnel*/)
	_, err = clt.UpsertNode(ctx, node)
	require.NoError(t, err)

	// add a database server
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

	// add a saml app
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

	// Add an app server
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

	// add desktop
	desktop, err := types.NewWindowsDesktopV3(
		"desktop",
		map[string]string{"label": string(make([]byte, 0))},
		types.WindowsDesktopSpecV3{
			Addr:   "addr",
			HostID: "HostID",
		})
	require.NoError(t, err)
	err = clt.UpsertWindowsDesktop(ctx, desktop)
	require.NoError(t, err)

	// add kube
	kube, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name:      "kube",
			Namespace: apidefaults.Namespace,
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3(
		types.Metadata{
			Name:      "kube_server",
			Namespace: apidefaults.Namespace,
		},
		types.KubernetesServerSpecV3{
			Cluster: kube,
			HostID:  "hostID",
		},
	)
	require.NoError(t, err)
	_, err = clt.UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)
	assert.Eventually(t, func() bool {
		res, _ := w.GetUnifiedResources(ctx)
		return len(res) == 6
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be added")

	// delete everything
	err = clt.DeleteNode(ctx, "default", node.GetName())
	require.NoError(t, err)
	err = clt.DeleteDatabaseServer(ctx, "default", dbServer.Spec.HostID, dbServer.GetName())
	require.NoError(t, err)
	err = clt.DeleteSAMLIdPServiceProvider(ctx, samlapp.GetName())
	require.NoError(t, err)
	err = clt.DeleteApplicationServer(ctx, "default", app.Spec.HostID, app.GetName())
	require.NoError(t, err)
	err = clt.DeleteWindowsDesktop(ctx, desktop.Spec.HostID, desktop.GetName())
	require.NoError(t, err)
	err = clt.DeleteKubernetesServer(ctx, kubeServer.Spec.HostID, kubeServer.GetName())
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		res, _ := w.GetUnifiedResources(ctx)
		return len(res) == 0
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be deleted")
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
