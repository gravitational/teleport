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

package machineidv1_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authtest"
	libevents "github.com/gravitational/teleport/lib/events"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// TestSPIFFEFederationService_CreateSPIFFEFederation is an integration test
// that uses a real gRPC client/server.
func TestSPIFFEFederationService_CreateSPIFFEFederation(t *testing.T) {
	t.Parallel()
	srv, mockEmitter := newTestTLSServer(t)
	ctx := context.Background()

	nothingRole, err := types.NewRole("nothing", types.RoleSpecV6{})
	require.NoError(t, err)
	unauthorizedUser, err := authtest.CreateUser(
		ctx,
		srv.Auth(),
		"unauthorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		nothingRole,
	)
	require.NoError(t, err)

	role, err := types.NewRole("federation-creator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSPIFFEFederation},
					Verbs:     []string{types.VerbCreate},
				},
			},
		},
	})
	require.NoError(t, err)
	authorizedUser, err := authtest.CreateUser(
		ctx,
		srv.Auth(),
		"authorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		role,
	)
	require.NoError(t, err)

	good := &machineidv1pb.SPIFFEFederation{
		Kind:    types.KindSPIFFEFederation,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "example.com",
		},
		Spec: &machineidv1pb.SPIFFEFederationSpec{
			BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
				HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
					BundleEndpointUrl: "https://example.com/bundle.json",
				},
			},
		},
	}

	tests := []struct {
		name           string
		user           string
		req            *machineidv1pb.CreateSPIFFEFederationRequest
		requireError   require.ErrorAssertionFunc
		requireSuccess bool
		requireEvent   *events.SPIFFEFederationCreate
	}{
		{
			name: "success",
			user: authorizedUser.GetName(),
			req: &machineidv1pb.CreateSPIFFEFederationRequest{
				SpiffeFederation: good,
			},
			requireError:   require.NoError,
			requireSuccess: true,
			requireEvent: &events.SPIFFEFederationCreate{
				Metadata: events.Metadata{
					Type: libevents.SPIFFEFederationCreateEvent,
					Code: libevents.SPIFFEFederationCreateCode,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: "example.com",
				},
				UserMetadata: events.UserMetadata{
					User:            authorizedUser.GetName(),
					UserKind:        events.UserKind_USER_KIND_HUMAN,
					UserRoles:       authorizedUser.GetRoles(),
					UserClusterName: "localhost",
				},
			},
		},
		{
			name: "unable to set status",
			user: authorizedUser.GetName(),
			req: &machineidv1pb.CreateSPIFFEFederationRequest{
				SpiffeFederation: func() *machineidv1pb.SPIFFEFederation {
					fed := proto.Clone(good).(*machineidv1pb.SPIFFEFederation)
					fed.Status = &machineidv1pb.SPIFFEFederationStatus{
						CurrentBundleSyncedAt: timestamppb.Now(),
					}
					return fed
				}(),
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "status: cannot be set")
			},
		},
		{
			name: "validation is run",
			user: authorizedUser.GetName(),
			req: &machineidv1pb.CreateSPIFFEFederationRequest{
				SpiffeFederation: func() *machineidv1pb.SPIFFEFederation {
					fed := proto.Clone(good).(*machineidv1pb.SPIFFEFederation)
					fed.Metadata.Name = "spiffe://im----invalid"
					return fed
				}(),
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "metadata.name: must not include the spiffe:// prefix")
			},
		},
		{
			name: "unauthorized",
			user: unauthorizedUser.GetName(),
			req: &machineidv1pb.CreateSPIFFEFederationRequest{
				SpiffeFederation: good,
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			mockEmitter.Reset()
			got, err := client.SPIFFEFederationServiceClient().CreateSPIFFEFederation(ctx, tt.req)
			tt.requireError(t, err)
			if tt.requireSuccess {
				// First check the response object matches our requested object.
				require.Empty(
					t,
					cmp.Diff(
						tt.req.SpiffeFederation,
						got,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)

				// Then check the response is actually stored in the backend
				got, err := srv.Auth().Services.SPIFFEFederations.GetSPIFFEFederation(
					ctx, got.Metadata.GetName(),
				)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						tt.req.SpiffeFederation,
						got,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
			}
			// Now we can ensure that the appropriate audit event was
			// generated.
			if tt.requireEvent != nil {
				evt, ok := mockEmitter.LastEvent().(*events.SPIFFEFederationCreate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.SPIFFEFederationCreate{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

// TestSPIFFEFederationService_DeleteSPIFFEFederation is an integration test
// that uses a real gRPC client/server.
func TestSPIFFEFederationService_DeleteSPIFFEFederation(t *testing.T) {
	t.Parallel()
	srv, mockEmitter := newTestTLSServer(t)
	ctx := context.Background()

	nothingRole, err := types.NewRole("nothing", types.RoleSpecV6{})
	require.NoError(t, err)
	unauthorizedUser, err := authtest.CreateUser(
		ctx,
		srv.Auth(),
		"unauthorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		nothingRole,
	)
	require.NoError(t, err)

	role, err := types.NewRole("federation-deleter", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSPIFFEFederation},
					Verbs:     []string{types.VerbDelete},
				},
			},
		},
	})
	require.NoError(t, err)
	authorizedUser, err := authtest.CreateUser(
		ctx,
		srv.Auth(),
		"authorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		role,
	)
	require.NoError(t, err)

	name := "example.com"

	tests := []struct {
		name           string
		user           string
		create         bool
		requireError   require.ErrorAssertionFunc
		requireSuccess bool
		requireEvent   *events.SPIFFEFederationDelete
	}{
		{
			name:           "success",
			user:           authorizedUser.GetName(),
			create:         true,
			requireError:   require.NoError,
			requireSuccess: true,
			requireEvent: &events.SPIFFEFederationDelete{
				Metadata: events.Metadata{
					Type: libevents.SPIFFEFederationDeleteEvent,
					Code: libevents.SPIFFEFederationDeleteCode,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: name,
				},
				UserMetadata: events.UserMetadata{
					User:            authorizedUser.GetName(),
					UserKind:        events.UserKind_USER_KIND_HUMAN,
					UserRoles:       authorizedUser.GetRoles(),
					UserClusterName: "localhost",
				},
			},
		},
		{
			name:   "not-exist",
			user:   authorizedUser.GetName(),
			create: false,
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "unauthorized",
			user:   unauthorizedUser.GetName(),
			create: true,
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			resource := &machineidv1pb.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: name,
				},
				Spec: &machineidv1pb.SPIFFEFederationSpec{
					BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/bundle.json",
						},
					},
				},
			}

			if tt.create {
				_, err := srv.Auth().Services.SPIFFEFederations.CreateSPIFFEFederation(
					ctx, resource,
				)
				require.NoError(t, err)
			}

			mockEmitter.Reset()
			_, err = client.SPIFFEFederationServiceClient().DeleteSPIFFEFederation(ctx, &machineidv1pb.DeleteSPIFFEFederationRequest{
				Name: resource.Metadata.GetName(),
			})
			tt.requireError(t, err)
			if tt.requireSuccess {
				// Check that it is no longer in the backend
				_, err := srv.Auth().Services.SPIFFEFederations.GetSPIFFEFederation(
					ctx, resource.Metadata.GetName(),
				)
				require.True(t, trace.IsNotFound(err))
			}
			// Now we can ensure that the appropriate audit event was
			// generated.
			if tt.requireEvent != nil {
				evt, ok := mockEmitter.LastEvent().(*events.SPIFFEFederationDelete)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.SPIFFEFederationDelete{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

// TestSPIFFEFederationService_GetSPIFFEFederation is an integration test
// that uses a real gRPC client/server.
func TestSPIFFEFederationService_GetSPIFFEFederation(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	role, err := types.NewRole("federation-reader", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSPIFFEFederation},
					Verbs:     []string{types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)
	authorizedUser, err := authtest.CreateUser(
		ctx,
		srv.Auth(),
		"authorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		role,
	)
	require.NoError(t, err)

	name := "example.com"
	resource, err := srv.Auth().Services.SPIFFEFederations.CreateSPIFFEFederation(
		ctx, &machineidv1pb.SPIFFEFederation{
			Kind:    types.KindSPIFFEFederation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &machineidv1pb.SPIFFEFederationSpec{
				BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
					HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
						BundleEndpointUrl: "https://example.com/bundle.json",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	tests := []struct {
		name           string
		user           string
		getName        string
		requireError   require.ErrorAssertionFunc
		requireSuccess bool
	}{
		{
			name:           "success",
			user:           authorizedUser.GetName(),
			getName:        name,
			requireError:   require.NoError,
			requireSuccess: true,
		},
		{
			name:    "not-exist",
			user:    authorizedUser.GetName(),
			getName: "do-not-exist",
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			got, err := client.SPIFFEFederationServiceClient().GetSPIFFEFederation(ctx, &machineidv1pb.GetSPIFFEFederationRequest{
				Name: tt.getName,
			})
			tt.requireError(t, err)
			if tt.requireSuccess {
				require.Empty(
					t,
					cmp.Diff(
						resource,
						got,
						protocmp.Transform(),
					),
				)
			}
		})
	}
}

// TestSPIFFEFederationService_ListSPIFFEFederations is an integration test
// that uses a real gRPC client/server.
func TestSPIFFEFederationService_ListSPIFFEFederations(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	role, err := types.NewRole("federation-reader", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSPIFFEFederation},
					Verbs:     []string{types.VerbRead, types.VerbList},
				},
			},
		},
	})
	require.NoError(t, err)
	authorizedUser, err := authtest.CreateUser(
		ctx,
		srv.Auth(),
		"authorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		role,
	)
	require.NoError(t, err)

	// Create entities to list
	createdObjects := []*machineidv1pb.SPIFFEFederation{}
	// Create 49 entities to test an incomplete page at the end.
	for i := range 49 {
		created, err := srv.AuthServer.AuthServer.Services.SPIFFEFederations.CreateSPIFFEFederation(
			ctx,
			&machineidv1pb.SPIFFEFederation{
				Kind:    types.KindSPIFFEFederation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: fmt.Sprintf("%d.example.com", i),
				},
				Spec: &machineidv1pb.SPIFFEFederationSpec{
					BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
						HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
							BundleEndpointUrl: "https://example.com/bundle.json",
						},
					},
				},
			},
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}

	tests := []struct {
		name           string
		user           string
		pageSize       int
		wantIterations int
		requireError   require.ErrorAssertionFunc
		assertResponse bool
	}{
		{
			name:           "success - one page",
			user:           authorizedUser.GetName(),
			wantIterations: 1,
			requireError:   require.NoError,
			assertResponse: true,
		},
		{
			name:           "success - small pages",
			pageSize:       10,
			wantIterations: 5,
			user:           authorizedUser.GetName(),
			requireError:   require.NoError,
			assertResponse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(authtest.TestUser(tt.user))
			require.NoError(t, err)

			fetched := []*machineidv1pb.SPIFFEFederation{}
			token := ""
			iterations := 0
			for {
				iterations++
				resp, err := client.SPIFFEFederationServiceClient().ListSPIFFEFederations(ctx, &machineidv1pb.ListSPIFFEFederationsRequest{
					PageSize:  int32(tt.pageSize),
					PageToken: token,
				})
				tt.requireError(t, err)
				if err != nil {
					return
				}
				fetched = append(fetched, resp.SpiffeFederations...)
				if resp.NextPageToken == "" {
					break
				}
				token = resp.NextPageToken
			}
			if tt.assertResponse {
				require.Equal(t, tt.wantIterations, iterations)
				require.Len(t, fetched, 49)
				for _, created := range createdObjects {
					require.True(t, slices.ContainsFunc(fetched, func(federation *machineidv1pb.SPIFFEFederation) bool {
						return proto.Equal(created, federation)
					}))
				}
			}
		})
	}
}

