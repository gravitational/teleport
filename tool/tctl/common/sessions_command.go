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
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
)

// SessionsCommand implements "tctl sessions" group of commands.
type SessionsCommand struct {
	config *service.Config

	// format is the output format (text, json, or yaml)
	format string
	// sessionsList implements the "tctl sessions ls" subcommand.
	sessionsList *kingpin.CmdClause
	// fromUTC is the start time to use for the range of sessions listed by the recorded session listing command
	fromUTC string
	// toUTC is the start time to use for the range of sessions listed by the recorded session listing command
	toUTC string
	// maxSessionsToShow is the maximum number of sessions to show per page of results
	maxSessionsToShow int
}

// Initialize allows SessionsCommand to plug itself into the CLI parser
func (c *SessionsCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	sessions := app.Command("sessions", "View and control recorded sessions.")
	c.sessionsList = sessions.Command("ls", "List recorded sessions.")
	c.sessionsList.Flag("format", client.FormatFlagDescription(client.DefaultFormats...)+". Defaults to 'text'.").Default(teleport.Text).StringVar(&c.format)
	c.sessionsList.Flag("from-utc", fmt.Sprintf("Start of time range in which sessions are listed. Format %s. Defaults to 24 hours ago.", time.RFC3339)).StringVar(&c.fromUTC)
	c.sessionsList.Flag("to-utc", fmt.Sprintf("End of time range in which sessions are listed. Format %s. Defaults to current time.", time.RFC3339)).StringVar(&c.toUTC)
	c.sessionsList.Flag("limit", fmt.Sprintf("Maximum number of sessions to show. Default %s.", defaults.TshTctlSessionListLimit)).Default(defaults.TshTctlSessionListLimit).IntVar(&c.maxSessionsToShow)
}

// TryRun attempts to run subcommands like "sessions ls".
func (c *SessionsCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.sessionsList.FullCommand():
		err = c.ListSessions(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *SessionsCommand) ListSessions(tc auth.ClientI) error {
	fromUTC, toUTC, err := client.DefaultSearchSessionRange(c.fromUTC, c.toUTC)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionGetter := func(startKey string) ([]apievents.AuditEvent, string, error) {
		return tc.SearchSessionEvents(fromUTC, toUTC,
			defaults.TshTctlSessionSearchPageSize, types.EventOrderAscending, startKey,
			nil /* where condition */, "" /* session ID */)
	}
	sessions, err := client.GetPaginatedSessions(c.maxSessionsToShow, sessionGetter)
	if err != nil {
		return trace.Errorf("getting session events: %v", err)
	}
	return trace.Wrap(client.ShowSessions(sessions, c.format, os.Stdout))
}
