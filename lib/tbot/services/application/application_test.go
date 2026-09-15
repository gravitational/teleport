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
	returnedApps []*types.AppV3
	req          *proto.ListResourcesRequest
}

func (c *clientMock) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	c.req = req

	getResourcesResp := &proto.ListResourcesResponse{}
	for _, app := range c.returnedApps {
		getResourcesResp.Resources = append(getResourcesResp.Resources, &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_AppServer{
				AppServer: &types.AppServerV3{
					Spec: types.AppServerSpecV3{
						App: app,
					},
				},
			},
		})
	}

	return getResourcesResp, nil
}

func TestGetApp(t *testing.T) {
	testCases := map[string]struct {
		sqn               scopes.QualifiedName
		validate          func(t *testing.T, app types.Application)
		wantErr           string
		wantPredicateExpr string
		returnedApps      []*types.AppV3
	}{
		"returns the first app": {
			returnedApps: []*types.AppV3{
				{Metadata: types.Metadata{Name: "first"}},
				{Metadata: types.Metadata{Name: "second"}},
				{Metadata: types.Metadata{Name: "third"}},
			},
			validate: func(t *testing.T, app types.Application) {
				require.Equal(t, "first", app.GetName())
			},
			wantPredicateExpr: "name == \"\" && resource.scope == \"\"",
		},
		"return error when app is not found": {
			returnedApps: []*types.AppV3{},
			wantErr:      "matching app not found",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			client := &clientMock{returnedApps: tc.returnedApps}
			app, err := getApp(t.Context(), client, tc.sqn)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err), "trace.IsNotFound(err)")
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, app)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantPredicateExpr, client.req.PredicateExpression)

			tc.validate(t, app)
		})
	}
}

func TestGetAppLegacy(t *testing.T) {
	testCases := map[string]struct {
		name              string
		wantErr           string
		wantPredicateExpr string
		validate          func(t *testing.T, app types.Application)
		returnedApps      []*types.AppV3
	}{
		"returns the first app": {
			name: "first",
			returnedApps: []*types.AppV3{
				{Metadata: types.Metadata{Name: "first"}},
				{Metadata: types.Metadata{Name: "second"}},
				{Metadata: types.Metadata{Name: "third"}},
			},
			wantPredicateExpr: `name == "first"`,
			validate: func(t *testing.T, app types.Application) {
				require.Equal(t, "first", app.GetName())
			},
		},
		"return error when app is not found": {
			returnedApps: []*types.AppV3{},
			wantErr:      "matching app not found",
		},
		"filters out scoped apps": {
			name: "first",
			returnedApps: []*types.AppV3{
				{Metadata: types.Metadata{Name: "first"}, Scope: "/scope"},
				{Metadata: types.Metadata{Name: "first"}}},
			validate: func(t *testing.T, app types.Application) {
				require.Equal(t, "first", app.GetName())
				require.Empty(t, app.GetScope(), "first")
			},
			wantPredicateExpr: `name == "first"`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			client := &clientMock{returnedApps: tc.returnedApps}
			app, err := getAppLegacy(t.Context(), client, tc.name)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err), "trace.IsNotFound(err)")
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, app)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantPredicateExpr, client.req.PredicateExpression)

			tc.validate(t, app)
		})
	}
}

func TestGetMatchingApps(t *testing.T) {
	testCases := map[string]struct {
		predicate         string
		validate          func(t *testing.T, apps []types.Application)
		wantErr           string
		wantPredicateExpr string
		returnedApps      []*types.AppV3
	}{
		"returns matching apps": {
			predicate: `name == "first"`,
			returnedApps: []*types.AppV3{
				{Metadata: types.Metadata{Name: "first"}},
				{Metadata: types.Metadata{Name: "second"}},
				{Metadata: types.Metadata{Name: "third"}},
			},
			wantPredicateExpr: `name == "first"`,
			validate: func(t *testing.T, apps []types.Application) {
				require.Equal(t, "first", apps[0].GetName())
				require.Equal(t, "second", apps[1].GetName())
				require.Equal(t, "third", apps[2].GetName())
			},
		},
		"return error when no apps are found": {
			returnedApps: []*types.AppV3{},
			wantErr:      "matching app not found",
		},
		"deduplicates apps": {
			returnedApps: []*types.AppV3{
				{Metadata: types.Metadata{Name: "first"}},
				{Metadata: types.Metadata{Name: "first"}},
				{Metadata: types.Metadata{Name: "second"}},
			},
			validate: func(t *testing.T, apps []types.Application) {
				require.Equal(t, "first", apps[0].GetName())
				require.Equal(t, "second", apps[1].GetName())
				require.Len(t, apps, 2)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			client := &clientMock{returnedApps: tc.returnedApps}
			apps, err := getMatchingApps(t.Context(), client, tc.predicate)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err), "trace.IsNotFound(err)")
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, apps)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.predicate, client.req.PredicateExpression)

			tc.validate(t, apps)
		})
	}
}
