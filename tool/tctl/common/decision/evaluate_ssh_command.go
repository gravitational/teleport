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
	"io"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
)

// EvaluateSSHCommand is a command to evaluate
// SSH access via the Teleport Decision Service.
type EvaluateSSHCommand struct {
	Output   io.Writer
	Username string
	ServerID string
	Login    string
	command  *kingpin.CmdClause
}

// Initialize sets up the "tctl decision evaluate ssh" command.
func (c *EvaluateSSHCommand) Initialize(cmd *kingpin.CmdClause, output io.Writer) {
	c.Output = output
	c.command = cmd.Command("evaluate-ssh-access", "Evaluate SSH access for a user.").Hidden()
	c.command.Flag("username", "The username to evaluate access for.").StringVar(&c.Username)
	c.command.Flag("login", "The os login to evaluate access for.").StringVar(&c.Login)
	c.command.Flag("server-id", "The host id of the target server.").StringVar(&c.ServerID)
}

// FullCommand returns the fully qualified name of
// the subcommand, i.e. tctl decision evaluate ssh.
func (c *EvaluateSSHCommand) FullCommand() string {
	return c.command.FullCommand()
}

// Run executes the subcommand.
func (c *EvaluateSSHCommand) Run(ctx context.Context, clt Client) error {
	if c.Username == "" {
		return trace.BadParameter("please specify an extant teleport user with --username")
	}

	clusterName, err := clt.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := clt.DecisionClient().EvaluateSSHAccess(ctx, &decisionpb.EvaluateSSHAccessRequest{
		Metadata: &decisionpb.RequestMetadata{
			PepVersionHint: teleport.Version,
			DryRun:         true,
			DryRunOptions: &decisionpb.DryRunOptions{
				GenerateIdentity: &decisionpb.DryRunIdentity{
					Username: c.Username,
				},
			},
		},
		SshAuthority: &decisionpb.SSHAuthority{
			ClusterName:   clusterName.GetClusterName(),
			AuthorityType: string(types.UserCA),
		},
		Node: &decisionpb.Resource{
			Kind: types.KindNode,
			Name: c.ServerID,
		},
		OsUser: c.Login,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := WriteProtoJSON(c.Output, resp); err != nil {
		return trace.Wrap(err, "failed to marshal result")
	}

	return nil
}
