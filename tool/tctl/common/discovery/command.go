// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/trace"
)

// Command implements `tctl discovery` troubleshooting commands.
type Command struct {
	config *servicecfg.Config
	Stdout io.Writer

	discovery *kingpin.CmdClause

	statusCmd         *kingpin.CmdClause
	statusState       string
	statusIntegration string
	statusFormat      string

	tasksCmd             *kingpin.CmdClause
	tasksListCmd         *kingpin.CmdClause
	tasksListState       string
	tasksListIntegration string
	tasksListTaskType    string
	tasksListIssueType   string
	tasksListFormat      string

	tasksShowCmd      *kingpin.CmdClause
	tasksShowName     string
	tasksShowFormat   string
	tasksShowPageSize int
	tasksShowPage     int

	ssmRunsCmd            *kingpin.CmdClause
	ssmRunsListCmd        *kingpin.CmdClause
	ssmRunsShowCmd        *kingpin.CmdClause
	ssmRunsSince          string
	ssmRunsShowInstanceID string
	ssmRunsFailedOnly     bool
	ssmRunsLimit          int
	ssmRunsPageSize       int
	ssmRunsPage           int
	ssmRunsShowAll        bool
	ssmRunsFormat         string
}

func (c *Command) output() io.Writer {
	if c.Stdout != nil {
		return c.Stdout
	}
	return os.Stdout
}

func buildTaskShowCommand(c *Command) string {
	return fmt.Sprintf("tctl discovery tasks show %s --page-size=%d", c.tasksShowName, c.tasksShowPageSize)
}

func buildSSMRunsCommand(c *Command, subCmd string, instanceID string) string {
	cmd := []string{"tctl discovery ssm-runs", subCmd}
	if instanceID != "" {
		cmd = append(cmd, instanceID)
	}
	if c.ssmRunsSince != "" {
		cmd = append(cmd, fmt.Sprintf("--since=%s", c.ssmRunsSince))
	}
	if c.ssmRunsFailedOnly {
		cmd = append(cmd, "--failed")
	}
	if c.ssmRunsShowAll {
		cmd = append(cmd, "--show-all-runs")
	}
	if instanceID == "" {
		cmd = append(cmd, fmt.Sprintf("--page-size=%d", c.ssmRunsPageSize))
	}
	return strings.Join(cmd, " ")
}

// Initialize allows Command to plug itself into the CLI parser.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	c.discovery = app.Command("discovery", "Troubleshoot Discovery auto-enrollment issues.").Alias("discover")

	c.statusCmd = c.discovery.Command("status", "Triage Discovery health: tasks, configs, integrations, and next actions.")
	c.statusCmd.Flag("state", "Task state filter: open, resolved, all.").Default("open").StringVar(&c.statusState)
	c.statusCmd.Flag("integration", "Filter tasks by integration.").StringVar(&c.statusIntegration)
	c.statusCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.statusFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.tasksCmd = c.discovery.Command("tasks", "Inspect Discovery user tasks.")
	c.tasksListCmd = c.tasksCmd.Command("ls", "List Discovery user tasks with filters and troubleshooting hints.").Alias("list")
	c.tasksListCmd.Flag("state", "Task state filter: open, resolved, all.").Default("open").StringVar(&c.tasksListState)
	c.tasksListCmd.Flag("integration", "Filter by integration.").StringVar(&c.tasksListIntegration)
	c.tasksListCmd.Flag("task-type", "Filter by task type (e.g. discover-ec2).").StringVar(&c.tasksListTaskType)
	c.tasksListCmd.Flag("issue-type", "Filter by issue type (e.g. ec2-ssm-script-failure).").StringVar(&c.tasksListIssueType)
	c.tasksListCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.tasksListFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.tasksShowCmd = c.tasksCmd.Command("show", "Inspect one Discovery task with affected resources and follow-up commands.")
	c.tasksShowCmd.Arg("name", "User task name.").Required().StringVar(&c.tasksShowName)
	c.tasksShowCmd.Flag("page-size", "Maximum affected resources per page.").Default("25").IntVar(&c.tasksShowPageSize)
	c.tasksShowCmd.Flag("page", "Page number (1-based).").Default("1").IntVar(&c.tasksShowPage)
	c.tasksShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.tasksShowFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.ssmRunsCmd = c.discovery.Command("ssm-runs", "Inspect SSM run audit events for discovery troubleshooting.").Alias("ssm")
	c.ssmRunsListCmd = c.ssmRunsCmd.Command("ls", "List failing VMs and summarize recent SSM run outcomes.").Alias("list")
	c.ssmRunsListCmd.Flag("since", "Time window to analyze, e.g. 1h, 30m, 24h.").Default("1h").StringVar(&c.ssmRunsSince)
	c.ssmRunsListCmd.Flag("failed", "Only include failed SSM runs.").BoolVar(&c.ssmRunsFailedOnly)
	c.ssmRunsListCmd.Flag("limit", "Maximum SSM run events to fetch from audit log.").Default("200").IntVar(&c.ssmRunsLimit)
	c.ssmRunsListCmd.Flag("page-size", "Maximum failing VMs per page.").Default("25").IntVar(&c.ssmRunsPageSize)
	c.ssmRunsListCmd.Flag("page", "Page number (1-based).").Default("1").IntVar(&c.ssmRunsPage)
	c.ssmRunsListCmd.Flag("failing-vms", "Deprecated alias for --page-size.").Hidden().IntVar(&c.ssmRunsPageSize)
	c.ssmRunsListCmd.Flag("show-all-runs", "Show full run history for each displayed VM.").BoolVar(&c.ssmRunsShowAll)
	c.ssmRunsListCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.ssmRunsFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.ssmRunsShowCmd = c.ssmRunsCmd.Command("show", "Show run history for one EC2 instance in the selected time window.")
	c.ssmRunsShowCmd.Arg("instance-id", "EC2 instance ID to inspect.").Required().StringVar(&c.ssmRunsShowInstanceID)
	c.ssmRunsShowCmd.Flag("since", "Time window to analyze, e.g. 1h, 30m, 24h.").Default("1h").StringVar(&c.ssmRunsSince)
	c.ssmRunsShowCmd.Flag("failed", "Only include failed SSM runs.").BoolVar(&c.ssmRunsFailedOnly)
	c.ssmRunsShowCmd.Flag("limit", "Maximum SSM run events to fetch from audit log.").Default("200").IntVar(&c.ssmRunsLimit)
	c.ssmRunsShowCmd.Flag("show-all-runs", "Show full run history for the selected VM.").BoolVar(&c.ssmRunsShowAll)
	c.ssmRunsShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.ssmRunsFormat, teleport.Text, teleport.JSON, teleport.YAML)
}

