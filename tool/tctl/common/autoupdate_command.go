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
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// getResponse is structure for formatting the client tools auto update response.
type getResponse struct {
	Mode          string `json:"mode"`
	TargetVersion string `json:"target_version"`
}

// AutoUpdateCommand implements the `tctl autoupdate` command for managing
// autoupdate process for tools and agents.
type AutoUpdateCommand struct {
	app *kingpin.Application
	ccf *tctlcfg.GlobalCLIFlags

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
func (c *AutoUpdateCommand) Initialize(app *kingpin.Application, ccf *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	c.app = app
	c.ccf = ccf
	autoUpdateCmd := app.Command("autoupdate", "Manage auto update configuration.")

	clientToolsCmd := autoUpdateCmd.Command("client-tools", "Manage client tools auto update configuration.")

	c.setCmd = clientToolsCmd.Command("set", "Modifies client tools auto update configuration.")
	c.setCmd.Flag("mode", "Specifies whether client tools auto updates are enabled for the cluster.").EnumVar(&c.mode, "on", "off")
	c.setCmd.Flag("target-version", "Defines client tools target version required to be updated.").StringVar(&c.toolsTargetVersion)

	c.getCmd = clientToolsCmd.Command("get", "Retrieve client tools auto update configuration.")
	c.getCmd.Flag("proxy", "Address of the Teleport proxy. When defined this address will be used to retrieve client tools auto update configuration.").StringVar(&c.proxy)
	c.getCmd.Flag("format", "Output format: 'yaml' or 'json'").Default(teleport.YAML).StringVar(&c.format)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AutoUpdateCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch {
	case cmd == c.setCmd.FullCommand():
		commandFunc = c.Set
	case
		c.proxy == "" && cmd == c.getCmd.FullCommand():
		commandFunc = c.Get
	case c.proxy != "" && cmd == c.getCmd.FullCommand():
		err = c.GetByProxy(ctx)
		return true, trace.Wrap(err)
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

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

// Get makes request to auth service to fetch tools autoupdate version.
func (c *AutoUpdateCommand) Get(ctx context.Context, client *authclient.Client) error {
	var response getResponse
	config, err := client.GetAutoUpdateConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if config != nil && config.Spec.Tools != nil {
		response.Mode = config.Spec.Tools.Mode
	}

	version, err := client.GetAutoUpdateVersion(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if version != nil && version.Spec.Tools != nil {
		response.TargetVersion = version.Spec.Tools.TargetVersion
	}

	return c.printResponse(response)
}

// GetByProxy makes request to find endpoint without auth to fetch tools autoupdate version.
func (c *AutoUpdateCommand) GetByProxy(ctx context.Context) error {
	find, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: c.proxy,
		Insecure:  c.ccf.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	mode := autoupdate.ToolsUpdateModeDisabled
	if find.AutoUpdate.ToolsAutoUpdate {
		mode = autoupdate.ToolsUpdateModeEnabled
	}
	return c.printResponse(getResponse{
		TargetVersion: find.AutoUpdate.ToolsVersion,
		Mode:          mode,
	})
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
	mode := autoupdate.ToolsUpdateModeDisabled
	if c.mode == "on" {
		mode = autoupdate.ToolsUpdateModeEnabled
	}
	if config.Spec.Tools == nil {
		config.Spec.Tools = &autoupdatev1pb.AutoUpdateConfigSpecTools{}
	}
	if config.Spec.Tools.Mode != mode {
		config.Spec.Tools.Mode = mode
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

func (c *AutoUpdateCommand) printResponse(response getResponse) error {
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
