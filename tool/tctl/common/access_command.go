/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/datalog"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// AccessCommand implements "tctl access" group of commands.
type AccessCommand struct {
	config *service.Config

	user      string
	login     string
	node      string
	namespace string

	// accessList implements the "tctl access ls" subcommand.
	accessList *kingpin.CmdClause
}

const (
	denyNullString   = "No denied access found.\n"
	accessNullString = "No access found.\n"
)

// Initialize allows AccessCommand to plug itself into the CLI parser
func (c *AccessCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	accesses := app.Command("access", "Get access information within the cluster.")
	c.accessList = accesses.Command("ls", "List all accesses within the cluster.")
	c.accessList.Flag("user", "Teleport user").StringVar(&c.user)
	c.accessList.Flag("login", "Teleport login").StringVar(&c.login)
	c.accessList.Flag("node", "Teleport node").StringVar(&c.node)
	c.accessList.Flag("namespace", "Teleport namespace").Default(defaults.Namespace).Hidden().StringVar(&c.namespace)
}

// TryRun attempts to run subcommands like "access ls".
func (c *AccessCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.accessList.FullCommand():
		access := datalog.NodeAccessRequest{Username: c.user, Login: c.login, Node: c.node, Namespace: c.namespace}
		resp, err := datalog.QueryNodeAccess(ctx, client, access)
		if err != nil {
			return false, trace.Wrap(err)
		}
		accessTable, denyTable, accessLen, denyLen := resp.ToTable()
		var denyOutputString string
		if denyLen == 0 {
			denyOutputString = denyNullString
		} else {
			denyOutputString = denyTable.AsBuffer().String()
		}

		var accessOutputString string
		if accessLen == 0 {
			accessOutputString = accessNullString
		} else {
			accessOutputString = accessTable.AsBuffer().String()
		}
		fmt.Println(accessOutputString + "\n" + denyOutputString)
	default:
		return false, nil
	}
	return true, nil
}
