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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	libevents "github.com/gravitational/teleport/lib/events"
)

// TestSPIFFEFederationService_CreateSPIFFEFederation is an integration test
// that uses a real gRPC client/server.
func TestSPIFFEFederationService_CreateSPIFFEFederation(t *testing.T) {
	t.Parallel()
	srv, mockEmitter := newTestTLSServer(t)
	ctx := context.Background()

	nothingRole, err := types.NewRole("nothing", types.RoleSpecV6{})
	require.NoError(t, err)
	unauthorizedUser, err := auth.CreateUser(
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
	authorizedUser, err := auth.CreateUser(
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
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := srv.NewClient(auth.TestUser(tt.user))
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
