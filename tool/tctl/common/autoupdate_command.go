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
	"maps"
	"os"
	"slices"
	"strconv"
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
	aur "github.com/gravitational/teleport/lib/autoupdate/report"
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

	toolsTargetCmd       *kingpin.CmdClause
	toolsEnableCmd       *kingpin.CmdClause
	toolsDisableCmd      *kingpin.CmdClause
	toolsStatusCmd       *kingpin.CmdClause
	agentsStatusCmd      *kingpin.CmdClause
	agentsReportCmd      *kingpin.CmdClause
	agentsStartUpdateCmd *kingpin.CmdClause
	agentsMarkDoneCmd    *kingpin.CmdClause
	agentsRollbackCmd    *kingpin.CmdClause

	toolsTargetVersion string
	proxy              string
	format             string
	groups             []string

	clear bool
	force bool

	// used for testing purposes
	now func() time.Time

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
	c.toolsTargetCmd.Flag("clear", "Removes the target version, Teleport will default to its current proxy version.").BoolVar(&c.clear)

	agentsCmd := autoUpdateCmd.Command("agents", "Manage agents auto update configuration.")
	c.agentsStatusCmd = agentsCmd.Command("status", "Prints agents auto update status.")
	c.agentsStatusCmd.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.agentsReportCmd = agentsCmd.Command("report", "Aggregates the agent autoupdate reports and displays agent count per version and per update group.")
	c.agentsReportCmd.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.agentsStartUpdateCmd = agentsCmd.Command("start-update", "Starts updating one or many groups.")
	c.agentsStartUpdateCmd.Arg("groups", "Groups to start updating.").StringsVar(&c.groups)
	c.agentsStartUpdateCmd.Flag("force", "Skips progressive deployment mechanism such as canaries or backpressure.").BoolVar(&c.force)
	c.agentsMarkDoneCmd = agentsCmd.Command("mark-done", "Marks one or many groups as done updating.")
	c.agentsMarkDoneCmd.Arg("groups", "Groups to mark as done updating.").StringsVar(&c.groups)
	c.agentsRollbackCmd = agentsCmd.Command("rollback", "Rolls back one or many groups.")
	c.agentsRollbackCmd.Arg("groups", "Groups to rollback. When empty, every group already started is rolled back.").StringsVar(&c.groups)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	if c.now == nil {
		c.now = time.Now
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
	case cmd == c.agentsReportCmd.FullCommand():
		commandFunc = c.agentsReportCommand
	case cmd == c.agentsStartUpdateCmd.FullCommand():
		commandFunc = c.agentsStartUpdateCommand
	case cmd == c.agentsMarkDoneCmd.FullCommand():
		commandFunc = c.agentsMarkDoneCommand
	case cmd == c.agentsRollbackCmd.FullCommand():
		commandFunc = c.agentsRollbackCommand
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
		for range maxRetries {
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
		for range maxRetries {
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
	TriggerAutoUpdateAgentGroup(ctx context.Context, groups []string, state autoupdatev1pb.AutoUpdateAgentGroupState) (*autoupdatev1pb.AutoUpdateAgentRollout, error)
	ForceAutoUpdateAgentGroup(ctx context.Context, groups []string) (*autoupdatev1pb.AutoUpdateAgentRollout, error)
	RollbackAutoUpdateAgentGroup(ctx context.Context, groups []string, allStartedGroups bool) (*autoupdatev1pb.AutoUpdateAgentRollout, error)
	ListAutoUpdateAgentReports(ctx context.Context, pageSize int, pageToken string) ([]*autoupdatev1pb.AutoUpdateAgentReport, string, error)
}

func (c *AutoUpdateCommand) agentsStatusCommand(ctx context.Context, client autoupdateClient) error {
	rollout, err := client.GetAutoUpdateAgentRollout(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	switch c.format {
	case teleport.Text:
		return trace.Wrap(c.writeAgentsStatusText(rollout))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSON(c.stdout, newAgentStatusOutput(rollout)))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(c.stdout, newAgentStatusOutput(rollout)))
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}
}

func (c *AutoUpdateCommand) writeAgentsStatusText(rollout *autoupdatev1pb.AutoUpdateAgentRollout) error {
	sb := strings.Builder{}
	if rollout.GetSpec() == nil {
		sb.WriteString("No active agent rollout (autoupdate_agent_rollout).\n")
	}
	if mode := rollout.GetSpec().GetAutoupdateMode(); mode != "" {
		sb.WriteString("Agent autoupdate mode: ")
		sb.WriteString(mode)
		sb.WriteString("\n")
	}
	if st := formatTimeIfNotEmpty(rollout.GetStatus().GetStartTime().AsTime(), time.DateTime); st != "" {
		sb.WriteString("Rollout creation date: ")
		sb.WriteString(st)
		sb.WriteString("\n")
	}
	if start := rollout.GetSpec().GetStartVersion(); start != "" {
		sb.WriteString("Start version: ")
		sb.WriteString(start)
		sb.WriteString("\n")
	}
	if target := rollout.GetSpec().GetTargetVersion(); target != "" {
		sb.WriteString("Target version: ")
		sb.WriteString(target)
		sb.WriteString("\n")
	}
	if state := rollout.GetStatus().GetState(); state != autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_UNSPECIFIED {
		sb.WriteString("Rollout state: ")
		sb.WriteString(aur.UserFriendlyState(state))
		sb.WriteString("\n")
	}
	if schedule := rollout.GetSpec().GetSchedule(); schedule == autoupdate.AgentsScheduleImmediate {
		sb.WriteString("Schedule is immediate. Every group immediately updates to the target version.\n")
	}
	if strategy := rollout.GetSpec().GetStrategy(); strategy != "" {
		sb.WriteString("Strategy: ")
		sb.WriteString(strategy)
		sb.WriteString("\n")
	}

	sb.WriteRune('\n')
	rolloutGroupTable(rollout, &sb)

	fmt.Fprint(c.stdout, sb.String())
	return nil
}

type agentStatusOutput struct {
	Active              bool                     `json:"active"`
	AutoupdateMode      string                   `json:"autoupdate_mode,omitempty"`
	RolloutCreationDate *time.Time               `json:"rollout_creation_date,omitempty"`
	StartVersion        string                   `json:"start_version,omitempty"`
	TargetVersion       string                   `json:"target_version,omitempty"`
	State               string                   `json:"state,omitempty"`
	Schedule            string                   `json:"schedule,omitempty"`
	Strategy            string                   `json:"strategy,omitempty"`
	Groups              []agentStatusGroupOutput `json:"groups,omitempty"`
}

type agentStatusGroupOutput struct {
	Name               string     `json:"name"`
	State              string     `json:"state"`
	StartTime          *time.Time `json:"start_time,omitempty"`
	LastUpdateReason   string     `json:"last_update_reason,omitempty"`
	PresentCount       uint64     `json:"present_count,omitempty"`
	UpToDateCount      uint64     `json:"up_to_date_count,omitempty"`
	CatchAll           bool       `json:"catch_all,omitempty"`
	CanarySuccessCount int        `json:"canary_success_count,omitempty"`
	CanaryCount        uint64     `json:"canary_count,omitempty"`
}

// agentRolloutStateName returns a stable, machine-friendly state name suitable
// for structured output. Unlike aur.UserFriendlyState, these names are part of
// the JSON/YAML contract and must not change for cosmetic reasons.
func agentRolloutStateName(state autoupdatev1pb.AutoUpdateAgentRolloutState) string {
	switch state {
	case autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_UNSPECIFIED:
		return "unspecified"
	case autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_UNSTARTED:
		return "unstarted"
	case autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ACTIVE:
		return "active"
	case autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_DONE:
		return "done"
	case autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_ROLLEDBACK:
		return "rolledback"
	default:
		return fmt.Sprintf("unknown_%d", state)
	}
}

// agentGroupStateName returns a stable, machine-friendly state name suitable
// for structured output. Unlike aur.UserFriendlyState, these names are part of
// the JSON/YAML contract and must not change for cosmetic reasons.
func agentGroupStateName(state autoupdatev1pb.AutoUpdateAgentGroupState) string {
	switch state {
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED:
		return "unspecified"
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:
		return "unstarted"
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:
		return "active"
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
		return "done"
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK:
		return "rolledback"
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:
		return "canary"
	default:
		return fmt.Sprintf("unknown_%d", state)
	}
}

// protoTimePtr returns nil for the Go zero time or the Unix epoch (which
// protobuf uses to represent unset timestamps), so omitempty drops the field.
func protoTimePtr(t time.Time) *time.Time {
	if t.IsZero() || t.Unix() == 0 {
		return nil
	}
	return &t
}

func newAgentStatusOutput(rollout *autoupdatev1pb.AutoUpdateAgentRollout) agentStatusOutput {
	out := agentStatusOutput{
		Active: rollout.GetSpec() != nil,
	}

	spec := rollout.GetSpec()
	status := rollout.GetStatus()

	if spec == nil {
		return out
	}

	out.AutoupdateMode = spec.GetAutoupdateMode()
	out.RolloutCreationDate = protoTimePtr(status.GetStartTime().AsTime())
	out.StartVersion = spec.GetStartVersion()
	out.TargetVersion = spec.GetTargetVersion()
	if state := status.GetState(); state != autoupdatev1pb.AutoUpdateAgentRolloutState_AUTO_UPDATE_AGENT_ROLLOUT_STATE_UNSPECIFIED {
		out.State = agentRolloutStateName(state)
	}
	out.Schedule = spec.GetSchedule()
	out.Strategy = spec.GetStrategy()
	out.Groups = agentStatusGroupOutputRows(rollout)
	return out
}

func agentStatusGroupOutputRows(rollout *autoupdatev1pb.AutoUpdateAgentRollout) []agentStatusGroupOutput {
	groups := rollout.GetStatus().GetGroups()
	out := make([]agentStatusGroupOutput, 0, len(groups))
	for i, group := range groups {
		row := agentStatusGroupOutput{
			Name:             group.GetName(),
			State:            agentGroupStateName(group.GetState()),
			StartTime:        protoTimePtr(group.GetStartTime().AsTime()),
			LastUpdateReason: group.GetLastUpdateReason(),
			PresentCount:     group.GetPresentCount(),
			UpToDateCount:    group.GetUpToDateCount(),
			CatchAll:         i == len(groups)-1,
			CanaryCount:      group.GetCanaryCount(),
		}
		for _, canary := range group.GetCanaries() {
			if canary.GetSuccess() {
				row.CanarySuccessCount++
			}
		}
		out = append(out, row)
	}
	return out
}

func (c *AutoUpdateCommand) agentsReportCommand(ctx context.Context, client autoupdateClient) error {
	now := c.now()
	reports, err := getAllReports(ctx, client)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err, "listing reports")
		}

		if c.format == teleport.Text {
			c.writeNoAgentReportsMessage()
		}
		return trace.Wrap(err)
	}

	if len(reports) == 0 {
		return trace.BadParameter("no reports returned, but the server did not return a NotFoundError, this ia a bug")
	}

	validReports := aur.ValidReports(reports, now)

	if len(validReports) == 0 {
		if c.format == teleport.Text {
			fmt.Fprintf(c.stdout, "Read %d reports, but they are expired. If you just (re)deployed the Auth service, you might want to retry after 60 seconds.\n", len(reports))
		}
		return trace.CompareFailed("reports expired")
	}

	summary := newAgentReportSummary(validReports)
	switch c.format {
	case teleport.Text:
		return trace.Wrap(c.writeAgentsReportText(summary))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSON(c.stdout, summary))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(c.stdout, summary))
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}
}

