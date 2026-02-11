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

package resources

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/tool/common"
)

// ServerCollection implements [Collection] for [types.Servers].
type ServerCollection struct {
	servers []types.Server
}

// NewServerCollection creates a [ServerCollection] over the provided servers.
func NewServerCollection(servers []types.Server) *ServerCollection {
	return &ServerCollection{servers: servers}
}

func (s *ServerCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(s.servers))
	for _, resource := range s.servers {
		r = append(r, resource)
	}
	return r
}

func (s *ServerCollection) WriteText(w io.Writer, verbose bool) error {
	rows := make([][]string, 0, len(s.servers))
	for _, se := range s.servers {
		labels := common.FormatLabels(se.GetAllLabels(), verbose)
		rows = append(rows, []string{
			se.GetHostname(), se.GetName(), se.GetAddr(), labels, se.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "UUID", "Public Address", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (s *ServerCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(w, s.servers)
}

func (s *ServerCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, s.servers)
}

func serverHandler() Handler {
	return Handler{
		getHandler:    getServer,
		createHandler: createServer,
		deleteHandler: deleteServer,
		singleton:     false,
		mfaRequired:   false,
		description:   "Represents an SSH instance in the cluster, either a Teleport SSH agent or OpenSSH",
	}
}

func createServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	server, err := services.UnmarshalServer(raw.Raw, types.KindNode, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	name := server.GetName()
	_, err = client.GetNode(ctx, server.GetNamespace(), name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := (err == nil)
	if !opts.Force && exists {
		return trace.AlreadyExists("node %q with Hostname %q and Addr %q already exists, use --force flag to override",
			name,
			server.GetHostname(),
			server.GetAddr(),
		)
	}

	_, err = client.UpsertNode(ctx, server)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("node %q has been %s\n", name, upsertVerb(exists, opts.Force))
	return nil
}

func deleteServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteNode(ctx, defaults.Namespace, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("node %v has been deleted\n", ref.Name)
	return nil
}

func getServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	var search []string
	if ref.Name != "" {
		search = []string{ref.Name}
	}

	req := proto.ListUnifiedResourcesRequest{
		Kinds:          []string{types.KindNode},
		SearchKeywords: search,
		SortBy:         types.SortBy{Field: types.ResourceKind},
	}

	var collection ServerCollection
	for {
		page, next, err := apiclient.GetUnifiedResourcePage(ctx, client, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range page {
			srv, ok := r.ResourceWithLabels.(types.Server)
			if !ok {
				slog.WarnContext(ctx, "expected types.Server but received unexpected type", "resource_type", logutils.TypeAttr(r))
				continue
			}

			if ref.Name == "" {
				collection.servers = append(collection.servers, srv)
				continue
			}

			if srv.GetName() == ref.Name || srv.GetHostname() == ref.Name {
				collection.servers = []types.Server{srv}
				return &collection, nil
			}
		}

		req.StartKey = next
		if req.StartKey == "" {
			break
		}
	}

	if len(collection.servers) == 0 && ref.Name != "" {
		return nil, trace.NotFound("node with ID %q not found", ref.Name)
	}

	return &collection, nil
}
