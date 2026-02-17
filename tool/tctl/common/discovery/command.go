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
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
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

// auditEventPageSize is the maximum number of events to fetch per SearchEvents
// API call. The server-side maximum is 10,000 (defaults.EventsMaxIterationLimit).
const auditEventPageSize = 1000

// defaultFetchLimit is the default --limit value for audit event fetch commands.
const defaultFetchLimit = 1000

// Command implements `tctl discovery` troubleshooting commands.
type Command struct {
	config *servicecfg.Config
	Stdout io.Writer
	cache  *eventCache

	discovery *kingpin.CmdClause

	statusCmd          *kingpin.CmdClause
	statusIntegration  string
	statusFormat       string
	statusLast         string
	statusFromUTC      string
	statusToUTC        string
	statusSSMLimit     int
	statusJoinLimit    int

	tasksCmd             *kingpin.CmdClause
	tasksListCmd         *kingpin.CmdClause
	tasksListState       string
	tasksListIntegration string
	tasksListTaskType    string
	tasksListIssueType   string
	tasksListFormat      string

	tasksShowCmd    *kingpin.CmdClause
	tasksShowName   string
	tasksShowFormat string
	tasksShowRange  string

	ssmRunsCmd            *kingpin.CmdClause
	ssmRunsListCmd        *kingpin.CmdClause
	ssmRunsShowCmd        *kingpin.CmdClause
	ssmRunsShowInstanceID string
	ssmRunsLimit          int
	ssmRunsRange          string
	ssmRunsShowAll        bool
	ssmRunsClusterErrors  bool
	ssmRunsFormat         string
	ssmRunsFromUTC        string
	ssmRunsToUTC          string
	ssmRunsLast           string

	integrationCmd        *kingpin.CmdClause
	integrationListCmd    *kingpin.CmdClause
	integrationListFormat string

	integrationShowCmd    *kingpin.CmdClause
	integrationShowName   string
	integrationShowFormat string

	joinsCmd        *kingpin.CmdClause
	joinsListCmd    *kingpin.CmdClause
	joinsShowCmd    *kingpin.CmdClause
	joinsShowHostID string
	joinsLimit       int
	joinsRange       string
	joinsShowAll     bool
	joinsHideUnknown bool
	joinsRaw         bool
	joinsFormat      string
	joinsFromUTC     string
	joinsToUTC       string
	joinsLast        string

	inventoryCmd          *kingpin.CmdClause
	inventoryListCmd      *kingpin.CmdClause
	inventoryShowCmd      *kingpin.CmdClause
	inventoryShowHostID   string
	inventoryLimit        int
	inventoryRange        string
	inventoryShowAll      bool
	inventoryFormat       string
	inventoryFromUTC      string
	inventoryToUTC        string
	inventoryLast         string
	inventoryStateFilter  string
	inventoryMethodFilter string

	cacheCmd          *kingpin.CmdClause
	cacheLoadCmd      *kingpin.CmdClause
	cacheLoadLast     string
	cacheLoadFromUTC  string
	cacheLoadToUTC    string
	cacheStatusCmd    *kingpin.CmdClause
	cachePruneCmd     *kingpin.CmdClause
}

func (c *Command) output() io.Writer {
	if c.Stdout != nil {
		return c.Stdout
	}
	return os.Stdout
}

func (c *Command) initCache(ctx context.Context, client *authclient.Client) {
	cn, err := client.GetClusterName(ctx)
	if err != nil {
		slog.WarnContext(ctx, "Could not get cluster name for cache", "error", err)
		return
	}
	clusterName := cn.GetClusterName()
	if clusterName == "" {
		return
	}
	c.cache = &eventCache{
		Dir: filepath.Join(".cache", clusterName),
	}
}

func buildTaskShowCommand(c *Command) string {
	cmd := fmt.Sprintf("tctl discovery tasks show %s", c.tasksShowName)
	if c.tasksShowRange != "" {
		cmd += fmt.Sprintf(" --range=%s", c.tasksShowRange)
	}
	return cmd
}

func buildSSMRunsCommand(c *Command, subCmd string, instanceID string) string {
	cmd := []string{"tctl discovery ssm-runs", subCmd}
	if instanceID != "" {
		cmd = append(cmd, instanceID)
	}
	if c.ssmRunsLast != "" {
		cmd = append(cmd, fmt.Sprintf("--last=%s", c.ssmRunsLast))
	}
	if c.ssmRunsFromUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--from-utc=%s", c.ssmRunsFromUTC))
	}
	if c.ssmRunsToUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--to-utc=%s", c.ssmRunsToUTC))
	}
	if c.ssmRunsLimit != 0 && c.ssmRunsLimit != defaultFetchLimit {
		cmd = append(cmd, fmt.Sprintf("--limit=%d", c.ssmRunsLimit))
	}
	if c.ssmRunsShowAll {
		cmd = append(cmd, "--show-all-runs")
	}
	if instanceID == "" && c.ssmRunsRange != "" {
		cmd = append(cmd, fmt.Sprintf("--range=%s", c.ssmRunsRange))
	}
	return strings.Join(cmd, " ")
}

func buildJoinsCommand(c *Command, subCmd string, hostID string) string {
	cmd := []string{"tctl discovery joins", subCmd}
	if hostID != "" {
		cmd = append(cmd, shellQuoteArg(hostID))
	}
	if c.joinsLast != "" {
		cmd = append(cmd, fmt.Sprintf("--last=%s", c.joinsLast))
	}
	if c.joinsFromUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--from-utc=%s", c.joinsFromUTC))
	}
	if c.joinsToUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--to-utc=%s", c.joinsToUTC))
	}
	if c.joinsLimit != 0 && c.joinsLimit != defaultFetchLimit {
		cmd = append(cmd, fmt.Sprintf("--limit=%d", c.joinsLimit))
	}
	if c.joinsShowAll {
		cmd = append(cmd, "--show-all-joins")
	}
	if c.joinsHideUnknown {
		cmd = append(cmd, "--hide-unknown")
	}
	if hostID == "" && c.joinsRange != "" {
		cmd = append(cmd, fmt.Sprintf("--range=%s", c.joinsRange))
	}
	return strings.Join(cmd, " ")
}