// TryRun takes the CLI command and executes it.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(context.Context, *authclient.Client) error

	switch {
	case matchesCommand(cmd, c.statusCmd):
		commandFunc = c.runStatus
	case matchesCommand(cmd, c.tasksListCmd):
		commandFunc = c.runTasksList
	case matchesCommand(cmd, c.tasksShowCmd):
		commandFunc = c.runTaskShow
	case matchesCommand(cmd, c.ssmRunsListCmd):
		commandFunc = c.runSSMRunsList
	case matchesCommand(cmd, c.ssmRunsShowCmd):
		commandFunc = c.runSSMRunsShow
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(commandFunc(ctx, client))
}

func matchesCommand(selected string, command *kingpin.CmdClause) bool {
	full := command.FullCommand()
	if selected == full {
		return true
	}
	alias := strings.Replace(full, "discovery ", "discover ", 1)
	return selected == alias
}

func (c *Command) runStatus(ctx context.Context, client *authclient.Client) error {
	state, err := normalizeTaskState(c.statusState)
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, c.statusIntegration, "")
	if err != nil {
		return trace.Wrap(err)
	}
	filteredTasks := filterUserTasks(tasks, taskFilters{State: state})

	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, client.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}

	summary := makeStatusSummary(tasks, filteredTasks, discoveryConfigs, state, c.statusIntegration)
	return trace.Wrap(writeOutputByFormat(c.output(), c.statusFormat, summary, func(w io.Writer) error {
		return renderStatusText(w, summary)
	}))
}

func (c *Command) runTasksList(ctx context.Context, client *authclient.Client) error {
	state, err := normalizeTaskState(c.tasksListState)
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, c.tasksListIntegration, state)
	if err != nil {
		return trace.Wrap(err)
	}
	tasks = filterUserTasks(tasks, taskFilters{
		State:       state,
		TaskType:    c.tasksListTaskType,
		IssueType:   c.tasksListIssueType,
		Integration: c.tasksListIntegration,
	})

	slices.SortFunc(tasks, func(a, b *usertasksv1.UserTask) int {
		if c := compareTimeDesc(taskLastStateChange(a), taskLastStateChange(b)); c != 0 {
			return c
		}
		return cmp.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})

	items := toTaskListItems(tasks)
	listOutput := tasksListOutput{
		Total: len(items),
		Items: items,
	}
	return trace.Wrap(writeOutputByFormat(c.output(), c.tasksListFormat, listOutput, func(w io.Writer) error {
		return renderTasksListText(w, items, taskListHintsInput{
			State:       state,
			Integration: c.tasksListIntegration,
			TaskType:    c.tasksListTaskType,
			IssueType:   c.tasksListIssueType,
		})
	}))
}

func (c *Command) runTaskShow(ctx context.Context, client *authclient.Client) error {
	tasks, err := listUserTasks(ctx, client, "", "")
	if err != nil {
		return trace.Wrap(err)
	}
	task, err := findTaskByNamePrefix(tasks, c.tasksShowName)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(writeOutputByFormat(c.output(), c.tasksShowFormat, task, func(w io.Writer) error {
		return renderTaskDetailsText(w, task, c.tasksShowPage, c.tasksShowPageSize, buildTaskShowCommand(c))
	}))
}

