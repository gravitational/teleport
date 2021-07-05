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
	"fmt"

	"github.com/gravitational/kingpin"
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

	// accessList implements the "tctl apps ls" subcommand.
	accessList *kingpin.CmdClause
}

// Initialize allows AccessCommand to plug itself into the CLI parser
func (c *AccessCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	accesses := app.Command("access", "Get access information within the cluster.")
	c.accessList = accesses.Command("ls", "List all accesses within the cluster.")
	c.accessList.Flag("user", "Teleport user").Default("").StringVar(&c.user)
	c.accessList.Flag("login", "Teleport login").Default("").StringVar(&c.login)
	c.accessList.Flag("node", "Teleport node").Default("").StringVar(&c.node)
	c.accessList.Flag("namespace", "Teleport namespace").Default("default").StringVar(&c.namespace)
}

// TryRun attempts to run subcommands like "access ls".
func (c *AccessCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.accessList.FullCommand():
		access := &datalog.AccessRequest{Username: c.user, Login: c.login, Node: c.node, Namespace: c.namespace}
		resp, err := access.QueryAccess(client)
		if err != nil {
			return false, trace.Wrap(err)
		}
		fmt.Println(resp.BuildStringOutput())
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}