// Initialize allows Command to plug itself into the CLI parser.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	c.discovery = app.Command("discovery", "Troubleshoot Discovery auto-enrollment issues.").Alias("discover")

	c.statusCmd = c.discovery.Command("status", "Triage Discovery health: tasks, configs, integrations, and next actions.")
	c.statusCmd.Flag("integration", "Filter tasks by integration.").StringVar(&c.statusIntegration)
	c.statusCmd.Flag("last", "Time window for audit event stats (SSM runs, joins).").Default("24h").StringVar(&c.statusLast)
	c.statusCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.statusFromUTC)
	c.statusCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.statusToUTC)
	c.statusCmd.Flag("ssm-limit", "Maximum SSM run events to fetch.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.statusSSMLimit)
	c.statusCmd.Flag("join-limit", "Maximum instance join events to fetch.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.statusJoinLimit)
	c.statusCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.statusFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.tasksCmd = c.discovery.Command("tasks", "Inspect Discovery user tasks.").Alias("task")
	c.tasksListCmd = c.tasksCmd.Command("ls", "List Discovery user tasks with filters and troubleshooting hints.").Alias("list")
	c.tasksListCmd.Flag("state", "Task state filter: open, resolved, all.").Default("open").StringVar(&c.tasksListState)
	c.tasksListCmd.Flag("integration", "Filter by integration.").StringVar(&c.tasksListIntegration)
	c.tasksListCmd.Flag("task-type", "Filter by task type (e.g. discover-ec2).").StringVar(&c.tasksListTaskType)
	c.tasksListCmd.Flag("issue-type", "Filter by issue type (e.g. ec2-ssm-script-failure).").StringVar(&c.tasksListIssueType)
	c.tasksListCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.tasksListFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.tasksShowCmd = c.tasksCmd.Command("show", "Inspect one Discovery task with affected resources and follow-up commands.")
	c.tasksShowCmd.Arg("name", "User task name.").Required().StringVar(&c.tasksShowName)
	c.tasksShowCmd.Flag("range", "Range of affected resources to display as start,end (0-indexed, exclusive end).").Default("0,50").StringVar(&c.tasksShowRange)
	c.tasksShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.tasksShowFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.ssmRunsCmd = c.discovery.Command("ssm-runs", "Inspect SSM run audit events for discovery troubleshooting.").Alias("ssm")
	c.ssmRunsListCmd = c.ssmRunsCmd.Command("ls", "List failing VMs and summarize recent SSM run outcomes.").Alias("list")
	c.ssmRunsListCmd.Flag("last", "Time window to analyze, e.g. 1h, 30m, 24h.").StringVar(&c.ssmRunsLast)
	c.ssmRunsListCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.ssmRunsFromUTC)
	c.ssmRunsListCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.ssmRunsToUTC)
	c.ssmRunsListCmd.Flag("limit", "Maximum SSM run events to fetch from audit log.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.ssmRunsLimit)
	c.ssmRunsListCmd.Flag("range", "Range of VMs to display as start,end (0-indexed, exclusive end).").Default("0,50").StringVar(&c.ssmRunsRange)
	c.ssmRunsListCmd.Flag("show-all-runs", "Show full run history for each displayed VM.").BoolVar(&c.ssmRunsShowAll)
	c.ssmRunsListCmd.Flag("cluster-errors", "Group similar SSM run errors into clusters using Drain+MinHash/LSH.").BoolVar(&c.ssmRunsClusterErrors)
	c.ssmRunsListCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.ssmRunsFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.ssmRunsShowCmd = c.ssmRunsCmd.Command("show", "Show run history for one EC2 instance in the selected time window.")
	c.ssmRunsShowCmd.Arg("instance-id", "EC2 instance ID to inspect.").Required().StringVar(&c.ssmRunsShowInstanceID)
	c.ssmRunsShowCmd.Flag("last", "Time window to analyze, e.g. 1h, 30m, 24h.").StringVar(&c.ssmRunsLast)
	c.ssmRunsShowCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.ssmRunsFromUTC)
	c.ssmRunsShowCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.ssmRunsToUTC)
	c.ssmRunsShowCmd.Flag("limit", "Maximum SSM run events to fetch from audit log.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.ssmRunsLimit)
	c.ssmRunsShowCmd.Flag("show-all-runs", "Show full run history for the selected VM.").BoolVar(&c.ssmRunsShowAll)
	c.ssmRunsShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.ssmRunsFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.integrationCmd = c.discovery.Command("integration", "Inspect integrations used by Discovery.").Alias("integrations")
	c.integrationListCmd = c.integrationCmd.Command("ls", "List integrations with discovery resource stats and open tasks.").Alias("list")
	c.integrationListCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.integrationListFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.integrationShowCmd = c.integrationCmd.Command("show", "Inspect one integration with resource stats, configs, and tasks.")
	c.integrationShowCmd.Arg("name", "Integration name.").Required().StringVar(&c.integrationShowName)
	c.integrationShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.integrationShowFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.joinsCmd = c.discovery.Command("joins", "Inspect instance join audit events.").Alias("join")
	c.joinsListCmd = c.joinsCmd.Command("ls", "List hosts and summarize recent instance join outcomes.").Alias("list")
	c.joinsListCmd.Flag("last", "Time window to analyze, e.g. 1h, 30m, 24h.").StringVar(&c.joinsLast)
	c.joinsListCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.joinsFromUTC)
	c.joinsListCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.joinsToUTC)
	c.joinsListCmd.Flag("limit", "Maximum join events to fetch from audit log.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.joinsLimit)
	c.joinsListCmd.Flag("range", "Range of hosts to display as start,end (0-indexed, exclusive end).").Default("0,50").StringVar(&c.joinsRange)
	c.joinsListCmd.Flag("show-all-joins", "Show full join history for each displayed host.").BoolVar(&c.joinsShowAll)
	c.joinsListCmd.Flag("hide-unknown", "Hide hosts with unknown/empty host ID (failed joins before identification).").BoolVar(&c.joinsHideUnknown)
	c.joinsListCmd.Flag("raw", "Dump raw audit events as JSON (for inspecting all available fields).").BoolVar(&c.joinsRaw)
	c.joinsListCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.joinsFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.joinsShowCmd = c.joinsCmd.Command("show", "Show join history for one host in the selected time window.")
	c.joinsShowCmd.Arg("host-id", "Host ID to inspect.").Required().StringVar(&c.joinsShowHostID)
	c.joinsShowCmd.Flag("last", "Time window to analyze, e.g. 1h, 30m, 24h.").StringVar(&c.joinsLast)
	c.joinsShowCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.joinsFromUTC)
	c.joinsShowCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.joinsToUTC)
	c.joinsShowCmd.Flag("limit", "Maximum join events to fetch from audit log.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.joinsLimit)
	c.joinsShowCmd.Flag("show-all-joins", "Show full join history for the selected host.").BoolVar(&c.joinsShowAll)
	c.joinsShowCmd.Flag("raw", "Dump raw audit events as JSON (for inspecting all available fields).").BoolVar(&c.joinsRaw)
	c.joinsShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.joinsFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.inventoryCmd = c.discovery.Command("inventory", "Unified view of discovered hosts across SSM runs, joins, and active nodes.").Alias("inv")
	c.inventoryListCmd = c.inventoryCmd.Command("ls", "List hosts with pipeline state derived from SSM runs, joins, and node heartbeats.").Alias("list")
	c.inventoryListCmd.Flag("last", "Time window for audit events, e.g. 1h, 30m, 24h.").Default("24h").StringVar(&c.inventoryLast)
	c.inventoryListCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryFromUTC)
	c.inventoryListCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryToUTC)
	c.inventoryListCmd.Flag("limit", "Maximum audit events to fetch per event type.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.inventoryLimit)
	c.inventoryListCmd.Flag("range", "Range of hosts to display as start,end (0-indexed, exclusive end).").Default("0,50").StringVar(&c.inventoryRange)
	c.inventoryListCmd.Flag("state", "Filter by state: online, offline, failed, attempted.").StringVar(&c.inventoryStateFilter)
	c.inventoryListCmd.Flag("method", "Filter by join method: ec2, iam, azure, token, etc.").StringVar(&c.inventoryMethodFilter)
	c.inventoryListCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.inventoryFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.inventoryShowCmd = c.inventoryCmd.Command("show", "Show timeline for one host combining SSM runs and join events.")
	c.inventoryShowCmd.Arg("host-id", "Host ID to inspect.").Required().StringVar(&c.inventoryShowHostID)
	c.inventoryShowCmd.Flag("last", "Time window for audit events, e.g. 1h, 30m, 24h.").Default("24h").StringVar(&c.inventoryLast)
	c.inventoryShowCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryFromUTC)
	c.inventoryShowCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryToUTC)
	c.inventoryShowCmd.Flag("limit", "Maximum audit events to fetch per event type.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.inventoryLimit)
	c.inventoryShowCmd.Flag("show-all-events", "Show full event timeline for the selected host.").BoolVar(&c.inventoryShowAll)
	c.inventoryShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.inventoryFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.cacheCmd = c.discovery.Command("cache", "Manage the local audit event cache.").Hidden()
	c.cacheLoadCmd = c.cacheCmd.Command("load", "Pre-fetch all audit events into the local cache.")
	c.cacheLoadCmd.Flag("last", "Time window to cache, e.g. 1h, 24h, 7d.").Default("24h").StringVar(&c.cacheLoadLast)
	c.cacheLoadCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.cacheLoadFromUTC)
	c.cacheLoadCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.cacheLoadToUTC)
	c.cacheStatusCmd = c.cacheCmd.Command("status", "Show cache file inventory.")
	c.cachePruneCmd = c.cacheCmd.Command("prune", "Delete all cached audit events.")
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
	case matchesCommand(cmd, c.integrationListCmd):
		commandFunc = c.runIntegrationList
	case matchesCommand(cmd, c.integrationShowCmd):
		commandFunc = c.runIntegrationShow
	case matchesCommand(cmd, c.joinsListCmd):
		commandFunc = c.runJoinsList
	case matchesCommand(cmd, c.joinsShowCmd):
		commandFunc = c.runJoinsShow
	case matchesCommand(cmd, c.inventoryListCmd):
		commandFunc = c.runInventoryList
	case matchesCommand(cmd, c.inventoryShowCmd):
		commandFunc = c.runInventoryShow
	case matchesCommand(cmd, c.cacheLoadCmd):
		commandFunc = c.runCacheLoad
	case matchesCommand(cmd, c.cacheStatusCmd):
		commandFunc = c.runCacheStatus
	case matchesCommand(cmd, c.cachePruneCmd):
		commandFunc = c.runCachePrune
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	c.initCache(ctx, client)

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
	slog.DebugContext(ctx, "Fetching user tasks", "integration_filter", c.statusIntegration)
	tasks, err := listUserTasks(ctx, client, c.statusIntegration, "")
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched user tasks", "count", len(tasks))

	slog.DebugContext(ctx, "Fetching discovery configs")
	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, client.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched discovery configs", "count", len(discoveryConfigs))

	slog.DebugContext(ctx, "Fetching integrations")
	integrations, err := listIntegrations(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched integrations", "count", len(integrations))

	summary := makeStatusSummary(tasks, discoveryConfigs, integrations, c.statusIntegration)

	slog.DebugContext(ctx, "Fetching SSM run events", "window", c.statusLast, "limit", c.statusSSMLimit)
	ssmStats, err := fetchSSMRunStats(ctx, client, c.cache, c.statusLast, c.statusFromUTC, c.statusToUTC, c.statusSSMLimit)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched SSM run events", "total", ssmStats.Total, "limit_reached", ssmStats.LimitReached)
	summary.SSMRunStats = ssmStats

	slog.DebugContext(ctx, "Fetching instance join events", "window", c.statusLast, "limit", c.statusJoinLimit)
	joinStats, err := fetchJoinStats(ctx, client, c.cache, c.statusLast, c.statusFromUTC, c.statusToUTC, c.statusJoinLimit)
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "Fetched instance join events", "total", joinStats.Total, "limit_reached", joinStats.LimitReached)
	summary.JoinStats = joinStats
	summary.CacheSummary = c.cache.cacheSummary()

	return trace.Wrap(writeOutputByFormat(c.output(), c.statusFormat, summary, func(w io.Writer) error {
		return renderStatusText(w, summary)
	}))
}

