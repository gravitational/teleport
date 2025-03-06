package plugin

import (
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	libcommon "github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/plugins"
)

func (p *PluginsCommand) initSCIMSailpoint(parent *kingpin.CmdClause) {
	p.install.scimSailpoint.cmd = parent.Command("sailpoint", "Install an SCIM SailPoint integration.")
	cmd := p.install.scimSailpoint.cmd

	cmd.
		Flag("teleport-connector-name", "Teleport connector name").
		StringVar(&p.install.scimSailpoint.samlConnector)
}

func (s scimArgs) validate() error {
	return nil
}

func (p *PluginsCommand) InstallSCIMSailpoint(ctx context.Context, args installPluginArgs) error {
	scimArgs := p.install.scimSailpoint
	if err := scimArgs.validate(); err != nil {
		return trace.Wrap(err)
	}

	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: "sailpoint",
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{
				Scim: &types.PluginSCIMSettings{
					SamlConnectorName: scimArgs.samlConnector,
				},
			},
		},
	}

	rawToken := uuid.NewString()
	hashedToken, err := bcrypt.GenerateFromPassword([]byte(rawToken), bcrypt.DefaultCost)
	if err != nil {
		return trace.Wrap(err)
	}
	req := &pluginspb.CreatePluginRequest{
		Plugin: plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{
			BuildSCIMCredentials(string(hashedToken)),
		},
	}
	if _, err := args.plugins.CreatePlugin(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	pingResp, err := args.authClient.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	scimBaseURL := fmt.Sprintf("https://%s/v1/webapi/scim/%s", pingResp.GetProxyPublicAddr(), "sailpoint")

	fmt.Printf("SCIM Base URL: %s\n", scimBaseURL)
	fmt.Printf("SCIM Bearer Token: %s\n", rawToken)
	return nil
}

func BuildSCIMCredentials(scimToken string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   "scim-generic-token-name" + uuid.NewString(),
				Labels: map[string]string{libcommon.CredPurposeLabel: libcommon.CredPurposeSCIMToken},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{APIToken: scimToken}},
	}
}
