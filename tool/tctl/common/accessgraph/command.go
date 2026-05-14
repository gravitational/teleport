/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package accessgraph

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// AccessGraphCommand implements experimental Access Graph commands.
//
// The user-facing surface is currently a single hidden diagnostic
// (`tctl accessgraph credentials`) that validates / re-issues the
// Access Graph credential without making any AG calls. It exists
// primarily so the credential plumbing has a reachable entry point
// ahead of the real `access`/`detections`/etc. subcommands.
type AccessGraphCommand struct {
	ccf    *tctlcfg.GlobalCLIFlags
	config *servicecfg.Config
	stdout io.Writer

	// accessgraph is the parent command grouping AG subcommands.
	// TODO(ghassan): remove when the real AG subcommands are implemented
	accessgraph *kingpin.CmdClause
	// check is a hidden diagnostic that exercises the credential
	// and client issuance and response code paths for diagnostic purposes
	// TODO(ghassan): remove when the real AG subcommands are implemented
	check *kingpin.CmdClause
}

// Initialize allows AccessGraphCommand to plug itself into the CLI parser.
func (c *AccessGraphCommand) Initialize(app *kingpin.Application, cliFlags *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.ccf = cliFlags
	c.config = config
	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	c.accessgraph = app.Command("accessgraph", "Manage Access Graph (experimental).").Hidden()
	c.check = c.accessgraph.Command("check", "Validate Access Graph flows (experimental)").Hidden()
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AccessGraphCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	// Access Graph commands bypass the normal tctl auth flow (ApplyConfig), so
	// the logger is never upgraded from its default Warn level. Do it here.
	if c.ccf.Debug {
		utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	}

	var commandFunc func(context.Context, commonclient.InitFunc) error
	switch cmd {
	case c.check.FullCommand():
		commandFunc = c.accessGraphCheck
	default:
		return false, nil
	}
	return true, trace.Wrap(commandFunc(ctx, clientFunc))
}

// accessGraphCheck is a hidden diagnostic command that validates the Access Graph credential
// loading and client issuance code paths, and exercises a simple AG API call to validate the client works.
// This is intended for diagnostic and code exercises purposes
//
// TODO(ghassan): remove when the real AG subcommands are implemented
func (c *AccessGraphCommand) accessGraphCheck(ctx context.Context, clientFunc commonclient.InitFunc) error {
	creds, err := c.loadAccessGraphCredentials(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := ensureAccessGraphCert(ctx, creds, clientFunc); err != nil {
		return trace.Wrap(err)
	}

	if creds.proxyAddr == "" {
		return trace.BadParameter("missing proxy address in Access Graph credential")
	}

	// Exercise the client issuance code path to validate it works with the
	client, err := newAccessGraphClient(ctx, creds.proxyAddr, creds.keyRing)
	if err != nil {
		return trace.Wrap(err)
	}

	// Exercise a client call to validate the client works.
	end := time.Now()
	start := end.Add(-time.Hour * 24)
	res, err := doRequest(client.ListAlertsV1WithResponse(ctx, &accessgraph.ListAlertsV1Params{
		StartTime: &start,
		EndTime:   &end,
	}))

	if err != nil {
		return trace.Wrap(err, "failed to list Access Graph alerts")
	}

	if res.JSON200 == nil || res.JSON200.Data == nil {
		return trace.BadParameter("unexpected response from Access Graph API: missing data")
	}

	return trace.Wrap(utils.WriteYAMLArray(c.stdout, res.JSON200.Data), "failed to write Access Graph alerts to output")
}
