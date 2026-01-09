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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/session"
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
	pwd := "."
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
