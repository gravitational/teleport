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
	"net/url"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type installArgs struct {
	cmd  *kingpin.CmdClause
	name string
	okta oktaArgs
}

type oktaArgs struct {
	cmd            *kingpin.CmdClause
	org            *url.URL
	appID          string
	samlConnector  string
	apiToken       string
	scimToken      string
	userSync       bool
	accessListSync bool
	defaultOwners  []string
	appFilters     []string
	groupFilters   []string
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

	p.install.okta.cmd = p.install.cmd.Command("okta", "Install an okta integration")
	p.install.okta.cmd.
		Flag("name", "name of the plugin resource to create").
		Default("okta").
		StringVar(&p.install.name)
	p.install.okta.cmd.
		Flag("org", "URL of Okta organization").
		Required().
		URLVar(&p.install.okta.org)
	p.install.okta.cmd.
		Flag("api-token", "API token to install with plugin").
		Required().
		StringVar(&p.install.okta.apiToken)
	p.install.okta.cmd.
		Flag("saml-connector", "SAML connector to link to plugin").
		StringVar(&p.install.okta.samlConnector)
	p.install.okta.cmd.
		Flag("app-id", "Okta ID of the APP used for SSO via SAML").
		StringVar(&p.install.okta.appID)
	p.install.okta.cmd.
		Flag("scim-token", "SCIM token to install with plugin").
		StringVar(&p.install.okta.scimToken)
	p.install.okta.cmd.
		Flag("sync-users", "Enable user synchronization").
		Default("true").
		BoolVar(&p.install.okta.userSync)
	p.install.okta.cmd.
		Flag("owner", "Set default owners for synced AccessLists").
		Short('o').
		StringsVar(&p.install.okta.defaultOwners)
	p.install.okta.cmd.
		Flag("sync-groups", "Enable group to AccessList synchronization").
		Default("true").
		BoolVar(&p.install.okta.accessListSync)
	p.install.okta.cmd.
		Flag("group", "Add a group filter. Supports globbing by default. Enclose in `^pattern$` for full regex support.").
		Short('g').
		StringsVar(&p.install.okta.groupFilters)
	p.install.okta.cmd.
		Flag("app", "Add an app filter. Supports globbing by default. Enclose in `^pattern$` for full regex support.").
		Short('a').
		StringsVar(&p.install.okta.appFilters)
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

type samlConnectorsClient interface {
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
}

type pluginsClient interface {
	CreatePlugin(ctx context.Context, in *pluginsv1.CreatePluginRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type installPluginArgs struct {
	samlConnectors samlConnectorsClient
	plugins        pluginsClient
}

func (p *PluginsCommand) InstallOkta(ctx context.Context, args installPluginArgs) error {
	oktaSettings := p.install.okta

	if oktaSettings.accessListSync {
		if len(oktaSettings.defaultOwners) == 0 {
			return trace.BadParameter("AccessList sync requires at least one default owner to be set")
		}
	}

	if oktaSettings.scimToken != "" {
		if len(oktaSettings.defaultOwners) == 0 {
			return trace.BadParameter("SCIM support requires at least one default owner to be set")
		}

		if oktaSettings.appID == "" {
			return trace.BadParameter("SCIM support requires app-id to be set")
		}

		if oktaSettings.samlConnector == "" {
			return trace.BadParameter("SCIM support requires saml-connector to be set")
		}
	}

	if oktaSettings.samlConnector != "" {
		if err := validateSAMLConnector(ctx, args.samlConnectors, oktaSettings.samlConnector); err != nil {
			fmt.Printf("Failed validating SAML connector: %v", err)
			return trace.Wrap(err)
		}
	}

	creds := []*types.PluginStaticCredentialsV1{
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: p.install.name,
					Labels: map[string]string{
						types.OktaCredPurposeLabel: types.OktaCredPurposeAuth,
					},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: oktaSettings.apiToken,
				},
			},
		},
	}

	if oktaSettings.scimToken != "" {
		scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(oktaSettings.scimToken), bcrypt.DefaultCost)
		if err != nil {
			return trace.Wrap(err)
		}

		creds = append(creds, &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: p.install.name + "-scim-token",
					Labels: map[string]string{
						types.OktaCredPurposeLabel: types.OktaCredPurposeSCIMToken,
					},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: string(scimTokenHash),
				},
			},
		})
	}

	req := &pluginsv1.CreatePluginRequest{
		Plugin: &types.PluginV1{
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Labels: map[string]string{
					types.HostedPluginLabel: "true",
				},
				Name: p.install.name,
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{
					Okta: &types.PluginOktaSettings{
						OrgUrl: oktaSettings.org.String(),
						SyncSettings: &types.PluginOktaSyncSettings{
							SsoConnectorId:  oktaSettings.samlConnector,
							AppId:           oktaSettings.appID,
							SyncUsers:       oktaSettings.userSync,
							SyncAccessLists: oktaSettings.accessListSync,
							DefaultOwners:   oktaSettings.defaultOwners,
							GroupFilters:    oktaSettings.groupFilters,
							AppFilters:      oktaSettings.appFilters,
						},
					},
				},
			},
		},
		StaticCredentialsList: creds,
		CredentialLabels: map[string]string{
			types.OktaOrgURLLabel: oktaSettings.org.String(),
		},
	}

	if _, err := args.plugins.CreatePlugin(ctx, req); err != nil {
		fmt.Printf("Plugin creation failed: %v", err)
		return trace.Wrap(err)
	}

	return nil
}

func validateSAMLConnector(ctx context.Context, samlConnectors samlConnectorsClient, name string) error {
	_, err := samlConnectors.GetSAMLConnector(ctx, name, false)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// TryRun runs the plugins command
func (p *PluginsCommand) TryRun(ctx context.Context, cmd string, client *auth.Client) (match bool, err error) {
	switch cmd {
	case p.cleanupCmd.FullCommand():
		err = p.Cleanup(ctx, client)
	case p.install.okta.cmd.FullCommand():
		args := installPluginArgs{samlConnectors: client, plugins: client.PluginsClient()}
		err = p.InstallOkta(ctx, args)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}
