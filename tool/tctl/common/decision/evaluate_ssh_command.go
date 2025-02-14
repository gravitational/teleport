// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
package decision

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// EvaluateSSHCommand is a command to evaluate
// SSH access via the Teleport Decision Service.
type EvaluateSSHCommand struct {
	output io.Writer

	sshDetails sshDetails
	command    *kingpin.CmdClause
}

type sshDetails struct {
	username string
	serverID string
	login    string
	identity string
}

// Initialize sets up the "tctl decision evaluate ssh" command.
func (c *EvaluateSSHCommand) Initialize(cmd *kingpin.CmdClause, output io.Writer) {
	c.output = output
	c.command = cmd.Command("evaluate-ssh-access", "Evaluate SSH access for a user.").Hidden()
	c.command.Flag("username", "The username to evaluate access for.").StringVar(&c.sshDetails.username)
	c.command.Flag("login", "The os login to evaluate access for.").StringVar(&c.sshDetails.login)
	c.command.Flag("server-id", "The host id of the target server.").StringVar(&c.sshDetails.serverID)
	c.command.Flag("ssh-identity", "The identity about which access is being evaluated.").StringVar(&c.sshDetails.identity)
}

// FullCommand returns the fully qualified name of
// the subcommand, i.e. tctl decision evaluate ssh.
func (c *EvaluateSSHCommand) FullCommand() string {
	return c.command.FullCommand()
}

// Run executes the subcommand.
func (c *EvaluateSSHCommand) Run(ctx context.Context, clt *authclient.Client) error {

	/*var identity *decisionpb.SSHIdentity
	switch {
	case c.sshDetails.identity != "" && c.sshDetails.username == "":
		identity = &decisionpb.SSHIdentity{}
		if err := readJSONFile(c.sshDetails.identity, &identity); err != nil {
			return trace.Wrap(err)
	}*/

	if c.sshDetails.username == "" {
		return trace.BadParameter("please specify an extant teleport user with --username")
	}

	clusterName, err := clt.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := clt.DecisionClient().EvaluateSSHAccess(ctx, &decisionpb.EvaluateSSHAccessRequest{
		Metadata: &decisionpb.RequestMetadata{
			PepVersionHint: teleport.Version,
			DryRun:         true,
			DryRunOptions: &decisionpb.DryRunOptions{
				GenerateIdentity: &decisionpb.DryRunIdentity{
					Username: c.sshDetails.username,
				},
			},
		},
		SshAuthority: &decisionpb.SSHAuthority{
			ClusterName:   clusterName.GetClusterName(),
			AuthorityType: string(types.UserCA),
		},
		Node: &decisionpb.Resource{
			Kind: types.KindNode,
			Name: c.sshDetails.serverID,
		},
		OsUser: c.sshDetails.login,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := WriteProtoJSON(c.output, resp); err != nil {
		return trace.Wrap(err, "failed to marshal result")
	}

	return nil
}

func readJSONFile(filename string, v interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return trace.Wrap(err)
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
