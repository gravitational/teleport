package azureoidc

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// singleSignOnMode represents the possible values for `currentSingleSignOnMode` in `adSingleSignOn`
type singleSignOnMode string

const (
	// singleSignOnModeNone indicates that the application does not have SSO set up.
	singleSignOnModeNone singleSignOnMode = "none"
	// singleSignOnModeFederated indicates federated SSO such as SAML.
	singleSignOnModeFederated singleSignOnMode = "federated"
)

// adSingleSignOn represents the response from https://main.iam.ad.ext.azure.com/api/ApplicationSso/{servicePrincipalID}/SingleSignOn
type adSingleSignOn struct {
	CurrentSingleSignOnMode singleSignOnMode `json:"currentSingleSignOnMode"`
}

type applicationSSOInfo struct {
	// FederatedSsoV2 is the payload from the FederatedSsoV2 for this app, gzip compressed.
	FederatedSsoV2 []byte `json:"federated_sso_v2"`
}

func getSingleSignOn(ctx context.Context, token string, servicePrincipalID string) (*adSingleSignOn, error) {
	payload, err := privateAPIGet(ctx, token, path.Join("ApplicationSso", servicePrincipalID, "SingleSignOn"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var result adSingleSignOn
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, trace.Wrap(err, "failed to deserialize SingleSignOn")
	}

	return &result, nil
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
			slog.ErrorContext(ctx, "could not retrieve service principal", "app_id", *appID, "error", err)
		}
		spID := sp.GetId()
		if spID == nil {
			slog.WarnContext(ctx, "service principal ID is nil", "app_id", *appID)
			continue
		}

		sso, err := getSingleSignOn(ctx, token, *spID)
		if sso.CurrentSingleSignOnMode != singleSignOnModeFederated {
			slog.InfoContext(ctx, "sso not set up for app, will skip it", "app_id", *appID, "sp_id", *spID)
			continue
		}

		federatedSSOV2, err := privateAPIGet(ctx, token, path.Join("ApplicationSso", *spID, "FederatedSsoV2"))
		if err != nil {
			slog.WarnContext(ctx, "getting federated SSO v2 info failed", "error", err)
		}

		apps[*appID] = applicationSSOInfo{
			FederatedSsoV2: gzipBytes(federatedSSOV2),
		}
	}

	payload, err := json.Marshal(apps)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println(string(payload))
	return nil
}

// gzipBytes compresses the given byte slice, returning the result as a new byte slice.
func gzipBytes(src []byte) []byte {
	out := new(bytes.Buffer)
	writer := gzip.NewWriter(out)

	_, err := io.Copy(writer, bytes.NewReader(src))
	// We do not expect in-memory bytes I/O to fail.
	if err != nil {
		panic(err)
	}

	err = writer.Close()
	// We do not expect in-memory bytes I/O to fail.
	if err != nil {
		panic(err)
	}
	return out.Bytes()
}