func (c *Command) runSSMRunsList(ctx context.Context, client *authclient.Client) error {
	analysis, vmGroups, err := c.fetchSSMRunData(ctx, client, "")
	if err != nil {
		return trace.Wrap(err)
	}
	allFailingVMGroups := selectFailingVMGroups(vmGroups, 0)
	failingVMGroups, vmPage := paginateSlice(allFailingVMGroups, c.ssmRunsPage, c.ssmRunsPageSize)

	output := c.buildSSMRunsOutput(analysis, vmGroups, allFailingVMGroups, failingVMGroups, vmPage)
	baseCommand := buildSSMRunsCommand(c, "ls", "")
	return trace.Wrap(c.writeSSMRunsOutput(output, "", baseCommand))
}

func (c *Command) runSSMRunsShow(ctx context.Context, client *authclient.Client) error {
	instanceID := c.ssmRunsShowInstanceID
	analysis, vmGroups, err := c.fetchSSMRunData(ctx, client, instanceID)
	if err != nil {
		return trace.Wrap(err)
	}
	allFailingVMGroups := selectFailingVMGroups(vmGroups, 0)
	var showGroups []ssmVMGroup
	for _, group := range vmGroups {
		if group.InstanceID == instanceID {
			showGroups = []ssmVMGroup{group}
			break
		}
	}
	vmPage := fullPageInfo(len(showGroups))

	output := c.buildSSMRunsOutput(analysis, vmGroups, allFailingVMGroups, showGroups, vmPage)
	baseCommand := buildSSMRunsCommand(c, "show", instanceID)
	return trace.Wrap(c.writeSSMRunsOutput(output, instanceID, baseCommand))
}

func (c *Command) fetchSSMRunData(ctx context.Context, client *authclient.Client, instanceIDFilter string) (ssmRunAnalysis, []ssmVMGroup, error) {
	since, err := time.ParseDuration(c.ssmRunsSince)
	if err != nil || since <= 0 {
		return ssmRunAnalysis{}, nil, trace.BadParameter("invalid --since value %q, expected duration like 1h or 30m", c.ssmRunsSince)
	}
	now := time.Now().UTC()
	fetchLimit := c.ssmRunsLimit
	if fetchLimit <= 0 {
		fetchLimit = 200
	}

	allEvents := make([]apievents.AuditEvent, 0, fetchLimit)
	var startKey string
	for len(allEvents) < fetchLimit {
		requestLimit := fetchLimit - len(allEvents)
		if requestLimit > 200 {
			requestLimit = 200
		}
		pageEvents, nextKey, err := client.SearchEvents(ctx, libevents.SearchEventsRequest{
			From:       now.Add(-since),
			To:         now,
			EventTypes: []string{libevents.SSMRunEvent},
			Limit:      requestLimit,
			Order:      types.EventOrderDescending,
			StartKey:   startKey,
		})
		if err != nil {
			return ssmRunAnalysis{}, nil, trace.Wrap(err)
		}
		allEvents = append(allEvents, pageEvents...)
		if nextKey == "" || len(pageEvents) == 0 {
			break
		}
		startKey = nextKey
	}

	records := parseSSMRunEvents(allEvents, ssmRunEventFilters{
		FailedOnly: c.ssmRunsFailedOnly,
		InstanceID: instanceIDFilter,
	})
	analysis := analyzeSSMRuns(records)
	vmGroups := groupSSMRunsByVM(records)
	return analysis, vmGroups, nil
}

func (c *Command) buildSSMRunsOutput(analysis ssmRunAnalysis, vmGroups, allFailingVMGroups, displayedVMGroups []ssmVMGroup, vmPage pageInfo) ssmRunsOutput {
	return ssmRunsOutput{
		Window:       c.ssmRunsSince,
		Query:        fmt.Sprintf("SearchEvents(event_type=%q)", libevents.SSMRunEvent),
		TotalRuns:    analysis.Total,
		SuccessRuns:  analysis.Success,
		FailedRuns:   analysis.Failed,
		TotalVMs:     len(vmGroups),
		FailingVMs:   len(allFailingVMGroups),
		DisplayedVMs: len(displayedVMGroups),
		VMPage:       vmPage,
		VMs:          displayedVMGroups,
	}
}

func (c *Command) writeSSMRunsOutput(output ssmRunsOutput, instanceIDFilter, baseCommand string) error {
	return trace.Wrap(writeOutputByFormat(c.output(), c.ssmRunsFormat, output, func(w io.Writer) error {
		return renderSSMRunsText(w, output, instanceIDFilter, c.ssmRunsShowAll, baseCommand)
	}))
}

func writeOutputByFormat(w io.Writer, format string, output any, renderText func(io.Writer) error) error {
	switch format {
	case teleport.JSON:
		return utils.WriteJSON(w, output)
	case teleport.YAML:
		return utils.WriteYAML(w, output)
	case teleport.Text:
		if renderText == nil {
			return trace.BadParameter("text output renderer is required")
		}
		return renderText(w)
	default:
		return trace.BadParameter("unknown format: %q", format)
	}
}
