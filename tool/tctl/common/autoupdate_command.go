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
	"io"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// versionResponse is structure for formatting the autoupdate version response.
type versionResponse struct {
	TargetVersion string `json:"target_version"`
}

// AutoUpdateCommand implements the `tctl autoupdate` command for managing
// autoupdate process for tools and agents.
type AutoUpdateCommand struct {
	app    *kingpin.Application
	config *servicecfg.Config

	configureCmd *kingpin.CmdClause
	getCmd       *kingpin.CmdClause
	watchCmd     *kingpin.CmdClause

	mode               string
	toolsTargetVersion string
	proxy              string

	// stdout allows to switch standard output source for resource command. Used in tests.
	stdout io.Writer
}

// Initialize allows AutoUpdateCommand to plug itself into the CLI parser.
func (c *AutoUpdateCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.app = app
	c.config = config
	autoUpdateCmd := app.Command("autoupdate", "Teleport auto update commands.")

	clientToolsCmd := autoUpdateCmd.Command("client-tools", "Client tools auto update commands.")

	c.configureCmd = clientToolsCmd.Command("configure", "Edit client tools auto update configuration.")
	c.configureCmd.Flag("set-mode", "Sets the mode to enable or disable tools auto update in cluster.").EnumVar(&c.mode, "enabled", "disabled", "on", "off")
	c.configureCmd.Flag("set-target-version", "Defines client tools target version required to be updated.").StringVar(&c.toolsTargetVersion)

	c.getCmd = clientToolsCmd.Command("get", "Receive tools auto update target version.")
	c.getCmd.Flag("proxy", "Address of the Teleport proxy. When defined this address going to be used for requesting target version for auto update.").StringVar(&c.proxy)

	c.watchCmd = clientToolsCmd.Command("watch", "Start monitoring auto update target version updates.")
	c.watchCmd.Flag("proxy", "Address of the Teleport proxy. When defined this address going to be used for requesting target version for auto update.").StringVar(&c.proxy)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AutoUpdateCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case c.configureCmd.FullCommand():
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

// Upsert works with AutoUpdateConfig and AutoUpdateVersion resources to create or update.
func (c *AutoUpdateCommand) Upsert(ctx context.Context, client *authclient.Client) error {
	if c.mode != "" {
		config, err := client.GetAutoUpdateConfig(ctx)
		if trace.IsNotFound(err) {
			if config, err = autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{}); err != nil {
				return trace.Wrap(err)
			}
		} else if err != nil {
			return trace.Wrap(err)
		}
		switch c.mode {
		case "on":
			c.mode = autoupdate.ToolsUpdateModeEnabled
		case "off":
			c.mode = autoupdate.ToolsUpdateModeDisabled
		}
		if config.Spec.Tools == nil {
			config.Spec.Tools = &autoupdatev1pb.AutoUpdateConfigSpecTools{}
		}
		if config.Spec.Tools.Mode != c.mode {
			config.Spec.Tools.Mode = c.mode
			if _, err := client.UpsertAutoUpdateConfig(ctx, config); err != nil {
				return trace.Wrap(err)
			}
			fmt.Fprint(c.stdout, "autoupdate_config has been upserted\n")
		}
	}

	if c.toolsTargetVersion != "" {
		version, err := client.GetAutoUpdateVersion(ctx)
		if trace.IsNotFound(err) {
			if version, err = autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{}); err != nil {
				return trace.Wrap(err)
			}
		} else if err != nil {
			return trace.Wrap(err)
		}
		if version.Spec.Tools == nil {
			version.Spec.Tools = &autoupdatev1pb.AutoUpdateVersionSpecTools{}
		}
		if version.Spec.Tools.TargetVersion != c.toolsTargetVersion {
			version.Spec.Tools.TargetVersion = c.toolsTargetVersion
			if _, err := client.UpsertAutoUpdateVersion(ctx, version); err != nil {
				return trace.Wrap(err)
			}
			fmt.Fprint(c.stdout, "autoupdate_version has been upserted\n")
		}
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

	if err := utils.WriteJSON(c.stdout, response); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Watch launch the watcher of the tools auto update target version updates to pull the version
// every minute.
func (c *AutoUpdateCommand) Watch(ctx context.Context, client *authclient.Client) error {
	var current semver.Version
	ticker := interval.New(interval.Config{
		Duration: time.Minute,
	})
	defer ticker.Stop()

	for {
		response, err := c.get(ctx, client)
		if err != nil {
			return trace.Wrap(err)
		}
		if response.TargetVersion != "" {
			semVersion, err := semver.NewVersion(response.TargetVersion)
			if err != nil {
				return trace.Wrap(err)
			}
			if !semVersion.Equal(current) {
				if err := utils.WriteJSON(c.stdout, response); err != nil {
					return trace.Wrap(err)
				}
				current = *semVersion
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.Next():
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
		response.TargetVersion = find.AutoUpdate.ToolsVersion
	} else {
		version, err := client.GetAutoUpdateVersion(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if version.Spec.Tools != nil {
			response.TargetVersion = version.Spec.Tools.TargetVersion
		}
	}

	return &response, nil
}
