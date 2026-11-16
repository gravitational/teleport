/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Package recordingsearch implements session recording search for tsh and tctl.
package recordingsearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
	recordingstui "github.com/gravitational/teleport/tool/tctl/common/recordings"
)

const (
	searchModeHybrid    = "hybrid"
	searchModeKeyword   = "keyword"
	searchModeEmbedding = "embeddings"
)

// SearchConfig holds the flags for tsh and tctl recordings search.
type SearchConfig struct {
	// Query is the free-text semantic/keyword query.
	Query []string
	// FromUTC is the start of the time range for the search.
	FromUTC string
	// ToUTC is the end of the time range for the search.
	ToUTC string
	// Label filters results by resource labels (key=value pairs).
	Label string
	// AccessRequests filters results by access request IDs.
	AccessRequests []string
	// Limit is the maximum number of results to return.
	Limit uint32
	// Format is the output format for search results.
	Format string
	// Kinds filters results by session kind (ssh, db, k8s, desktop, beam).
	Kinds []string
	// Username filters results by the Teleport username that initiated the session.
	Username string
	// Roles filters results by roles held by the user during the session.
	Roles []string
	// ResourceKind filters results by the Teleport resource type (node, kube_cluster, db, windows_desktop, linux_desktop, beam).
	ResourceKind string
	// ResourceName filters results by the resource name.
	ResourceName string
	// ServerHostname filters SSH results by server hostname.
	ServerHostname string
	// ServerAddr filters SSH results by server address.
	ServerAddr string
	// PodNamespace filters Kubernetes results by pod namespace.
	PodNamespace string
	// PodName filters Kubernetes results by pod name.
	PodName string
	// DatabaseName filters database results by database name.
	DatabaseName string
	// Severity filters results by minimum severity level (low/medium/high/critical).
	Severity string
	// Mode controls which search strategy to use: hybrid (default), keyword, or embedding.
	Mode string
	// ResumeToken resumes a previous JSON/YAML search from a truncated result set.
	ResumeToken string
	// ReviewReasons filters results to sessions that have at least one of
	// the specified review reasons. An empty slice disables this filter.
	ReviewReasons []string
}