func (c *AutoUpdateCommand) writeNoAgentReportsMessage() {
	fmt.Fprintln(c.stdout, "No autoupdate_agent_report found.")
	if c.ccf != nil && len(c.ccf.AuthServerAddr) > 0 && !strings.HasSuffix(c.ccf.AuthServerAddr[0], ".teleport.sh") {
		fmt.Fprintln(c.stdout, "Managed Updates agent reports require enabling Managed Updates v2 by creating the autoupdate_version resource.")
		fmt.Fprintln(c.stdout, "See: https://goteleport.com/docs/upgrading/agent-managed-updates/#configuring-managed-agent-updates")
	}
}

type agentReportSummary struct {
	ReportCount int                  `json:"report_count"`
	Groups      []string             `json:"groups"`
	Versions    []string             `json:"versions"`
	Counts      []agentReportCount   `json:"counts"`
	Omitted     []agentReportOmitted `json:"omitted,omitempty"`
}

type agentReportCount struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Count   int    `json:"count"`
}

type agentReportOmitted struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

func newAgentReportSummary(validReports []*autoupdatev1pb.AutoUpdateAgentReport) agentReportSummary {
	groupSet := make(map[string]struct{})
	versionsSet := make(map[string]struct{})
	for _, report := range validReports {
		for groupName, group := range report.GetSpec().GetGroups() {
			groupSet[groupName] = struct{}{}
			for versionName := range group.GetVersions() {
				versionsSet[versionName] = struct{}{}
			}
		}
	}

	// Use empty non-nil slices so JSON/YAML output renders [] when
	// no reports contain any agents; structured consumers can iterate safely.
	groupNames := make([]string, 0, len(groupSet))
	for name := range groupSet {
		groupNames = append(groupNames, name)
	}
	versionNames := make([]string, 0, len(versionsSet))
	for name := range versionsSet {
		versionNames = append(versionNames, name)
	}
	slices.Sort(groupNames)
	slices.Sort(versionNames)

	summary := agentReportSummary{
		ReportCount: len(validReports),
		Groups:      groupNames,
		Versions:    versionNames,
		Counts:      []agentReportCount{},
	}
	for _, versionName := range versionNames {
		for _, groupName := range groupNames {
			var count int
			for _, report := range validReports {
				count += int(report.GetSpec().GetGroups()[groupName].GetVersions()[versionName].GetCount())
			}
			summary.Counts = append(summary.Counts, agentReportCount{
				Group:   groupName,
				Version: versionName,
				Count:   count,
			})
		}
	}

	omitted := omittedCounts(validReports)
	for _, reason := range slices.Sorted(maps.Keys(omitted)) {
		summary.Omitted = append(summary.Omitted, agentReportOmitted{
			Reason: reason,
			Count:  omitted[reason],
		})
	}
	return summary
}

