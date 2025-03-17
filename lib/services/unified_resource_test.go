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
	"iter"
	"maps"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	apimetadata "github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type client struct {
	bk backend.Backend
	services.Presence
	services.WindowsDesktops
	services.SAMLIdPServiceProviders
	services.GitServers
	services.IdentityCenterAccounts
	services.IdentityCenterAccountAssignments
	types.Events
}

func newClient(t *testing.T) *client {
	t.Helper()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	samlService, err := local.NewSAMLIdPServiceProviderService(bk)
	require.NoError(t, err)
	gitService, err := local.NewGitServerService(bk)
	require.NoError(t, err)
	icService, err := local.NewIdentityCenterService(local.IdentityCenterServiceConfig{
		Backend: bk,
	})
	require.NoError(t, err)

	return &client{
		bk:                               bk,
		Presence:                         local.NewPresenceService(bk),
		WindowsDesktops:                  local.NewWindowsDesktopService(bk),
		SAMLIdPServiceProviders:          samlService,
		Events:                           local.NewEventsService(bk),
		GitServers:                       gitService,
		IdentityCenterAccounts:           icService,
		IdentityCenterAccountAssignments: icService,
	}
}

func TestUnifiedResourceWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clt := newClient(t)

	// Add node to the backend.
	node := newNodeServer(t, "node1", "hostname1", "127.0.0.1:22", false /*tunnel*/)
	_, err := clt.UpsertNode(ctx, node)
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
	gitServer := newGitServer(t, "my-org")
	_, err = clt.CreateGitServer(ctx, gitServer)
	require.NoError(t, err)

	w, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentUnifiedResource,
			Client:    clt,
		},
		ResourceGetter: clt,
	})
	require.NoError(t, err)
	// node, db, and git_server expected initially
	res, err := w.GetUnifiedResources(ctx)
	require.NoError(t, err)
	require.Len(t, res, 3)

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

	// add another git server
	gitServer2 := newGitServer(t, "my-org-2")
	_, err = clt.UpsertGitServer(ctx, gitServer2)
	require.NoError(t, err)

	icAcct := newICAccount(t, ctx, clt)
	icAcctAssignment := newICAccountAssignment(t, ctx, clt)

	// we expect each of the resources above to exist
	expectedRes := []types.ResourceWithLabels{node, app, samlapp, dbServer, win,
		gitServer, gitServer2,
		types.Resource153ToUnifiedResource(icAcct),
		types.Resource153ToUnifiedResource(icAcctAssignment),
	}
	assert.Eventually(t, func() bool {
		res, err = w.GetUnifiedResources(ctx)
		return len(res) == len(expectedRes)
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be added")
	assert.Empty(t, cmp.Diff(
		expectedRes,
		res,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),

		// Allow comparison of the wrapped resource inside a resource153ToLegacyAdapter
		cmp.Transformer("Unwrap",
			func(t types.Resource153Unwrapper) types.Resource153 {
				return t.Unwrap()
			}),

		// Ignore unexported values in RFD153-style resources
		cmpopts.IgnoreUnexported(
			headerv1.Metadata{},
			identitycenterv1.Account{},
			identitycenterv1.AccountSpec{},
			identitycenterv1.PermissionSetInfo{},
			identitycenterv1.AccountAssignment{},
			identitycenterv1.AccountAssignmentSpec{}),
	))

	// // Update and remove some resources.
	nodeUpdated := newNodeServer(t, "node1", "hostname1", "192.168.0.1:22", false /*tunnel*/)
	_, err = clt.UpsertNode(ctx, nodeUpdated)
	require.NoError(t, err)
	err = clt.DeleteApplicationServer(ctx, defaults.Namespace, "app1-host-id", "app1")
	require.NoError(t, err)

	// this should include the updated node, and shouldn't have any apps included
	expectedRes = []types.ResourceWithLabels{nodeUpdated, samlapp, dbServer, win,
		gitServer, gitServer2,
		types.Resource153ToUnifiedResource(icAcct),
		types.Resource153ToUnifiedResource(icAcctAssignment)}

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
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),

		// Allow comparison of the wrapped values inside a Resource153ToLegacyAdapter
		cmp.Transformer("Unwrap",
			func(t types.Resource153Unwrapper) types.Resource153 {
				return t.Unwrap()
			}),

		// Ignore unexported values in RFD153-style resources
		cmpopts.IgnoreUnexported(
			headerv1.Metadata{},
			identitycenterv1.Account{},
			identitycenterv1.AccountSpec{},
			identitycenterv1.PermissionSetInfo{},
			identitycenterv1.AccountAssignment{},
			identitycenterv1.AccountAssignmentSpec{}),

		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))
}

