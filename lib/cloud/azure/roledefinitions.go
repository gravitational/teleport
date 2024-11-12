package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/gravitational/trace"
)

type RoleDefinitionsClient struct {
	cli *armauthorization.RoleDefinitionsClient
}

func NewRoleDefinitionsClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (*RoleDefinitionsClient, error) {
	clientFactory, err := armauthorization.NewClientFactory(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleDefCli := clientFactory.NewRoleDefinitionsClient()
	return &RoleDefinitionsClient{cli: roleDefCli}, nil
}

func (c *RoleDefinitionsClient) ListRoleDefinitions(ctx context.Context, scope string) ([]*armauthorization.RoleDefinition, error) {
	pager := c.cli.NewListPager(scope, nil)
	roleDefs := make([]*armauthorization.RoleDefinition, 0, 128)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roleDefs = append(roleDefs, page.Value...)
	}
	return roleDefs, nil
}
