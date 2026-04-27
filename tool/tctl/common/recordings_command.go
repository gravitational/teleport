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

package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	recordingstui "github.com/gravitational/teleport/tool/tctl/common/recordings"
)

const (
	searchModeHybrid    = "hybrid"
	searchModeKeyword   = "keyword"
	searchModeEmbedding = "embeddings"
)

// RecordingsCommand implements "tctl recordings" group of commands.
type RecordingsCommand struct {
	config *servicecfg.Config

	// format is the output format (text, json, or yaml)
	format string
	// recordingsList implements the "tctl recordings ls" subcommand.
	recordingsList *kingpin.CmdClause
	// recordingsDownload implements the "tctl recordings download" subcommand.
	recordingsDownload *kingpin.CmdClause
	// recordingsEncryption implements the "tctl recordings encryption" subcommand.
	recordingsEncryption recordingsEncryptionCommand
	// fromUTC is the start time to use for the range of recordings listed by the recorded session listing command
	fromUTC string
	// toUTC is the start time to use for the range of recordings listed by the recorded session listing command
	toUTC string
	// maxRecordingsToShow is the maximum number of recordings to show per page of results
	maxRecordingsToShow int
	// recordingsSince is a duration which sets the time into the past in which to list session recordings
	recordingsSince string

	// recordingsDownloadSessionID is the session ID to download recordings for
	recordingsDownloadSessionID string
	// recordingsDownloadOutputDir is the output directory to download session recordings to
	recordingsDownloadOutputDir string

	// recordingsSearch implements the "tctl recordings search" subcommand.
	recordingsSearch *kingpin.CmdClause
	// searchQuery is the free-text semantic/keyword query.
	searchQuery []string
	// searchFromUTC is the start of the time range for the search.
	searchFromUTC string
	// searchToUTC is the end of the time range for the search.
	searchToUTC string
	// searchLabel filters results by resource labels (key=value pairs).
	searchLabel string
	// searchAccessRequests filters results by access request IDs.
	searchAccessRequests []string
	// searchLimit is the maximum number of results to return.
	searchLimit uint32
	// searchFormat is the output format for search results.
	searchFormat string
	// searchKinds filters results by session kind (ssh, db, k8s, desktop).
	searchKinds []string
	// searchUsername filters results by the Teleport username that initiated the session.
	searchUsername string
	// searchRoles filters results by roles held by the user during the session.
	searchRoles []string
	// searchResourceKind filters results by the Teleport resource type (node, kube_cluster, db).
	searchResourceKind string
	// searchResourceName filters results by the resource name.
	searchResourceName string
	// searchSeverity filters results by minimum severity level (low/medium/high/critical).
	searchSeverity string
	// searchMode controls which search strategy to use: hybrid (default), keyword, or embedding.
	searchMode string

	// stdout allows to switch standard output source for resource command. Used in tests.
	stdout io.Writer
}