func (c *AutoUpdateCommand) writeAgentsReportText(summary agentReportSummary) error {
	fmt.Fprintf(c.stdout, "%d autoupdate agent reports aggregated\n\n", summary.ReportCount)

	var err error
	if len(summary.Groups) == 0 || len(summary.Versions) == 0 {
		fmt.Fprintln(c.stdout, "Reports contain no agents.")
	} else {
		t := asciitable.MakeTable(append([]string{"Agent Version"}, summary.Groups...))
		counts := make(map[string]map[string]int)
		for _, count := range summary.Counts {
			if counts[count.Version] == nil {
				counts[count.Version] = make(map[string]int)
			}
			counts[count.Version][count.Group] = count.Count
		}
		for _, versionName := range summary.Versions {
			row := make([]string, len(summary.Groups)+1)
			row[0] = versionName
			for j, groupName := range summary.Groups {
				row[j+1] = strconv.Itoa(counts[versionName][groupName])
			}
			t.AddRow(row)
		}

		_, err = t.AsBuffer().WriteTo(c.stdout)
	}

	fmt.Fprint(c.stdout, omittedSummary(summary.Omitted))

	return trace.Wrap(err)
}

func omittedCounts(reports []*autoupdatev1pb.AutoUpdateAgentReport) map[string]int {
	aggregated := make(map[string]int)
	for _, report := range reports {
		for _, omitted := range report.GetSpec().GetOmitted() {
			aggregated[omitted.GetReason()] += int(omitted.GetCount())
		}
	}
	return aggregated
}

