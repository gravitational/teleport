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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// Command is a group of commands to interact with the Teleport Decision Service.
type Command struct {
	// Output is the writer that any command output should be written to.
	Output io.Writer

	evaluateSSHCommand      EvaluateSSHCommand
	evaluateDatabaseCommand EvaluateDatabaseCommand
}

// Initialize sets up the "tctl decision" command.
func (c *Command) Initialize(app *kingpin.Application, _ *servicecfg.Config) {
	if c.Output == nil {
		c.Output = os.Stdout
	}

	cmd := app.Command("decision", "Interact with the Teleport Decision Service.").Hidden()
	c.evaluateSSHCommand.Initialize(cmd, c.Output)
	c.evaluateDatabaseCommand.Initialize(cmd, c.Output)
}

// TryRun attempts to run subcommands.
func (c *Command) TryRun(ctx context.Context, cmd string, client *authclient.Client) (bool, error) {
	var run func(context.Context, decisionpb.DecisionServiceClient) error
	switch cmd {
	case c.evaluateSSHCommand.FullCommand():
		run = c.evaluateSSHCommand.Run
	case c.evaluateDatabaseCommand.FullCommand():
		run = c.evaluateDatabaseCommand.Run
	default:
		return false, nil
	}

	return true, trace.Wrap(run(ctx, client.DecisionClient()))
}

// WriteProtoJSON outputs the the given [proto.Message] in
// JSON format to the given [io.Writer].
func WriteProtoJSON(w io.Writer, v proto.Message) error {
	out, err := protojson.MarshalOptions{
		UseProtoNames: true,
		Indent:        "    ",
	}.Marshal(v)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = w.Write(out)
	return trace.Wrap(err)
}
