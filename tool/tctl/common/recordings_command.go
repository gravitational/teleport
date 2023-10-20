/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/common"
)

// RecordingsCommand implements "tctl recordings" group of commands.
type RecordingsCommand struct {
	config *servicecfg.Config

	// format is the output format (text, json, or yaml)
	format string
	// recordingsList implements the "tctl recordings ls" subcommand.
	recordingsList *kingpin.CmdClause
	// fromUTC is the start time to use for the range of recordings listed by the recorded session listing command
	fromUTC string
	// toUTC is the start time to use for the range of recordings listed by the recorded session listing command
	toUTC string
	// maxRecordingsToShow is the maximum number of recordings to show per page of results
	maxRecordingsToShow int
	// recordingsSince is a duration which sets the time into the past in which to list session recordings
	recordingsSince string
}

// Initialize allows RecordingsCommand to plug itself into the CLI parser
func (c *RecordingsCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config
	recordings := app.Command("recordings", "View and control session recordings.")
	c.recordingsList = recordings.Command("ls", "List recorded sessions.")
	c.recordingsList.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)+". Defaults to 'text'.").Default(teleport.Text).StringVar(&c.format)
	c.recordingsList.Flag("from-utc", fmt.Sprintf("Start of time range in which recordings are listed. Format %s. Defaults to 24 hours ago.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.fromUTC)
	c.recordingsList.Flag("to-utc", fmt.Sprintf("End of time range in which recordings are listed. Format %s. Defaults to current time.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.toUTC)
	c.recordingsList.Flag("limit", fmt.Sprintf("Maximum number of recordings to show. Default %s.", defaults.TshTctlSessionListLimit)).Default(defaults.TshTctlSessionListLimit).IntVar(&c.maxRecordingsToShow)
	c.recordingsList.Flag("last", "Duration into the past from which session recordings should be listed. Format 5h30m40s").StringVar(&c.recordingsSince)
}

// TryRun attempts to run subcommands like "recordings ls".
func (c *RecordingsCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.recordingsList.FullCommand():
		err = c.ListRecordings(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *RecordingsCommand) ListRecordings(ctx context.Context, tc auth.ClientI) error {
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
	return trace.Wrap(common.ShowSessions(recordings, c.format, os.Stdout))
}
