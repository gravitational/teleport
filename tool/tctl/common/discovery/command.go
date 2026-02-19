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
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"

	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/trace"
)

// discoveryClient abstracts the authclient.Client methods used by discovery
// commands, enabling unit tests with mock implementations.
type discoveryClient interface {
	// SearchEvents searches audit events.
	SearchEvents(ctx context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error)
	// ListIntegrations lists integrations with pagination.
	ListIntegrations(ctx context.Context, pageSize int, nextToken string) ([]types.Integration, string, error)
	// GetIntegration returns a single integration by name.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)
	// UserTasksClient returns a client for managing user tasks.
	UserTasksClient() services.UserTasks
	// DiscoveryConfigClient returns a client for managing discovery configs.
	DiscoveryConfigClient() services.DiscoveryConfigWithStatusUpdater
	// GetResources lists resources with pagination (used by GetAllResources).
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
	// GetClusterName returns the cluster name.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// formatCSV is the CSV output format identifier.
const formatCSV = "csv"

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
	ssmRunsGroup        bool
	ssmRunsGroupDebug   bool
	ssmRunsSimilarity     float64
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

	groupByAccount bool
	csvDir         string

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

func (c *Command) initCache(ctx context.Context, client discoveryClient) {
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

// buildOpt appends domain-specific flags to a command being built.
type buildOpt func(c *Command) []string

// buildSubcommand builds a "tctl discovery <sub> <subCmd>" string,
// appending common time/limit flags and any domain-specific opts.
func buildSubcommand(c *Command, sub, subCmd string, last, fromUTC, toUTC string, limit int, entityID string, opts ...buildOpt) string {
	cmd := []string{"tctl discovery " + sub, subCmd}
	if entityID != "" {
		cmd = append(cmd, shellQuoteArg(entityID))
	}
	if last != "" {
		cmd = append(cmd, fmt.Sprintf("--last=%s", last))
	}
	if fromUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--from-utc=%s", fromUTC))
	}
	if toUTC != "" {
		cmd = append(cmd, fmt.Sprintf("--to-utc=%s", toUTC))
	}
	if limit != 0 && limit != defaultFetchLimit {
		cmd = append(cmd, fmt.Sprintf("--limit=%d", limit))
	}
	for _, opt := range opts {
		cmd = append(cmd, opt(c)...)
	}
	return strings.Join(cmd, " ")
}

func buildSSMRunsCommand(c *Command, subCmd string, instanceID string) string {
	return buildSubcommand(c, "ssm-runs", subCmd, c.ssmRunsLast, c.ssmRunsFromUTC, c.ssmRunsToUTC, c.ssmRunsLimit, instanceID,
		func(c *Command) []string {
			var flags []string
			if c.ssmRunsGroup {
				flags = append(flags, "--group")
				if c.ssmRunsSimilarity != groupingDefaults().drainSimThreshold {
					flags = append(flags, fmt.Sprintf("--similarity=%.2f", c.ssmRunsSimilarity))
				}
			}
			if c.ssmRunsShowAll {
				flags = append(flags, "--show-all-runs")
			}
			if instanceID == "" && c.ssmRunsRange != "" {
				flags = append(flags, fmt.Sprintf("--range=%s", c.ssmRunsRange))
			}
			return flags
		},
	)
}

func buildJoinsCommand(c *Command, subCmd string, hostID string) string {
	return buildSubcommand(c, "joins", subCmd, c.joinsLast, c.joinsFromUTC, c.joinsToUTC, c.joinsLimit, hostID,
		func(c *Command) []string {
			var flags []string
			if c.joinsShowAll {
				flags = append(flags, "--show-all-joins")
			}
			if c.joinsHideUnknown {
				flags = append(flags, "--hide-unknown")
			}
			if hostID == "" && c.joinsRange != "" {
				flags = append(flags, fmt.Sprintf("--range=%s", c.joinsRange))
			}
			return flags
		},
	)
}

// Initialize allows Command to plug itself into the CLI parser.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	c.discovery = app.Command("discovery", "Troubleshoot Discovery auto-enrollment issues.").Alias("discover")

	c.statusCmd = c.discovery.Command("status", "Triage Discovery health: tasks, configs, integrations, and next actions.")
	c.statusCmd.Flag("integration", "Filter tasks by integration.").StringVar(&c.statusIntegration)
	c.statusCmd.Flag("last", "Time window for audit event stats (SSM runs, joins), e.g. 1h, 30m, 24h.").StringVar(&c.statusLast)
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
	c.ssmRunsListCmd.Flag("group", "Group similar SSM runs using Drain+MinHash/LSH.").BoolVar(&c.ssmRunsGroup)
	c.ssmRunsListCmd.Flag("similarity", "Drain similarity threshold for grouping (0.0-1.0, lower merges more aggressively).").Default("0.4").Float64Var(&c.ssmRunsSimilarity)
	c.ssmRunsListCmd.Flag("group-debug", "Include grouping pipeline diagnostics and pairwise similarities.").BoolVar(&c.ssmRunsGroupDebug)
	c.ssmRunsListCmd.Flag("group-by-account", "Group results by AWS account ID in JSON/YAML output.").BoolVar(&c.groupByAccount)
	c.ssmRunsListCmd.Flag("format", "Output format: text, json, yaml, or csv.").Default(teleport.Text).EnumVar(&c.ssmRunsFormat, teleport.Text, teleport.JSON, teleport.YAML, formatCSV)
	c.ssmRunsListCmd.Flag("csv-dir", "Directory to write CSV files into (only with --format=csv).").Default(".").StringVar(&c.csvDir)

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
	c.joinsListCmd.Flag("group-by-account", "Group results by AWS account ID in JSON/YAML output.").BoolVar(&c.groupByAccount)
	c.joinsListCmd.Flag("format", "Output format: text, json, yaml, or csv.").Default(teleport.Text).EnumVar(&c.joinsFormat, teleport.Text, teleport.JSON, teleport.YAML, formatCSV)
	c.joinsListCmd.Flag("csv-dir", "Directory to write CSV files into (only with --format=csv).").Default(".").StringVar(&c.csvDir)

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
	c.inventoryListCmd.Flag("last", "Time window for audit events, e.g. 1h, 30m, 24h.").StringVar(&c.inventoryLast)
	c.inventoryListCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryFromUTC)
	c.inventoryListCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryToUTC)
	c.inventoryListCmd.Flag("limit", "Maximum audit events to fetch per event type.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.inventoryLimit)
	c.inventoryListCmd.Flag("range", "Range of hosts to display as start,end (0-indexed, exclusive end).").Default("0,50").StringVar(&c.inventoryRange)
	c.inventoryListCmd.Flag("state", "Filter by state: online, offline, failed, attempted.").StringVar(&c.inventoryStateFilter)
	c.inventoryListCmd.Flag("method", "Filter by join method: ec2, iam, azure, token, etc.").StringVar(&c.inventoryMethodFilter)
	c.inventoryListCmd.Flag("group-by-account", "Group results by AWS account ID in JSON/YAML output.").BoolVar(&c.groupByAccount)
	c.inventoryListCmd.Flag("format", "Output format: text, json, yaml, or csv.").Default(teleport.Text).EnumVar(&c.inventoryFormat, teleport.Text, teleport.JSON, teleport.YAML, formatCSV)
	c.inventoryListCmd.Flag("csv-dir", "Directory to write CSV files into (only with --format=csv).").Default(".").StringVar(&c.csvDir)

	c.inventoryShowCmd = c.inventoryCmd.Command("show", "Show timeline for one host combining SSM runs and join events.")
	c.inventoryShowCmd.Arg("host-id", "Host ID to inspect.").Required().StringVar(&c.inventoryShowHostID)
	c.inventoryShowCmd.Flag("last", "Time window for audit events, e.g. 1h, 30m, 24h.").StringVar(&c.inventoryLast)
	c.inventoryShowCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryFromUTC)
	c.inventoryShowCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.inventoryToUTC)
	c.inventoryShowCmd.Flag("limit", "Maximum audit events to fetch per event type.").Default(strconv.Itoa(defaultFetchLimit)).IntVar(&c.inventoryLimit)
	c.inventoryShowCmd.Flag("show-all-events", "Show full event timeline for the selected host.").BoolVar(&c.inventoryShowAll)
	c.inventoryShowCmd.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.inventoryFormat, teleport.Text, teleport.JSON, teleport.YAML)

	c.cacheCmd = c.discovery.Command("cache", "Manage the local audit event cache.").Hidden()
	c.cacheLoadCmd = c.cacheCmd.Command("load", "Pre-fetch all audit events into the local cache.")
	c.cacheLoadCmd.Flag("last", "Time window to cache, e.g. 1h, 24h, 7d.").StringVar(&c.cacheLoadLast)
	c.cacheLoadCmd.Flag("from-utc", "Start of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.cacheLoadFromUTC)
	c.cacheLoadCmd.Flag("to-utc", "End of time range in UTC (format: 2006-01-02T15:04).").StringVar(&c.cacheLoadToUTC)
	c.cacheStatusCmd = c.cacheCmd.Command("status", "Show cache file inventory.")
	c.cachePruneCmd = c.cacheCmd.Command("prune", "Delete all cached audit events.")
}

