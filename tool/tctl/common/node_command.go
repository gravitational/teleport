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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
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
}

// Invite generates a token which can be used to add another SSH node
// to a cluster
func (u *NodeCommand) Invite(client *auth.TunClient) error {
	if u.count < 1 {
		return trace.BadParameter("count should be > 0, got %v", u.count)
	}
	// parse --roles flag
	roles, err := teleport.ParseRoles(u.roles)
	if err != nil {
		return trace.Wrap(err)
	}
	var tokens []string
	for i := 0; i < u.count; i++ {
		token, err := client.GenerateToken(roles, u.ttl)
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
	if u.format == "text" {
		for _, token := range tokens {
			fmt.Printf(
				"The invite token: %v\nRun this on the new node to join the cluster:\n> teleport start --roles=%s --token=%v --auth-server=%v\n\nPlease note:\n",
				token, strings.ToLower(roles.String()), token, authServers[0].GetAddr())
		}
		fmt.Printf("  - This invitation token will expire in %d minutes\n", int(u.ttl.Minutes()))
		fmt.Printf("  - %v must be reachable from the new node, see --advertise-ip server flag\n", authServers[0].GetAddr())
		fmt.Printf(`  - For tokens of type "trustedcluster", tctl needs to be used to create a TrustedCluster resource. See the Admin Guide for more details.`)
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
func (u *NodeCommand) ListActive(client *auth.TunClient) error {
	nodes, err := client.GetNodes(u.namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &serverCollection{servers: nodes}
	coll.writeText(os.Stdout)
	return nil
}
