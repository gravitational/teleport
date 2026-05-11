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
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

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

	// accessGraphUser is the username to issue the credential as on the
	// auth-host path; ignored when a profile is loaded.
	accessGraphUser string

	// accessgraph is the parent command grouping AG subcommands.
	// TODO(ghassan): remove when the real AG subcommands are implemented
	accessgraph *kingpin.CmdClause
	// credentials is a hidden diagnostic that exercises the credential
	// resolve/re-issue path.
	// TODO(ghassan): remove when the real AG subcommands are implemented
	credentials *kingpin.CmdClause
}

// Initialize allows AccessGraphCommand to plug itself into the CLI parser.
func (c *AccessGraphCommand) Initialize(app *kingpin.Application, cliFlags *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.ccf = cliFlags
	c.config = config
	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	c.accessgraph = app.Command("accessgraph", "Manage Access Graph (experimental).").Hidden()
	c.credentials = c.accessgraph.Command("credentials", "Validate and re-issue the Access Graph credential.").Hidden()
	c.credentials.Flag("cert-user", "User to issue the credential as. Only consulted on the auth host.").StringVar(&c.accessGraphUser)
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
	case c.credentials.FullCommand():
		commandFunc = c.runCredentialsCheck
	default:
		return false, nil
	}
	return true, trace.Wrap(commandFunc(ctx, clientFunc))
}

// runCredentialsCheck resolves the Access Graph credential and re-issues
// it if missing or stale, then reports the outcome. It does not contact
// the Access Graph service itself — only the auth path is exercised.
//
// TODO(ghassan): remove when the real AG subcommands are implemented
func (c *AccessGraphCommand) runCredentialsCheck(ctx context.Context, clientFunc commonclient.InitFunc) error {
	creds, err := c.loadAccessGraphCredentials(ctx, c.accessGraphUser)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := ensureAccessGraphCert(ctx, creds, clientFunc); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.stdout, "Access Graph credential is valid (proxy: %s).\n", creds.proxyAddr)
	return nil
}
