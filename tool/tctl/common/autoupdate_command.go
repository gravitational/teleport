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

	"github.com/alecthomas/kingpin/v2"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

// getResponse is structure for formatting the client tools auto update response.
type getResponse struct {
	Mode          string `json:"mode"`
	TargetVersion string `json:"target_version"`
}

// AutoUpdateCommand implements the `tctl autoupdate` command for managing
// autoupdate process for tools and agents.
type AutoUpdateCommand struct {
	app    *kingpin.Application
	config *servicecfg.Config

	setCmd *kingpin.CmdClause
	getCmd *kingpin.CmdClause

	mode               string
	toolsTargetVersion string
	proxy              string
	format             string

	// stdout allows to switch standard output source for resource command. Used in tests.
	stdout io.Writer
}

// Initialize allows AutoUpdateCommand to plug itself into the CLI parser.
func (c *AutoUpdateCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.app = app
	c.config = config
	autoUpdateCmd := app.Command("autoupdate", "Teleport auto update commands.")

	clientToolsCmd := autoUpdateCmd.Command("client-tools", "Client tools auto update commands.")

	c.setCmd = clientToolsCmd.Command("set", "Sets client tools auto update configuration.")
	c.setCmd.Flag("mode", "Defines the mode to enable or disable tools auto update in cluster.").EnumVar(&c.mode, "enabled", "disabled", "on", "off")
	c.setCmd.Flag("target-version", "Defines client tools target version required to be updated.").StringVar(&c.toolsTargetVersion)

	c.getCmd = clientToolsCmd.Command("get", "Receive tools auto update target version.")
	c.getCmd.Flag("proxy", "Address of the Teleport proxy. When defined this address going to be used for requesting target version for auto update.").StringVar(&c.proxy)
	c.getCmd.Flag("format", "Output format: 'yaml' or 'json'").Default(teleport.YAML).StringVar(&c.format)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AutoUpdateCommand) TryRun(ctx context.Context, cmd string, client *authclient.Client) (match bool, err error) {
	switch cmd {
	case c.setCmd.FullCommand():
		err = c.Set(ctx, client)
	case c.getCmd.FullCommand():
		err = c.Get(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// Set creates or updates AutoUpdateConfig and AutoUpdateVersion resources with specified parameters.
func (c *AutoUpdateCommand) Set(ctx context.Context, client *authclient.Client) error {
	if c.mode != "" {
		if err := c.setAutoUpdateConfig(ctx, client); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.toolsTargetVersion != "" {
		if err := c.setAutoUpdateVersion(ctx, client); err != nil {
			return trace.Wrap(err)
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

	switch c.format {
	case teleport.JSON:
		if err := utils.WriteJSON(c.stdout, response); err != nil {
			return trace.Wrap(err)
		}
	case teleport.YAML:
		if err := utils.WriteYAML(c.stdout, response); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported output format %s, supported values are %s and %s", c.format, teleport.JSON, teleport.YAML)
	}

	return nil
}

func (c *AutoUpdateCommand) setAutoUpdateConfig(ctx context.Context, client *authclient.Client) error {
	configExists := true
	config, err := client.GetAutoUpdateConfig(ctx)
	if trace.IsNotFound(err) {
		configExists = false
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
		if configExists {
			if _, err := client.UpdateAutoUpdateConfig(ctx, config); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if _, err := client.CreateAutoUpdateConfig(ctx, config); err != nil {
				return trace.Wrap(err)
			}
		}
		fmt.Fprint(c.stdout, "client tools auto update mode has been set\n")
	}
	return nil
}

func (c *AutoUpdateCommand) setAutoUpdateVersion(ctx context.Context, client *authclient.Client) error {
	if _, err := semver.NewVersion(c.toolsTargetVersion); err != nil {
		return trace.WrapWithMessage(err, "not semantic version")
	}
	versionExists := true
	version, err := client.GetAutoUpdateVersion(ctx)
	if trace.IsNotFound(err) {
		versionExists = false
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
		if versionExists {
			if _, err := client.UpdateAutoUpdateVersion(ctx, version); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if _, err := client.CreateAutoUpdateVersion(ctx, version); err != nil {
				return trace.Wrap(err)
			}
		}
		fmt.Fprint(c.stdout, "client tools auto update target version has been set\n")
	}
	return nil
}

func (c *AutoUpdateCommand) get(ctx context.Context, client *authclient.Client) (*getResponse, error) {
	var response getResponse
	if c.proxy != "" {
		find, err := webclient.Find(&webclient.Config{Context: ctx, ProxyAddr: c.proxy, Insecure: true})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.TargetVersion = find.AutoUpdate.ToolsVersion
		response.Mode = autoupdate.ToolsUpdateModeDisabled
		if find.AutoUpdate.ToolsAutoUpdate {
			response.Mode = autoupdate.ToolsUpdateModeEnabled
		}
	} else {
		config, err := client.GetAutoUpdateConfig(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if config != nil && config.Spec.Tools != nil {
			response.Mode = config.Spec.Tools.Mode
		}

		version, err := client.GetAutoUpdateVersion(ctx)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if version != nil && version.Spec.Tools != nil {
			response.TargetVersion = version.Spec.Tools.TargetVersion
		}
	}

	return &response, nil
}
