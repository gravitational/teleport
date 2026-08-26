/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package azuresync

import (
	"testing"

	"github.com/stretchr/testify/require"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func TestReconcileResults(t *testing.T) {
	principals := generatePrincipals()
	roleDefs := generateRoleDefs()
	roleAssigns := generateRoleAssigns()
	vms := generateVms()

	tests := []struct {
		name            string
		oldResults      *Resources
		newResults      *Resources
		expectedUpserts *accessgraphv1alpha.AzureResourceList
		expectedDeletes *accessgraphv1alpha.AzureResourceList
	}{
		// Overlapping old and new results
		{
			name: "OverlapOldAndNewResults",
			oldResults: &Resources{
				Principals:      principals[0:2],
				RoleDefinitions: roleDefs[0:2],
				RoleAssignments: roleAssigns[0:2],
				VirtualMachines: vms[0:2],
			},
			newResults: &Resources{
				Principals:      principals[1:3],
				RoleDefinitions: roleDefs[1:3],
				RoleAssignments: roleAssigns[1:3],
				VirtualMachines: vms[1:3],
			},
			expectedUpserts: generateExpected(principals[2:3], roleDefs[2:3], roleAssigns[2:3], vms[2:3]),
			expectedDeletes: generateExpected(principals[0:1], roleDefs[0:1], roleAssigns[0:1], vms[0:1]),
		},
		// Completely new results
		{
			name: "CompletelyNewResults",
			oldResults: &Resources{
				Principals:      nil,
				RoleDefinitions: nil,
				RoleAssignments: nil,
				VirtualMachines: nil,
			},
			newResults: &Resources{
				Principals:      principals[1:3],
				RoleDefinitions: roleDefs[1:3],
				RoleAssignments: roleAssigns[1:3],
				VirtualMachines: vms[1:3],
			},
			expectedUpserts: generateExpected(principals[1:3], roleDefs[1:3], roleAssigns[1:3], vms[1:3]),
			expectedDeletes: generateExpected(nil, nil, nil, nil),
		},
		// No new results
		{
			name: "NoNewResults",
			oldResults: &Resources{
				Principals:      principals[1:3],
				RoleDefinitions: roleDefs[1:3],
				RoleAssignments: roleAssigns[1:3],
				VirtualMachines: vms[1:3],
			},
			newResults: &Resources{
				Principals:      nil,
				RoleDefinitions: nil,
				RoleAssignments: nil,
				VirtualMachines: nil,
			},
			expectedUpserts: generateExpected(nil, nil, nil, nil),
			expectedDeletes: generateExpected(principals[1:3], roleDefs[1:3], roleAssigns[1:3], vms[1:3]),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upserts, deletes := ReconcileResults(tt.oldResults, tt.newResults)
			require.ElementsMatch(t, upserts.Resources, tt.expectedUpserts.Resources)
			require.ElementsMatch(t, deletes.Resources, tt.expectedDeletes.Resources)
		})
	}

}

func generateExpected(
	principals []*accessgraphv1alpha.AzurePrincipal,
	roleDefs []*accessgraphv1alpha.AzureRoleDefinition,
	roleAssigns []*accessgraphv1alpha.AzureRoleAssignment,
	vms []*accessgraphv1alpha.AzureVirtualMachine,
) *accessgraphv1alpha.AzureResourceList {
	resList := &accessgraphv1alpha.AzureResourceList{
		Resources: make([]*accessgraphv1alpha.AzureResource, 0),
	}
	for _, principal := range principals {
		resList.Resources = append(resList.Resources, azurePrincipalsWrap(principal))
	}
	for _, roleDef := range roleDefs {
		resList.Resources = append(resList.Resources, azureRoleDefWrap(roleDef))
	}
	for _, roleAssign := range roleAssigns {
		resList.Resources = append(resList.Resources, azureRoleAssignWrap(roleAssign))
	}
	for _, vm := range vms {
		resList.Resources = append(resList.Resources, azureVmWrap(vm))
	}
	return resList
}

func generatePrincipals() []*accessgraphv1alpha.AzurePrincipal {
	return []*accessgraphv1alpha.AzurePrincipal{
		{
			Id:          "/principals/foo",
			DisplayName: "userFoo",
		},
		{
			Id:          "/principals/bar",
			DisplayName: "userBar",
		},
		{
			Id:          "/principals/charles",
			DisplayName: "userCharles",
		},
	}
}

func generateRoleDefs() []*accessgraphv1alpha.AzureRoleDefinition {
	return []*accessgraphv1alpha.AzureRoleDefinition{
		{
			Id:   "/roledefinitions/foo",
			Name: "roleFoo",
		},
		{
			Id:   "/roledefinitions/bar",
			Name: "roleBar",
		},
		{
			Id:   "/roledefinitions/charles",
			Name: "roleCharles",
		},
	}
}

func generateRoleAssigns() []*accessgraphv1alpha.AzureRoleAssignment {
	return []*accessgraphv1alpha.AzureRoleAssignment{
		{
			Id:          "/roleassignments/foo",
			PrincipalId: "userFoo",
		},
		{
			Id:          "/roleassignments/bar",
			PrincipalId: "userBar",
		},
		{
			Id:          "/roleassignments/charles",
			PrincipalId: "userCharles",
		},
	}
}

func generateVms() []*accessgraphv1alpha.AzureVirtualMachine {
	return []*accessgraphv1alpha.AzureVirtualMachine{
		{
			Id:   "/vms/foo",
			Name: "userFoo",
		},
		{
			Id:   "/vms/bar",
			Name: "userBar",
		},
		{
			Id:   "/vms/charles",
			Name: "userCharles",
		},
	}
}
