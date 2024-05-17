package azureoidc

import (
	"bytes"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// createGraphClient creates a new graph client from ambient credentials (Azure CLI credentials cache).
func createGraphClient() (*msgraphsdk.GraphServiceClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an auth provider using the credential
	authProvider, err := auth.NewAzureIdentityAuthenticationProviderWithScopes(credential, []string{
		"https://graph.microsoft.com/.default",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a request adapter using the auth provider
	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a Graph client using request adapter
	return msgraphsdk.NewGraphServiceClient(adapter), nil
}

func getAzureDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(usr.HomeDir, ".azure"), nil
}

type azureCLIProfile struct {
	Subscriptions []azureCLISubscription `json:"subscriptions"`
}

type azureCLISubscription struct {
	TenantID string `json:"tenantID"`
}

// getTenantID infers the Azure tenant ID from the Azure CLI profiles.
func getTenantID() (string, error) {
	azureDir, err := getAzureDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	payload, err := os.ReadFile(filepath.Join(azureDir, "azureProfile.json"))
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Remove UTF-8 BOM
	payload = bytes.TrimPrefix(payload, []byte("\xef\xbb\xbf"))

	var profile azureCLIProfile
	if err := json.Unmarshal(payload, &profile); err != nil {
		return "", trace.Wrap(err)
	}
	if len(profile.Subscriptions) == 0 {
		return "", trace.BadParameter("subscription not found")
	}
	return profile.Subscriptions[0].TenantID, nil

}
