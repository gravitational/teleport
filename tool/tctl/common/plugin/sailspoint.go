package plugin

import (
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/lib/utils"
)

func (p *PluginsCommand) initSCIMSailpoint(parent *kingpin.CmdClause) {
	p.install.scimSailpoint.cmd = parent.Command("sailpoint", "Install and configure a Teleport SCIM integration for SailPoint.")
	cmd := p.install.scimSailpoint.cmd

	cmd.Flag("connector", "Name of the Teleport SAML connector to use.").
		Required().
		StringVar(&p.install.scimSailpoint.samlConnector)

	cmd.Flag("plugin-name", "Name of SCIM Plugin to create.").
		Default("generic").
		StringVar(&p.install.scimSailpoint.pluginName)

}

func (p *PluginsCommand) InstallSCIMSailpoint(ctx context.Context, args installPluginArgs) error {
	scimArgs := p.install.scimSailpoint

	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: scimArgs.pluginName,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{
				Scim: &types.PluginSCIMSettings{
					SamlConnectorName: scimArgs.samlConnector,
				},
			},
		},
	}

	clientID, err := utils.CryptoRandomHex(16)
	if err != nil {
		return trace.Wrap(err)
	}
	clientSecret, err := utils.CryptoRandomHex(32)
	if err != nil {
		return trace.Wrap(err)
	}
	req := &pluginspb.CreatePluginRequest{
		Plugin: plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{
			buildOauthCreds(clientID, clientSecret),
		},
	}
	if _, err := args.plugins.CreatePlugin(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	pingResp, err := args.authClient.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	scimBaseURL := fmt.Sprintf("https://%s/v1/webapi/scim/%s", pingResp.GetProxyPublicAddr(), scimArgs.pluginName)
	scimTokenURL := fmt.Sprintf("https://%s/v1/webapi/plugin/%s/token", pingResp.GetProxyPublicAddr(), scimArgs.pluginName)

	fmt.Printf("\nSCIM SailPoint Plugin Installed Successfully\n")
	fmt.Println(" Base URL:        ", scimBaseURL)
	fmt.Println(" Client ID:       ", clientID)
	fmt.Println(" Client Secret:   ", clientSecret)
	fmt.Println(" Token URL:       ", scimTokenURL)
	return nil
}

func buildOauthCreds(clientID, clientSecret string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: uuid.NewString(),
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
			OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
				ClientId:     clientID,
				ClientSecret: clientSecret,
			},
		}},
	}
}