// TryRun takes the CLI command and executes it.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(context.Context, discoveryClient) error

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

// effectiveFetchLimit returns the fetch limit, defaulting to 200 if <= 0.
func effectiveFetchLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	return limit
}

// buildFetchMeta constructs a fetchMeta from a cached search result.
func buildFetchMeta(fetchLimit int, result cachedSearchResult) fetchMeta {
	return fetchMeta{
		FetchLimit:   fetchLimit,
		LimitReached: result.LimitReached,
		CacheHits:    result.CacheHits,
		CacheMisses:  result.CacheMisses,
		CacheFiles:   result.CacheFiles,
	}
}

type fetchMeta struct {
	FetchLimit     int
	LimitReached   bool
	SuggestedLimit int
	CacheHits      int
	CacheMisses    int
	CacheFiles     int
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

func fetchAuditEventStats(ctx context.Context, client discoveryClient, cache *eventCache, last, fromUTC, toUTC string, eventType string, limit int) ([]apievents.AuditEvent, bool, error) {
	from, to, err := resolveTimeRangeFromFlags(last, fromUTC, toUTC)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	limit = effectiveFetchLimit(limit)

	result, err := cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, eventType, limit)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	if result.CacheHits > 0 {
		slog.DebugContext(ctx, "Audit event cache stats", "event_type", eventType, "cache_hits", result.CacheHits, "cache_misses", result.CacheMisses, "cache_files", result.CacheFiles)
	}
	return result.Events, result.LimitReached, nil
}

