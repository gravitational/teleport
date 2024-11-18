package azure_sync

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/stretchr/testify/require"
)

type testRoleDefCli struct {
	returnErr bool
	roleDefs  []*armauthorization.RoleDefinition
}

func (t testRoleDefCli) ListRoleDefinitions(ctx context.Context, scope string) ([]*armauthorization.RoleDefinition, error) {
	if t.returnErr {
		return nil, fmt.Errorf("error")
	}
	return t.roleDefs, nil
}

type testRoleAssignCli struct {
	returnErr   bool
	roleAssigns []*armauthorization.RoleAssignment
}

func (t testRoleAssignCli) ListRoleAssignments(ctx context.Context, scope string) ([]*armauthorization.RoleAssignment, error) {
	if t.returnErr {
		return nil, fmt.Errorf("error")
	}
	return t.roleAssigns, nil
}

type testVmCli struct {
	returnErr bool
	vms       []*armcompute.VirtualMachine
}

func (t testVmCli) ListVirtualMachines(ctx context.Context, scope string) ([]*armcompute.VirtualMachine, error) {
	if t.returnErr {
		return nil, fmt.Errorf("error")
	}
	fmt.Printf("CLIENT VMS: %v\n", t.vms)
	return t.vms, nil
}

func newRoleDef(id string, name string) *armauthorization.RoleDefinition {
	role_name := "test_role_name"
	return &armauthorization.RoleDefinition{
		ID:   &id,
		Name: &name,
		Properties: &armauthorization.RoleDefinitionProperties{
			Permissions: []*armauthorization.Permission{},
			RoleName:    &role_name,
		},
	}
}

func newRoleAssign(id string, name string) *armauthorization.RoleAssignment {
	scope := "test_scope"
	principalId := "test_principal_id"
	roleDefId := "test_role_def_id"
	return &armauthorization.RoleAssignment{
		ID:   &id,
		Name: &name,
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID:      &principalId,
			RoleDefinitionID: &roleDefId,
			Scope:            &scope,
		},
	}
}

func newVm(id string, name string) *armcompute.VirtualMachine {
	return &armcompute.VirtualMachine{
		ID:   &id,
		Name: &name,
	}
}

func TestPoll(t *testing.T) {
	roleDefs := []*armauthorization.RoleDefinition{
		newRoleDef("id1", "name1"),
	}
	roleAssigns := []*armauthorization.RoleAssignment{
		newRoleAssign("id1", "name1"),
	}
	vms := []*armcompute.VirtualMachine{
		newVm("id1", "name2"),
	}
	roleDefClient := testRoleDefCli{}
	roleAssignClient := testRoleAssignCli{}
	vmClient := testVmCli{}
	fetcher := Fetcher{
		Config:           Config{},
		lastResult:       &Resources{},
		roleDefClient:    &roleDefClient,
		roleAssignClient: &roleAssignClient,
		vmClient:         &vmClient,
	}
	ctx := context.Background()
	allFeats := Features{
		RoleDefinitions: true,
		RoleAssignments: true,
		VirtualMachines: true,
	}
	noVmsFeats := allFeats
	noVmsFeats.VirtualMachines = false

	tests := []struct {
		returnErr   bool
		roleDefs    []*armauthorization.RoleDefinition
		roleAssigns []*armauthorization.RoleAssignment
		vms         []*armcompute.VirtualMachine
		feats       Features
	}{
		// Process no results from clients
		{
			returnErr:   false,
			roleDefs:    []*armauthorization.RoleDefinition{},
			roleAssigns: []*armauthorization.RoleAssignment{},
			vms:         []*armcompute.VirtualMachine{},
			feats:       allFeats,
		},
		// Process test results from clients
		{
			returnErr:   false,
			roleDefs:    roleDefs,
			roleAssigns: roleAssigns,
			vms:         vms,
			feats:       allFeats,
		},
		// Handle errors from clients
		{
			returnErr:   true,
			roleDefs:    roleDefs,
			roleAssigns: roleAssigns,
			vms:         vms,
			feats:       allFeats,
		},
		// Handle VM features being disabled
		{
			returnErr:   false,
			roleDefs:    roleDefs,
			roleAssigns: roleAssigns,
			vms:         vms,
			feats:       noVmsFeats,
		},
	}

	for _, tt := range tests {
		// Set the test data
		roleDefClient.returnErr = tt.returnErr
		roleDefClient.roleDefs = tt.roleDefs
		roleAssignClient.returnErr = tt.returnErr
		roleAssignClient.roleAssigns = tt.roleAssigns
		vmClient.returnErr = tt.returnErr
		vmClient.vms = tt.vms

		// Poll for resources
		resources, err := fetcher.Poll(ctx, tt.feats)

		// Require no error unless otherwise specified
		if tt.returnErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)

		// Verify the results, based on the features set
		require.NotNil(t, resources)
		if tt.feats.RoleDefinitions {
			require.Len(t, resources.RoleDefinitions, len(tt.roleDefs))
		}
		if tt.feats.RoleAssignments {
			require.Len(t, resources.RoleAssignments, len(tt.roleAssigns))
		}
		if tt.feats.VirtualMachines {
			require.Len(t, resources.VirtualMachines, len(tt.vms))
		}
	}
}
