package plugins

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	auth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
)

// msGraphAPIScope is the OAuth scope for the Microsoft Graph API.
const msGraphAPIScope = "https://graph.microsoft.com/.default"

// entraIDInstanceFactory will create Entra ID services based on the plugin specification.
func entraIDInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	entraSpec := plugin.Spec.GetEntraId()
	if entraSpec == nil {
		return nil, trace.BadParameter("field Spec.EntraId must be present")
	}

	authServer := deps.parentProcess.GetAuthServer()
	integration, err := authServer.GetIntegration(ctx, plugin.GetName())
	if err != nil {
		// Emit a friendly status
		return nil, trace.Wrap(err)
	}
	azureSpec := integration.GetAzureOIDCIntegrationSpec()
	if azureSpec == nil {
		return nil, trace.BadParameter("expected %q to be an %q integration, was %q instead", integration.GetName(), types.IntegrationSubKindAzureOIDC, integration.GetSubKind())
	}

	return func() error {
		ctx := deps.lifetime

		getAssertion := func(ctx context.Context) (string, error) {
			token, err := azureoidc.GenerateEntraOIDCToken(ctx, authServer, authServer.GetKeyStore(), deps.parentProcess.Clock)
			if err != nil {
				return "", err
			}
			deps.log.Info("Entra token signed")
			return token, nil
		}

		graphClient, err := constructGraphClient(azureSpec.TenantID, azureSpec.ClientID, getAssertion)
		if err != nil {
			return trace.Wrap(err)
		}

		deps.log.Info("Entra ID plugin running")
		t := time.NewTicker(2 * time.Minute)
		defer t.Stop()
		for {
			func() {
				resp, err := graphClient.Users().Get(ctx, nil)
				if err != nil {
					deps.log.Error(err)
					return
				}
				if len(resp.GetValue()) == 0 {
					deps.log.Error("Users() response empty!")
				}
				deps.log.Infof("Entra request successful. First user: %+v", resp.GetValue()[0])
			}()
			select {
			case <-t.C:
			case <-deps.lifetime.Done():
				deps.log.Info("Entra ID plugin has stopped")
				return nil
			}
		}
	}, nil
}

// constructGraphClient returns a new MS Graph API client using the given function to retrieve the client assertion.
func constructGraphClient(tenantID string, clientID string, getAssertion func(context.Context) (string, error)) (*msgraphsdk.GraphServiceClient, error) {
	credential, err := azidentity.NewClientAssertionCredential(tenantID, clientID, getAssertion, nil)
	if err != nil {
		return nil, err
	}

	authProvider, err := auth.NewAzureIdentityAuthenticationProviderWithScopes(credential, []string{msGraphAPIScope})
	if err != nil {
		return nil, err
	}

	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return nil, err
	}

	return msgraphsdk.NewGraphServiceClient(adapter), nil
}