// Register adds the search subcommand and flags to a recordings command.
func (c *SearchConfig) Register(recordings *kingpin.CmdClause) *kingpin.CmdClause {
	command := recordings.Command("search", "Search session recordings and beams using semantic and keyword queries.")
	command.Arg("query", `Natural language description of the sessions to find (e.g. "SSH sessions exfiltrating data to external endpoints").`).StringsVar(&c.Query)
	command.Flag("from", fmt.Sprintf("Start of time range. Format %s. Defaults to 24 hours ago.", defaults.TshTctlSessionListTimeFormat)).Hidden().StringVar(&c.FromUTC)
	command.Flag("to", fmt.Sprintf("End of time range. Format %s. Defaults to current time.", defaults.TshTctlSessionListTimeFormat)).Hidden().StringVar(&c.ToUTC)
	command.Flag("from-utc", fmt.Sprintf("Start of time range. Format %s. Defaults to 24 hours ago.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.FromUTC)
	command.Flag("to-utc", fmt.Sprintf("End of time range. Format %s. Defaults to current time.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.ToUTC)
	command.Flag("label", "Filter by resource labels (key=value pairs), e.g. env/prod=true,db/type=postgres.").StringVar(&c.Label)
	command.Flag("access-request", "Filter by access request ID. Can be specified multiple times.").StringsVar(&c.AccessRequests)
	command.Flag("kind", "Filter by session kind (ssh, db, k8s, desktop, beam). Can be specified multiple times.").StringsVar(&c.Kinds)
	command.Flag("username", "Filter by the Teleport username that initiated the session or owns the beam.").StringVar(&c.Username)
	command.Flag("role", "Filter by role held during the session. Can be specified multiple times.").StringsVar(&c.Roles)
	command.Flag("resource-kind", "Filter by Teleport resource type (node, kube_cluster, db, windows_desktop, linux_desktop, beam).").StringVar(&c.ResourceKind)
	command.Flag("resource-name", "Filter by resource name.").StringVar(&c.ResourceName)
	command.Flag("server-hostname", "Filter SSH sessions by server hostname.").StringVar(&c.ServerHostname)
	command.Flag("server-addr", "Filter SSH sessions by server address.").StringVar(&c.ServerAddr)
	command.Flag("pod-namespace", "Filter Kubernetes sessions by pod namespace.").StringVar(&c.PodNamespace)
	command.Flag("pod-name", "Filter Kubernetes sessions by pod name.").StringVar(&c.PodName)
	command.Flag("database-name", "Filter database sessions by database name.").StringVar(&c.DatabaseName)
	command.Flag("severity", "Minimum severity level to include (low, medium, high, critical).").StringVar(&c.Severity)
	command.Flag("search-mode", "Search strategy to use when search queries are provided.").Default(searchModeHybrid).EnumVar(&c.Mode, searchModeHybrid, searchModeKeyword, searchModeEmbedding)
	command.Flag("limit", "Maximum number of results to return.").Default(defaults.TshTctlSessionListLimit).Uint32Var(&c.Limit)
	command.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)+". Defaults to 'text'.").Default(teleport.Text).StringVar(&c.Format)
	command.Flag("resume-token", "Resume a previous JSON/YAML search from a truncated result set (token printed to stderr when results are truncated).").StringVar(&c.ResumeToken)
	command.Flag("review-reason", "Filter to sessions that have at least one of the given review reasons. Use 'any' to match any flagged session. Can be specified multiple times.").
		EnumsVar(&c.ReviewReasons, append(reviewReasonFlagOptions(), "any")...)

	return command
}

// Run searches recordings with authenticated clients supplied by the CLI.
func (c *SearchConfig) Run(ctx context.Context, searchClient sessionsearchv1pb.SessionSearchServiceClient, summaryClient summarizerv1pb.SummarizerServiceClient, stdout, stderr io.Writer) error {
	if err := checkSessionSearchEnabled(ctx, searchClient); err != nil {
		return trace.Wrap(err)
	}

	fromUTC, toUTC, err := defaults.SearchSessionRange(clockwork.NewRealClock(), c.FromUTC, c.ToUTC, "")
	if err != nil {
		return trace.Wrap(err)
	}

	var labels map[string]string
	if c.Label != "" {
		labels, err = parse.LabelSelectorSpec(c.Label)
		if err != nil {
			return trace.Wrap(err, "parsing --label")
		}
	}
	var search []string
	if len(c.Query) > 0 {
		search = []string{strings.Join(c.Query, " ")}
	}
	req := sessionsearchv1pb.SearchSessionSummariesRequest_builder{
		StartTime:        timestamppb.New(fromUTC),
		EndTime:          timestamppb.New(toUTC),
		SearchQueries:    search,
		ResourceLabels:   labels,
		AccessRequestIds: c.AccessRequests,
		Kinds:            c.Kinds,
		UserRoles:        c.Roles,
		MaxResults:       c.Limit,
	}.Build()
	resourceProperties, err := c.buildSearchResourceProperties()
	if err != nil {
		return trace.Wrap(err)
	}
	req.SetResourceProperties(resourceProperties)
	if c.Username != "" {
		req.SetUsername(c.Username)
	}
	if c.ResourceKind != "" {
		req.SetResourceKind(c.ResourceKind)
	}
	if c.ResourceName != "" {
		req.SetResourceName(c.ResourceName)
	}
	if c.Severity != "" {
		level, err := parseSeverity(c.Severity)
		if err != nil {
			return trace.Wrap(err)
		}
		req.SetSeverity(level)
	}
	if c.Mode != "" {
		mode, err := parseSearchMode(c.Mode)
		if err != nil {
			return trace.Wrap(err)
		}
		req.SetSearchMode(mode)
	}
	if len(c.ReviewReasons) > 0 {
		var reasons []summarizerv1pb.NeedsReviewReason
		hasAny := slices.Contains(c.ReviewReasons, "any")
		if hasAny {
			for _, k := range reviewReasonFlagOptions() {
				reasons = append(reasons, reviewReasonNames[k])
			}
		} else {
			reasons = make([]summarizerv1pb.NeedsReviewReason, 0, len(c.ReviewReasons))
			for _, s := range c.ReviewReasons {
				r, err := parseReviewReason(s)
				if err != nil {
					return trace.Wrap(err)
				}
				reasons = append(reasons, r)
			}
		}
		req.SetFilterNeedsFurtherReviewReasons(reasons)
	}

	// fetcher resends the full request with the batch token set. It is used for
	// both the first page (empty or resume token) and subsequent "load more"
	// pages. The batch token is only a cursor: the server requires and applies
	// the filter fields (start_time, end_time, ...) on every request, so the
	// original request must be replayed each time rather than sending the token
	// alone.
	fetcher := recordingstui.BatchFetcher(func(ctx context.Context, token string) ([]*sessionsearchv1pb.SessionSummary, string, error) {
		pageReq := proto.CloneOf(req)
		pageReq.SetBatchToken(token)
		stream, err := searchClient.SearchSessionSummaries(ctx, pageReq)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return collectStream(stream)
	})

	// c.ResumeToken is empty for a fresh search and set when resuming a
	// previous one; either way the first page is just a fetch with that token.
	sessions, nextToken, err := fetcher(ctx, c.ResumeToken)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(showSessionSummaries(ctx, sessions, nextToken, c.Format, stdout, stderr, summaryClient, fetcher, resumeCommand))
}

// maxJSONYAMLSessions is the maximum number of sessions collected before truncating
// structured (JSON/YAML) output and printing a --resume-token hint to stderr.
const maxJSONYAMLSessions = 100

func showSessionSummaries(ctx context.Context, sessions []*sessionsearchv1pb.SessionSummary, nextToken, format string, w, stderr io.Writer, summaryGetter recordingstui.SummaryGetter, fetcher recordingstui.BatchFetcher, resumeCmd func(token string) string) error {
	switch format {
	case teleport.JSON, teleport.YAML:
		// Paginate automatically up to maxJSONYAMLSessions for structured output.
		all := sessions
		for nextToken != "" && len(all) < maxJSONYAMLSessions {
			more, tok, err := fetcher(ctx, nextToken)
			if err != nil {
				return trace.Wrap(err)
			}
			all = append(all, more...)
			nextToken = tok
		}
		if nextToken != "" {
			fmt.Fprintf(stderr, "Showing %d sessions (more available). Resume with:\n  %s\n", len(all), resumeCmd(nextToken))
		}
		if format == teleport.JSON {
			return trace.Wrap(utils.WriteJSONArray(w, all))
		}
		return trace.Wrap(utils.WriteYAML(w, all))
	default:
		if len(sessions) == 0 {
			fmt.Fprintln(w, "No sessions found.")
			return nil
		}
		return recordingstui.RunSearchTUI(ctx, sessions, nextToken, summaryGetter, fetcher)
	}
}

// resumeCommand renders a replayable command for the given resume token by
// reusing the exact arguments of the current invocation (os.Args) and rewriting
// only the resume token, replaced in place when already present or appended
// otherwise. All other flags (including any time range the user passed) are
// preserved verbatim.
func resumeCommand(token string) string {
	args := append([]string(nil), os.Args...)
	if len(args) > 0 {
		args[0] = filepath.Base(args[0])
	}
	args = replaceOrAppendFlag(args, []string{"--resume-token"}, "--resume-token", token)

	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

// replaceOrAppendFlag removes every occurrence of the given flag names (in both
// "--flag value" and "--flag=value" forms) from args, then appends
// "canonical value" so the flag ends up set exactly once to value. The multiple
// names allow collapsing aliases (e.g. --from and --from-utc) onto a single
// canonical flag.
func replaceOrAppendFlag(args, names []string, canonical, value string) []string {
	match := func(arg string) (isFlag, inlineValue bool) {
		for _, n := range names {
			if arg == n {
				return true, false
			}
			if strings.HasPrefix(arg, n+"=") {
				return true, true
			}
		}
		return false, false
	}
	out := make([]string, 0, len(args)+2)
	for i := 0; i < len(args); i++ {
		isFlag, inline := match(args[i])
		if !isFlag {
			out = append(out, args[i])
			continue
		}
		if !inline {
			i++ // skip the value token that follows "--flag value"
		}
	}
	return append(out, canonical, value)
}

// shellQuote wraps s in single quotes when it contains characters that a shell
// would otherwise interpret, so the rendered resume command can be pasted as-is.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`*?(){}[]|&;<>~#!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (c *SearchConfig) buildSearchResourceProperties() (*sessionsearchv1pb.ResourceProperties, error) {
	sshSet := c.ServerHostname != "" || c.ServerAddr != ""
	kubernetesSet := c.PodNamespace != "" || c.PodName != ""
	databaseSet := c.DatabaseName != ""

	var variants []string
	if sshSet {
		variants = append(variants, "SSH")
	}
	if kubernetesSet {
		variants = append(variants, "Kubernetes")
	}
	if databaseSet {
		variants = append(variants, "Database")
	}
	if len(variants) > 1 {
		return nil, trace.BadParameter("resource property filters can only target one session kind at a time, got %s", strings.Join(variants, ", "))
	}

	switch {
	case sshSet:
		props := &sessionsearchv1pb.SSHProperties{}
		if c.ServerHostname != "" {
			props.SetServerHostname(c.ServerHostname)
		}
		if c.ServerAddr != "" {
			props.SetServerAddr(c.ServerAddr)
		}
		return sessionsearchv1pb.ResourceProperties_builder{
			Ssh: proto.ValueOrDefault(props),
		}.Build(), nil
	case kubernetesSet:
		props := &sessionsearchv1pb.KubernetesProperties{}
		if c.PodNamespace != "" {
			props.SetPodNamespace(c.PodNamespace)
		}
		if c.PodName != "" {
			props.SetPodName(c.PodName)
		}
		return sessionsearchv1pb.ResourceProperties_builder{
			Kubernetes: proto.ValueOrDefault(props),
		}.Build(), nil
	case databaseSet:
		return sessionsearchv1pb.ResourceProperties_builder{
			Database: sessionsearchv1pb.DatabaseProperties_builder{
				DatabaseName: &c.DatabaseName,
			}.Build(),
		}.Build(), nil
	default:
		return nil, nil
	}
}

// checkSessionSearchEnabled returns an error if session search is not active on this cluster.
func checkSessionSearchEnabled(ctx context.Context, sc sessionsearchv1pb.SessionSearchServiceClient) error {
	resp, err := sc.IsEnabled(ctx, &sessionsearchv1pb.IsEnabledRequest{})
	if err != nil {
		return trace.Wrap(err)
	}
	switch resp.GetAvailability() {
	case sessionsearchv1pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_AVAILABLE,
		sessionsearchv1pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED:
		// UNSPECIFIED is the proto zero-value; older servers that predate this field
		// return it. Treat it as available for forward-compatibility.
		return nil
	case sessionsearchv1pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_NOT_IMPLEMENTED:
		return trace.NotImplemented("session search requires Access Graph to be enabled with session recording support")
	case sessionsearchv1pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_TRGM_UNAVAILABLE:
		return trace.NotImplemented("session search requires the pg_trgm PostgreSQL extension to be installed")
	case sessionsearchv1pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_PG_VECTOR_UNAVAILABLE:
		return trace.NotImplemented("session search requires the pgvector PostgreSQL extension to be installed")
	default:
		return nil
	}
}

// collectStream drains a SearchSessionSummaries server stream and returns the
// accumulated sessions and the next-page token (empty when there are no more pages).
func collectStream(stream sessionsearchv1pb.SessionSearchService_SearchSessionSummariesClient) ([]*sessionsearchv1pb.SessionSummary, string, error) {
	var sessions []*sessionsearchv1pb.SessionSummary
	var nextToken string
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		switch resp.WhichPayload() {
		case sessionsearchv1pb.SearchSessionSummariesResponse_Summary_case:
			sessions = append(sessions, resp.GetSummary())
		case sessionsearchv1pb.SearchSessionSummariesResponse_BatchComplete_case:
			if resp.GetBatchComplete().GetHasMore() {
				nextToken = resp.GetBatchComplete().GetNextBatchToken()
			}
		}
	}
	return sessions, nextToken, nil
}

// parseSearchMode converts a CLI search-mode string to a SearchMode proto enum value.
func parseSearchMode(s string) (sessionsearchv1pb.SearchMode, error) {
	switch s {
	case searchModeHybrid:
		return sessionsearchv1pb.SearchMode_SEARCH_MODE_HYBRID, nil
	case searchModeKeyword:
		return sessionsearchv1pb.SearchMode_SEARCH_MODE_KEYWORD_ONLY, nil
	case searchModeEmbedding:
		return sessionsearchv1pb.SearchMode_SEARCH_MODE_EMBEDDING_ONLY, nil
	default:
		return 0, trace.BadParameter("invalid --search-mode %q: must be one of %s, %s, %s", s, searchModeHybrid, searchModeKeyword, searchModeEmbedding)
	}
}

// parseSeverity converts a CLI severity string to a RiskLevel proto enum value.
func parseSeverity(s string) (summarizerv1pb.RiskLevel, error) {
	switch s {
	case "low":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_LOW, nil
	case "medium":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_MEDIUM, nil
	case "high":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH, nil
	case "critical":
		return summarizerv1pb.RiskLevel_RISK_LEVEL_CRITICAL, nil
	default:
		return 0, trace.BadParameter("invalid --severity %q: must be one of low, medium, high, critical", s)
	}
}

// reviewReasonNames maps every valid --review-reason CLI value to its proto
// enum counterpart. It is the single source of truth: the flag validates
// against its keys, and parseReviewReason converts using its values.
// A parity test asserts that exactly one entry exists per non-UNSPECIFIED
// NeedsReviewReason enum value.
var reviewReasonNames = map[string]summarizerv1pb.NeedsReviewReason{
	"access_request_resource_mismatch": summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_ACCESS_REQUEST_RESOURCE_MISMATCH,
	"analysis_declined":                summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_ANALYSIS_DECLINED,
	"classification_failed":            summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_CLASSIFICATION_FAILED,
	"command_analysis_failed":          summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_COMMAND_ANALYSIS_FAILED,
	"failed_to_fetch_access_request":   summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_FAILED_TO_FETCH_ACCESS_REQUEST,
	"output_not_fully_captured":        summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_OUTPUT_NOT_FULLY_CAPTURED,
	"too_large":                        summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_TOO_LARGE,
	"classifier_matched":               summarizerv1pb.NeedsReviewReason_NEEDS_REVIEW_REASON_CLASSIFIER_MATCHED,
}

// reviewReasonFlagOptions returns the valid --review-reason values in
// alphabetical order, used in flag help text and error messages.
func reviewReasonFlagOptions() []string {
	keys := slices.Collect(maps.Keys(reviewReasonNames))
	sort.Strings(keys)
	return keys
}

// parseReviewReason converts a validated --review-reason string to its proto
// enum value using reviewReasonNames.
func parseReviewReason(s string) (summarizerv1pb.NeedsReviewReason, error) {
	if r, ok := reviewReasonNames[s]; ok {
		return r, nil
	}
	return 0, trace.BadParameter("invalid --review-reason %q: must be one of %s", s, strings.Join(reviewReasonFlagOptions(), ", "))
}
