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
	format         string
	searchKeywords string
	predicateExpr  string
	// sessionsList implements the "tctl sessions ls" subcommand.
	sessionsList *kingpin.CmdClause
}

// Initialize allows SessionsCommand to plug itself into the CLI parser
func (c *SessionsCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	sessions := app.Command("sessions", "Operate on recorded sessions.")
	c.sessionsList = sessions.Command("ls", "List recorded sessions.")
	c.sessionsList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)
}

// TODO: Move this somewhere more appropriate
const defaultSearchSessionPageLimit = 50

// TryRun attempts to run subcommands like "sessions ls".
func (c *SessionsCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
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
func (c *SessionsCommand) ListSessions(clt auth.ClientI) error {
	prevEventKey := ""
	sessions := []events.AuditEvent{}
	for {
		nextEvents, eventKey, err := clt.SearchSessionEvents(
			time.Unix(0, 0), time.Now(), defaultSearchSessionPageLimit,
			types.EventOrderDescending, prevEventKey, nil)
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
