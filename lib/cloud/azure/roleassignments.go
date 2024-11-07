package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/gravitational/trace"
)

type RoleAssignmentsClient interface {
	ListRoleAssignments(ctx context.Context, scope string) ([]*armauthorization.RoleAssignment, error)
}

type roleAssignmentsClient struct {
	cli *armauthorization.RoleAssignmentsClient
}

func (c *roleAssignmentsClient) ListRoleAssignments(ctx context.Context, scope string) ([]*armauthorization.RoleAssignment, error) {
	pager := c.cli.NewListForScopePager(scope, nil)
	roleDefs := make([]*armauthorization.RoleAssignment, 0, 128)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roleDefs = append(roleDefs, page.Value...)
	}
	return roleDefs, nil
}

func NewRoleAssignmentsClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (RoleAssignmentsClient, error) {
	clientFactory, err := armauthorization.NewClientFactory(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleDefCli := clientFactory.NewRoleAssignmentsClient()
	return &roleAssignmentsClient{cli: roleDefCli}, nil
}
