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
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/api/client/proto"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	bearerAuthType = "bearer"
	oauthAuthType  = "oauth"
)

var allAuthTypes = []string{
	bearerAuthType,
	oauthAuthType,
}

var supportedConnectors = []string{
	types.KindOIDC,
	types.KindSAML,
}

func (p *PluginsCommand) initInstallSCIM(parent *kingpin.CmdClause) {
	p.install.scim.cmd = parent.Command("scim", "Install a Teleport SCIM plugin.")
	cmd := p.install.scim.cmd

	cmd.Flag("connector", "Name of the Teleport connector to use.").
		Required().
		StringVar(&p.install.scim.connector)

	cmd.Flag("connector-type", "Type of the Teleport connector to use.").
		EnumVar(&p.install.scim.connectorType, supportedConnectors...)

	cmd.Flag("auth", "Plugin Authentication type.").
		Default(oauthAuthType).
		EnumVar(&p.install.scim.auth, allAuthTypes...)
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
					ConnectorInfo: &types.PluginSCIMSettings_ConnectorInfo{
						Name: scimArgs.connector,
						Type: scimArgs.connectorType,
					},
				},
			},
		},
	}

	creds, err := generateSCIMCredentials(scimArgs.auth)
	if err != nil {
		return trace.Wrap(err)
	}
	req := &pluginspb.CreatePluginRequest{
		Plugin:                plugin,
		StaticCredentialsList: []*types.PluginStaticCredentialsV1{creds.PluginStaticCredentialsV1},
	}
	if _, err := args.plugins.CreatePlugin(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	pingResp, err := args.authClient.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("\nSCIM Plugin Installed Successfully\n")
	if err := printSCIMIntegrationInfo(creds, pingResp, pluginName); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func printSCIMIntegrationInfo(creds *credsWrapper, pingResp proto.PingResponse, pluginName string) error {
	scimBaseURL := fmt.Sprintf("https://%s/v1/webapi/scim/%s", pingResp.GetProxyPublicAddr(), pluginName)
	fmt.Println(" Base URL:              ", scimBaseURL)

	switch t := creds.Spec.Credentials.(type) {
	case *types.PluginStaticCredentialsSpecV1_OAuthClientSecret:
		scimTokenURL := fmt.Sprintf("https://%s/v1/webapi/plugin/%s/token", pingResp.GetProxyPublicAddr(), pluginName)
		fmt.Println(" OAuth Token URL:       ", scimTokenURL)
		fmt.Println(" OAuth Client ID:       ", t.OAuthClientSecret.ClientId)
		fmt.Println(" OAuth Client Secret:   ", t.OAuthClientSecret.ClientSecret)
	case *types.PluginStaticCredentialsSpecV1_APIToken:
		fmt.Println(" API Bearer Token:      ", creds.rawToken)
	default:
		return trace.BadParameter("unsupported credentials type %T", creds.Spec.Credentials)
	}
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

func buildBearerCreds(token string) *types.PluginStaticCredentialsV1 {
	return &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: fmt.Sprintf("%s-%s", types.PluginTypeSCIM, uuid.NewString()),
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: token,
			},
		},
	}
}

type credsWrapper struct {
	*types.PluginStaticCredentialsV1
	// rawToken is the raw bearer token generated for the SCIM plugin
	// in case of PluginStaticCredentialsSpecV1_APIToken.
	// Note that value of APIToken is bcrypt hashed, so the raw token
	// is used ot printed to the user.
	rawToken string
}

func generateSCIMCredentials(authType string) (*credsWrapper, error) {
	switch authType {
	case oauthAuthType:
		clientID, err := utils.CryptoRandomHex(16)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clientSecret, err := utils.CryptoRandomHex(32)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &credsWrapper{
			PluginStaticCredentialsV1: buildOauthCreds(clientID, clientSecret),
		}, nil
	case bearerAuthType:
		bearerToken, err := utils.CryptoRandomHex(32)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		hashedToken, err := bcrypt.GenerateFromPassword([]byte(bearerToken), bcrypt.DefaultCost)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &credsWrapper{
			PluginStaticCredentialsV1: buildBearerCreds(string(hashedToken)),
			rawToken:                  bearerToken,
		}, nil
	default:
		return nil, trace.BadParameter("unsupported auth type %q", authType)
	}
}
