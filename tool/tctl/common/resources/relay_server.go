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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type relayServerCollection struct {
	relayServers []*presencev1.RelayServer
}

func (c *relayServerCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.relayServers))
	for _, resource := range c.relayServers {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *relayServerCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Hostname", "Relay Group", "Peer Address"})
	for _, relay := range c.relayServers {
		t.AddRow([]string{
			relay.GetMetadata().GetName(),
			relay.GetSpec().GetHostname(),
			relay.GetSpec().GetRelayGroup(),
			relay.GetSpec().GetPeerAddr(),
		})
	}
	return trace.Wrap(t.WriteTo(w))
}

func relayServerHandler() Handler {
	return Handler{
		getHandler:    getRelayServer,
		deleteHandler: deleteRelayServer,
		singleton:     false,
		mfaRequired:   false,
		description:   "A lightweight proxy that routes connections from clients to resources without the need for the connection to go through the broader control plane",
	}
}

func getRelayServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		rs, err := client.GetRelayServer(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &relayServerCollection{relayServers: []*presencev1.RelayServer{rs}}, nil
	}

	resources, err := stream.Collect(
		clientutils.Resources(ctx, client.ListRelayServers),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &relayServerCollection{relayServers: resources}, nil
}

func deleteRelayServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteRelayServer(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("relay_server %+q has been deleted\n", ref.Name)
	return nil
}
