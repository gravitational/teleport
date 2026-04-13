package client

import "github.com/gravitational/teleport/api/client"

// TerraformClient is a wrapper that contains necessary APIs required by the terraform provider.
type TerraformClient struct {
	*AccessClient
	*client.Client
}

// NewTerraformClient creates a new terraform client based on api/client.
func NewTerraformClient(client *client.Client) *TerraformClient {
	return &TerraformClient{
		AccessClient: NewAccessClient(client.ScopedAccessServiceClient()),
		Client:       client,
	}
}
