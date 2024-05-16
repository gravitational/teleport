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
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const (
	logFieldPlugin        = "plugin"
	logFieldSAMLConnector = "saml_connector"
	logFieldRole          = "role"
)

func logErrorMessage(err error) slog.Attr {
	const logFieldError = "err"
	return slog.String(logFieldError, err.Error())
}

type pluginInstallArgs struct {
	cmd  *kingpin.CmdClause
	name string
	scim scimArgs
}

type scimArgs struct {
	cmd           *kingpin.CmdClause
	samlConnector string
	token         string
	role          string
	force         bool
}

type pluginDeleteArgs struct {
	cmd  *kingpin.CmdClause
	name string
}

// PluginsCommand allows for management of plugins.
type PluginsCommand struct {
	config     *servicecfg.Config
	cleanupCmd *kingpin.CmdClause
	pluginType string
	dryRun     bool
	install    pluginInstallArgs
	delete     pluginDeleteArgs
}

// Initialize creates the plugins command and subcommands
func (p *PluginsCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	p.config = config
	p.dryRun = true

	pluginsCommand := app.Command("plugins", "Manage Teleport plugins.").Hidden()

	p.cleanupCmd = pluginsCommand.Command("cleanup", "Cleans up the given plugin type.")
	p.cleanupCmd.Arg("type", "The type of plugin to cleanup. Only supports okta at present.").Required().EnumVar(&p.pluginType, string(types.PluginTypeOkta))
	p.cleanupCmd.Flag("dry-run", "Dry run the cleanup command. Dry run defaults to on.").Default("true").BoolVar(&p.dryRun)

	p.initInstall(pluginsCommand)
	p.initDelete(pluginsCommand)
}

func (p *PluginsCommand) initInstall(parent *kingpin.CmdClause) {
	p.install.cmd = parent.Command("install", "Install new plugin instance")

	p.install.scim.cmd = p.install.cmd.Command("scim", "Install a new SCIM integration")
	p.install.scim.cmd.
		Flag("name", "The name of the SCIM plugin resource to create").
		Default("scim").
		StringVar(&p.install.name)
	p.install.scim.cmd.
		Flag("saml-connector", "The name of the Teleport SAML connector users will log in with.").
		Required().
		StringVar(&p.install.scim.samlConnector)
	p.install.scim.cmd.
		Flag("role", "The Teleport role to assign users created by the plugin").
		Short('r').
		Default(teleport.PresetRequesterRoleName).
		StringVar(&p.install.scim.role)
	p.install.scim.cmd.
		Flag("token", "The bearer token used by the SCIM client to authenticate").
		Short('t').
		Required().
		Envar("TELEPORT_SCIM_BEARER_TOKEN").
		StringVar(&p.install.scim.token)
	p.install.scim.cmd.
		Flag("force", "Proceed with installation even if validation fails").
		Short('f').
		Default("false").
		BoolVar(&p.install.scim.force)

}

func (p *PluginsCommand) initDelete(parent *kingpin.CmdClause) {
	p.delete.cmd = parent.Command("delete", "Remove a plugin instance")
	p.delete.cmd.
		Arg("name", "The name of the SCIM plugin resource to delete").
		StringVar(&p.delete.name)
}

// Delete implements `tctl plugins delete`, deleting a plugin from the Teleport cluster
func (p *PluginsCommand) Delete(ctx context.Context, client *authclient.Client) error {
	log := p.config.Logger.With("plugin", p.delete.name)
	plugins := client.PluginsClient()

	req := &pluginsv1.DeletePluginRequest{Name: p.delete.name}
	if _, err := plugins.DeletePlugin(ctx, req); err != nil {
		if trace.IsNotFound(err) {
			log.InfoContext(ctx, "Plugin not found")
			return nil
		}
		log.ErrorContext(ctx, "Failed deleting plugin", logErrorMessage(err))
		return trace.Wrap(err)
	}
	return nil
}

// Cleanup cleans up the given plugin.
func (p *PluginsCommand) Cleanup(ctx context.Context, clusterAPI *authclient.Client) error {
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

// InstallSCIM implements `tctl plugins install scim`, installing a SCIM integration
// plugin into the teleport cluster
func (p *PluginsCommand) InstallSCIM(ctx context.Context, client *authclient.Client) error {
	log := p.config.Logger.With(logFieldPlugin, p.install.name)

	log.DebugContext(ctx, "Fetching cluster info...")
	info, err := client.Ping(ctx)
	if err != nil {
		log.ErrorContext(ctx, "Failed fetching cluster info", logErrorMessage(err))
		return trace.Wrap(err)
	}

	scimBaseURL := fmt.Sprintf("https://%s/v1/webapi/scim/%s", info.ProxyPublicAddr, p.install.name)

	scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(p.install.scim.token), bcrypt.DefaultCost)
	if err != nil {
		return trace.Wrap(err)
	}

	connectorID := p.install.scim.samlConnector
	log.DebugContext(ctx, "Validating SAML Connector...", logFieldSAMLConnector, connectorID)
	connector, err := client.GetSAMLConnector(ctx, p.install.scim.samlConnector, false)
	if err != nil {
		log.ErrorContext(ctx, "Failed validating SAML connector",
			slog.String(logFieldSAMLConnector, connectorID),
			logErrorMessage(err))

		if !p.install.scim.force {
			return trace.Wrap(err)
		}
	}

	role := p.install.scim.role
	log.DebugContext(ctx, "Validating Default Role...", logFieldRole, role)
	if _, err := client.GetRole(ctx, role); err != nil {
		log.ErrorContext(ctx, "Failed validating role", slog.String(logFieldRole, role), logErrorMessage(err))
		if !p.install.scim.force {
			return trace.Wrap(err)
		}
	}

	request := &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindProvisioning,
			Metadata: types.Metadata{
				Labels: map[string]string{
					types.HostedPluginLabel: "true",
					types.SCIMBaseURLLabel:  scimBaseURL,
				},
				Name: p.install.name,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Scim{
					Scim: &types.PluginSCIMSettings{
						SamlConnectorName: p.install.scim.samlConnector,
						DefaultRole:       p.install.scim.role,
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

	log.DebugContext(ctx, "Creating SCIM Plugin...")
	if _, err := client.PluginsClient().CreatePlugin(ctx, request); err != nil {
		log.ErrorContext(ctx, "Failed to create SCIM integration", logErrorMessage(err))
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully created SCIM plugin %q\n", p.install.name)
	fmt.Printf("SCIM base URL: %s\n", scimBaseURL)

	if connector == nil {
		return nil
	}

	switch connector.Origin() {
	case types.OriginOkta:
		fmt.Println("Follow this guide to configure SCIM provisioning on Okta side: https://goteleport.com/docs/application-access/okta/hosted-guide/#configuring-scim-provisioning")
	}
	return nil
}

// TryRun runs the plugins command
func (p *PluginsCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case p.cleanupCmd.FullCommand():
		err = p.Cleanup(ctx, client)
	case p.install.scim.cmd.FullCommand():
		err = p.InstallSCIM(ctx, client)
	case p.delete.cmd.FullCommand():
		err = p.Delete(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}
