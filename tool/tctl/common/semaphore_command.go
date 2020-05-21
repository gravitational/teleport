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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// SemaphoreCommand implements basic semaphore operations.
type SemaphoreCommand struct {
	config *service.Config

	name    string
	kind    string
	leaseID string

	format string

	semList   *kingpin.CmdClause
	semDelete *kingpin.CmdClause
	semCancel *kingpin.CmdClause
}

// setFilterFlags sets the filter flags for the supplied commands
func (c *SemaphoreCommand) setFilterFlags(cmds ...*kingpin.CmdClause) {
	for _, cmd := range cmds {
		cmd.Flag("kind", fmt.Sprintf("Semaphore kind (e.g. %s)", services.SemaphoreKindSessionControl)).StringVar(&c.kind)
		cmd.Flag("name", "Semaphore name (e.g. alice@example.com)").StringVar(&c.name)
	}
}

// Initialize allows SemaphoreCommand to plug itself into the CLI parser
func (c *SemaphoreCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	sem := app.Command("sem", "Manage cluster-level semaphores").Alias("semaphores").Alias("semaphore")

	c.semList = sem.Command("ls", "List all matching semaphores").Alias("list")
	c.semList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)

	c.semDelete = sem.Command("rm", "Delete all matching semaphores").Hidden().Alias("delete")

	c.semCancel = sem.Command("cancel", "Cancel a specific semaphore lease").Hidden()
	c.semCancel.Arg("lease-id", "Unique ID of the target lease").Required().StringVar(&c.leaseID)

	c.setFilterFlags(c.semList, c.semDelete, c.semCancel)
}

// TryRun takes the CLI command as an argument (like "access-request list") and executes it.
func (c *SemaphoreCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.semList.FullCommand():
		err = c.List(client)
	case c.semDelete.FullCommand():
		err = c.Delete(client)
	case c.semCancel.FullCommand():
		err = c.Cancel(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// filter constructs a SemaphoreFilter matching the filter parameters
// supplied to the semaphore command.
func (c *SemaphoreCommand) filter() services.SemaphoreFilter {
	return services.SemaphoreFilter{
		SemaphoreName: c.name,
		SemaphoreKind: c.kind,
	}
}

// List lists all matching semaphores.
func (c *SemaphoreCommand) List(client auth.ClientI) error {
	sems, err := client.GetSemaphores(context.TODO(), c.filter())
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.PrintSemaphores(sems, c.format))
}

// Delete deletes all matching semaphores.
func (c *SemaphoreCommand) Delete(client auth.ClientI) error {
	return trace.Wrap(client.DeleteSemaphores(context.TODO(), c.filter()))
}

// Cancel a specific semaphore lease.
func (c *SemaphoreCommand) Cancel(client auth.ClientI) error {
	return trace.Wrap(client.CancelSemaphoreLease(context.TODO(), services.SemaphoreLease{
		SemaphoreKind: c.kind,
		SemaphoreName: c.name,
		LeaseID:       c.leaseID,
		Expires:       time.Now().UTC().Add(time.Minute),
	}))
}

func (c *SemaphoreCommand) PrintSemaphores(sems []services.Semaphore, format string) error {
	if format == teleport.Text {
		table := asciitable.MakeTable([]string{"Kind", "Name", "LeaseID", "Holder", "Expires"})
		for _, sem := range sems {
			for _, ref := range sem.LeaseRefs() {
				table.AddRow([]string{
					sem.GetSubKind(),
					sem.GetName(),
					ref.LeaseID,
					ref.Holder,
					ref.Expires.Format(time.RFC822),
				})
			}
		}
		_, err := table.AsBuffer().WriteTo(os.Stdout)
		return trace.Wrap(err)
	} else {
		out, err := json.MarshalIndent(sems, "", "  ")
		if err != nil {
			return trace.Wrap(err, "failed to marshal semaphores")
		}
		fmt.Printf("%s\n", out)
	}
	return nil
}