func omittedSummary(omitted []agentReportOmitted) string {
	var totalOmitted int
	for _, row := range omitted {
		totalOmitted += row.Count
	}
	if totalOmitted == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteRune('\n')
	fmt.Fprintf(&sb, "%d agents were omitted from the reports:\n", totalOmitted)
	for _, row := range omitted {
		fmt.Fprintf(&sb, "- %d omitted because: %s\n", row.Count, row.Reason)
	}
	return sb.String()
}

func getAllReports(ctx context.Context, client autoupdateClient) ([]*autoupdatev1pb.AutoUpdateAgentReport, error) {
	const pageSize = 50
	var pageToken string
	var reports []*autoupdatev1pb.AutoUpdateAgentReport
	for {
		page, nextToken, err := client.ListAutoUpdateAgentReports(ctx, pageSize, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, page...)
		if nextToken == "" {
			return reports, nil
		}
		pageToken = nextToken
	}
}

func rolloutHasAgentCounters(rollout *autoupdatev1pb.AutoUpdateAgentRollout) bool {
	for _, group := range rollout.GetStatus().GetGroups() {
		if group.PresentCount != 0 {
			return true
		}
	}
	return false
}

func rolloutGroupTable(rollout *autoupdatev1pb.AutoUpdateAgentRollout, writer io.Writer) {
	groups := rollout.GetStatus().GetGroups()
	switch {
	case len(groups) != 0 && rolloutHasAgentCounters(rollout):
		headers := []string{"Group Name", "State", "Start Time", "State Reason", "Agent Count", "Up-to-date"}
		table := asciitable.MakeTable(headers)
		for i, group := range groups {
			groupName := group.GetName()
			groupCount := group.PresentCount
			groupUpToDate := group.UpToDateCount
			if i == len(groups)-1 {
				groupName = groupName + " (catch-all)"
			}
			state := aur.UserFriendlyState(group.GetState())

			// If this is the canary state, we annotate the group state with the canary progress
			if group.GetState() == autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY {
				successCount := 0
				for _, canary := range group.Canaries {
					if canary.Success {
						successCount++
					}
				}
				state = fmt.Sprintf("%s (%d/%d)", state, successCount, group.CanaryCount)
			}

			table.AddRow([]string{
				groupName,
				state,
				formatTimeIfNotEmpty(group.GetStartTime().AsTime(), time.DateTime),
				group.GetLastUpdateReason(),
				strconv.FormatUint(groupCount, 10),
				strconv.FormatUint(groupUpToDate, 10),
			})
		}
		writer.Write(table.AsBuffer().Bytes())

	case len(groups) != 0:
		headers := []string{"Group Name", "State", "Start Time", "State Reason"}
		table := asciitable.MakeTable(headers)
		for _, group := range groups {
			groupName := group.GetName()
			table.AddRow([]string{
				groupName,
				aur.UserFriendlyState(group.GetState()),
				formatTimeIfNotEmpty(group.GetStartTime().AsTime(), time.DateTime),
				group.GetLastUpdateReason()})
		}
		writer.Write(table.AsBuffer().Bytes())
	default:
	}
}

