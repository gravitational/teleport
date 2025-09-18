// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package plugin

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestEntraIDGroupFilters(t *testing.T) {
	testCases := []struct {
		name                   string
		groupFilterIncludeID   []string
		groupFilterIncludeName []string
		groupFilterExcludeID   []string
		groupFilterExcludeName []string
		expectedFilters        []*types.PluginSyncFilter
		errorAssertion         require.ErrorAssertionFunc
	}{
		{
			name:            "empty filter",
			expectedFilters: []*types.PluginSyncFilter{},
			errorAssertion:  require.NoError,
		},
		{
			name:                   "valid filters",
			groupFilterIncludeID:   []string{"id1"},
			groupFilterIncludeName: []string{"a*"},
			groupFilterExcludeID:   []string{"id2"},
			groupFilterExcludeName: []string{"b*"},
			expectedFilters: []*types.PluginSyncFilter{
				{Include: &types.PluginSyncFilter_Id{Id: "id1"}},
				{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
				{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "id2"}},
				{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "b*"}},
			},
			errorAssertion: require.NoError,
		},
		{
			name:                   "valid multiple filters",
			groupFilterIncludeID:   []string{"id1", "id2"},
			groupFilterIncludeName: []string{"a*", "b*"},
			groupFilterExcludeID:   []string{"id3", "id4"},
			groupFilterExcludeName: []string{"b*", "c*"},
			expectedFilters: []*types.PluginSyncFilter{
				{Include: &types.PluginSyncFilter_Id{Id: "id1"}},
				{Include: &types.PluginSyncFilter_Id{Id: "id2"}},
				{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
				{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "b*"}},
				{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "id3"}},
				{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "id4"}},
				{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "b*"}},
				{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "c*"}},
			},
			errorAssertion: require.NoError,
		},
		{
			name:                 "empty include id string",
			groupFilterIncludeID: []string{""},
			errorAssertion:       require.Error,
		},
		{
			name:                   "bad regex",
			groupFilterIncludeName: []string{"^[)$"},
			errorAssertion:         require.Error,
		},
		{
			name:                   "bad exclude regex",
			groupFilterExcludeName: []string{"^[)$"},
			errorAssertion:         require.Error,
		},
		{
			name:                 "empty exclude id string",
			groupFilterExcludeID: []string{""},
			errorAssertion:       require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			authClient := &mockAuthClient{}
			authClient.
				On("Ping", anyContext).
				Return(proto.PingResponse{
					ProxyPublicAddr: "example.com",
				}, nil)
			authClient.
				On("CreateSAMLConnector", anyContext, mock.Anything).
				Return(&types.SAMLConnectorV2{}, nil)

			pluginsClient := &mockPluginsClient{}
			pluginsClient.
				On("CreatePlugin", anyContext, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					require.IsType(t, (*pluginsv1.CreatePluginRequest)(nil), args.Get(1))
					request := args.Get(1).(*pluginsv1.CreatePluginRequest)
					require.Empty(t, cmp.Diff(test.expectedFilters, request.GetPlugin().Spec.GetEntraId().SyncSettings.GroupFilters))
				}).
				Return(&emptypb.Empty{}, nil)
			pluginArgs := pluginServices{
				authClient: authClient,
				plugins:    pluginsClient,
			}

			var output bytes.Buffer
			var tenantID, clientID bytes.Buffer
			_, err := io.WriteString(&tenantID, "55fe2b7f-85c7-43c6-a8ba-897ce8570503\n")
			require.NoError(t, err)
			_, err = io.WriteString(&clientID, "3658a550-f173-44fa-a670-74b9fd7e3ae7\n")
			require.NoError(t, err)
			inputs := io.MultiReader(&tenantID, &clientID)

			cmd := PluginsCommand{
				install: pluginInstallArgs{
					name: "entra-id-default",
					entraID: entraArgs{
						authConnectorName:      "fake-saml-connector",
						accessGraph:            false,
						manualEntraIDSetup:     true,
						useSystemCredentials:   true,
						groupFilterIncludeID:   test.groupFilterIncludeID,
						groupFilterIncludeName: test.groupFilterIncludeName,
						groupFilterExcludeID:   test.groupFilterExcludeID,
						groupFilterExcludeName: test.groupFilterExcludeName,
					},
				},
				stdin:  inputs,
				stdout: &output,
			}

			err = cmd.InstallEntra(t.Context(), pluginArgs)
			test.errorAssertion(t, err)
		})
	}
}
