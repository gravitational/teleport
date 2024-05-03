package azureoidc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

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

func ProvisionTeleportSSO(ctx context.Context) error {
	graphClient, err := createGraphClient()
	if err != nil {
		return trace.Wrap(err)
	}
	me, err := graphClient.Me().Get(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.Info("graph call success", "me", me.GetDisplayName())

	// TODO(justinas): create required objects (enterprise app, etc.)

	return nil
}

type applicationSSOInfo struct {
	SSOApplication json.RawMessage `json:"sso_application"`
	FederatedSsoV2 json.RawMessage `json:"federated_sso_v2"`
}

func RetrieveTAGInfo(ctx context.Context) error {
	graphClient, err := createGraphClient()
	if err != nil {
		return trace.Wrap(err)
	}

	// Get information about enterprise apps
	appResp, err := graphClient.Applications().Get(ctx, nil)
	if err != nil {
		panic(err)
	}

	// Authorize to the private API
	tenantID, err := getTenantID()
	if err != nil {
		return trace.Wrap(err)
	}
	token, err := getPrivateAPIToken(ctx, tenantID)
	if err != nil {
		return trace.Wrap(err)
	}

	apps := make(map[string]applicationSSOInfo)

	for _, app := range appResp.GetValue() {
		appID := app.GetAppId()
		if appID == nil {
			slog.WarnContext(ctx, "app ID is nil", "app", app)
			continue
		}
		sp, err := graphClient.ServicePrincipalsWithAppId(appID).Get(ctx, nil)
		if err != nil {
			slog.ErrorContext(ctx, "could not retrieve service principal", "app_id", appID, "error", err)
		}
		spID := sp.GetId()
		if spID == nil {
			slog.WarnContext(ctx, "service principal ID is nil", "app_id", appID)
			continue
		}

		ssoApplication, err := privateAPIGet(ctx, token, path.Join("ApplicationSso", *spID, "SsoApplication"))
		if err != nil {
			slog.WarnContext(ctx, "getting SSO application info failed", "error", err)
		}

		federatedSSOV2, err := privateAPIGet(ctx, token, path.Join("ApplicationSso", *spID, "FederatedSsoV2"))
		if err != nil {
			slog.WarnContext(ctx, "getting federated SSO v2 info failed", "error", err)
		}

		apps[*appID] = applicationSSOInfo{
			SSOApplication: json.RawMessage(ssoApplication),
			FederatedSsoV2: json.RawMessage(federatedSSOV2),
		}
	}

	payload, err := json.Marshal(apps)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println(string(payload))
	return nil
}
