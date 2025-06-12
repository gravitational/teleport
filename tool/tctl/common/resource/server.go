/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package resource

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var node = resource{
	getHandler:    getNode,
	createHandler: createNode,
	deleteHandler: deleteNode,
}

func getNode(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	var search []string
	if ref.Name != "" {
		search = []string{ref.Name}
	}

	req := proto.ListUnifiedResourcesRequest{
		Kinds:          []string{types.KindNode},
		SearchKeywords: search,
		SortBy:         types.SortBy{Field: types.ResourceKind},
	}

	var servers []types.Server
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
				servers = append(servers, srv)
				continue
			}

			if srv.GetName() == ref.Name || srv.GetHostname() == ref.Name {
				servers = []types.Server{srv}
				return collections.NewServerCollection(servers), nil
			}
		}

		req.StartKey = next
		if req.StartKey == "" {
			break
		}
	}

	if len(servers) == 0 && ref.Name != "" {
		return nil, trace.NotFound("node with ID %q not found", ref.Name)
	}

	return collections.NewServerCollection(servers), nil
}

func deleteNode(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteNode(ctx, apidefaults.Namespace, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("node %v has been deleted\n", ref.Name)
	return nil
}

func createNode(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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
	if !opts.force && exists {
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
	fmt.Printf("node %q has been %s\n", name, UpsertVerb(exists, opts.force))
	return nil
}

var authServer = resource{
	getHandler: getAuthServer,
}

func getAuthServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	servers, err := client.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewServerCollection(servers), nil
	}
	for _, server := range servers {
		if server.GetName() == ref.Name || server.GetHostname() == ref.Name {
			return collections.NewServerCollection([]types.Server{server}), nil
		}
	}
	return nil, trace.NotFound("auth server with ID %q not found", ref.Name)
}

var proxy = resource{
	getHandler:    getProxy,
	deleteHandler: deleteProxy,
}

func getProxy(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	servers, err := client.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return collections.NewServerCollection(servers), nil
	}
	for _, server := range servers {
		if server.GetName() == ref.Name || server.GetHostname() == ref.Name {
			return collections.NewServerCollection([]types.Server{server}), nil
		}
	}
	return nil, trace.NotFound("proxy with ID %q not found", ref.Name)
}

func deleteProxy(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteProxy(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Proxy %q has been deleted\n", ref.Name)
	return nil
}

var serverInfo = resource{
	getHandler:    getServerInfo,
	createHandler: createServerInfo,
	deleteHandler: deleteServerInfo,
}

func createServerInfo(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	si, err := services.UnmarshalServerInfo(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if the ServerInfo already exists.
	name := si.GetName()
	_, err = client.GetServerInfo(ctx, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := (err == nil)
	if !opts.force && exists {
		return trace.AlreadyExists("server info %q already exists", name)
	}

	err = client.UpsertServerInfo(ctx, si)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Server info %q has been %s\n",
		name, UpsertVerb(exists, opts.force),
	)
	return nil
}

func getServerInfo(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		si, err := client.GetServerInfo(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewServerInfoCollection([]types.ServerInfo{si}), nil
	}
	serverInfos, err := stream.Collect(client.GetServerInfos(ctx))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewServerInfoCollection(serverInfos), nil
}

func deleteServerInfo(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteServerInfo(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Server info %q has been deleted\n", ref.Name)
	return nil
}
