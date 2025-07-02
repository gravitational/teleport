/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package plugin

import (
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func (p *PluginsCommand) initInstallSCIM(parent *kingpin.CmdClause) {
	p.install.scim.cmd = parent.Command("scim", "Install a Teleport SCIM plugin.")
	cmd := p.install.scim.cmd

	cmd.Flag("connector", "Name of the Teleport SAML connector to use.").
		Required().
		StringVar(&p.install.scim.samlConnector)
}

// InstallSCIM implements `tctl plugins install scim`, installing a SCIM integration
// plugin into the teleport cluster
func (p *PluginsCommand) InstallSCIM(ctx context.Context, args installPluginArgs) error {
	scimArgs := p.install.scim

	pluginName := types.PluginTypeSCIM
	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				"teleport.dev/hosted-plugin": "true",
			},
			Name: pluginName,
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

	scimBaseURL := fmt.Sprintf("https://%s/v1/webapi/scim/%s", pingResp.GetProxyPublicAddr(), pluginName)
	scimTokenURL := fmt.Sprintf("https://%s/v1/webapi/plugin/%s/token", pingResp.GetProxyPublicAddr(), pluginName)

	fmt.Printf("\nSCIM Plugin Installed Successfully\n")
	fmt.Println(" Base URL:        ", scimBaseURL)
	fmt.Println(" OAuth Client ID:       ", clientID)
	fmt.Println(" OAuth Client Secret:   ", clientSecret)
	fmt.Println(" OAuth Token URL:       ", scimTokenURL)
	return nil
}

func buildOauthCreds(clientID, clientSecret string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: fmt.Sprintf("%s-%s", types.PluginTypeSCIM, uuid.NewString()),
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
