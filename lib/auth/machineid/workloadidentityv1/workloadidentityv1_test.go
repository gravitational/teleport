// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package workloadidentityv1_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func newTestTLSServer(t testing.TB) (*auth.TestTLSServer, *eventstest.MockRecorderEmitter) {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}
	srv, err := as.NewTestTLSServer(func(config *auth.TestTLSServerConfig) {
		config.APIConfig.Emitter = emitter
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv, emitter
}

func TestResourceService_CreateWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbCreate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.CreateWorkloadIdentityRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityCreate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityCreate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityCreateCode,
					Type: libevents.WorkloadIdentityCreateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: "new",
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "pre-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: preExisting.GetMetadata().GetName(),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "spec.spiffe.id: is required")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "unauthorized",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.CreateWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentity,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentity(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityCreate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					tt.requireEvent,
					evt,
					cmpopts.IgnoreFields(events.WorkloadIdentityCreate{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

func TestResourceService_DeleteWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name             string
		client           *authclient.Client
		req              *workloadidentityv1pb.DeleteWorkloadIdentityRequest
		requireError     require.ErrorAssertionFunc
		checkNonExisting bool
		requireEvent     *events.WorkloadIdentityDelete
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: preExisting.GetMetadata().GetName(),
			},
			requireError:     require.NoError,
			checkNonExisting: true,
			requireEvent: &events.WorkloadIdentityDelete{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityDeleteCode,
					Type: libevents.WorkloadIdentityDeleteEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: preExisting.GetMetadata().GetName(),
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "non-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: "i-do-not-exist",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "name: must be non-empty")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: "unauthorized",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			_, err := client.DeleteWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkNonExisting {
				_, err := srv.Auth().GetWorkloadIdentity(ctx, tt.req.Name)
				require.True(t, trace.IsNotFound(err))
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityDelete)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					tt.requireEvent,
					evt,
					cmpopts.IgnoreFields(events.WorkloadIdentityDelete{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

func TestResourceService_GetWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbRead},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name         string
		client       *authclient.Client
		req          *workloadidentityv1pb.GetWorkloadIdentityRequest
		wantRes      *workloadidentityv1pb.WorkloadIdentity
		requireError require.ErrorAssertionFunc
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: preExisting.GetMetadata().GetName(),
			},
			wantRes:      preExisting,
			requireError: require.NoError,
		},
		{
			name:   "non-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: "i-do-not-exist",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "name: must be non-empty")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: "unauthorized",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			got, err := client.GetWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.wantRes != nil {
				require.Empty(
					t,
					cmp.Diff(
						tt.wantRes,
						got,
						protocmp.Transform(),
					),
				)
			}
		})
	}
}

func TestResourceService_ListWorkloadIdentities(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbRead, types.VerbList},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identities
	// Two complete pages of ten, plus one incomplete page of nine
	created := []*workloadidentityv1pb.WorkloadIdentity{}
	for i := 0; i < 29; i++ {
		r, err := srv.Auth().CreateWorkloadIdentity(
			ctx,
			&workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: fmt.Sprintf("preexisting-%d", i),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			})
		require.NoError(t, err)
		created = append(created, r)
	}

	t.Run("unauthorized", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
			unauthorizedClient.GetConnection(),
		)

		_, err := client.ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{})
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success - default page", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
			authorizedClient.GetConnection(),
		)

		// For the default page size, we expect to get all results in one page
		res, err := client.ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{})
		require.NoError(t, err)
		require.Len(t, res.WorkloadIdentities, 29)
		require.Empty(t, res.NextPageToken)
		for _, created := range created {
			slices.ContainsFunc(res.WorkloadIdentities, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			})
		}
	})

	t.Run("success - page size 10", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
			authorizedClient.GetConnection(),
		)

		fetched := []*workloadidentityv1pb.WorkloadIdentity{}
		token := ""
		iterations := 0
		for {
			iterations++
			res, err := client.ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{
				PageSize:  10,
				PageToken: token,
			})
			require.NoError(t, err)
			fetched = append(fetched, res.WorkloadIdentities...)
			if res.NextPageToken == "" {
				break
			}
			token = res.NextPageToken
		}

		require.Len(t, fetched, 29)
		require.Equal(t, 3, iterations)
		for _, created := range created {
			slices.ContainsFunc(fetched, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			})
		}
	})
}

func TestResourceService_UpdateWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbUpdate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)
	preExisting2, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting-2",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.UpdateWorkloadIdentityRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityUpdate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
				WorkloadIdentity: preExisting,
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityUpdate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityUpdateCode,
					Type: libevents.WorkloadIdentityUpdateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: preExisting.GetMetadata().GetName(),
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "incorrect revision",
			client: authorizedClient,
			req: (func() *workloadidentityv1pb.UpdateWorkloadIdentityRequest {
				preExisting2.Metadata.Revision = "incorrect"
				return &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
					WorkloadIdentity: preExisting2,
				}
			})(),
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsCompareFailed(err))
			},
		},
		{
			name:   "not existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/test",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
				WorkloadIdentity: preExisting,
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.UpdateWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				require.NotEqual(t, tt.req.WorkloadIdentity.GetMetadata().GetRevision(), res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentity,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentity(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityUpdate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					tt.requireEvent,
					evt,
					cmpopts.IgnoreFields(events.WorkloadIdentityUpdate{}, "ConnectionMetadata"),
				))
			}
		})
	}
}
