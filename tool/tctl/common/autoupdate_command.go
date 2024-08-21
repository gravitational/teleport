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
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// versionResponse is structure for formatting the autoupdate version response.
type versionResponse struct {
	ToolsVersion string `json:"tools_version"`
}

// AutoUpdateCommand implements the `tctl autoupdate` command for managing
// autoupdate process for tools and agents.
type AutoUpdateCommand struct {
	app    *kingpin.Application
	config *servicecfg.Config

	updateCmd *kingpin.CmdClause
	getCmd    *kingpin.CmdClause
	watchCmd  *kingpin.CmdClause

	toolsAutoUpdate        string
	toolsAutoUpdateVersion string
	proxy                  string
}

// Initialize allows AutoUpdateCommand to plug itself into the CLI parser.
func (c *AutoUpdateCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.app = app
	c.config = config
	autoUpdateCmd := app.Command("autoupdate", "Teleport autoupdate commands.")

	c.updateCmd = autoUpdateCmd.Command("update", "Edit autoupdate configuration.")
	c.updateCmd.Flag("set-tools-auto-update", `Enable or disable tools autoupdate in cluster.`).EnumVar(&c.toolsAutoUpdate, "on", "off")
	c.updateCmd.Flag("set-tools-version", `Defines client tools version required to be force updated.`).StringVar(&c.toolsAutoUpdateVersion)

	c.getCmd = autoUpdateCmd.Command("get", "Receive tools autoupdate version.")
	c.getCmd.Flag("proxy", `URL of the proxy`).StringVar(&c.proxy)

	c.watchCmd = autoUpdateCmd.Command("watch", "Start monitoring autoupdate version updates.")
	c.watchCmd.Flag("proxy", `URL of the proxy`).StringVar(&c.proxy)
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AutoUpdateCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case c.updateCmd.FullCommand():
		err = c.Upsert(ctx, client)
	case c.getCmd.FullCommand():
		err = c.Get(ctx, client)
	case c.watchCmd.FullCommand():
		err = c.Watch(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// Upsert works with cluster_autoupdate_config and autoupdate_version resources to create or update
func (c *AutoUpdateCommand) Upsert(ctx context.Context, client *authclient.Client) error {
	serviceClient := client.AutoUpdateServiceClient()

	if c.toolsAutoUpdate != "" {
		config, err := serviceClient.GetClusterAutoUpdateConfig(ctx, &autoupdate.GetClusterAutoUpdateConfigRequest{})
		if trace.IsNotFound(err) {
			if config, err = update.NewClusterAutoUpdateConfig(&autoupdate.ClusterAutoUpdateConfigSpec{}); err != nil {
				return trace.Wrap(err)
			}
		} else if err != nil {
			return trace.Wrap(err)
		}
		if c.toolsAutoUpdate != config.Spec.ToolsAutoUpdate {
			config.Spec.ToolsAutoUpdate = c.toolsAutoUpdate
			if _, err := serviceClient.UpsertClusterAutoUpdateConfig(ctx, &autoupdate.UpsertClusterAutoUpdateConfigRequest{
				Config: config,
			}); err != nil {
				return trace.Wrap(err)
			}
			fmt.Println("cluster_autoupdate_config has been upserted")
		}
	}

	version, err := client.AutoUpdateServiceClient().GetAutoUpdateVersion(ctx, &autoupdate.GetAutoUpdateVersionRequest{})
	if trace.IsNotFound(err) {
		if version, err = update.NewAutoUpdateVersion(&autoupdate.AutoUpdateVersionSpec{}); err != nil {
			return trace.Wrap(err)
		}
	} else if err != nil {
		return trace.Wrap(err)
	}
	if version.Spec.ToolsVersion != c.toolsAutoUpdateVersion {
		version.Spec.ToolsVersion = c.toolsAutoUpdateVersion
		if _, err := serviceClient.UpsertAutoUpdateVersion(ctx, &autoupdate.UpsertAutoUpdateVersionRequest{
			Version: version,
		}); err != nil {
			return trace.Wrap(err)
		}
		fmt.Println("autoupdate_version has been upserted")
	}

	return nil
}

// Get makes request to fetch tools autoupdate version, if proxy flag is not set
// authorized handler should be used.
func (c *AutoUpdateCommand) Get(ctx context.Context, client *authclient.Client) error {
	response, err := c.get(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := utils.WriteJSON(os.Stdout, response); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Watch launch the watcher of the tools autoupdate version updates.
func (c *AutoUpdateCommand) Watch(ctx context.Context, client *authclient.Client) error {
	current := teleport.SemVersion
	ticker := interval.New(interval.Config{
		Duration:      time.Minute,
		FirstDuration: time.Second,
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.Next():
			response, err := c.get(ctx, client)
			if err != nil {
				return trace.Wrap(err)
			}
			if response.ToolsVersion == "" {
				continue
			}

			semVersion, err := semver.NewVersion(response.ToolsVersion)
			if err != nil {
				return trace.Wrap(err)
			}
			if !semVersion.Equal(*current) {
				if err := utils.WriteJSON(os.Stdout, response); err != nil {
					return trace.Wrap(err)
				}
				current = semVersion
			}
		}
	}
}

func (c *AutoUpdateCommand) get(ctx context.Context, client *authclient.Client) (*versionResponse, error) {
	var response versionResponse
	if c.proxy != "" {
		find, err := webclient.Find(&webclient.Config{Context: ctx, ProxyAddr: c.proxy, Insecure: true})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.ToolsVersion = find.ToolsVersion
	} else {
		version, err := client.AutoUpdateServiceClient().GetAutoUpdateVersion(ctx, &autoupdate.GetAutoUpdateVersionRequest{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.ToolsVersion = version.Spec.ToolsVersion
	}

	return &response, nil
}