func fetchAuditEventStats(ctx context.Context, client *authclient.Client, cache *eventCache, last, fromUTC, toUTC string, eventType string, limit int) ([]apievents.AuditEvent, bool, error) {
	from, to, err := resolveTimeRangeFromFlags(last, fromUTC, toUTC)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	if limit <= 0 {
		limit = 200
	}

	result, err := cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, eventType, limit)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	if result.CacheHits > 0 {
		slog.DebugContext(ctx, "Audit event cache stats", "event_type", eventType, "cache_hits", result.CacheHits, "cache_misses", result.CacheMisses, "cache_files", result.CacheFiles)
	}
	return result.Events, result.LimitReached, nil
}

func fetchSSMRunStats(ctx context.Context, client *authclient.Client, cache *eventCache, last, fromUTC, toUTC string, limit int) (*auditEventStats, error) {
	events, limitReached, err := fetchAuditEventStats(ctx, client, cache, last, fromUTC, toUTC, libevents.SSMRunEvent, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	records := parseSSMRunEvents(events, ssmRunEventFilters{})
	analysis := analyzeSSMRuns(records)
	from, to, _ := resolveTimeRangeFromFlags(last, fromUTC, toUTC)
	stats := &auditEventStats{
		Window:        timeRangeDescriptionFromFlags(last, fromUTC, toUTC),
		From:          from,
		To:            to,
		Total:         analysis.Total,
		Success:       analysis.Success,
		Failed:        analysis.Failed,
		DistinctHosts: len(analysis.ByInstance),
		FailingHosts:  len(analysis.FailedByInstance),
		LimitReached:  limitReached,
	}
	if len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		stats.OldestEvent = oldest
		if limitReached {
			stats.EffectiveWindow = formatRelativeDelta(oldest, now, false)
			stats.SuggestedLimit = estimateRequiredLimit(stats.Total, oldest, now, from)
		}
	}
	return stats, nil
}

