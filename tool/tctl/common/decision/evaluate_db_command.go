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

// EvaluateDatabaseCommand is a command to evaluate
// database access via the Teleport Decision Service.
type EvaluateDatabaseCommand struct {
	Output     io.Writer
	DatabaseID string
	command    *kingpin.CmdClause
}

// Initialize sets up the "tctl decision evaluate db" command.
func (c *EvaluateDatabaseCommand) Initialize(cmd *kingpin.CmdClause, output io.Writer) {
	c.Output = output
	c.command = cmd.Command("evaluate-db-access", "Evaluate database access for a user.").Hidden()
	c.command.Flag("database-id", "The id of the target database.").StringVar(&c.DatabaseID)
}

// FullCommand returns the fully qualified name of
// the subcommand, i.e. tctl decision evaluate db.
func (c *EvaluateDatabaseCommand) FullCommand() string {
	return c.command.FullCommand()
}

// Run executes the subcommand.
func (c *EvaluateDatabaseCommand) Run(ctx context.Context, clt Client) error {
	resp, err := clt.DecisionClient().EvaluateDatabaseAccess(ctx, &decisionpb.EvaluateDatabaseAccessRequest{
		Metadata:    &decisionpb.RequestMetadata{PepVersionHint: teleport.Version},
		TlsIdentity: &decisionpb.TLSIdentity{},
		Database: &decisionpb.Resource{
			Kind: types.KindDatabase,
			Name: c.DatabaseID,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := WriteProtoJSON(c.Output, resp); err != nil {
		return trace.Wrap(err, "failed to marshal result")
	}

	return nil
}