// TestSPIFFEFederationService_ScopedIdentity verifies that a scope-pinned
// identity can read SPIFFE federations via GetSPIFFEFederation and
// ListSPIFFEFederations. SPIFFE federations are cluster-global config readable
// by all identities (via the default implicit role), so an empty scoped role is
// sufficient.
func TestSPIFFEFederationService_ScopedIdentity(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	srv, _ := newTestTLSServer(t)
	ctx := t.Context()

	adminClient, err := srv.NewClient(authtest.TestAdmin())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = adminClient.Close()
	})

	// Create a scoped role with an empty allow block (no explicit rules). Reads
	// of cluster-global SPIFFE federations are granted via the default implicit
	// role, so no extra permissions are required.
	scopedSvc := adminClient.ScopedAccessServiceClient()
	scopedRole, err := scopedSvc.CreateScopedRole(ctx, &scopedaccessv1.CreateScopedRoleRequest{
		Role: &scopedaccessv1.ScopedRole{
			Kind:    scopedaccess.KindScopedRole,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "spiffe-federation-reader",
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleSpec{
				AssignableScopes: []string{"/scopes/granted"},
			},
		},
	})
	require.NoError(t, err)

	scopedUser, err := authtest.CreateUser(ctx, srv.Auth(), "scoped-reader")
	require.NoError(t, err)

	sraResp, err := scopedSvc.CreateScopedRoleAssignment(ctx, &scopedaccessv1.CreateScopedRoleAssignmentRequest{
		Assignment: &scopedaccessv1.ScopedRoleAssignment{
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: uuid.NewString(),
			},
			Scope: "/scopes",
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: scopedUser.GetName(),
				Assignments: []*scopedaccessv1.Assignment{
					{Role: scopedRole.Role.Metadata.Name, Scope: "/scopes/granted"},
				},
			},
		},
	})
	require.NoError(t, err)
	waitForSRACache(t, srv, sraResp)

	name := "example.com"
	resource, err := srv.Auth().Services.SPIFFEFederations.CreateSPIFFEFederation(
		ctx, &machineidv1pb.SPIFFEFederation{
			Kind:    types.KindSPIFFEFederation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &machineidv1pb.SPIFFEFederationSpec{
				BundleSource: &machineidv1pb.SPIFFEFederationBundleSource{
					HttpsWeb: &machineidv1pb.SPIFFEFederationBundleSourceHTTPSWeb{
						BundleEndpointUrl: "https://example.com/bundle.json",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	scopedClient, err := srv.NewClient(authtest.TestScopedUser(scopedUser.GetName(), "/scopes/granted"))
	require.NoError(t, err)
	defer scopedClient.Close()

	t.Run("GetSPIFFEFederation", func(t *testing.T) {
		got, err := scopedClient.SPIFFEFederationServiceClient().GetSPIFFEFederation(ctx, &machineidv1pb.GetSPIFFEFederationRequest{
			Name: name,
		})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(resource, got, protocmp.Transform()))
	})

	t.Run("ListSPIFFEFederations", func(t *testing.T) {
		resp, err := scopedClient.SPIFFEFederationServiceClient().ListSPIFFEFederations(ctx, &machineidv1pb.ListSPIFFEFederationsRequest{})
		require.NoError(t, err)
		require.True(t, slices.ContainsFunc(resp.SpiffeFederations, func(federation *machineidv1pb.SPIFFEFederation) bool {
			return proto.Equal(resource, federation)
		}))
	})
}
