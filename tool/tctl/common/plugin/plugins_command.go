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

package plugin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

func logErrorMessage(err error) slog.Attr {
	const logFieldError = "err"
	return slog.String(logFieldError, err.Error())
}

type pluginInstallArgs struct {
	cmd     *kingpin.CmdClause
	name    string
	okta    oktaArgs
	scim    scimArgs
	entraID entraArgs
	netIQ   netIQArgs
	awsIC   awsICArgs
	github  githubArgs
}

type scimArgs struct {
	cmd           *kingpin.CmdClause
	connector     string
	connectorType string
	auth          string
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
func (p *PluginsCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	p.config = config
	p.dryRun = true

	pluginsCommand := app.Command("plugins", "Manage Teleport plugins.").Hidden()

	p.cleanupCmd = pluginsCommand.Command("cleanup", "Cleans up the given plugin type.")
	p.cleanupCmd.Arg("type", "The type of plugin to clean up. Only supports Okta at present.").Required().EnumVar(&p.pluginType, string(types.PluginTypeOkta))
	p.cleanupCmd.Flag("dry-run", "Dry run the cleanup command. Dry run defaults to on.").Default("true").BoolVar(&p.dryRun)

	p.initInstall(pluginsCommand, config)
	p.initDelete(pluginsCommand)
}

func (p *PluginsCommand) initInstall(parent *kingpin.CmdClause, config *servicecfg.Config) {
	p.install.cmd = parent.Command("install", "Install new plugin instance")

	p.initInstallOkta(p.install.cmd)
	p.initInstallSCIM(p.install.cmd)
	p.initInstallEntra(p.install.cmd)
	p.initInstallNetIQ(p.install.cmd)
	p.initInstallAWSIC(p.install.cmd)
	p.initInstallGithub(p.install.cmd)
}

func (p *PluginsCommand) initDelete(parent *kingpin.CmdClause) {
	p.delete.cmd = parent.Command("delete", "Remove a plugin instance.")
	p.delete.cmd.
		Arg("name", "The name of the SCIM plugin resource to delete").
		StringVar(&p.delete.name)
}

// Delete implements `tctl plugins delete`, deleting a plugin from the Teleport cluster
func (p *PluginsCommand) Delete(ctx context.Context, args installPluginArgs) error {
	log := p.config.Logger.With("plugin", p.delete.name)

	req := &pluginsv1.DeletePluginRequest{Name: p.delete.name}
	if _, err := args.plugins.DeletePlugin(ctx, req); err != nil {
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
func (p *PluginsCommand) Cleanup(ctx context.Context, args installPluginArgs) error {
	needsCleanup, err := args.plugins.NeedsCleanup(ctx, &pluginsv1.NeedsCleanupRequest{
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

	if _, err := args.plugins.Cleanup(ctx, &pluginsv1.CleanupRequest{
		Type: p.pluginType,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully cleaned up resources for plugin %q\n", p.pluginType)

	return nil
}

type authClient interface {
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
	CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error)
	UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error)
	CreateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error)
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
	UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error)
	Ping(ctx context.Context) (proto.PingResponse, error)
	PerformMFACeremony(ctx context.Context, challengeRequest *proto.CreateAuthenticateChallengeRequest, promptOpts ...mfa.PromptOpt) (*proto.MFAAuthenticateResponse, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
}

type pluginsClient interface {
	CreatePlugin(ctx context.Context, in *pluginsv1.CreatePluginRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	GetPlugin(ctx context.Context, in *pluginsv1.GetPluginRequest, opts ...grpc.CallOption) (*types.PluginV1, error)
	UpdatePlugin(ctx context.Context, in *pluginsv1.UpdatePluginRequest, opts ...grpc.CallOption) (*types.PluginV1, error)
	NeedsCleanup(ctx context.Context, in *pluginsv1.NeedsCleanupRequest, opts ...grpc.CallOption) (*pluginsv1.NeedsCleanupResponse, error)
	Cleanup(ctx context.Context, in *pluginsv1.CleanupRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	DeletePlugin(ctx context.Context, in *pluginsv1.DeletePluginRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type installPluginArgs struct {
	authClient authClient
	plugins    pluginsClient
}

// TryRun runs the plugins command
func (p *PluginsCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, args installPluginArgs) error
	switch cmd {
	case p.cleanupCmd.FullCommand():
		commandFunc = p.Cleanup
	case p.install.okta.cmd.FullCommand():
		commandFunc = p.InstallOkta
	case p.install.scim.cmd.FullCommand():
		commandFunc = p.InstallSCIM
	case p.install.entraID.cmd.FullCommand():
		commandFunc = p.InstallEntra
	case p.install.netIQ.cmd.FullCommand():
		commandFunc = p.InstallNetIQ
	case p.install.awsIC.cmd.FullCommand():
		commandFunc = p.InstallAWSIC
	case p.install.github.cmd.FullCommand():
		commandFunc = p.InstallGithub
	case p.delete.cmd.FullCommand():
		commandFunc = p.Delete
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, installPluginArgs{authClient: client, plugins: client.PluginsClient()})
	closeFn(ctx)

	return true, trace.Wrap(err)
}
