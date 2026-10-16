/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

type intuneArgs struct {
	cmd                  *kingpin.CmdClause
	tenant               string
	clientID             string
	useSystemCredentials bool
	loginEndpoint        string
	graphEndpoint        string
	clientSecret         string
}

func (p *PluginsCommand) initInstallIntune(parent *kingpin.CmdClause) {
	p.install.intune.cmd = parent.Command("intune", "Install a Microsoft Intune integration.")
	cmd := p.install.intune.cmd

	cmd.
		Flag("tenant", "Primary domain name (e.g. contoso.onmicrosoft.com) or tenant ID of the Microsoft Entra ID organization.").
		Required().
		StringVar(&p.install.intune.tenant)

	cmd.
		Flag("use-system-credentials", "Uses credentials available in the environment of the Auth Service (e.g. Azure Workload Identity) instead of a client secret.").
		BoolVar(&p.install.intune.useSystemCredentials)

	cmd.
		Flag("client-id", "Application (client) ID of the Microsoft Entra ID app registration. Required unless --use-system-credentials is set.").
		StringVar(&p.install.intune.clientID)

	cmd.
		Flag("login-endpoint", "Microsoft Entra ID login endpoint.").
		Default(types.MSGraphDefaultLoginEndpoint).
		StringVar(&p.install.intune.loginEndpoint)

	cmd.
		Flag("graph-endpoint", "Microsoft Graph endpoint.").
		Default(types.MSGraphDefaultEndpoint).
		StringVar(&p.install.intune.graphEndpoint)
}

// InstallIntune implements `tctl plugins install intune`.
func (p *PluginsCommand) InstallIntune(ctx context.Context, args pluginServices) error {
	if p.stdout == nil {
		p.stdout = os.Stdout
	}

	createPluginRequest, err := p.buildIntuneCreatePluginRequest(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := args.plugins.CreatePlugin(ctx, createPluginRequest); err != nil {
		return trace.Wrap(err, "failed to create Intune plugin")
	}

	fmt.Fprintln(p.stdout, "Intune plugin has been successfully installed.")
	return nil
}

func (p *PluginsCommand) buildIntuneCreatePluginRequest(ctx context.Context) (*pluginspb.CreatePluginRequest, error) {
	inputs := p.install.intune

	intuneSettings := &types.PluginIntuneSettings{
		Tenant:            inputs.tenant,
		LoginEndpoint:     inputs.loginEndpoint,
		GraphEndpoint:     inputs.graphEndpoint,
		CredentialsSource: types.IntuneCredentialsSource_INTUNE_CREDENTIALS_SOURCE_CLIENT_SECRET,
	}
	createPluginRequest := pluginspb.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindMDM,
			Metadata: types.Metadata{
				Name: types.PluginTypeIntune,
				Labels: map[string]string{
					types.HostedPluginLabel: "true",
				},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Intune{
					Intune: intuneSettings,
				},
			},
		},
	}

	if inputs.useSystemCredentials {
		if inputs.clientID != "" {
			return nil, trace.BadParameter("--client-id cannot be used together with --use-system-credentials")
		}
		intuneSettings.CredentialsSource = types.IntuneCredentialsSource_INTUNE_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS
		return createPluginRequest.Build(), nil
	}

	if inputs.clientID == "" {
		return nil, trace.BadParameter("--client-id is required unless --use-system-credentials is set")
	}
	clientSecret := inputs.clientSecret
	if clientSecret == "" {
		var err error
		clientSecret, err = promptForPassword(ctx, p.stdout, "Enter the client secret of the Microsoft Entra ID app registration")
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if clientSecret == "" {
		return nil, trace.BadParameter("client secret required")
	}

	createPluginRequest.StaticCredentials = &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: types.PluginTypeIntune,
				Labels: map[string]string{
					"intune/tenant": inputs.tenant,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
				OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
					ClientId:     inputs.clientID,
					ClientSecret: clientSecret,
				},
			},
		},
	}
	return createPluginRequest.Build(), nil
}
