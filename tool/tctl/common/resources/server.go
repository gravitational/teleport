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

	"github.com/gravitational/trace"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
		id := se.GetName()
		if scope := se.GetScope(); scope != "" {
			id = scopes.QualifiedName{Scope: scope, Name: se.GetName()}.String()
		}
		rows = append(rows, []string{
			se.GetHostname(), id, se.GetAddr(), labels, se.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "ID", "Public Address", "Labels", "Version"}
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

// serverScopedHandler returns a limited [ScopedHandler] for nodes that are registered with a scope. This is necessary to support
// nodes being accessed with a mix of scoped and unscoped semantics.
//
// TODO(fspmmarshall/scopes): revisit this pattern. would it be better to represent handlers with mixed semantics in some more
// unified manner?
func serverScopedHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:    getScopedNode,
		deleteHandler: deleteScopedNode,
		description:   "Represents an SSH instance in the cluster, either a Teleport SSH agent or OpenSSH",
	}
}

func getScopedNode(ctx context.Context, client *authclient.Client, subKind string, sqn *scopes.QualifiedName, opts GetOpts) (Collection, error) {
	if subKind != "" {
		return nil, rejectSubKind(types.KindNode, subKind)
	}
	if sqn == nil {
		// this isn't expected to happen, the unscoped handler should catch anything that didn't include an explicit SQN, but there's no
		// harm in falling back to the unscoped handler here.
		return getServer(ctx, client, services.Ref{Kind: types.KindNode}, opts)
	}
	node, err := client.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServerCollection{servers: []types.Server{node}}, nil
}

func deleteScopedNode(ctx context.Context, client *authclient.Client, subKind string, sqn scopes.QualifiedName) error {
	if subKind != "" {
		return rejectSubKind(types.KindNode, subKind)
	}
	if err := client.DeleteSSHServer(ctx, presencev1.DeleteSSHServerRequest_builder{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("node %v has been deleted\n", sqn.String())
	return nil
}

func createServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	server, err := services.UnmarshalServer(raw.Raw, types.KindNode, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	name := server.GetName()
	scope := server.GetScope()
	_, err = client.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{
		Scope: scope,
		Name:  name,
	}.Build())
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
	fmt.Printf("node %q has been %s\n", scopes.QualifiedName{Name: name, Scope: scope}.String(), upsertVerb(exists, opts.Force))
	return nil
}

func deleteServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	sqn, err := scopes.ParseQualifiedName(ref.Name)
	if err != nil {
		sqn = scopes.QualifiedName{Name: ref.Name}
	}
	if err := client.DeleteSSHServer(ctx, presencev1.DeleteSSHServerRequest_builder{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}.Build()); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("node %v has been deleted\n", sqn.String())
	return nil
}

func getServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	nodes, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
		return client.ListSSHServers(ctx, presencev1.ListSSHServersRequest_builder{
			PageSize:    int32(pageSize),
			PageToken:   pageToken,
			ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
		}.Build())
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ref.Name == "" {
		return NewServerCollection(nodes), nil
	}

	sqn, err := scopes.ParseQualifiedName(ref.Name)
	if err != nil {
		sqn = scopes.QualifiedName{Name: ref.Name}
	}

	// Nodes may be referenced by ID or by hostname, so hostname is supplied as
	// an alternate name.
	nodes = FilterBySQNOrDiscoveredName(nodes, sqn, types.Server.GetHostname)
	if len(nodes) == 0 {
		return nil, trace.NotFound("node with ID %q not found", sqn.String())
	}

	return NewServerCollection(nodes), nil
}
