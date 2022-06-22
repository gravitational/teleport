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
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
)

// SessionsCommand implements "tctl sessions" group of commands.
type SessionsCommand struct {
	config *service.Config

	// format is the output format (text, json, or yaml)
	format string
	// sessionsList implements the "tctl sessions ls" subcommand.
	sessionsList *kingpin.CmdClause
	// FromUTC is the start time to use for the range of sessions listed by the recorded session listing command
	FromUTC string

	// ToUTC is the start time to use for the range of sessions listed by the recorded session listing command
	ToUTC string
}

// Initialize allows SessionsCommand to plug itself into the CLI parser
func (c *SessionsCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	sessions := app.Command("sessions", "Operate on recorded sessions.")
	c.sessionsList = sessions.Command("ls", "List recorded sessions.")
	c.sessionsList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.sessionsList.Flag("from-utc", fmt.Sprintf("Start of time range in which sessions are listed. Format %s", time.RFC3339)).StringVar(&c.FromUTC)
	c.sessionsList.Flag("to-utc", fmt.Sprintf("End of time range in which sessions are listed. Format %s", time.RFC3339)).StringVar(&c.ToUTC)
}

// TODO: Move this somewhere more appropriate
const defaultSearchSessionPageLimit = 50

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

//TODO: Deduplicate this logic, same functions defined for tsh
// ListApps prints the list of recorded sessions.
func (c *SessionsCommand) ListSessions(tc auth.ClientI) error {
	fromUTC := time.Unix(0, 0)
	toUTC := time.Now()
	var err error
	if c.FromUTC != "" {
		fromUTC, err = time.Parse(time.RFC3339, c.FromUTC)
		if err != nil {
			return trace.Errorf("parsing session listing start time: %v", err)
		}
	}
	if c.ToUTC != "" {
		toUTC, err = time.Parse(time.RFC3339, c.ToUTC)
		if err != nil {
			return trace.Errorf("parsing session listing end time: %v", err)
		}
	}

	var sessions []events.AuditEvent
	// Get a list of all Sessions joining pages
	prevEventKey := ""
	sessions = []events.AuditEvent{}
	for {
		nextEvents, eventKey, err := tc.SearchSessionEvents(fromUTC, toUTC,
			defaultSearchSessionPageLimit, types.EventOrderDescending, prevEventKey, nil, "")
		if err != nil {
			return trace.Wrap(err)
		}
		sessions = append(sessions, nextEvents...)
		if eventKey == "" {
			break
		}
		prevEventKey = eventKey
	}
	return trace.Wrap(c.showSessions(sessions))
}

func (c *SessionsCommand) showSessions(events []events.AuditEvent) error {
	sessions := &SessionsCollection{SessionEvents: events}
	switch c.format {
	case teleport.Text:
		return trace.Wrap(sessions.WriteText(os.Stdout))
	case teleport.YAML:
		return trace.Wrap(sessions.WriteYAML(os.Stdout))
	case teleport.JSON:
		return trace.Wrap(sessions.WriteJSON(os.Stdout))
	default:
		return trace.BadParameter("unknown format %q", c.format)

	}
}
