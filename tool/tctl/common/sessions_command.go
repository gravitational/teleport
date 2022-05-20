/*
Copyright 2020 Gravitational, Inc.

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
	"fmt"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
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
	c.sessionsList.Flag("search", searchHelp).StringVar(&c.searchKeywords)
	c.sessionsList.Flag("query", queryHelp).StringVar(&c.predicateExpr)
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
	return trace.Wrap(showSessions(sessions, c.format))
}

func showSessions(events []events.AuditEvent, format string) error {
	format = strings.ToLower(format)
	switch format {
	case teleport.Text, "":
		err := showSessionsAsText(events)
		if err != nil {
			return trace.Wrap(err)
		}
	case teleport.JSON, teleport.YAML:
		out, err := serializeSessions(events, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", format)
	}
	return nil
}

func showSessionsAsText(endEvents []events.AuditEvent) error {
	// session ID, session type (can we?), session timestamp, participants, server information.
	t := asciitable.MakeTable([]string{"ID", "Type", "Participants", "Hostname", "Timestamp"})
	for _, event := range endEvents {
		session, ok := event.(*events.SessionEnd)
		if !ok {
			return trace.BadParameter("unsupported event type: expected SessionEnd: got: %T", event)
		}
		t.AddRow([]string{
			session.GetSessionID(),
			"TODO: Get session type",
			strings.Join(session.Participants, ", "),
			session.ServerHostname,
			session.GetTime().Format(constants.HumanDateFormatSeconds),
		})
	}
	fmt.Println(t.AsBuffer().String())
	return nil
}

func serializeSessions(sessionEvents []events.AuditEvent, format string) (string, error) {
	if sessionEvents == nil {
		sessionEvents = []events.AuditEvent{}
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(sessionEvents, "", "  ")
	} else {
		out, err = yaml.Marshal(sessionEvents)
	}
	return string(out), trace.Wrap(err)
}