func fetchAuditEventsInRange(ctx context.Context, clt discoveryClient, cache *eventCache, from, to time.Time, eventType string, limit int) (cachedSearchResult, error) {
	result, err := cache.cachedSearchEvents(ctx, clt.SearchEvents, from, to, eventType, limit)
	if err != nil {
		return cachedSearchResult{}, trace.Wrap(err)
	}
	return result, nil
}

func buildInventoryCommand(c *Command, subCmd, hostID string) string {
	return buildSubcommand(c, "inventory", subCmd, c.inventoryLast, c.inventoryFromUTC, c.inventoryToUTC, c.inventoryLimit, hostID,
		func(c *Command) []string {
			var flags []string
			if c.inventoryShowAll {
				flags = append(flags, "--show-all-events")
			}
			if subCmd == "ls" {
				if c.inventoryStateFilter != "" {
					flags = append(flags, fmt.Sprintf("--state=%s", c.inventoryStateFilter))
				}
				if c.inventoryMethodFilter != "" {
					flags = append(flags, fmt.Sprintf("--method=%s", c.inventoryMethodFilter))
				}
				if c.inventoryRange != "" && c.inventoryRange != "0,50" {
					flags = append(flags, fmt.Sprintf("--range=%s", c.inventoryRange))
				}
			}
			return flags
		},
	)
}