func fetchJoinStats(ctx context.Context, client *authclient.Client, cache *eventCache, last, fromUTC, toUTC string, limit int) (*auditEventStats, error) {
	events, limitReached, err := fetchAuditEventStats(ctx, client, cache, last, fromUTC, toUTC, libevents.InstanceJoinEvent, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	records := parseInstanceJoinEvents(events, joinEventFilters{})
	analysis := analyzeInstanceJoins(records)
	from, to, _ := resolveTimeRangeFromFlags(last, fromUTC, toUTC)
	stats := &auditEventStats{
		Window:        timeRangeDescriptionFromFlags(last, fromUTC, toUTC),
		From:          from,
		To:            to,
		Total:         analysis.Total,
		Success:       analysis.Success,
		Failed:        analysis.Failed,
		DistinctHosts: len(analysis.ByHost),
		FailingHosts:  len(analysis.FailedByHost),
		LimitReached:  limitReached,
	}
	if len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		stats.OldestEvent = oldest
		if limitReached {
			stats.EffectiveWindow = formatRelativeDelta(oldest, now, false)
			stats.SuggestedLimit = estimateRequiredLimit(stats.Total, oldest, now, from)
		}
	}
	return stats, nil
}

// estimateRequiredLimit calculates the limit needed to cover the full requested
// window based on the observed event rate. It extrapolates from the coverage so
// far (fetched events covering oldest→now) to the full window (from→now), then
// adds 5% headroom.
func estimateRequiredLimit(fetched int, oldest, now, from time.Time) int {
	covered := now.Sub(oldest)
	requested := now.Sub(from)
	if covered <= 0 || requested <= covered {
		return 0
	}
	ratio := float64(requested) / float64(covered)
	suggested := int(float64(fetched)*ratio*1.05) + 1
	return roundUpSignificant(suggested, 2)
}

// roundUpSignificant rounds n up to the top `digits` most significant digits.
// e.g. roundUpSignificant(1234567, 4) = 1235000.
func roundUpSignificant(n, digits int) int {
	if n <= 0 || digits <= 0 {
		return n
	}
	divisor := 1
	v := n
	for v >= 10 {
		v /= 10
		divisor *= 10
	}
	// divisor is 10^(numDigits-1), reduce by (digits-1) to keep `digits` significant
	for i := 1; i < digits && divisor > 1; i++ {
		divisor /= 10
	}
	if divisor <= 1 {
		return n
	}
	return ((n + divisor - 1) / divisor) * divisor
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
	start, end, err := parseRange(c.tasksShowRange)
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, "", "")
	if err != nil {
		return trace.Wrap(err)
	}
	task, err := findTaskByNamePrefix(tasks, c.tasksShowName)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(writeOutputByFormat(c.output(), c.tasksShowFormat, task, func(w io.Writer) error {
		return renderTaskDetailsText(w, task, start, end, buildTaskShowCommand(c))
	}))
}

func (c *Command) runSSMRunsList(ctx context.Context, client *authclient.Client) error {
	start, end, err := parseRange(c.ssmRunsRange)
	if err != nil {
		return trace.Wrap(err)
	}

	analysis, vmGroups, meta, err := c.fetchSSMRunData(ctx, client, "")
	if err != nil {
		return trace.Wrap(err)
	}
	slog.DebugContext(ctx, "SSM run data fetched", "total_runs", analysis.Total, "failed", analysis.Failed, "vm_groups", len(vmGroups))

	allFailingVMGroups := selectFailingVMGroups(vmGroups, 0)
	displayedVMs, vmPage := paginateSlice(vmGroups, start, end)

	output := c.buildSSMRunsOutput(analysis, vmGroups, allFailingVMGroups, displayedVMs, vmPage, meta)
	if c.ssmRunsClusterErrors {
		slog.DebugContext(ctx, "Starting error clustering")
		output.ErrorClusters = clusterSSMRunErrors(vmGroups)
		slog.DebugContext(ctx, "Error clustering complete", "clusters", len(output.ErrorClusters))
	}
	baseCommand := buildSSMRunsCommand(c, "ls", "")
	return trace.Wrap(c.writeSSMRunsOutput(output, "", baseCommand))
}

func (c *Command) runSSMRunsShow(ctx context.Context, client *authclient.Client) error {
	instanceID := c.ssmRunsShowInstanceID
	analysis, vmGroups, meta, err := c.fetchSSMRunData(ctx, client, instanceID)
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

	output := c.buildSSMRunsOutput(analysis, vmGroups, allFailingVMGroups, showGroups, vmPage, meta)
	baseCommand := buildSSMRunsCommand(c, "show", instanceID)
	return trace.Wrap(c.writeSSMRunsOutput(output, instanceID, baseCommand))
}

