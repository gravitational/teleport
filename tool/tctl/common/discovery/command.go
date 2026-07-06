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

package discovery

import (
	"context"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// discoveryClient abstracts the auth client methods used by discovery commands.
// It is a strict subset of authclient.Client.
type discoveryClient interface {
	// SearchEvents searches audit events.
	SearchEvents(ctx context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error)
	// GetResources lists resources with pagination.
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
	// UserTasksClient returns a client for managing user tasks.
	UserTasksClient() services.UserTasks
	// DiscoveryConfigClient returns a client for accessing discovery config.
	DiscoveryConfigClient() services.DiscoveryConfigWithStatusUpdater
}

// Command implements the "tctl discovery" CLI command group.
type Command struct {
	stdout io.Writer

	nodes nodesArgs
}

// Initialize registers the "discovery" command and its subcommands with the CLI parser.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	discovery := app.Command("discovery", "Troubleshoot auto-discovery issues.")
	c.nodes.initNodes(discovery)
}

// TryRun attempts to run the matched subcommand.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	var commandFunc func(context.Context, discoveryClient, io.Writer) error

	switch cmd {
	case c.nodes.cmd.FullCommand():
		commandFunc = c.nodes.run
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(commandFunc(ctx, client, c.stdout))
}
