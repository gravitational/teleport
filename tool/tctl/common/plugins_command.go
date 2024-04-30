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
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/utils/io"
	"os"
)

// PluginsCommand allows for management of plugins.
type PluginsCommand struct {
	config     *servicecfg.Config
	cleanupCmd *kingpin.CmdClause
	installCmd *kingpin.CmdClause
	pluginType string
	dryRun     bool

	install struct {
		definitionFile string
		token          string
		bcryptToken    bool
	}
}

// Initialize creates the plugins command and subcommands
func (p *PluginsCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	p.config = config
	p.dryRun = true

	pluginsCommand := app.Command("plugins", "Manage Teleport plugins.").Hidden()

	p.cleanupCmd = pluginsCommand.Command("cleanup", "Cleans up the given plugin type.")
	p.cleanupCmd.Arg("type", "The type of plugin to cleanup. Only supports okta at present.").Required().EnumVar(&p.pluginType, string(types.PluginTypeOkta))
	p.cleanupCmd.Flag("dry-run", "Dry run the cleanup command. Dry run defaults to on.").Default("true").BoolVar(&p.dryRun)

	p.installCmd = pluginsCommand.Command("install", "Install new plugin instance")
	p.installCmd.Arg("filename", "File containing the plugin definition").
		Required().
		ExistingFileVar(&p.install.definitionFile)
	p.installCmd.Flag("api-token", "API token to install with plugin").
		StringVar(&p.install.token)
	p.installCmd.Flag("hash-token", "Hash the token before storage.").
		Default("false").
		BoolVar(&p.install.bcryptToken)
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

func (p *PluginsCommand) Install(ctx context.Context, client *auth.Client) error {
	loadPlugin(p.install.definitionFile)

	return nil
}

func loadPlugin(filename string) (types.Plugin, err) {
	var src io.Reader
	if filename == "" {
		src = os.Stdin
	} else {
		f, err := utils.OpenFileAllowingUnsafeLinks(filename)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				logrus.Debugf("Failed closing plugin definition file: %v", err)
			}
		}()
		src = f
	}

	text, err := io.ReadAtMost(src, 1024)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plugin, err := services.UnmarshalPlugin(text)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return plugin, nil
}

// TryRun runs the plugins command
func (p *PluginsCommand) TryRun(ctx context.Context, cmd string, client *auth.Client) (match bool, err error) {
	switch cmd {
	case p.cleanupCmd.FullCommand():
		err = p.Cleanup(ctx, client)
	case p.installCmd.FullCommand():
		err = p.Install(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}
