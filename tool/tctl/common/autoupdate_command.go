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
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/asciitable"
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

	toolsTargetCmd  *kingpin.CmdClause
	toolsEnableCmd  *kingpin.CmdClause
	toolsDisableCmd *kingpin.CmdClause
	toolsStatusCmd  *kingpin.CmdClause
	agentsStatusCmd *kingpin.CmdClause

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

	c.toolsStatusCmd = clientToolsCmd.Command("status", "Prints if the client tools updates are enabled/disabled, and the target version in specified format.")
	c.toolsStatusCmd.Flag("proxy", "Address of the Teleport proxy. When defined this address will be used to retrieve client tools auto update configuration.").StringVar(&c.proxy)
	c.toolsStatusCmd.Flag("format", "Output format: 'yaml' or 'json'").Default(teleport.YAML).StringVar(&c.format)

	c.toolsEnableCmd = clientToolsCmd.Command("enable", "Enables client tools auto updates. Clients will be told to update to the target version.")
	c.toolsDisableCmd = clientToolsCmd.Command("disable", "Disables client tools auto updates. Clients will not be told to update to the target version.")

	c.toolsTargetCmd = clientToolsCmd.Command("target", "Sets the client tools target version. This command is not supported on Teleport Cloud.")
	c.toolsTargetCmd.Arg("version", "Client tools target version. Clients will be told to update to this version.").StringVar(&c.toolsTargetVersion)
	c.toolsTargetCmd.Flag("clear", "removes the target version, Teleport will default to its current proxy version.").BoolVar(&c.clear)

	agentsCmd := autoUpdateCmd.Command("agents", "Manage agents auto update configuration.")
	c.agentsStatusCmd = agentsCmd.Command("status", "Prints agents auto update status.")

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AutoUpdateCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client autoupdateClient) error
	switch {
	case cmd == c.toolsTargetCmd.FullCommand():
		commandFunc = c.TargetVersion
	case cmd == c.toolsEnableCmd.FullCommand():
		commandFunc = c.SetModeCommand(true)
	case cmd == c.toolsDisableCmd.FullCommand():
		commandFunc = c.SetModeCommand(false)
	case c.proxy == "" && cmd == c.toolsStatusCmd.FullCommand():
		commandFunc = c.ToolsStatus
	case c.proxy != "" && cmd == c.toolsStatusCmd.FullCommand():
		err = c.ToolsStatusByProxy(ctx)
		return true, trace.Wrap(err)
	case cmd == c.agentsStatusCmd.FullCommand():
		commandFunc = c.agentsStatusCommand
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
func (c *AutoUpdateCommand) TargetVersion(ctx context.Context, client autoupdateClient) error {
	var err error
	switch {
	case c.clear:
		err = c.clearToolsTargetVersion(ctx, client)
	case c.toolsTargetVersion != "":
		// For parallel requests where we attempt to create a resource simultaneously, retries should be implemented.
		// The same approach applies to updates if the resource has been deleted during the process.
		// Second create request must return `AlreadyExists` error, update for deleted resource `NotFound` error.
		for i := 0; i < maxRetries; i++ {
			err = c.setToolsTargetVersion(ctx, client)
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
func (c *AutoUpdateCommand) SetModeCommand(enabled bool) func(ctx context.Context, client autoupdateClient) error {
	return func(ctx context.Context, client autoupdateClient) error {
		// For parallel requests where we attempt to create a resource simultaneously, retries should be implemented.
		// The same approach applies to updates if the resource has been deleted during the process.
		// Second create request must return `AlreadyExists` error, update for deleted resource `NotFound` error.
		for i := 0; i < maxRetries; i++ {
			err := c.setToolsMode(ctx, client, enabled)
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

// autoupdateClient is a subset of the Teleport client, with functions used to interact with automatic update resources.
// Not every AU function is part of the interface, we'll add them as we need.
type autoupdateClient interface {
	GetAutoUpdateAgentRollout(context.Context) (*autoupdatev1pb.AutoUpdateAgentRollout, error)
	GetAutoUpdateVersion(context.Context) (*autoupdatev1pb.AutoUpdateVersion, error)
	GetAutoUpdateConfig(context.Context) (*autoupdatev1pb.AutoUpdateConfig, error)
	CreateAutoUpdateConfig(context.Context, *autoupdatev1pb.AutoUpdateConfig) (*autoupdatev1pb.AutoUpdateConfig, error)
	CreateAutoUpdateVersion(context.Context, *autoupdatev1pb.AutoUpdateVersion) (*autoupdatev1pb.AutoUpdateVersion, error)
	UpdateAutoUpdateConfig(context.Context, *autoupdatev1pb.AutoUpdateConfig) (*autoupdatev1pb.AutoUpdateConfig, error)
	UpdateAutoUpdateVersion(context.Context, *autoupdatev1pb.AutoUpdateVersion) (*autoupdatev1pb.AutoUpdateVersion, error)
}

func (c *AutoUpdateCommand) agentsStatusCommand(ctx context.Context, client autoupdateClient) error {
	rollout, err := client.GetAutoUpdateAgentRollout(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	sb := strings.Builder{}
	if rollout.GetSpec() == nil {
		sb.WriteString("No active agent rollout (autoupdate_agent_rollout).\n")
	}
	if mode := rollout.GetSpec().GetAutoupdateMode(); mode != "" {
		sb.WriteString("Agent autoupdate mode: " + mode + "\n")
	}
	if st := formatTimeIfNotEmpty(rollout.GetStatus().GetStartTime().AsTime(), time.DateTime); st != "" {
		sb.WriteString("Rollout creation date: " + st + "\n")
	}
	if start := rollout.GetSpec().GetStartVersion(); start != "" {
		sb.WriteString("Start version: " + start + "\n")
	}
	if target := rollout.GetSpec().GetTargetVersion(); target != "" {
		sb.WriteString("Target version: " + target + "\n")
	}
	if state := rollout.GetStatus().GetState(); state != autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_UNSPECIFIED {
		sb.WriteString("Rollout state: " + userFriendlyState(state) + "\n")
	}
	if schedule := rollout.GetSpec().GetSchedule(); schedule == autoupdate.AgentsScheduleImmediate {
		sb.WriteString("Schedule is immediate. Every group immediately updates to the target version.\n")
	}
	if strategy := rollout.GetSpec().GetStrategy(); strategy != "" {
		sb.WriteString("Strategy: " + strategy + "\n")
	}

	if groups := rollout.GetStatus().GetGroups(); len(groups) > 0 {
		sb.WriteRune('\n')
		headers := []string{"Group Name", "State", "Start Time", "State Reason"}
		table := asciitable.MakeTable(headers)
		for _, group := range groups {
			table.AddRow([]string{
				group.GetName(),
				userFriendlyState(group.GetState()),
				formatTimeIfNotEmpty(group.GetStartTime().AsTime(), time.DateTime),
				group.GetLastUpdateReason()})
		}
		sb.Write(table.AsBuffer().Bytes())
	}

	fmt.Fprint(c.stdout, sb.String())
	return nil
}

func formatTimeIfNotEmpty(t time.Time, format string) string {
	if t.IsZero() || t.Unix() == 0 {
		return ""
	}
	return t.Format(format)
}

func userFriendlyState[T autoupdatev1pb.AutoUpdateAgentGroupState | autoupdatev1pb.AutoUpdateAgentRolloutState](state T) string {
	switch state {
	case 0:
		return "Unknown"
	case 1:
		return "Unstarted"
	case 2:
		return "Active"
	case 3:
		return "Done"
	case 4:
		return "Rolledback"
	default:
		// If we don't know anything about this state, we display its integer
		return fmt.Sprintf("Unknown state (%d)", state)
	}
}

// ToolsStatus makes request to auth service to fetch client tools auto update version and mode.
func (c *AutoUpdateCommand) ToolsStatus(ctx context.Context, client autoupdateClient) error {
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

	return c.printToolsResponse(response)
}

// ToolsStatusByProxy makes request to `webapi/find` endpoint to fetch tools auto update version and mode
// without authentication.
func (c *AutoUpdateCommand) ToolsStatusByProxy(ctx context.Context) error {
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
	return c.printToolsResponse(getResponse{
		TargetVersion: find.AutoUpdate.ToolsVersion,
		Mode:          mode,
	})
}

func (c *AutoUpdateCommand) setToolsMode(ctx context.Context, client autoupdateClient, enabled bool) error {
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

func (c *AutoUpdateCommand) setToolsTargetVersion(ctx context.Context, client autoupdateClient) error {
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

func (c *AutoUpdateCommand) clearToolsTargetVersion(ctx context.Context, client autoupdateClient) error {
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

func (c *AutoUpdateCommand) printToolsResponse(response getResponse) error {
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