func (c *AutoUpdateCommand) agentsStartUpdateCommand(ctx context.Context, client autoupdateClient) error {
	groups := make([]string, 0, len(c.groups))
	for _, group := range c.groups {
		if grp := strings.TrimSpace(group); grp != "" {
			groups = append(groups, grp)
		}
	}

	if len(c.groups) == 0 {
		return trace.BadParameter("no groups specified")
	}

	state := autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED
	if c.force {
		state = autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE
	}
	rollout, err := client.TriggerAutoUpdateAgentGroup(ctx, groups, state)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.stdout, "Successfully started updating agents groups: %v.\n", groups)

	fmt.Fprint(c.stdout, "New agent rollout status:\n\n")
	rolloutGroupTable(rollout, c.stdout)
	return nil
}

func (c *AutoUpdateCommand) agentsMarkDoneCommand(ctx context.Context, client autoupdateClient) error {
	groups := make([]string, 0, len(c.groups))
	for _, group := range c.groups {
		if grp := strings.TrimSpace(group); grp != "" {
			groups = append(groups, grp)
		}
	}

	rollout, err := client.ForceAutoUpdateAgentGroup(ctx, groups)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(c.groups) == 0 {
		fmt.Fprintln(c.stdout, "Successfully marked every started agent group as completed.")
	} else {
		fmt.Fprintf(c.stdout, "Successfully marked agent groups as completed: %v.\n", groups)
	}

	fmt.Fprint(c.stdout, "New agent rollout status:\n\n")
	rolloutGroupTable(rollout, c.stdout)
	return nil
}

func (c *AutoUpdateCommand) agentsRollbackCommand(ctx context.Context, client autoupdateClient) error {
	groups := make([]string, 0, len(c.groups))
	for _, group := range c.groups {
		if grp := strings.TrimSpace(group); grp != "" {
			groups = append(groups, grp)
		}
	}

	rollbackAllSartedGroups := len(c.groups) == 0

	rollout, err := client.RollbackAutoUpdateAgentGroup(ctx, groups, rollbackAllSartedGroups)
	if err != nil {
		return trace.Wrap(err)
	}

	if rollbackAllSartedGroups {
		fmt.Fprintln(c.stdout, "Successfully rolled back already started groups.")
	} else {
		fmt.Fprintf(c.stdout, "Successfully rolled back the following agent groups: %v.\n", groups)
	}

	fmt.Fprint(c.stdout, "New agent rollout status:\n\n")
	rolloutGroupTable(rollout, c.stdout)
	return nil
}

func formatTimeIfNotEmpty(t time.Time, format string) string {
	if t.IsZero() || t.Unix() == 0 {
		return ""
	}
	return t.Format(format)
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
