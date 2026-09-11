/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package application

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
)

type clientMock struct {
	getResourcesResp *proto.ListResourcesResponse
}

func (c *clientMock) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	return c.getResourcesResp, nil
}

func TestGetApp(t *testing.T) {
	testCases := map[string]struct {
		sqn          scopes.QualifiedName
		validate     func(t *testing.T, app types.Application)
		wantErr      string
		returnedApps []*types.AppV3
	}{
		"unqualified name rejects scoped apps": {
			sqn:     scopes.QualifiedName{Name: "my-app"},
			wantErr: `app "my-app" not found`,
			returnedApps: []*types.AppV3{
				{Scope: "/hello", Metadata: types.Metadata{Name: "my-app"}},
			},
		},
		"scope-qualified name selects exact scope": {
			sqn: scopes.QualifiedName{Name: "my-app", Scope: "/parent/child"},
			returnedApps: []*types.AppV3{
				{Scope: "/parent/child", Metadata: types.Metadata{Name: "my-app"}},
			},
			validate: func(t *testing.T, app types.Application) {
				require.Equal(t, "my-app", app.GetName())
				require.Equal(t, "/parent/child", app.GetScope())
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			clientMock := new(clientMock)
			clientMock.getResourcesResp = &proto.ListResourcesResponse{
				Resources: []*proto.PaginatedResource{},
			}

			for _, app := range tc.returnedApps {
				clientMock.getResourcesResp.Resources = append(clientMock.getResourcesResp.Resources, &proto.PaginatedResource{
					Resource: &proto.PaginatedResource_AppServer{
						AppServer: &types.AppServerV3{
							Spec: types.AppServerSpecV3{
								App: app,
							},
						},
					},
				})
			}

			app, err := getApp(t.Context(), clientMock, tc.sqn)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, app)
				return
			}

			tc.validate(t, app)
		})
	}
}