// Initialize allows RecordingsCommand to plug itself into the CLI parser
func (c *RecordingsCommand) Initialize(app *kingpin.Application, t *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	c.config = config
	recordings := app.Command("recordings", "View and control session recordings.")
	c.recordingsList = recordings.Command("ls", "List recorded sessions.")
	c.recordingsList.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)+". Defaults to 'text'.").Default(teleport.Text).StringVar(&c.format)
	c.recordingsList.Flag("from-utc", fmt.Sprintf("Start of time range in which recordings are listed. Format %s. Defaults to 24 hours ago.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.fromUTC)
	c.recordingsList.Flag("to-utc", fmt.Sprintf("End of time range in which recordings are listed. Format %s. Defaults to current time.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.toUTC)
	c.recordingsList.Flag("limit", fmt.Sprintf("Maximum number of recordings to show. Default %s.", defaults.TshTctlSessionListLimit)).Default(defaults.TshTctlSessionListLimit).IntVar(&c.maxRecordingsToShow)
	c.recordingsList.Flag("last", "Duration into the past from which session recordings should be listed. Format 5h30m40s").StringVar(&c.recordingsSince)
	c.recordingsSearch = recordings.Command("search", "Search session recordings using semantic and keyword queries.")
	c.recordingsSearch.Arg("query", `Natural language description of the sessions to find (e.g. "SSH sessions exfiltrating data to external endpoints").`).StringsVar(&c.searchQuery)
	c.recordingsSearch.Flag("from", fmt.Sprintf("Start of time range. Format %s. Defaults to 24 hours ago.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.searchFromUTC)
	c.recordingsSearch.Flag("to", fmt.Sprintf("End of time range. Format %s. Defaults to current time.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.searchToUTC)
	c.recordingsSearch.Flag("label", "Filter by resource labels (key=value pairs), e.g. env/prod=true,db/type=postgres.").StringVar(&c.searchLabel)
	c.recordingsSearch.Flag("access-request", "Filter by access request ID. Can be specified multiple times.").StringsVar(&c.searchAccessRequests)
	c.recordingsSearch.Flag("kind", "Filter by session kind (ssh, db, k8s, desktop). Can be specified multiple times.").StringsVar(&c.searchKinds)
	c.recordingsSearch.Flag("username", "Filter by the Teleport username that initiated the session.").StringVar(&c.searchUsername)
	c.recordingsSearch.Flag("role", "Filter by role held during the session. Can be specified multiple times.").StringsVar(&c.searchRoles)
	c.recordingsSearch.Flag("resource-kind", "Filter by Teleport resource type (node, kube_cluster, db).").StringVar(&c.searchResourceKind)
	c.recordingsSearch.Flag("resource-name", "Filter by resource name.").StringVar(&c.searchResourceName)
	c.recordingsSearch.Flag("severity", "Minimum severity level to include (low, medium, high, critical).").StringVar(&c.searchSeverity)
	c.recordingsSearch.Flag("search-mode", "Search strategy to use when search queries are provided.").Default(searchModeHybrid).EnumVar(&c.searchMode, searchModeHybrid, searchModeKeyword, searchModeEmbedding)
	c.recordingsSearch.Flag("limit", "Maximum number of results to return.").Default(defaults.TshTctlSessionListLimit).Uint32Var(&c.searchLimit)
	c.recordingsSearch.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)+". Defaults to 'text'.").Default(teleport.Text).StringVar(&c.searchFormat)

	c.recordingsEncryption.Initialize(recordings, c.stdout)

	download := recordings.Command("download", "Download session recordings.")
	download.Arg("session-id", "ID of the session to download recordings for.").Required().StringVar(&c.recordingsDownloadSessionID)
	pwd, err := os.Getwd()
	if err != nil {
		pwd = "."
	}
	download.Flag("output-dir", "Directory to download session recordings to.").Short('o').Default(pwd).StringVar(&c.recordingsDownloadOutputDir)
	c.recordingsDownload = download

	if c.recordingsEncryption.stdout == nil {
		c.recordingsEncryption.stdout = c.stdout
	}
}

// TryRun attempts to run subcommands like "recordings ls".
func (c *RecordingsCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.recordingsList.FullCommand():
		commandFunc = c.ListRecordings
	case c.recordingsDownload.FullCommand():
		commandFunc = c.DownloadRecordings
	case c.recordingsSearch.FullCommand():
		commandFunc = c.SearchRecordings
	default:
		return c.recordingsEncryption.TryRun(ctx, cmd, clientFunc)
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

func (c *RecordingsCommand) ListRecordings(ctx context.Context, tc *authclient.Client) error {
	fromUTC, toUTC, err := defaults.SearchSessionRange(clockwork.NewRealClock(), c.fromUTC, c.toUTC, c.recordingsSince)
	if err != nil {
		return trace.Errorf("cannot request recordings: %v", err)
	}
	// Max number of days is limited to prevent too many requests being sent if dynamo is used as a backend.
	if days := toUTC.Sub(fromUTC).Hours() / 24; days > defaults.TshTctlSessionDayLimit {
		return trace.Errorf("date range for recording listing too large: %v days specified: limit %v days",
			days, defaults.TshTctlSessionDayLimit)
	}
	recordings, err := client.GetPaginatedSessions(ctx, fromUTC, toUTC,
		apidefaults.DefaultChunkSize, types.EventOrderDescending, c.maxRecordingsToShow, tc)
	if err != nil {
		return trace.Errorf("getting session events: %v", err)
	}
	return trace.Wrap(common.ShowSessions(recordings, c.format, c.stdout))
}

func (c *RecordingsCommand) DownloadRecordings(ctx context.Context, tc *authclient.Client) (err error) {
	sessionID, err := session.ParseID(c.recordingsDownloadSessionID)
	if err != nil {
		return trace.BadParameter("invalid session id")
	}

	e, err := createFileWriter(ctx, *sessionID, c.recordingsDownloadOutputDir)
	if err != nil {
		return trace.Wrap(err, "creating file downloader")
	}

	path := filepath.Join(c.recordingsDownloadOutputDir, string(*sessionID)+".tar")
	defer func() {
		completeErr := e.Complete(ctx)
		if err == nil && completeErr == nil {
			return
		}
		localRemErr := os.Remove(path)
		// ignore file not found errors
		if os.IsNotExist(localRemErr) {
			localRemErr = nil
		}
		err = trace.NewAggregate(err, completeErr, localRemErr)
	}()

	recC, errC := tc.StreamSessionEvents(ctx, *sessionID, 0)
loop:
	for {
		select {
		case rec, ok := <-recC:
			if !ok {
				break loop
			}
			prepared, err := e.PrepareSessionEvent(rec)
			if err != nil {
				return trace.Wrap(err, "preparing recording event")
			}
			err = e.RecordEvent(ctx, prepared)
			if err != nil {
				return trace.Wrap(err, "recording session event")
			}
		case err := <-errC:
			if err != nil && !trace.IsEOF(err) {
				return trace.Wrap(err, "downloading session recordings")
			}
			return nil
		}
	}
	fmt.Fprintf(c.stdout, "Session recording %q downloaded to %s\n", string(*sessionID), path)
	return nil
}

// SearchRecordings implements "tctl recordings search <query>".
func (c *RecordingsCommand) SearchRecordings(ctx context.Context, tc *authclient.Client) error {
	fromUTC, toUTC, err := defaults.SearchSessionRange(clockwork.NewRealClock(), c.searchFromUTC, c.searchToUTC, "")
	if err != nil {
		return trace.Wrap(err)
	}

	searchClient := tc.SessionSearchServiceClient()
	if err := checkSessionSearchEnabled(ctx, searchClient); err != nil {
		return trace.Wrap(err)
	}

	var labels map[string]string
	if c.searchLabel != "" {
		labels, err = client.ParseLabelSpec(c.searchLabel)
		if err != nil {
			return trace.Wrap(err, "parsing --label")
		}
	}
	var search []string
	if len(c.searchQuery) > 0 {
		search = []string{strings.Join(c.searchQuery, " ")}
	}
	req := &sessionsearchv1pb.SearchSessionSummariesRequest{
		StartTime:        timestamppb.New(fromUTC),
		EndTime:          timestamppb.New(toUTC),
		SearchQueries:    search,
		ResourceLabels:   labels,
		AccessRequestIds: c.searchAccessRequests,
		Kinds:            c.searchKinds,
		UserRoles:        c.searchRoles,
		MaxResults:       c.searchLimit,
	}
	if c.searchUsername != "" {
		req.Username = &c.searchUsername
	}
	if c.searchResourceKind != "" {
		req.ResourceKind = &c.searchResourceKind
	}
	if c.searchResourceName != "" {
		req.ResourceName = &c.searchResourceName
	}
	if c.searchSeverity != "" {
		level, err := parseSeverity(c.searchSeverity)
		if err != nil {
			return trace.Wrap(err)
		}
		req.Severity = &level
	}
	if c.searchMode != "" {
		mode, err := parseSearchMode(c.searchMode)
		if err != nil {
			return trace.Wrap(err)
		}
		req.SearchMode = mode
	}

	stream, err := searchClient.SearchSessionSummaries(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	sessions, nextToken, err := collectStream(stream)
	if err != nil {
		return trace.Wrap(err)
	}

	fetcher := recordingstui.BatchFetcher(func(ctx context.Context, token string) ([]*sessionsearchv1pb.SessionSummary, string, error) {
		// When BatchToken is set the server ignores all other filter fields per the proto contract.
		batchStream, err := searchClient.SearchSessionSummaries(ctx, &sessionsearchv1pb.SearchSessionSummariesRequest{
			BatchToken: token,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return collectStream(batchStream)
	})

	return trace.Wrap(showSessionSummaries(ctx, sessions, nextToken, c.searchFormat, c.stdout, tc.SummarizerServiceClient(), fetcher))
}

func showSessionSummaries(ctx context.Context, sessions []*sessionsearchv1pb.SessionSummary, nextToken, format string, w io.Writer, summaryGetter recordingstui.SummaryGetter, fetcher recordingstui.BatchFetcher) error {
	switch format {
	case teleport.JSON:
		// Only the first batch is output; nextToken and fetcher are unused for
		// non-interactive formats.
		return trace.Wrap(utils.WriteJSONArray(w, sessions))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(w, sessions))
	default:
		if len(sessions) == 0 {
			fmt.Fprintln(w, "No sessions found.")
			return nil
		}
		return recordingstui.RunSearchTUI(ctx, sessions, nextToken, summaryGetter, fetcher)
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
		switch p := resp.Payload.(type) {
		case *sessionsearchv1pb.SearchSessionSummariesResponse_Summary:
			sessions = append(sessions, p.Summary)
		case *sessionsearchv1pb.SearchSessionSummariesResponse_BatchComplete_:
			if p.BatchComplete.GetHasMore() {
				nextToken = p.BatchComplete.GetNextBatchToken()
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

// createFileWriter creates a file-based session event writer that outputs to a file.
func createFileWriter(ctx context.Context, sessionID session.ID, outputDir string) (*events.SessionWriter, error) {
	fileStreamer, err := filesessions.NewStreamer(
		filesessions.StreamerConfig{
			Dir: outputDir,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create fileStreamer")
	}

	e, err := events.NewSessionWriter(
		events.SessionWriterConfig{
			SessionID: sessionID,
			Component: "downloader",
			// Use NoOpPreparer as events are already prepared by the server.
			Preparer: &events.NoOpPreparer{},
			Context:  ctx,
			Clock:    clockwork.NewRealClock(),
			Streamer: fileStreamer,
		},
	)
	return e, trace.Wrap(err, "creating session writer")
}
