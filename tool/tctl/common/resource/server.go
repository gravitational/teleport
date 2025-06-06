package resource

import (
	"context"
	"fmt"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) getNode(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	var search []string
	if rc.ref.Name != "" {
		search = []string{rc.ref.Name}
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

			if rc.ref.Name == "" {
				servers = append(servers, srv)
				continue
			}

			if srv.GetName() == rc.ref.Name || srv.GetHostname() == rc.ref.Name {
				servers = []types.Server{srv}
				return collections.NewServerCollection(servers), nil
			}
		}

		req.StartKey = next
		if req.StartKey == "" {
			break
		}
	}

	if len(servers) == 0 && rc.ref.Name != "" {
		return nil, trace.NotFound("node with ID %q not found", rc.ref.Name)
	}

	return collections.NewServerCollection(servers), nil
}

func (rc *ResourceCommand) deleteNode(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteNode(ctx, apidefaults.Namespace, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("node %v has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) createNode(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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
	if !rc.IsForced() && exists {
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
	fmt.Printf("node %q has been %s\n", name, UpsertVerb(exists, rc.IsForced()))
	return nil
}

func (rc *ResourceCommand) getAuthServer(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	servers, err := client.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewServerCollection(servers), nil
	}
	for _, server := range servers {
		if server.GetName() == rc.ref.Name || server.GetHostname() == rc.ref.Name {
			return collections.NewServerCollection([]types.Server{server}), nil
		}
	}
	return nil, trace.NotFound("auth server with ID %q not found", rc.ref.Name)
}

func (rc *ResourceCommand) getProxy(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	servers, err := client.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rc.ref.Name == "" {
		return collections.NewServerCollection(servers), nil
	}
	for _, server := range servers {
		if server.GetName() == rc.ref.Name || server.GetHostname() == rc.ref.Name {
			return collections.NewServerCollection([]types.Server{server}), nil
		}
	}
	return nil, trace.NotFound("proxy with ID %q not found", rc.ref.Name)
}

func (rc *ResourceCommand) deleteProxy(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteProxy(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Proxy %q has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) createServerInfo(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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
	if !rc.force && exists {
		return trace.AlreadyExists("server info %q already exists", name)
	}

	err = client.UpsertServerInfo(ctx, si)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Server info %q has been %s\n",
		name, UpsertVerb(exists, rc.force),
	)
	return nil
}

func (rc *ResourceCommand) getServerInfo(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		si, err := client.GetServerInfo(ctx, rc.ref.Name)
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

func (rc *ResourceCommand) deleteServerInfo(ctx context.Context, client *authclient.Client) error {
	if err := client.DeleteServerInfo(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Server info %q has been deleted\n", rc.ref.Name)
	return nil
}