func TestUnifiedResourceCacheIterateResources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clt := newClient(t)

	node := newNodeServer(t, "node1", "hostname1", "127.0.0.1:22", false /*tunnel*/)
	_, err := clt.UpsertNode(ctx, node)
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
	gitServer := newGitServer(t, "my-org")
	_, err = clt.CreateGitServer(ctx, gitServer)
	require.NoError(t, err)

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

	kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{Name: "kube-cluster"}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "foo", "1")
	require.NoError(t, err)
	_, err = clt.UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	icAcct := newICAccount(t, ctx, clt)
	icAcctAssignment := newICAccountAssignment(t, ctx, clt)

	w, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentUnifiedResource,
			Client:    clt,
		},
		ResourceGetter: clt,
	})
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return w.IsInitialized()
	}, 5*time.Second, 10*time.Millisecond, "unified resource watcher never initialized")

	compareResourceOpts := []cmp.Option{cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),

		// Allow comparison of the wrapped values inside a Resource153ToLegacyAdapter
		cmp.Transformer("Unwrap",
			func(t types.Resource153Unwrapper) types.Resource153 {
				return t.Unwrap()
			}),

		// Ignore unexported values in RFD153-style resources
		cmpopts.IgnoreUnexported(
			headerv1.Metadata{},
			identitycenterv1.Account{},
			identitycenterv1.AccountSpec{},
			identitycenterv1.PermissionSetInfo{},
			identitycenterv1.AccountAssignment{},
			identitycenterv1.AccountAssignmentSpec{}),

		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	}

	expected := map[string]types.ResourceWithLabels{
		types.KindApp:                             app,
		types.KindDatabase:                        dbServer,
		types.KindNode:                            node,
		types.KindWindowsDesktop:                  win,
		types.KindKubernetesCluster:               kubeServer,
		types.KindSAMLIdPServiceProvider:          samlapp,
		types.KindGitServer:                       gitServer,
		types.KindIdentityCenterAccount:           types.Resource153ToUnifiedResource(icAcct),
		types.KindIdentityCenterAccountAssignment: types.Resource153ToUnifiedResource(icAcctAssignment),
	}

	for r, err := range w.Resources(ctx, "", types.SortBy{Field: services.SortByKind}) {
		require.NoError(t, err)

		kind := r.GetKind()
		switch kind {
		case types.KindAppServer:
			kind = types.KindApp
		case types.KindDatabaseServer:
			kind = types.KindDatabase
		case types.KindKubeServer:
			kind = types.KindKubernetesCluster
		}

		expectedResource, ok := expected[kind]
		require.True(t, ok, "resource not expected %v", r)

		assert.Empty(t, cmp.Diff(
			expectedResource,
			r,
			compareResourceOpts...,
		))
	}

	for len(expected) > 0 {
		count := 0
		kinds := slices.Collect(maps.Keys(expected))
		for r, err := range w.Resources(ctx, "", types.SortBy{Field: services.SortByKind}, kinds...) {
			require.NoError(t, err)

			kind := r.GetKind()
			switch kind {
			case types.KindAppServer:
				kind = types.KindApp
			case types.KindDatabaseServer:
				kind = types.KindDatabase
			case types.KindKubeServer:
				kind = types.KindKubernetesCluster
			}

			expectedResource, ok := expected[kind]
			require.True(t, ok, "resource not expected %v", r)

			if count == 0 {
				delete(expected, kind)
			}

			assert.Empty(t, cmp.Diff(
				expectedResource,
				r,
				compareResourceOpts...,
			))
			count++
		}
		assert.Equal(t, len(kinds), count)
	}
}

