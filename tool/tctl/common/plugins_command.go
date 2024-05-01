/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"github.com/alecthomas/kingpin/v2"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
)

type installArgs struct {
	cmd  *kingpin.CmdClause
	name string
	scim scimArgs
}

type scimArgs struct {
	cmd           *kingpin.CmdClause
	samlConnector string
	token         string
}

// PluginsCommand allows for management of plugins.
type PluginsCommand struct {
	config     *servicecfg.Config
	cleanupCmd *kingpin.CmdClause
	pluginType string
	dryRun     bool
	install    installArgs
}

// Initialize creates the plugins command and subcommands
func (p *PluginsCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	p.config = config
	p.dryRun = true

	pluginsCommand := app.Command("plugins", "Manage Teleport plugins.").Hidden()

	p.cleanupCmd = pluginsCommand.Command("cleanup", "Cleans up the given plugin type.")
	p.cleanupCmd.Arg("type", "The type of plugin to cleanup. Only supports okta at present.").Required().EnumVar(&p.pluginType, string(types.PluginTypeOkta))
	p.cleanupCmd.Flag("dry-run", "Dry run the cleanup command. Dry run defaults to on.").Default("true").BoolVar(&p.dryRun)

	p.install.cmd = pluginsCommand.Command("install", "Install new plugin instance")

	p.install.scim.cmd = p.install.cmd.Command("scim", "Install a new SCIM integration")
	p.install.scim.cmd.
		Flag("name", "The name of the SCIM plugin resource to create").
		Default("scim").
		StringVar(&p.install.name)
	p.install.scim.cmd.
		Flag("saml-connector", "The name of the Teleport SAML connector users wil log in with.").
		Required().
		StringVar(&p.install.scim.samlConnector)
	p.install.scim.cmd.
		Flag("token", "The bearer token used by the SCIM client to authentcate").
		Required().
		Envar("SCIM_BEARER_TOKEN").
		StringVar(&p.install.scim.token)
}

// Cleanup cleans up the given plugin.
func (p *PluginsCommand) Cleanup(ctx context.Context, clusterAPI *auth.Client) error {
	needsCleanup, err := clusterAPI.PluginsClient().NeedsCleanup(ctx, &pluginsv1.NeedsCleanupRequest{
		Type: p.pluginType,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if needsCleanup.PluginActive {
		fmt.Printf("Plugin of type %q is currently active, can't cleanup!\n", p.pluginType)
		return nil
	}

	if !needsCleanup.NeedsCleanup {
		fmt.Printf("Plugin of type %q doesn't need a cleanup!\n", p.pluginType)
		return nil
	}

	if p.dryRun {
		fmt.Println("Would be deleting the following resources:")
	} else {
		fmt.Println("Deleting the following resources:")
	}

	for _, resource := range needsCleanup.ResourcesToCleanup {
		fmt.Printf("- %s\n", resource)
	}

	if p.dryRun {
		fmt.Println("Since dry run is indicated, nothing will be deleted. Run this command with --no-dry-run if you'd like to perform the cleanup.")
		return nil
	}

	if _, err := clusterAPI.PluginsClient().Cleanup(ctx, &pluginsv1.CleanupRequest{
		Type: p.pluginType,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully cleaned up resources for plugin %q\n", p.pluginType)

	return nil
}

func (p *PluginsCommand) InstallSCIM(ctx context.Context, client *auth.Client) error {
	scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(p.install.scim.token), bcrypt.DefaultCost)
	if err != nil {
		return trace.Wrap(err)
	}

	request := &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					types.HostedPluginLabel: "true",
				},
				Name: p.install.name,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Scim{
					Scim: &types.PluginSCIMSettings{
						SsoConnectorId: p.install.scim.samlConnector,
					},
				},
			},
		},
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: p.install.name,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: string(scimTokenHash),
				},
			},
		},
	}

	if _, err := client.PluginsClient().CreatePlugin(ctx, request); err != nil {
		fmt.Println("Failed to create SCIM itegration: %v", err)
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully created SCIM plugin %q\n", p.install.name)
	return nil
}

// TryRun runs the plugins command
func (p *PluginsCommand) TryRun(ctx context.Context, cmd string, client *auth.Client) (match bool, err error) {
	switch cmd {
	case p.cleanupCmd.FullCommand():
		err = p.Cleanup(ctx, client)
	case p.install.scim.cmd.FullCommand():
		err = p.InstallSCIM(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}
