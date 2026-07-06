// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package integrations

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestFilterICAccounts(t *testing.T) {
	t.Parallel()

	node := &types.ServerV2{
		Kind:     types.KindNode,
		Version:  types.V2,
		Metadata: types.Metadata{Name: "node1"},
	}

	regularApp := &types.AppServerV3{
		Spec: types.AppServerSpecV3{App: &types.AppV3{}},
	}

	resources := []*types.EnrichedResource{
		{ResourceWithLabels: newICAppServer("222222222222", "alpaca")},
		{ResourceWithLabels: regularApp},
		{ResourceWithLabels: node},
		{ResourceWithLabels: newICAppServer("111111111111", "llama")},
	}

	got := filterICAccounts(resources)

	// Non IC resources should be dropped.
	require.Len(t, got, 2)
	require.Equal(t, "alpaca", icAccountName(got[0].GetApp()))
	require.Equal(t, "llama", icAccountName(got[1].GetApp()))
}

func TestWriteAWSICText(t *testing.T) {
	t.Parallel()

	servers := []types.AppServer{
		newICAppServer("111111111111", "Production",
			&types.IdentityCenterPermissionSet{
				Name: "AdminAccess", ARN: "arn:aws:sso:::permissionSet/abc/ps-aaaa",
			},
			&types.IdentityCenterPermissionSet{
				Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/abc/ps-bbbb",
			},
		),
		newICAppServer("222222222222", "Staging",
			&types.IdentityCenterPermissionSet{
				Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/abc/ps-bbbb",
			},
		),
	}

	var buf bytes.Buffer
	c := &Command{Stdout: &buf}
	require.NoError(t, c.writeAWSICText(servers))

	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

func newICAppServer(accountID, accountName string, pss ...*types.IdentityCenterPermissionSet) *types.AppServerV3 {
	return &types.AppServerV3{
		Spec: types.AppServerSpecV3{
			App: &types.AppV3{
				Metadata: types.Metadata{Description: accountName},
				Spec: types.AppSpecV3{
					IdentityCenter: &types.AppIdentityCenter{
						AccountID:      accountID,
						PermissionSets: pss,
					},
				},
			},
		},
	}
}
