package plugin

import (
	"context"
	"errors"
	"fmt"
	"github.com/alecthomas/kingpin/v2"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	entraapiutils "github.com/gravitational/teleport/api/utils/entraid"
	"github.com/gravitational/trace"
	"strings"
)

var manualAzureConfigTemplate = `
` + bold("Step 1: Input Tenant ID and Client ID") + `
To finish the Azure integration, manually configure the Application in Microsoft Azure.
Follow the instructions provided in the Teleport documentation: [https://goteleport.com/docs/admin-guides/teleport-policy/integrations/azure/].
After completing the Azure setup, copy and paste the following information:
`

type azureArgs struct {
	cmd                  *kingpin.CmdClause
	authConnectorName    string
	defaultOwners        []string
	useSystemCredentials bool
	accessGraph          bool
	force                bool
	manualEntraIDSetup   bool
}

func (p *PluginsCommand) InstallAzure(ctx context.Context, args installPluginArgs) error {
	inputs := p.install

	proxyPublicAddr, err := getProxyPublicAddr(ctx, args.authClient)
	if err != nil {
		return trace.Wrap(err)
	}

	templates := entraSetupTemplates{
		step1:        step1Template,
		step2:        step2Template,
		manualConfig: manualAzureConfigTemplate,
	}
	settings, err := p.entraSetupGuide(proxyPublicAddr, inputs.entraID.manualEntraIDSetup, templates)
	if err != nil {
		if errors.Is(err, errCancel) {
			return nil
		}
		return trace.Wrap(err)
	}

	var tagSyncSettings *types.PluginEntraIDAccessGraphSettings
	if settings.accessGraphCache != nil {
		tagSyncSettings = &types.PluginEntraIDAccessGraphSettings{
			AppSsoSettingsCache: settings.accessGraphCache.AppSsoSettingsCache,
		}
	}

	// Prompt for admin action MFA if required, allowing reuse for UpsertToken and CreateBot.
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, args.authClient.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	saml, err := types.NewSAMLConnector(inputs.entraID.authConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: strings.TrimRight(proxyPublicAddr, "/") + "/v1/webapi/saml/acs/" + inputs.entraID.authConnectorName,
		AllowIDPInitiated:        true,
		// AttributesToRoles is required, but Entra ID does not have a default group (like Okta's "Everyone"),
		// so we add a dummy claim that will always be fulfilled and map them to the "requester" role.
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
				Value: "*",
				Roles: []string{"requester"},
			},
		},
		Display:             "Entra ID",
		EntityDescriptorURL: entraapiutils.FederationMetadataURL(settings.tenantID, settings.clientID),
	})
	if err != nil {
		return trace.Wrap(err, "failed to create SAML connector")
	}

	if _, err = args.authClient.CreateSAMLConnector(ctx, saml); err != nil {
		if !trace.IsAlreadyExists(err) || !inputs.entraID.force {
			return trace.Wrap(err, "failed to create SAML connector")
		}
		if _, err = args.authClient.UpsertSAMLConnector(ctx, saml); err != nil {
			return trace.Wrap(err, "failed to upsert SAML connector")
		}
	}

	if !inputs.entraID.useSystemCredentials {
		integrationSpec, err := types.NewIntegrationAzureOIDC(
			types.Metadata{Name: inputs.name},
			&types.AzureOIDCIntegrationSpecV1{
				TenantID: settings.tenantID,
				ClientID: settings.clientID,
			},
		)
		if err != nil {
			return trace.Wrap(err, "failed to create Azure OIDC integration")
		}

		if _, err = args.authClient.CreateIntegration(ctx, integrationSpec); err != nil {
			if !trace.IsAlreadyExists(err) || !inputs.entraID.force {
				return trace.Wrap(err, "failed to create Azure OIDC integration")
			}

			integration, err := args.authClient.GetIntegration(ctx, integrationSpec.GetName())
			if err != nil {
				return trace.Wrap(err, "failed to get Azure OIDC integration")
			}
			integration.SetAWSOIDCIntegrationSpec(integrationSpec.GetAWSOIDCIntegrationSpec())
			if _, err = args.authClient.UpdateIntegration(ctx, integration); err != nil {
				return trace.Wrap(err, "failed to create Azure OIDC integration")
			}
		}
	}

	credentialsSource := types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC
	if inputs.entraID.useSystemCredentials {
		credentialsSource = types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS
	}
	req := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: inputs.name,
				Labels: map[string]string{
					"teleport.dev/hosted-plugin": "true",
				},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_EntraId{
					EntraId: &types.PluginEntraIDSettings{
						SyncSettings: &types.PluginEntraIDSyncSettings{
							DefaultOwners:     inputs.entraID.defaultOwners,
							SsoConnectorId:    inputs.entraID.authConnectorName,
							CredentialsSource: credentialsSource,
							TenantId:          settings.tenantID,
							EntraAppId:        settings.clientID,
						},
						AccessGraphSettings: tagSyncSettings,
					},
				},
			},
		},
	}

	_, err = args.plugins.CreatePlugin(ctx, req)
	if err != nil {
		if !trace.IsAlreadyExists(err) || !inputs.entraID.force {
			return trace.Wrap(err)
		}
		plugin := req.GetPlugin()
		{
			oldPlugin, err := args.plugins.GetPlugin(ctx, &pluginspb.GetPluginRequest{
				Name: inputs.name,
			})
			if err != nil {
				return trace.Wrap(err)
			}
			plugin.Metadata.Revision = oldPlugin.GetMetadata().Revision
		}
		if _, err = args.plugins.UpdatePlugin(ctx, &pluginspb.UpdatePluginRequest{
			Plugin: plugin,
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Printf("Successfully created EntraID plugin %q\n\n", p.install.name)
	return nil
}
