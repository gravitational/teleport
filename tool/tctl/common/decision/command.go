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
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// Client is the interface used by the decision family of commands to represent the remote auth server.
type Client interface {
	services.ClusterNameGetter
	DecisionClient() decisionpb.DecisionServiceClient
}

// Command is a group of commands to interact with the Teleport Decision Service.
type Command struct {
	// Output is the writer that any command output should be written to.
	Output io.Writer

	evaluateSSHCommand      EvaluateSSHCommand
	evaluateDatabaseCommand EvaluateDatabaseCommand
}

// Initialize sets up the "tctl decision" command.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	if c.Output == nil {
		c.Output = os.Stdout
	}

	cmd := app.Command("decision", "Interact with the Teleport Decision Service.").Hidden()
	c.evaluateSSHCommand.Initialize(cmd, c.Output)
	c.evaluateDatabaseCommand.Initialize(cmd, c.Output)
}

// TryRun attempts to run subcommands.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	var run func(context.Context, Client) error
	switch cmd {
	case c.evaluateSSHCommand.FullCommand():
		run = c.evaluateSSHCommand.Run
	case c.evaluateDatabaseCommand.FullCommand():
		run = c.evaluateDatabaseCommand.Run
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}

	defer closeFn(ctx)

	fmt.Fprintf(os.Stderr, "WARNING: decision service and its associated commands are experimental. APIs and behavior may change without warning.\n")

	return true, trace.Wrap(run(ctx, client))
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

	out = append(out, '\n')
	_, err = w.Write(out)
	return trace.Wrap(err)
}

// WriteProtoYAML outputs the given [proto.Message] in YAML format to the given
// [io.Writer], preserving the same proto field names as WriteProtoJSON.
func WriteProtoYAML(w io.Writer, v proto.Message) error {
	out, err := protojson.MarshalOptions{
		UseProtoNames: true,
		Indent:        "    ",
	}.Marshal(v)
	if err != nil {
		return trace.Wrap(err)
	}
	out, err = yaml.JSONToYAML(out)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(out)
	return trace.Wrap(err)
}

// WriteProto outputs the given [proto.Message] in the requested structured
// format to the given [io.Writer].
func WriteProto(w io.Writer, format string, v proto.Message) error {
	switch format {
	case "", teleport.JSON:
		return trace.Wrap(WriteProtoJSON(w, v))
	case teleport.YAML:
		return trace.Wrap(WriteProtoYAML(w, v))
	default:
		return trace.BadParameter("unknown format %q", format)
	}
}
