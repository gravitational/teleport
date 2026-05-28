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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// AccessGraphCommand implements experimental Access Graph commands.
type AccessGraphCommand struct {
	ccf    *tctlcfg.GlobalCLIFlags
	config *servicecfg.Config
	stdout io.Writer

	detections detectionsArgs
}

// Initialize allows AccessGraphCommand to plug itself into the CLI parser.
func (c *AccessGraphCommand) Initialize(app *kingpin.Application, cliFlags *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.ccf = cliFlags
	c.config = config
	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	// Initialize AG subcommands.
	c.initDetections(app)
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AccessGraphCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	// Access Graph commands bypass the normal tctl auth flow (ApplyConfig), so
	// the logger is never upgraded from its default Warn level. Do it here.
	if c.ccf.Debug {
		utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	}

	var commandFunc func(context.Context, *accessgraph.ClientWithResponses) error

	switch cmd {
	case c.detections.ls.cmd.FullCommand():
		commandFunc = c.DetectionsList
	case c.detections.get.cmd.FullCommand():
		commandFunc = c.DetectionsGet
	default:
		return false, nil
	}

	// Load credentials and ensure the AG cert is valid.
	creds, err := c.loadAccessGraphCredentials(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the cert is not present or valid, ensureAccessGraphCert will fetch a new one,
	// and load it into the keyring.
	if err := ensureAccessGraphCert(ctx, creds, clientFunc); err != nil {
		return true, trace.Wrap(err)
	}

	if creds.proxyAddr == "" {
		return true, trace.BadParameter("missing proxy address in Access Graph credential")
	}

	// Create an AG client using the resolved proxy address and credentials.
	accessGraphClient, err := newAccessGraphClient(ctx, creds.proxyAddr, creds.keyRing)
	if err != nil {
		return true, trace.Wrap(err)
	}

	// Run the command.
	return true, trace.Wrap(commandFunc(ctx, accessGraphClient))
}
