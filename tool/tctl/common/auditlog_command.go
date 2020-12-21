/*
Copyright 2019 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

type AuditLogCommand struct {
	config *service.Config
	since  time.Duration
	limit  int

	requestSearch *kingpin.CmdClause
}

// Initialize allows AuditLogCommand to plug itself into the CLI parser
func (c *AuditLogCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	auditLog := app.Command("auditlog", "Manage audit log")

	c.requestSearch = auditLog.Command("search", "Show active access requests")
	c.requestSearch.Flag("since", "Get all events newer than some time").Default("1h").DurationVar(&c.since)
	c.requestSearch.Flag("limit", "Maximum number of events to return").Default("500").IntVar(&c.limit)
}

// TryRun takes the CLI command as an argument (like "access-request list") and executes it.
func (c *AuditLogCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.requestSearch.FullCommand():
		err = c.Search(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *AuditLogCommand) Search(client auth.ClientI) error {
	from := time.Now().UTC()
	to := from.Add(-c.since)
	events, err := client.SearchEvents(from, to, "", c.limit)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, event := range events {
		e, err := json.Marshal(event)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("%s\n", e)
	}
	return nil
}
