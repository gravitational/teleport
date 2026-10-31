// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package common

import (
	"context"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

type LLMCommand struct {
	usageCmd *kingpin.CmdClause
	endpoint string

	format string

	stdout io.Writer
}

// Initialize allows LLMCommand to plug itself into the CLI parser.
func (c *LLMCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	llm := app.Command("llm", "Commands related to large language model (LLM) access")

	c.usageCmd = llm.Command("usage", "List current LLM usage")
	c.usageCmd.Flag("endpoint", "Retrieve usage only from the specified LLM endpoint").StringVar(&c.endpoint)
	c.usageCmd.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, defaults.DefaultFormats...)

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument and executes it.
func (c *LLMCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.usageCmd.FullCommand():
		commandFunc = c.usage
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

func (c *LLMCommand) usage(ctx context.Context, client *authclient.Client) error {
	coll, err := resources.GetLLMEndpointQuotas(ctx, client, services.Ref{Name: c.endpoint}, resources.GetOpts{})
	if err != nil {
		return trace.Wrap(err)
	}

	switch c.format {
	case teleport.Text:
		return coll.WriteText(c.stdout, false /* not used */)
	case teleport.YAML:
		return utils.WriteYAML(c.stdout, coll.Resources())
	case teleport.JSON:
		return utils.WriteJSONArray(c.stdout, coll.Resources())
	default:
		return trace.BadParameter("unsupported format")
	}
}
