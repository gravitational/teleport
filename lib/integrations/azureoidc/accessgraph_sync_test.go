package azureoidc

import (
	"bytes"
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/gravitational/teleport/lib/msgraph"
	"testing"
)

type mockAzureConfigClient struct {
}

func (c *mockAzureConfigClient) CreateRoleDefinition(ctx context.Context, scope string, roleDefinition armauthorization.RoleDefinition) (string, error) {
	roleId := "foo"
	return roleId, nil
}

func (c *mockAzureConfigClient) CreateRoleAssignment(ctx context.Context, scope string, roleAssignment armauthorization.RoleAssignmentCreateParameters) error {
	return nil
}

func (c *mockAzureConfigClient) GetServicePrincipalByAppId(ctx context.Context, appId string) (*msgraph.ServicePrincipal, error) {
	spId := "foo"
	appRoleValue := "bar"
	return &msgraph.ServicePrincipal{
		DirectoryObject: msgraph.DirectoryObject{
			ID: &spId,
		},
		AppRoles: []*msgraph.AppRole{
			{
				ID:    &appId,
				Value: &appRoleValue,
			},
		},
	}, nil
}

func (c *mockAzureConfigClient) GrantAppRoleToServicePrincipal(ctx context.Context, roleAssignment msgraph.AppRoleAssignment) error {
	return nil
}

func TestAccessGraphAzureConfigOutput(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	req := AccessGraphAzureConfigureRequest{
		ManagedIdentity: "foo",
		RoleName:        "bar",
		SubscriptionID:  "1234567890",
		AutoConfirm:     true,
		stdout:          &buf,
	}
	clt := &mockAzureConfigClient{}
	err := ConfigureAccessGraphSyncAzure(ctx, clt, req)
	if err != nil {
		return
	}
}