const timeRangeFormat = "2006-01-02T15:04"

// parseDurationWithDays extends time.ParseDuration to support "d" (days) suffix.
// Examples: "7d", "2d12h", "30m".
func parseDurationWithDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	// Check for a "d" component and expand it to hours.
	if i := strings.Index(s, "d"); i > 0 {
		dayPart := s[:i]
		rest := s[i+1:]
		days, err := strconv.Atoi(dayPart)
		if err != nil {
			return 0, fmt.Errorf("invalid day count %q", dayPart)
		}
		expanded := fmt.Sprintf("%dh", days*24)
		if rest != "" {
			expanded += rest
		}
		return time.ParseDuration(expanded)
	}
	return time.ParseDuration(s)
}

func resolveTimeRangeFromFlags(last, fromUTC, toUTC string) (from, to time.Time, err error) {
	hasLast := strings.TrimSpace(last) != ""
	hasFrom := strings.TrimSpace(fromUTC) != ""
	hasTo := strings.TrimSpace(toUTC) != ""

	if hasLast && (hasFrom || hasTo) {
		return time.Time{}, time.Time{}, trace.BadParameter("--last cannot be combined with --from-utc/--to-utc")
	}

	now := time.Now().UTC()

	if hasFrom || hasTo {
		if hasFrom {
			from, err = time.Parse(timeRangeFormat, strings.TrimSpace(fromUTC))
			if err != nil {
				return time.Time{}, time.Time{}, trace.BadParameter("invalid --from-utc value %q, expected format %s", fromUTC, timeRangeFormat)
			}
		} else {
			from = now.Add(-24 * time.Hour)
		}
		if hasTo {
			to, err = time.Parse(timeRangeFormat, strings.TrimSpace(toUTC))
			if err != nil {
				return time.Time{}, time.Time{}, trace.BadParameter("invalid --to-utc value %q, expected format %s", toUTC, timeRangeFormat)
			}
		} else {
			to = now
		}
		return from, to, nil
	}

	if hasLast {
		d, err := parseDurationWithDays(last)
		if err != nil || d <= 0 {
			return time.Time{}, time.Time{}, trace.BadParameter("invalid --last value %q, expected duration like 1h, 30m, or 7d", last)
		}
		return now.Add(-d), now, nil
	}

	// Default: last 1h.
	return now.Add(-time.Hour), now, nil
}

func timeRangeDescriptionFromFlags(last, fromUTC, toUTC string) string {
	if last := strings.TrimSpace(last); last != "" {
		return "last " + last
	}
	hasFrom := strings.TrimSpace(fromUTC) != ""
	hasTo := strings.TrimSpace(toUTC) != ""
	if hasFrom && hasTo {
		return fromUTC + " to " + toUTC
	}
	if hasFrom {
		return "from " + fromUTC
	}
	if hasTo {
		return "to " + toUTC
	}
	return "last 1h"
}

type fetchMeta struct {
	FetchLimit     int
	LimitReached   bool
	SuggestedLimit int
	CacheHits      int
	CacheMisses    int
	CacheFiles     int
}

func (c *Command) fetchSSMRunData(ctx context.Context, client *authclient.Client, instanceIDFilter string) (ssmRunAnalysis, []ssmVMGroup, fetchMeta, error) {
	from, to, err := resolveTimeRangeFromFlags(c.ssmRunsLast, c.ssmRunsFromUTC, c.ssmRunsToUTC)
	if err != nil {
		return ssmRunAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}
	fetchLimit := c.ssmRunsLimit
	if fetchLimit <= 0 {
		fetchLimit = 200
	}

	result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, libevents.SSMRunEvent, fetchLimit)
	if err != nil {
		return ssmRunAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}

	meta := fetchMeta{
		FetchLimit:   fetchLimit,
		LimitReached: result.LimitReached,
		CacheHits:    result.CacheHits,
		CacheMisses:  result.CacheMisses,
		CacheFiles:   result.CacheFiles,
	}
	slog.DebugContext(ctx, "Parsing SSM run events", "events", len(result.Events))
	parseStart := time.Now()
	records := parseSSMRunEvents(result.Events, ssmRunEventFilters{
		InstanceID: instanceIDFilter,
	})
	slog.DebugContext(ctx, "Parsed SSM run events", "records", len(records), "elapsed", time.Since(parseStart).Round(time.Millisecond))
	if meta.LimitReached && len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		meta.SuggestedLimit = estimateRequiredLimit(len(records), oldest, now, from)
	}
	analysis := analyzeSSMRuns(records)
	vmGroups := groupSSMRunsByVM(records)
	return analysis, vmGroups, meta, nil
}

func (c *Command) buildSSMRunsOutput(analysis ssmRunAnalysis, vmGroups, allFailingVMGroups, displayedVMGroups []ssmVMGroup, vmPage pageInfo, meta fetchMeta) ssmRunsOutput {
	from, to, _ := resolveTimeRangeFromFlags(c.ssmRunsLast, c.ssmRunsFromUTC, c.ssmRunsToUTC)
	return ssmRunsOutput{
		Window:         timeRangeDescriptionFromFlags(c.ssmRunsLast, c.ssmRunsFromUTC, c.ssmRunsToUTC),
		From:           from,
		To:             to,
		FetchLimit:     meta.FetchLimit,
		LimitReached:   meta.LimitReached,
		SuggestedLimit: meta.SuggestedLimit,
		CacheSummary: c.cache.cacheSummary(),
		TotalRuns:    analysis.Total,
		SuccessRuns:  analysis.Success,
		FailedRuns:   analysis.Failed,
		TotalVMs:     len(vmGroups),
		FailingVMs:   len(allFailingVMGroups),
		VMPage:       vmPage,
		VMs:          displayedVMGroups,
	}
}

func (c *Command) writeSSMRunsOutput(output ssmRunsOutput, instanceIDFilter, baseCommand string) error {
	return trace.Wrap(writeOutputByFormat(c.output(), c.ssmRunsFormat, output, func(w io.Writer) error {
		return renderSSMRunsText(w, output, instanceIDFilter, c.ssmRunsShowAll, baseCommand)
	}))
}

