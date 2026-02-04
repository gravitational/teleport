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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
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

	// Summary subcommands
	// summaryView implements the "tctl recordings summary view" subcommand.
	summaryView *kingpin.CmdClause
	// summaryDownload implements the "tctl recordings summary download" subcommand.
	summaryDownload *kingpin.CmdClause
	// summarySessionID is the session ID for summary operations
	summarySessionID string
	// summaryFormat is the output format for summary view
	summaryFormat string
	// summaryUsePager enables interactive paging for text output
	summaryUsePager bool
	// summaryDownloadOutputFile is the output file path to download the summary to
	summaryDownloadOutputFile string

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
	c.recordingsEncryption.Initialize(recordings, c.stdout)

	download := recordings.Command("download", "Download session recordings.")
	download.Arg("session-id", "ID of the session to download recordings for.").Required().StringVar(&c.recordingsDownloadSessionID)
	pwd, err := os.Getwd()
	if err != nil {
		pwd = "."
	}
	download.Flag("output-dir", "Directory to download session recordings to.").Short('o').Default(pwd).StringVar(&c.recordingsDownloadOutputDir)
	c.recordingsDownload = download

	// Summary subcommands
	summary := recordings.Command("summary", "View and download AI-generated session summaries.")
	c.summaryView = summary.Command("view", "View a session summary.")
	c.summaryView.Arg("session-id", "ID of the session to view the summary for.").Required().StringVar(&c.summarySessionID)
	c.summaryView.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)+". Defaults to 'text'.").Default(teleport.Text).StringVar(&c.summaryFormat)
	c.summaryView.Flag("pager", "Use an interactive pager for text output. Defaults to true for terminals.").Default("true").BoolVar(&c.summaryUsePager)

	c.summaryDownload = summary.Command("download", "Download a session summary to a file.")
	c.summaryDownload.Arg("session-id", "ID of the session to download the summary for.").Required().StringVar(&c.summarySessionID)
	c.summaryDownload.Flag("output", "File path to save the summary to.").Short('o').Default(filepath.Join(pwd, "summary.json")).StringVar(&c.summaryDownloadOutputFile)

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
	case c.summaryView.FullCommand():
		commandFunc = c.ViewSummary
	case c.summaryDownload.FullCommand():
		commandFunc = c.DownloadSummary
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
	defer func() {
		completeErr := e.Complete(ctx)
		if err == nil && completeErr == nil {
			return
		}
		localRemErr := os.Remove(filepath.Join(c.recordingsDownloadOutputDir, string(*sessionID)+".tar"))
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
	return nil
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

// ViewSummary retrieves and displays a session summary
func (c *RecordingsCommand) ViewSummary(ctx context.Context, tc *authclient.Client) error {
	summarizerClient := tc.SummarizerServiceClient()

	resp, err := summarizerClient.GetSummary(ctx, &summarizerv1.GetSummaryRequest{
		SessionId: c.summarySessionID,
	})
	if err != nil {
		return trace.Wrap(err, "failed to get session summary")
	}

	summary := resp.GetSummary()
	if summary == nil {
		return trace.NotFound("no summary found for session %s", c.summarySessionID)
	}

	return trace.Wrap(c.formatSummary(summary))
}

// DownloadSummary downloads a session summary to a file
func (c *RecordingsCommand) DownloadSummary(ctx context.Context, tc *authclient.Client) error {
	summarizerClient := tc.SummarizerServiceClient()

	resp, err := summarizerClient.GetSummary(ctx, &summarizerv1.GetSummaryRequest{
		SessionId: c.summarySessionID,
	})
	if err != nil {
		return trace.Wrap(err, "failed to get session summary")
	}

	summary := resp.GetSummary()
	if summary == nil {
		return trace.NotFound("no summary found for session %s", c.summarySessionID)
	}

	rBytes, err := marshalSessionSummary(summary)
	if err != nil {
		return trace.Wrap(err, "failed to marshal summary")
	}
	// Write the summary content to the output file
	if err := os.WriteFile(c.summaryDownloadOutputFile, rBytes, 0o644); err != nil {
		return trace.Wrap(err, "failed to write summary to file")
	}

	fmt.Fprintf(c.stdout, "Summary downloaded to %s\n", c.summaryDownloadOutputFile)
	return nil
}

// formatSummary formats and displays the summary based on the output format
func (c *RecordingsCommand) formatSummary(summary *summarizerv1.Summary) error {
	switch c.summaryFormat {
	case teleport.Text:
		return c.formatSummaryText(summary)
	case teleport.JSON:
		return c.formatSummaryJSON(summary)
	case teleport.YAML:
		return c.formatSummaryYAML(summary)
	default:
		return trace.BadParameter("unsupported format %q", c.summaryFormat)
	}
}

func marshalSessionSummary(summary *summarizerv1.Summary) ([]byte, error) {
	rBytes, err := protojson.MarshalOptions{UseProtoNames: true, Multiline: true, Indent: "  "}.Marshal(summary)
	if err != nil {
		return nil, trace.Wrap(err, "failed to marshal summary to JSON")
	}
	return rBytes, nil
}

// formatSummaryText formats the summary in human-readable text format
func (c *RecordingsCommand) formatSummaryText(summary *summarizerv1.Summary) error {
	// Build the output in a buffer first
	var buf bytes.Buffer
	bold := func(s string) string { return "\x1b[1m" + s + "\x1b[0m" }

	fmt.Fprintf(&buf, "%s %s\n", bold("Session ID:"), summary.GetSessionId())
	fmt.Fprintf(&buf, "%s %s\n", bold("State:"), summary.GetState())
	fmt.Fprintf(&buf, "%s %s\n", bold("Model:"), summary.GetModelName())

	if summary.GetInferenceStartedAt() != nil {
		fmt.Fprintf(&buf, "%s %s\n", bold("Started:"), summary.GetInferenceStartedAt().AsTime())
	}
	if summary.GetInferenceFinishedAt() != nil {
		fmt.Fprintf(&buf, "%s %s\n", bold("Finished:"), summary.GetInferenceFinishedAt().AsTime())
	}

	// Enhanced summary information
	if enhanced := summary.GetEnhancedSummary(); enhanced != nil {
		fmt.Fprintf(&buf, "\n%s %s\n", bold("Risk Level:"), enhanced.GetRiskLevel())

		if enhanced.GetCompromiseIndicators() {
			fmt.Fprintf(&buf, "\n%s Compromise indicators detected!\n", bold("WARNING:"))
		}

		if len(enhanced.GetSuspiciousActivities()) > 0 {
			fmt.Fprintf(&buf, "\n%s\n", bold("Suspicious Activities:"))
			for _, activity := range enhanced.GetSuspiciousActivities() {
				fmt.Fprintf(&buf, "  - %s\n", activity)
			}
		}
	}

	if summary.GetErrorMessage() != "" {
		fmt.Fprintf(&buf, "\n%s %s\n", bold("Error:"), summary.GetErrorMessage())
	}

	if summary.GetContent() != "" {
		// Convert markdown bold to terminal bold for better readability
		content := convertMarkdownBoldToANSI(summary.GetContent())
		fmt.Fprintf(&buf, "\n%s\n%s\n", bold("Summary:"), content)
	}

	// Display commands with their details
	if enhanced := summary.GetEnhancedSummary(); enhanced != nil && len(enhanced.GetCommands()) > 0 {
		fmt.Fprintf(&buf, "\n%s\n", bold(fmt.Sprintf("Commands Executed (%d total)", len(enhanced.GetCommands()))))

		for i, cmd := range enhanced.GetCommands() {
			fmt.Fprintf(&buf, "[%d] %s\n", i+1, cmd.GetCommand())

			// Risk and category information
			fmt.Fprintf(&buf, "    %s %s", bold("Risk Level:"), cmd.GetRiskLevel())
			if cmd.GetRiskScore() > 0 {
				fmt.Fprintf(&buf, " (Score: %d)", cmd.GetRiskScore())
			}
			fmt.Fprintf(&buf, "\n")

			if cmd.GetCategory() != summarizerv1.CommandCategory_COMMAND_CATEGORY_UNSPECIFIED {
				fmt.Fprintf(&buf, "    %s %s\n", bold("Category:"), cmd.GetCategory())
			}

			// Description
			if desc := cmd.GetShortDescription(); desc != "" {
				fmt.Fprintf(&buf, "    %s %s\n", bold("Description:"), desc)
			}

			// Timing information
			if start := cmd.GetStartOffset(); start != nil {
				fmt.Fprintf(&buf, "    %s %s", bold("Time:"), start.AsDuration())
				if end := cmd.GetEndOffset(); end != nil {
					duration := end.AsDuration() - start.AsDuration()
					fmt.Fprintf(&buf, " (duration: %s)", duration)
				}
				fmt.Fprintf(&buf, "\n")
			}

			// Status
			if cmd.GetSuccess() {
				fmt.Fprintf(&buf, "    %s Success\n", bold("Status:"))
			} else {
				fmt.Fprintf(&buf, "    %s Failed\n", bold("Status:"))
			}

			// Security flags
			var flags []string
			if cmd.GetPrivilegeEscalation() {
				flags = append(flags, "Privilege Escalation")
			}
			if cmd.GetDataExfiltration() {
				flags = append(flags, "Data Exfiltration")
			}
			if cmd.GetPersistence() {
				flags = append(flags, "Persistence")
			}
			if cmd.GetHasSensitiveData() {
				flags = append(flags, "Sensitive Data")
			}
			if len(flags) > 0 {
				fmt.Fprintf(&buf, "    %s %s\n", bold("Security Flags:"), strings.Join(flags, ", "))
			}

			// Threat information
			if cmd.GetThreatCategory() != summarizerv1.ThreatCategory_THREAT_CATEGORY_UNSPECIFIED {
				fmt.Fprintf(&buf, "    %s %s\n", bold("Threat Category:"), cmd.GetThreatCategory())
			}

			// MITRE ATT&CK
			if len(cmd.GetMitreAttackIds()) > 0 {
				fmt.Fprintf(&buf, "    %s %s\n", bold("MITRE ATT&CK:"), strings.Join(cmd.GetMitreAttackIds(), ", "))
			}

			// Detailed description if available
			if detailed := cmd.GetDetailedDescription(); detailed != "" {
				fmt.Fprintf(&buf, "\n    %s\n", bold("Details:"))
				for _, line := range strings.Split(detailed, "\n") {
					fmt.Fprintf(&buf, "      %s\n", line)
				}
			}

			// Error messages
			if len(cmd.GetErrorMessages()) > 0 {
				fmt.Fprintf(&buf, "\n    %s\n", bold("Errors:"))
				for _, errMsg := range cmd.GetErrorMessages() {
					fmt.Fprintf(&buf, "      - %s\n", errMsg)
				}
			}

			// Suspicious patterns and IOCs
			if len(cmd.GetSuspiciousPatterns()) > 0 {
				fmt.Fprintf(&buf, "\n    %s\n", bold("Suspicious Patterns:"))
				for _, pattern := range cmd.GetSuspiciousPatterns() {
					fmt.Fprintf(&buf, "      - %s\n", pattern)
				}
			}

			if len(cmd.GetIocs()) > 0 {
				fmt.Fprintf(&buf, "\n    %s\n", bold("IOCs:"))
				for _, ioc := range cmd.GetIocs() {
					fmt.Fprintf(&buf, "      - %s\n", ioc)
				}
			}

			fmt.Fprintf(&buf, "\n")
		}
	}

	// Use pager if enabled and stdout is a terminal
	if c.summaryUsePager && utils.IsTerminal(c.stdout) {
		return runInteractivePager(buf.String(), c.stdout)
	}

	// Otherwise write directly to stdout
	_, err := c.stdout.Write(buf.Bytes())
	return trace.Wrap(err)
}

// convertMarkdownBoldToANSI converts markdown bold (**text**) to ANSI bold escape codes
func convertMarkdownBoldToANSI(text string) string {
	// ANSI escape codes: \x1b[1m for bold, \x1b[0m to reset
	result := text

	// Replace **text** with ANSI bold
	for {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}

		// Find the closing **
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			// No closing **, leave as is
			break
		}

		// Calculate actual end position
		end = start + 2 + end

		// Extract the bold text
		boldText := result[start+2 : end]

		// Replace with ANSI codes
		replacement := "\x1b[1m" + boldText + "\x1b[0m"
		result = result[:start] + replacement + result[end+2:]
	}

	return result
}

// formatSummaryJSON formats the summary in JSON format
func (c *RecordingsCommand) formatSummaryJSON(summary *summarizerv1.Summary) error {
	rBytes, err := marshalSessionSummary(summary)
	if err != nil {
		return trace.Wrap(err, "failed to marshal summary")
	}
	_, err = c.stdout.Write(rBytes)
	return trace.Wrap(err)
}

// formatSummaryYAML formats the summary in YAML format
func (c *RecordingsCommand) formatSummaryYAML(summary *summarizerv1.Summary) error {
	rBytes, err := marshalSessionSummary(summary)
	if err != nil {
		return trace.Wrap(err, "failed to marshal summary")
	}
	var jsonObj map[string]any
	if err := json.Unmarshal(rBytes, &jsonObj); err != nil {
		return trace.Wrap(err, "failed to unmarshal JSON")
	}
	encoder := yaml.NewEncoder(c.stdout)
	encoder.SetIndent(2)
	defer encoder.Close()
	return trace.Wrap(encoder.Encode(jsonObj))
}
