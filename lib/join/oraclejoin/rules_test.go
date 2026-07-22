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

package oraclejoin_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/oraclejoin"
)

func makeOCID(resourceType, region, id string) string {
	return fmt.Sprintf("ocid1.%s.oc1.%s.%s", resourceType, region, id)
}

func makeTenancyID(id string) string {
	return makeOCID("tenancy", "", id)
}

func makeCompartmentID(id string) string {
	return makeOCID("compartment", "", id)
}

func makeInstanceID(region, id string) string {
	return makeOCID("instance", region, id)
}

func TestCheckOracleAllowRules(t *testing.T) {
	t.Parallel()
	isAccessDenied := func(t require.TestingT, err error, msgAndArgs ...any) {
		require.ErrorAs(t, err, new(*trace.AccessDeniedError), msgAndArgs...)
	}
	tests := []struct {
		name       string
		claims     oraclejoin.Claims
		allowRules []*types.ProvisionTokenSpecV2Oracle_Rule
		assert     require.ErrorAssertionFunc
	}{
		{
			name: "ok",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with instance",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
					Instances:          []string{makeInstanceID("us-phoenix-1", "baz")},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with compartment wildcard",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("foo"),
					Regions: []string{"us-phoenix-1"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with region wildcard",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with region abbreviation in id",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("phx", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with region abbreviation in token",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"phx"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "wrong tenancy",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("something-else"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
					Instances:          []string{makeInstanceID("us-phoenix-1", "baz")},
				},
			},
			assert: isAccessDenied,
		},
		{
			name: "wrong compartment",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("something-else"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
					Instances:          []string{makeInstanceID("us-phoenix-1", "baz")},
				},
			},
			assert: isAccessDenied,
		},
		{
			name: "wrong region",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-ashburn-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
					Instances:          []string{makeInstanceID("us-phoenix-1", "baz")},
				},
			},
			assert: isAccessDenied,
		},
		{
			name: "wrong instance",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "notallowed"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
					Instances:          []string{makeInstanceID("us-phoenix-1", "allowed")},
				},
			},
			assert: isAccessDenied,
		},
		{
			name: "block match across rules",
			claims: oraclejoin.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("foo"),
					Regions: []string{"us-ashburn-1"},
				},
				{
					Tenancy: makeTenancyID("something-else"),
				},
			},
			assert: isAccessDenied,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := types.NewProvisionTokenFromSpec("mytoken", time.Time{}, types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleInstance},
				Oracle: &types.ProvisionTokenSpecV2Oracle{
					Allow: tc.allowRules,
				},
			})
			require.NoError(t, err)
			tc.assert(t, oraclejoin.CheckOracleAllowRules(&tc.claims, token))
		})
	}
}