func (c *Command) runIntegrationList(ctx context.Context, client *authclient.Client) error {
	integrations, err := listIntegrations(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}

	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, client.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, "", usertasksapi.TaskStateOpen)
	if err != nil {
		return trace.Wrap(err)
	}

	statsMap := buildIntegrationStatsMap(discoveryConfigs)
	taskCountMap := countTasksByIntegration(tasks)
	items := toIntegrationListItems(integrations, statsMap, taskCountMap)

	listOutput := integrationListOutput{
		Total: len(items),
		Items: items,
	}
	return trace.Wrap(writeOutputByFormat(c.output(), c.integrationListFormat, listOutput, func(w io.Writer) error {
		return renderIntegrationListText(w, items)
	}))
}

func (c *Command) runIntegrationShow(ctx context.Context, client *authclient.Client) error {
	ig, err := client.GetIntegration(ctx, c.integrationShowName)
	if err != nil {
		return trace.Wrap(err)
	}

	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, client.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, c.integrationShowName, usertasksapi.TaskStateOpen)
	if err != nil {
		return trace.Wrap(err)
	}

	detail := buildIntegrationDetail(ig, discoveryConfigs, tasks)
	return trace.Wrap(writeOutputByFormat(c.output(), c.integrationShowFormat, detail, func(w io.Writer) error {
		return renderIntegrationShowText(w, detail)
	}))
}

func (c *Command) runJoinsList(ctx context.Context, client *authclient.Client) error {
	if c.joinsRaw {
		return trace.Wrap(c.runJoinsRaw(ctx, client, ""))
	}

	start, end, err := parseRange(c.joinsRange)
	if err != nil {
		return trace.Wrap(err)
	}

	analysis, hostGroups, meta, err := c.fetchJoinData(ctx, client, "")
	if err != nil {
		return trace.Wrap(err)
	}
	allFailingGroups := selectFailingJoinGroups(hostGroups, 0)
	displayGroups := hostGroups
	if c.joinsHideUnknown {
		displayGroups = filterOutUnknownJoinGroups(hostGroups)
	}
	displayedGroups, hostPage := paginateSlice(displayGroups, start, end)

	output := c.buildJoinsOutput(analysis, hostGroups, allFailingGroups, displayedGroups, hostPage, meta)
	baseCommand := buildJoinsCommand(c, "ls", "")
	return trace.Wrap(c.writeJoinsOutput(output, "", baseCommand))
}

func (c *Command) runJoinsShow(ctx context.Context, client *authclient.Client) error {
	hostID := c.joinsShowHostID
	if c.joinsRaw {
		return trace.Wrap(c.runJoinsRaw(ctx, client, hostID))
	}
	analysis, hostGroups, meta, err := c.fetchJoinData(ctx, client, hostID)
	if err != nil {
		return trace.Wrap(err)
	}
	allFailingGroups := selectFailingJoinGroups(hostGroups, 0)
	var showGroups []joinGroup
	for _, group := range hostGroups {
		if group.HostID == hostID {
			showGroups = []joinGroup{group}
			break
		}
	}
	hostPage := fullPageInfo(len(showGroups))

	output := c.buildJoinsOutput(analysis, hostGroups, allFailingGroups, showGroups, hostPage, meta)
	baseCommand := buildJoinsCommand(c, "show", hostID)
	return trace.Wrap(c.writeJoinsOutput(output, hostID, baseCommand))
}

// runJoinsRaw fetches raw instance.join audit events and dumps them as JSON.
// When hostIDFilter is non-empty, only events matching that HostID are included.
func (c *Command) runJoinsRaw(ctx context.Context, client *authclient.Client, hostIDFilter string) error {
	from, to, err := resolveTimeRangeFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC)
	if err != nil {
		return trace.Wrap(err)
	}
	fetchLimit := c.joinsLimit
	if fetchLimit <= 0 {
		fetchLimit = 200
	}

	result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, libevents.InstanceJoinEvent, fetchLimit)
	if err != nil {
		return trace.Wrap(err)
	}

	// Filter to matching host ID if specified, and collect raw events.
	var rawEvents []any
	for _, ev := range result.Events {
		join, ok := ev.(*apievents.InstanceJoin)
		if !ok {
			continue
		}
		if hostIDFilter != "" && join.HostID != hostIDFilter {
			continue
		}
		rawEvents = append(rawEvents, join)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return trace.Wrap(enc.Encode(rawEvents))
}

func (c *Command) fetchJoinData(ctx context.Context, client *authclient.Client, hostIDFilter string) (joinAnalysis, []joinGroup, fetchMeta, error) {
	from, to, err := resolveTimeRangeFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC)
	if err != nil {
		return joinAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}
	fetchLimit := c.joinsLimit
	if fetchLimit <= 0 {
		fetchLimit = 200
	}

	result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, libevents.InstanceJoinEvent, fetchLimit)
	if err != nil {
		return joinAnalysis{}, nil, fetchMeta{}, trace.Wrap(err)
	}

	meta := fetchMeta{
		FetchLimit:   fetchLimit,
		LimitReached: result.LimitReached,
		CacheHits:    result.CacheHits,
		CacheMisses:  result.CacheMisses,
		CacheFiles:   result.CacheFiles,
	}
	records := parseInstanceJoinEvents(result.Events, joinEventFilters{
		HostID: hostIDFilter,
	})
	if meta.LimitReached && len(records) > 0 {
		now := time.Now().UTC()
		oldest := records[len(records)-1].parsedEventTime
		meta.SuggestedLimit = estimateRequiredLimit(len(records), oldest, now, from)
	}
	analysis := analyzeInstanceJoins(records)
	hostGroups := groupJoinsByHost(records)
	return analysis, hostGroups, meta, nil
}

func (c *Command) buildJoinsOutput(analysis joinAnalysis, hostGroups, allFailingGroups, displayedGroups []joinGroup, hostPage pageInfo, meta fetchMeta) joinsOutput {
	from, to, _ := resolveTimeRangeFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC)
	return joinsOutput{
		Window:         timeRangeDescriptionFromFlags(c.joinsLast, c.joinsFromUTC, c.joinsToUTC),
		From:           from,
		To:             to,
		FetchLimit:     meta.FetchLimit,
		LimitReached:   meta.LimitReached,
		SuggestedLimit: meta.SuggestedLimit,
		CacheSummary: c.cache.cacheSummary(),
		TotalJoins:   analysis.Total,
		SuccessJoins: analysis.Success,
		FailedJoins:  analysis.Failed,
		TotalHosts:   len(hostGroups),
		FailingHosts: len(allFailingGroups),
		HostPage:     hostPage,
		Hosts:        displayedGroups,
	}
}

