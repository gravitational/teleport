package azureoidc

import (
	"bytes"
	"context"
	"testing"
)

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
	err := ConfigureAccessGraphSyncAzure(ctx, req)
	if err != nil {
		return
	}
}
