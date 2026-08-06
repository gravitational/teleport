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
			require.ElementsMatch(t, upserts.GetResources(), tt.expectedUpserts.GetResources())
			require.ElementsMatch(t, deletes.GetResources(), tt.expectedDeletes.GetResources())
		})
	}

}

func generateExpected(
	principals []*accessgraphv1alpha.AzurePrincipal,
	roleDefs []*accessgraphv1alpha.AzureRoleDefinition,
	roleAssigns []*accessgraphv1alpha.AzureRoleAssignment,
	vms []*accessgraphv1alpha.AzureVirtualMachine,
) *accessgraphv1alpha.AzureResourceList {
	resList := accessgraphv1alpha.AzureResourceList_builder{
		Resources: make([]*accessgraphv1alpha.AzureResource, 0),
	}.Build()
	for _, principal := range principals {
		resList.SetResources(append(resList.GetResources(), azurePrincipalsWrap(principal)))
	}
	for _, roleDef := range roleDefs {
		resList.SetResources(append(resList.GetResources(), azureRoleDefWrap(roleDef)))
	}
	for _, roleAssign := range roleAssigns {
		resList.SetResources(append(resList.GetResources(), azureRoleAssignWrap(roleAssign)))
	}
	for _, vm := range vms {
		resList.SetResources(append(resList.GetResources(), azureVmWrap(vm)))
	}
	return resList
}

func generatePrincipals() []*accessgraphv1alpha.AzurePrincipal {
	return []*accessgraphv1alpha.AzurePrincipal{
		accessgraphv1alpha.AzurePrincipal_builder{
			Id:          "/principals/foo",
			DisplayName: "userFoo",
		}.Build(),
		accessgraphv1alpha.AzurePrincipal_builder{
			Id:          "/principals/bar",
			DisplayName: "userBar",
		}.Build(),
		accessgraphv1alpha.AzurePrincipal_builder{
			Id:          "/principals/charles",
			DisplayName: "userCharles",
		}.Build(),
	}
}

func generateRoleDefs() []*accessgraphv1alpha.AzureRoleDefinition {
	return []*accessgraphv1alpha.AzureRoleDefinition{
		accessgraphv1alpha.AzureRoleDefinition_builder{
			Id:   "/roledefinitions/foo",
			Name: "roleFoo",
		}.Build(),
		accessgraphv1alpha.AzureRoleDefinition_builder{
			Id:   "/roledefinitions/bar",
			Name: "roleBar",
		}.Build(),
		accessgraphv1alpha.AzureRoleDefinition_builder{
			Id:   "/roledefinitions/charles",
			Name: "roleCharles",
		}.Build(),
	}
}

func generateRoleAssigns() []*accessgraphv1alpha.AzureRoleAssignment {
	return []*accessgraphv1alpha.AzureRoleAssignment{
		accessgraphv1alpha.AzureRoleAssignment_builder{
			Id:          "/roleassignments/foo",
			PrincipalId: "userFoo",
		}.Build(),
		accessgraphv1alpha.AzureRoleAssignment_builder{
			Id:          "/roleassignments/bar",
			PrincipalId: "userBar",
		}.Build(),
		accessgraphv1alpha.AzureRoleAssignment_builder{
			Id:          "/roleassignments/charles",
			PrincipalId: "userCharles",
		}.Build(),
	}
}

func generateVms() []*accessgraphv1alpha.AzureVirtualMachine {
	return []*accessgraphv1alpha.AzureVirtualMachine{
		accessgraphv1alpha.AzureVirtualMachine_builder{
			Id:   "/vms/foo",
			Name: "userFoo",
		}.Build(),
		accessgraphv1alpha.AzureVirtualMachine_builder{
			Id:   "/vms/bar",
			Name: "userBar",
		}.Build(),
		accessgraphv1alpha.AzureVirtualMachine_builder{
			Id:   "/vms/charles",
			Name: "userCharles",
		}.Build(),
	}
}