func TestUnifiedResourceCacheIteration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	const resourceCount = 1234
	ids := make([]string, 0, resourceCount)
	for i := 0; i < resourceCount; i++ {
		ids = append(ids, "resource"+strconv.Itoa(i))
	}

	slices.Sort(ids)

	type GetNamer interface {
		GetName() string
	}

	tests := []struct {
		name             string
		createResource   func(name string, c *client) error
		iterateResources func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error]
	}{
		{
			name: "nodes",
			createResource: func(name string, c *client) error {
				node := newNodeServer(t, name, "hostname1", "127.0.0.1:22", false)
				_, err := c.UpsertNode(ctx, node)
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.Nodes(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(n, nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "databases",
			createResource: func(name string, c *client) error {
				db, err := types.NewDatabaseV3(types.Metadata{
					Name: name,
				}, types.DatabaseSpecV3{
					Protocol: "test-protocol",
					URI:      "test-uri",
				})
				if err != nil {
					return err
				}
				dbServer, err := types.NewDatabaseServerV3(types.Metadata{
					Name: name,
				}, types.DatabaseServerSpecV3{
					Hostname: "hostname:" + name,
					HostID:   uuid.NewString(),
					Database: db,
				})
				if err != nil {
					return err
				}
				_, err = c.UpsertDatabaseServer(ctx, dbServer)
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.DatabaseServers(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(n, nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "apps",
			createResource: func(name string, c *client) error {
				app, err := types.NewAppServerV3(
					types.Metadata{Name: name},
					types.AppServerSpecV3{
						HostID: "app1-host-id",
						App:    newApp(t, name),
					},
				)
				if err != nil {
					return err
				}
				_, err = c.UpsertApplicationServer(ctx, app)
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.AppServers(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(n, nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "kubernetes",
			createResource: func(name string, c *client) error {
				kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{Name: name}, types.KubernetesClusterSpecV3{})
				if err != nil {
					return err
				}
				kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, name, "1")
				if err != nil {
					return err
				}
				_, err = c.UpsertKubernetesServer(ctx, kubeServer)
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.KubernetesServers(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(n, nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "git",
			createResource: func(name string, c *client) error {
				gitServer, err := types.NewGitHubServerWithName(name, types.GitHubServerMetadata{
					Organization: name,
					Integration:  name,
				})
				if err != nil {
					return err
				}

				_, err = c.CreateGitServer(ctx, gitServer)
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.GitServers(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(n, nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "desktops",
			createResource: func(name string, c *client) error {
				win, err := types.NewWindowsDesktopV3(
					name,
					nil,
					types.WindowsDesktopSpecV3{Addr: "localhost", HostID: "win1-host-id"},
				)
				if err != nil {
					return err
				}
				err = c.UpsertWindowsDesktop(ctx, win)
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.WindowsDesktops(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(n, nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "saml",
			createResource: func(name string, c *client) error {
				sp, err := types.NewSAMLIdPServiceProvider(
					types.Metadata{
						Name: name,
					},
					types.SAMLIdPServiceProviderSpecV1{
						EntityDescriptor: newTestEntityDescriptor(name),
						EntityID:         name,
					},
				)
				if err != nil {
					return err
				}

				// Items are manually inserted into the backend to avoid
				// the penalty associated ensuring entity ids are unique
				// in CreateSAMLIdPServiceProvider.
				raw, err := services.MarshalSAMLIdPServiceProvider(sp)
				if err != nil {
					return err
				}

				_, err = c.bk.Create(ctx, backend.Item{
					Key:      backend.NewKey("saml_idp_service_provider", sp.GetName()),
					Value:    raw,
					Revision: sp.GetRevision(),
				})
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.SAMLIdPServiceProviders(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(n, nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "identity center account",
			createResource: func(name string, c *client) error {
				_, err := c.CreateIdentityCenterAccount(ctx, services.IdentityCenterAccount{
					Account: &identitycenterv1.Account{
						Kind:    types.KindIdentityCenterAccount,
						Version: types.V1,
						Metadata: &headerv1.Metadata{
							Name: name,
							Labels: map[string]string{
								types.OriginLabel: common.OriginAWSIdentityCenter,
							},
						},
						Spec: &identitycenterv1.AccountSpec{
							Id:          name,
							Arn:         "arn:aws:sso:::account/" + name,
							Name:        "Test AWS Account",
							Description: "Used for testing",
							PermissionSetInfo: []*identitycenterv1.PermissionSetInfo{
								{
									Name: "Alpha",
									Arn:  "arn:aws:sso:::permissionSet/ssoins-1234567890/ps-alpha",
								},
								{
									Name: "Beta",
									Arn:  "arn:aws:sso:::permissionSet/ssoins-1234567890/ps-beta",
								},
							},
						},
					}})
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.IdentityCenterAccounts(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(types.Resource153ToResourceWithLabels(n), nil) {
							return
						}
					}
				}
			},
		},
		{
			name: "identity center account assignment",
			createResource: func(name string, c *client) error {
				_, err := c.CreateAccountAssignment(ctx, services.IdentityCenterAccountAssignment{
					AccountAssignment: &identitycenterv1.AccountAssignment{
						Kind:    types.KindIdentityCenterAccountAssignment,
						Version: types.V1,
						Metadata: &headerv1.Metadata{
							Name: name,
							Labels: map[string]string{
								types.OriginLabel: common.OriginAWSIdentityCenter,
							},
						},
						Spec: &identitycenterv1.AccountAssignmentSpec{
							Display: "Admin access on Production",
							PermissionSet: &identitycenterv1.PermissionSetInfo{
								Arn:          "arn:aws::::ps-Admin",
								Name:         "Admin",
								AssignmentId: "production--admin",
							},
							AccountName: "Production",
							AccountId:   "99999999",
						},
					}})
				return err
			},
			iterateResources: func(urc *services.UnifiedResourceCache) iter.Seq2[GetNamer, error] {
				return func(yield func(GetNamer, error) bool) {
					for n, err := range urc.IdentityCenterAccountAssignments(ctx, services.UnifiedResourcesIterateParams{}) {
						if err != nil {
							yield(nil, err)
							return
						}

						if !yield(types.Resource153ToResourceWithLabels(n), nil) {
							return
						}
					}
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clt := newClient(t)

			w, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
				ResourceWatcherConfig: services.ResourceWatcherConfig{
					Component: teleport.ComponentUnifiedResource,
					Client:    clt,
				},
				ResourceGetter: clt,
			})
			require.NoError(t, err)

			for i := 0; i < resourceCount; i++ {
				require.NoError(t, test.createResource(ids[i], clt), "creating resource %d", i)
			}

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				found, err := w.GetUnifiedResources(ctx)
				assert.NoError(t, err)
				assert.Len(t, found, resourceCount)
			}, 10*time.Second, 100*time.Millisecond)

			count := 0
			for r, err := range test.iterateResources(w) {
				require.NoError(t, err)

				if r.GetName() != ids[count] {
					t.Fatalf("expected resource named %s, got %s", ids[count], r.GetName())
				}
				count++
			}

			if count != resourceCount {
				t.Fatalf("iteration completed early, expected %d apps, got %d", resourceCount, count)
			}
		})
	}
}

func TestUnifiedResourceWatcher_PreventDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clt := newClient(t)
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
	clt := newClient(t)
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
			Namespace: defaults.Namespace,
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3(
		types.Metadata{
			Name:      "kube_server",
			Namespace: defaults.Namespace,
		},
		types.KubernetesServerSpecV3{
			Cluster: kube,
			HostID:  "hostID",
		},
	)
	require.NoError(t, err)
	_, err = clt.UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	icAcct := newICAccount(t, ctx, clt)

	// add git server
	gitServer := newGitServer(t, "my-org")
	_, err = clt.CreateGitServer(ctx, gitServer)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		res, _ := w.GetUnifiedResources(ctx)
		return len(res) == 8
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
	err = clt.DeleteIdentityCenterAccount(ctx, services.IdentityCenterAccountID(icAcct.GetMetadata().GetName()))
	require.NoError(t, err)
	err = clt.DeleteGitServer(ctx, gitServer.GetName())
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		res, _ := w.GetUnifiedResources(ctx)
		return len(res) == 0
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be deleted")
}

func Test_PaginatedResourcesSAMLIdPServiceProviderCompatibility(t *testing.T) {
	samlApp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newTestEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	)
	require.NoError(t, err)

	// for a v15 client, expect AppServerOrSAMLIdPServiceProvider response
	v15ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{apimetadata.VersionKey: "15.0.0"}))
	v15response, err := services.MakePaginatedResources(v15ctx, types.KindUnifiedResource, []types.ResourceWithLabels{samlApp}, map[string]struct{}{})
	require.NoError(t, err)
	require.Equal(t,
		&proto.PaginatedResource{
			Resource: &proto.PaginatedResource_AppServerOrSAMLIdPServiceProvider{
				//nolint:staticcheck // SA1019. TODO(gzdunek): DELETE IN 17.0 (with the entire test)
				AppServerOrSAMLIdPServiceProvider: &types.AppServerOrSAMLIdPServiceProviderV1{
					Resource: &types.AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
						SAMLIdPServiceProvider: samlApp.(*types.SAMLIdPServiceProviderV1),
					},
				}}},
		v15response[0],
	)

	// for a v16 client, expect SAMLIdPServiceProvider response
	v16ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{apimetadata.VersionKey: "16.0.0"}))
	v16response, err := services.MakePaginatedResources(v16ctx, types.KindUnifiedResource, []types.ResourceWithLabels{samlApp}, map[string]struct{}{})
	require.NoError(t, err)
	require.Equal(t,
		&proto.PaginatedResource{
			Resource: &proto.PaginatedResource_SAMLIdPServiceProvider{
				SAMLIdPServiceProvider: samlApp.(*types.SAMLIdPServiceProviderV1),
			}},
		v16response[0],
	)
}

func newTestEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

func newGitServer(t *testing.T, githubOrg string) types.Server {
	t.Helper()
	gitServer, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Organization: githubOrg,
		Integration:  githubOrg,
	})
	require.NoError(t, err)
	return gitServer
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

func newICAccount(t *testing.T, ctx context.Context, svc services.IdentityCenterAccounts) services.IdentityCenterAccount {
	t.Helper()

	accountID := t.Name()

	icAcct, err := svc.CreateIdentityCenterAccount(ctx, services.IdentityCenterAccount{
		Account: &identitycenterv1.Account{
			Kind:    types.KindIdentityCenterAccount,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: t.Name(),
				Labels: map[string]string{
					types.OriginLabel: common.OriginAWSIdentityCenter,
				},
			},
			Spec: &identitycenterv1.AccountSpec{
				Id:          accountID,
				Arn:         "arn:aws:sso:::account/" + accountID,
				Name:        "Test AWS Account",
				Description: "Used for testing",
				PermissionSetInfo: []*identitycenterv1.PermissionSetInfo{
					{
						Name: "Alpha",
						Arn:  "arn:aws:sso:::permissionSet/ssoins-1234567890/ps-alpha",
					},
					{
						Name: "Beta",
						Arn:  "arn:aws:sso:::permissionSet/ssoins-1234567890/ps-beta",
					},
				},
			},
		}})
	require.NoError(t, err, "creating Identity Center Account")
	return icAcct
}

func newICAccountAssignment(t *testing.T, ctx context.Context, svc services.IdentityCenterAccountAssignments) services.IdentityCenterAccountAssignment {
	t.Helper()

	assignment, err := svc.CreateAccountAssignment(ctx, services.IdentityCenterAccountAssignment{
		AccountAssignment: &identitycenterv1.AccountAssignment{
			Kind:    types.KindIdentityCenterAccountAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: t.Name(),
				Labels: map[string]string{
					types.OriginLabel: common.OriginAWSIdentityCenter,
				},
			},
			Spec: &identitycenterv1.AccountAssignmentSpec{
				Display: "Admin access on Production",
				PermissionSet: &identitycenterv1.PermissionSetInfo{
					Arn:          "arn:aws::::ps-Admin",
					Name:         "Admin",
					AssignmentId: "production--admin",
				},
				AccountName: "Production",
				AccountId:   "99999999",
			},
		}})
	require.NoError(t, err, "creating Identity Center Account Assignment")
	return assignment
}

func TestOktaAppServers(t *testing.T) {
	clt := newClient(t)
	ctx := context.Background()

	appsServer := []*types.AppServerV3{
		mustCreateOktaAppServer(t, uuid.NewString(), "App 1"),
		mustCreateOktaAppServer(t, uuid.NewString(), "App 1"),
		mustCreateOktaAppServer(t, uuid.NewString(), "App 1"),
	}
	for _, v := range appsServer {
		_, err := clt.UpsertApplicationServer(ctx, v)
		require.NoError(t, err)
	}

	w, err := services.NewUnifiedResourceCache(context.Background(), services.UnifiedResourceCacheConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentUnifiedResource,
			Client:    clt,
		},
		ResourceGetter: clt,
	})
	require.NoError(t, err)
	res, err := w.GetUnifiedResources(ctx)
	require.NoError(t, err)
	require.Len(t, res, 3)

}

func mustCreateOktaAppServer(t *testing.T, name, friendlyName string) *types.AppServerV3 {
	app, err := types.NewAppV3(types.Metadata{
		Name: fmt.Sprintf("app-%v", name),
		Labels: map[string]string{
			types.OriginLabel:      common.OriginOkta,
			types.OktaAppNameLabel: friendlyName,
		},
	}, types.AppSpecV3{
		URI: "localhost",
	})
	require.NoError(t, err)

	resource, err := types.NewAppServerV3(types.Metadata{
		Name: name,
	}, types.AppServerSpecV3{
		HostID: "localhost",
		App:    app,
	})
	require.NoError(t, err)
	return resource
}
