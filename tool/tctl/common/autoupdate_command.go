/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// maxRetries is the default number of RPC call retries to prevent parallel create/update errors.
const maxRetries = 3

// AutoUpdateCommand implements the `tctl autoupdate` command for managing
// autoupdate process for tools and agents.
type AutoUpdateCommand struct {
	app *kingpin.Application
	ccf *tctlcfg.GlobalCLIFlags

	targetCmd  *kingpin.CmdClause
	enableCmd  *kingpin.CmdClause
	disableCmd *kingpin.CmdClause
	statusCmd  *kingpin.CmdClause

	toolsTargetVersion string
	proxy              string
	format             string

	clear bool

	// stdout allows to switch standard output source for resource command. Used in tests.
	stdout io.Writer
}

// Initialize allows AutoUpdateCommand to plug itself into the CLI parser.
func (c *AutoUpdateCommand) Initialize(app *kingpin.Application, ccf *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	c.app = app
	c.ccf = ccf
	autoUpdateCmd := app.Command("autoupdate", "Manage auto update configuration.")

	clientToolsCmd := autoUpdateCmd.Command("client-tools", "Manage client tools auto update configuration.")

	c.statusCmd = clientToolsCmd.Command("status", "Prints if the client tools updates are enabled/disabled, and the target version in specified format.")
	c.statusCmd.Flag("proxy", "Address of the Teleport proxy. When defined this address will be used to retrieve client tools auto update configuration.").StringVar(&c.proxy)
	c.statusCmd.Flag("format", "Output format: 'yaml' or 'json'").Default(teleport.YAML).StringVar(&c.format)

	c.enableCmd = clientToolsCmd.Command("enable", "Enables client tools auto updates. Clients will be told to update to the target version.")
	c.disableCmd = clientToolsCmd.Command("disable", "Disables client tools auto updates. Clients will not be told to update to the target version.")

	c.targetCmd = clientToolsCmd.Command("target", "Sets the client tools target version. This command is not supported on Teleport Cloud.")
	c.targetCmd.Arg("version", "Client tools target version. Clients will be told to update to this version.").StringVar(&c.toolsTargetVersion)
	c.targetCmd.Flag("clear", "Removes the target version, Teleport will default to its current proxy version.").BoolVar(&c.clear)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AutoUpdateCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch {
	case cmd == c.targetCmd.FullCommand():
		commandFunc = c.TargetVersion
	case cmd == c.enableCmd.FullCommand():
		commandFunc = c.SetModeCommand(true)
	case cmd == c.disableCmd.FullCommand():
		commandFunc = c.SetModeCommand(false)
	case c.proxy == "" && cmd == c.statusCmd.FullCommand():
		commandFunc = c.Status
	case c.proxy != "" && cmd == c.statusCmd.FullCommand():
		err = c.StatusByProxy(ctx)
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

// TargetVersion creates or updates AutoUpdateVersion resource with client tools target version.
func (c *AutoUpdateCommand) TargetVersion(ctx context.Context, client *authclient.Client) error {
	var err error
	switch {
	case c.clear:
		err = c.clearTargetVersion(ctx, client)
	case c.toolsTargetVersion != "":
		// For parallel requests where we attempt to create a resource simultaneously, retries should be implemented.
		// The same approach applies to updates if the resource has been deleted during the process.
		// Second create request must return `AlreadyExists` error, update for deleted resource `NotFound` error.
		for i := 0; i < maxRetries; i++ {
			err = c.setTargetVersion(ctx, client)
			if err == nil {
				break
			}
			if !trace.IsNotFound(err) && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
	}
	return trace.Wrap(err)
}

// SetModeCommand returns a command to enable or disable client tools auto-updates in the cluster.
func (c *AutoUpdateCommand) SetModeCommand(enabled bool) func(ctx context.Context, client *authclient.Client) error {
	return func(ctx context.Context, client *authclient.Client) error {
		// For parallel requests where we attempt to create a resource simultaneously, retries should be implemented.
		// The same approach applies to updates if the resource has been deleted during the process.
		// Second create request must return `AlreadyExists` error, update for deleted resource `NotFound` error.
		for i := 0; i < maxRetries; i++ {
			err := c.setMode(ctx, client, enabled)
			if err == nil {
				break
			}
			if !trace.IsNotFound(err) && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
		return nil
	}
}

// getResponse is structure for formatting the client tools auto update response.
type getResponse struct {
	Mode          string `json:"mode"`
	TargetVersion string `json:"target_version"`
}

// Status makes request to auth service to fetch client tools auto update version and mode.
func (c *AutoUpdateCommand) Status(ctx context.Context, client *authclient.Client) error {
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

// StatusByProxy makes request to `webapi/find` endpoint to fetch tools auto update version and mode
// without authentication.
func (c *AutoUpdateCommand) StatusByProxy(ctx context.Context) error {
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

func (c *AutoUpdateCommand) setMode(ctx context.Context, client *authclient.Client, enabled bool) error {
	setMode := client.UpdateAutoUpdateConfig
	config, err := client.GetAutoUpdateConfig(ctx)
	if trace.IsNotFound(err) {
		if config, err = autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{}); err != nil {
			return trace.Wrap(err)
		}
		setMode = client.CreateAutoUpdateConfig
	} else if err != nil {
		return trace.Wrap(err)
	}

	if config.Spec.Tools == nil {
		config.Spec.Tools = &autoupdatev1pb.AutoUpdateConfigSpecTools{}
	}

	config.Spec.Tools.Mode = autoupdate.ToolsUpdateModeDisabled
	if enabled {
		config.Spec.Tools.Mode = autoupdate.ToolsUpdateModeEnabled
	}
	if _, err := setMode(ctx, config); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(c.stdout, "client tools auto update mode has been changed")

	return nil
}

func (c *AutoUpdateCommand) setTargetVersion(ctx context.Context, client *authclient.Client) error {
	if _, err := semver.NewVersion(c.toolsTargetVersion); err != nil {
		return trace.WrapWithMessage(err, "not semantic version")
	}
	setTargetVersion := client.UpdateAutoUpdateVersion
	version, err := client.GetAutoUpdateVersion(ctx)
	if trace.IsNotFound(err) {
		if version, err = autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{}); err != nil {
			return trace.Wrap(err)
		}
		setTargetVersion = client.CreateAutoUpdateVersion
	} else if err != nil {
		return trace.Wrap(err)
	}
	if version.Spec.Tools == nil {
		version.Spec.Tools = &autoupdatev1pb.AutoUpdateVersionSpecTools{}
	}
	if version.Spec.Tools.TargetVersion != c.toolsTargetVersion {
		version.Spec.Tools.TargetVersion = c.toolsTargetVersion
		if _, err := setTargetVersion(ctx, version); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(c.stdout, "client tools auto update target version has been set")
	}
	return nil
}

func (c *AutoUpdateCommand) clearTargetVersion(ctx context.Context, client *authclient.Client) error {
	version, err := client.GetAutoUpdateVersion(ctx)
	if trace.IsNotFound(err) {
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}
	if version.Spec.Tools != nil {
		version.Spec.Tools = nil
		if _, err := client.UpdateAutoUpdateVersion(ctx, version); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(c.stdout, "client tools auto update target version has been cleared")
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