func (c *Command) writeJoinsOutput(output joinsOutput, hostIDFilter, baseCommand string) error {
	return trace.Wrap(writeOutputByFormat(c.output(), c.joinsFormat, output, func(w io.Writer) error {
		return renderJoinsText(w, output, hostIDFilter, c.joinsShowAll, baseCommand)
	}))
}

func (c *Command) runInventoryList(ctx context.Context, clt *authclient.Client) error {
	start, end, err := parseRange(c.inventoryRange)
	if err != nil {
		return trace.Wrap(err)
	}

	allHosts, meta, err := c.fetchInventoryData(ctx, clt, "")
	if err != nil {
		return trace.Wrap(err)
	}

	filteredHosts := filterInventoryHosts(allHosts, c.inventoryStateFilter, c.inventoryMethodFilter)
	displayedHosts, hostPage := paginateSlice(filteredHosts, start, end)

	output := c.buildInventoryOutput(allHosts, displayedHosts, hostPage, meta)
	baseCommand := buildInventoryCommand(c, "ls", "")
	return trace.Wrap(c.writeInventoryOutput(output, "", baseCommand))
}

func (c *Command) runInventoryShow(ctx context.Context, clt *authclient.Client) error {
	hostID := c.inventoryShowHostID

	allHosts, meta, err := c.fetchInventoryData(ctx, clt, hostID)
	if err != nil {
		return trace.Wrap(err)
	}

	var showHosts []inventoryHost
	for _, h := range allHosts {
		if h.DisplayID == hostID || h.HostID == hostID || h.InstanceID == hostID {
			showHosts = []inventoryHost{h}
			break
		}
	}
	hostPage := fullPageInfo(len(showHosts))

	output := c.buildInventoryOutput(allHosts, showHosts, hostPage, meta)
	baseCommand := buildInventoryCommand(c, "show", hostID)
	return trace.Wrap(c.writeInventoryOutput(output, hostID, baseCommand))
}

func (c *Command) fetchInventoryData(ctx context.Context, clt *authclient.Client, hostIDFilter string) ([]inventoryHost, fetchMeta, error) {
	from, to, err := resolveTimeRangeFromFlags(c.inventoryLast, c.inventoryFromUTC, c.inventoryToUTC)
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}
	fetchLimit := c.inventoryLimit
	if fetchLimit <= 0 {
		fetchLimit = 200
	}

	// Fetch all three data sources. Nodes are always current (not windowed).
	nodes, err := client.GetAllResources[types.Server](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
	})
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}

	ssmResult, err := fetchAuditEventsInRange(ctx, clt, c.cache, from, to, libevents.SSMRunEvent, fetchLimit)
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}
	ssmRecords := parseSSMRunEvents(ssmResult.Events, ssmRunEventFilters{InstanceID: hostIDFilter})

	joinResult, err := fetchAuditEventsInRange(ctx, clt, c.cache, from, to, libevents.InstanceJoinEvent, fetchLimit)
	if err != nil {
		return nil, fetchMeta{}, trace.Wrap(err)
	}
	joinRecords := parseInstanceJoinEvents(joinResult.Events, joinEventFilters{HostID: hostIDFilter})

	meta := fetchMeta{
		FetchLimit:   fetchLimit,
		LimitReached: ssmResult.LimitReached || joinResult.LimitReached,
		CacheHits:    ssmResult.CacheHits + joinResult.CacheHits,
		CacheMisses:  ssmResult.CacheMisses + joinResult.CacheMisses,
		CacheFiles:   ssmResult.CacheFiles + joinResult.CacheFiles,
	}

	if meta.LimitReached {
		// Use the worst case: whichever event type needs the larger limit.
		now := time.Now().UTC()
		var suggested int
		if ssmResult.LimitReached && len(ssmResult.Events) > 0 {
			oldest := ssmResult.Events[len(ssmResult.Events)-1].GetTime()
			if s := estimateRequiredLimit(len(ssmResult.Events), oldest, now, from); s > suggested {
				suggested = s
			}
		}
		if joinResult.LimitReached && len(joinResult.Events) > 0 {
			oldest := joinResult.Events[len(joinResult.Events)-1].GetTime()
			if s := estimateRequiredLimit(len(joinResult.Events), oldest, now, from); s > suggested {
				suggested = s
			}
		}
		meta.SuggestedLimit = suggested
	}

	return buildInventoryHosts(nodes, ssmRecords, joinRecords), meta, nil
}

func fetchAuditEventsInRange(ctx context.Context, clt *authclient.Client, cache *eventCache, from, to time.Time, eventType string, limit int) (cachedSearchResult, error) {
	result, err := cache.cachedSearchEvents(ctx, clt.SearchEvents, from, to, eventType, limit)
	if err != nil {
		return cachedSearchResult{}, trace.Wrap(err)
	}
	return result, nil
}

func filterInventoryHosts(hosts []inventoryHost, stateFilter, methodFilter string) []inventoryHost {
	if stateFilter == "" && methodFilter == "" {
		return hosts
	}
	stateFilter = strings.ToLower(strings.TrimSpace(stateFilter))
	methodFilter = strings.ToLower(strings.TrimSpace(methodFilter))

	filtered := make([]inventoryHost, 0, len(hosts))
	for _, h := range hosts {
		if stateFilter != "" && !matchesStateFilter(h.State, stateFilter) {
			continue
		}
		if methodFilter != "" && !strings.EqualFold(h.Method, methodFilter) {
			continue
		}
		filtered = append(filtered, h)
	}
	return filtered
}

func matchesStateFilter(state inventoryHostState, filter string) bool {
	switch filter {
	case "online":
		return state == inventoryStateOnline || state == inventoryStateJoinedOnly
	case "offline":
		return state == inventoryStateOffline
	case "failed":
		return state == inventoryStateJoinFailed || state == inventoryStateSSMFailed
	case "attempted":
		return state == inventoryStateSSMAttempted
	default:
		return strings.EqualFold(string(state), filter)
	}
}

