/*
Copyright 2015 Gravitational, Inc.

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
	"os"
	"strings"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// NodeCommand implements `tctl nodes` group of commands
type NodeCommand struct {
	config *service.Config
	// count is optional hidden field that will cause
	// tctl issue count tokens and output them in JSON format
	count int
	// format is the output format, e.g. text or json
	format string
	// list of roles for the new node to assume
	roles string
	// TTL: duration of time during which a generated node token will
	// be valid.
	ttl time.Duration
	// namespace is node namespace
	namespace string

	// CLI subcommands (clauses)
	nodeAdd  *kingpin.CmdClause
	nodeList *kingpin.CmdClause
}

// Initialize allows NodeCommand to plug itself into the CLI parser
func (c *NodeCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	// add node command
	nodes := app.Command("nodes", "Issue invites for other nodes to join the cluster")
	c.nodeAdd = nodes.Command("add", "Generate a node invitation token")
	c.nodeAdd.Flag("roles", "Comma-separated list of roles for the new node to assume [node]").Default("node").StringVar(&c.roles)
	c.nodeAdd.Flag("ttl", "Time to live for a generated token").Default(defaults.ProvisioningTokenTTL.String()).DurationVar(&c.ttl)
	c.nodeAdd.Flag("count", "add count tokens and output JSON with the list").Hidden().Default("1").IntVar(&c.count)
	c.nodeAdd.Flag("format", "output format, 'text' or 'json'").Hidden().Default("text").StringVar(&c.format)
	c.nodeAdd.Alias(AddNodeHelp)

	c.nodeList = nodes.Command("ls", "List all active SSH nodes within the cluster")
	c.nodeList.Flag("namespace", "Namespace of the nodes").Default(defaults.Namespace).StringVar(&c.namespace)
	c.nodeList.Alias(ListNodesHelp)
}

// TryRun takes the CLI command as an argument (like "nodes ls") and executes it.
func (c *NodeCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.nodeAdd.FullCommand():
		err = c.Invite(client)
	case c.nodeList.FullCommand():
		err = c.ListActive(client)

	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// Invite generates a token which can be used to add another SSH node
// to a cluster
func (c *NodeCommand) Invite(client auth.ClientI) error {
	if c.count < 1 {
		return trace.BadParameter("count should be > 0, got %v", c.count)
	}
	// parse --roles flag
	roles, err := teleport.ParseRoles(c.roles)
	if err != nil {
		return trace.Wrap(err)
	}
	var tokens []string
	for i := 0; i < c.count; i++ {
		token, err := client.GenerateToken(roles, c.ttl)
		if err != nil {
			return trace.Wrap(err)
		}
		tokens = append(tokens, token)
	}

	authServers, err := client.GetAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(authServers) == 0 {
		return trace.Errorf("This cluster does not have any auth servers running")
	}

	// output format swtich:
	if c.format == "text" {
		for _, token := range tokens {
			fmt.Printf(
				"The invite token: %v\nRun this on the new node to join the cluster:\n> teleport start --roles=%s --token=%v --auth-server=%v\n\nPlease note:\n",
				token, strings.ToLower(roles.String()), token, authServers[0].GetAddr())
		}
		fmt.Printf("  - This invitation token will expire in %d minutes\n", int(c.ttl.Minutes()))
		fmt.Printf("  - %v must be reachable from the new node, see --advertise-ip server flag\n", authServers[0].GetAddr())
	} else {
		out, err := json.Marshal(tokens)
		if err != nil {
			return trace.Wrap(err, "failed to marshal tokens")
		}
		fmt.Printf(string(out))
	}
	return nil
}

// ListActive retreives the list of nodes who recently sent heartbeats to
// to a cluster and prints it to stdout
func (c *NodeCommand) ListActive(client auth.ClientI) error {
	nodes, err := client.GetNodes(c.namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &serverCollection{servers: nodes}
	coll.writeText(os.Stdout)
	return nil
}