func (c *Command) buildInventoryOutput(allHosts, displayedHosts []inventoryHost, hostPage pageInfo, meta fetchMeta) inventoryOutput {
	var online, offline, failed int
	for _, h := range allHosts {
		switch h.State {
		case inventoryStateOnline, inventoryStateJoinedOnly:
			online++
		case inventoryStateOffline:
			offline++
		case inventoryStateJoinFailed, inventoryStateSSMFailed:
			failed++
		}
	}
	from, to, _ := resolveTimeRangeFromFlags(c.inventoryLast, c.inventoryFromUTC, c.inventoryToUTC)
	return inventoryOutput{
		Window:         timeRangeDescriptionFromFlags(c.inventoryLast, c.inventoryFromUTC, c.inventoryToUTC),
		From:           from,
		To:             to,
		CacheSummary:   c.cache.cacheSummary(),
		TotalHosts:     len(allHosts),
		OnlineHosts:    online,
		OfflineHosts:   offline,
		FailedHosts:    failed,
		FetchLimit:     meta.FetchLimit,
		LimitReached:   meta.LimitReached,
		SuggestedLimit: meta.SuggestedLimit,
		HostPage:       hostPage,
		Hosts:          displayedHosts,
	}
}

func (c *Command) writeInventoryOutput(output inventoryOutput, hostIDFilter, baseCommand string) error {
	return trace.Wrap(writeOutputByFormat(c.output(), c.inventoryFormat, output, func(w io.Writer) error {
		return renderInventoryText(w, output, hostIDFilter, c.inventoryShowAll, baseCommand)
	}))
}

func buildInventoryCommand(c *Command, subCmd, hostID string) string {
	cmd := []string{"tctl discovery inventory", subCmd}
	if hostID != "" {
		cmd = append(cmd, shellQuoteArg(hostID))
	}
	if c.inventoryLast != "" && c.inventoryLast != "24h" {
		cmd = append(cmd, fmt.Sprintf("--last=%s", c.inventoryLast))
	}
	if c.inventoryFromUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--from-utc=%s", c.inventoryFromUTC))
	}
	if c.inventoryToUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--to-utc=%s", c.inventoryToUTC))
	}
	if c.inventoryLimit != 0 && c.inventoryLimit != defaultFetchLimit {
		cmd = append(cmd, fmt.Sprintf("--limit=%d", c.inventoryLimit))
	}
	if c.inventoryShowAll {
		cmd = append(cmd, "--show-all-events")
	}
	if subCmd == "ls" {
		if c.inventoryStateFilter != "" {
			cmd = append(cmd, fmt.Sprintf("--state=%s", c.inventoryStateFilter))
		}
		if c.inventoryMethodFilter != "" {
			cmd = append(cmd, fmt.Sprintf("--method=%s", c.inventoryMethodFilter))
		}
		if c.inventoryRange != "" && c.inventoryRange != "0,50" {
			cmd = append(cmd, fmt.Sprintf("--range=%s", c.inventoryRange))
		}
	}
	return strings.Join(cmd, " ")
}

func (c *Command) runCacheLoad(ctx context.Context, client *authclient.Client) error {
	if c.cache == nil {
		return trace.BadParameter("cache not initialized (could not resolve cluster name)")
	}

	from, to, err := resolveTimeRangeFromFlags(c.cacheLoadLast, c.cacheLoadFromUTC, c.cacheLoadToUTC)
	if err != nil {
		return trace.Wrap(err)
	}

	w := c.output()
	for _, eventType := range []string{libevents.SSMRunEvent, libevents.InstanceJoinEvent} {
		fmt.Fprintf(w, "Loading %s events for %s ...\n", eventType, timeRangeDescriptionFromFlags(c.cacheLoadLast, c.cacheLoadFromUTC, c.cacheLoadToUTC))
		result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, eventType, 0)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(w, "  %d events (%d cached, %d fetched)\n", len(result.Events), result.CacheHits, result.CacheMisses)
	}
	fmt.Fprintf(w, "\nCache directory: %s\n", c.cache.Dir)
	return nil
}

func (c *Command) runCacheStatus(_ context.Context, _ *authclient.Client) error {
	if c.cache == nil {
		return trace.BadParameter("cache not initialized (could not resolve cluster name)")
	}

	w := c.output()
	style := newTextStyle(w)
	now := time.Now().UTC()

	fmt.Fprintf(w, "Cache directory: %s\n", c.cache.Dir)

	var allFiles []cacheFile
	for _, eventType := range []string{libevents.SSMRunEvent, libevents.InstanceJoinEvent} {
		files, err := c.cache.listCacheFiles(eventType)
		if err != nil {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("Error listing %s cache files: %v", eventType, err)))
			continue
		}
		allFiles = append(allFiles, files...)
	}

	if len(allFiles) == 0 {
		fmt.Fprintf(w, "\n%s\n", style.warning("No cached files."))
		return nil
	}

	fmt.Fprintf(w, "\n%s\n", style.section(fmt.Sprintf("Cached Files [%d]", len(allFiles))))
	for i, f := range allFiles {
		if i > 0 {
			fmt.Fprintln(w)
		}
		h := f.Header
		details := []keyValue{
			{Key: "EVENT TYPE", Value: style.section(h.EventType)},
			{Key: "FROM", Value: h.From.Format(cacheTimeFormat)},
			{Key: "TO", Value: h.To.Format(cacheTimeFormat)},
			{Key: "EVENTS", Value: style.good(fmt.Sprintf("%d", h.Count))},
			{Key: "FETCHED", Value: fmt.Sprintf("%s (%s)", h.FetchedAt.Format(time.RFC3339), formatRelativeTime(h.FetchedAt, now))},
			{Key: "FILE", Value: filepath.Base(f.Path)},
		}
		if err := style.numberedBlock(w, i, details); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *Command) runCachePrune(_ context.Context, _ *authclient.Client) error {
	if c.cache == nil {
		return trace.BadParameter("cache not initialized (could not resolve cluster name)")
	}

	entries, err := os.ReadDir(c.cache.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(c.output(), "No cache directory found at %s\n", c.cache.Dir)
			return nil
		}
		return trace.Wrap(err)
	}

	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if err := os.Remove(filepath.Join(c.cache.Dir, e.Name())); err != nil {
			slog.Warn("Failed to remove cache file", "file", e.Name(), "error", err)
			continue
		}
		removed++
	}

	fmt.Fprintf(c.output(), "Removed %d cache file(s) from %s\n", removed, c.cache.Dir)
	return nil
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
