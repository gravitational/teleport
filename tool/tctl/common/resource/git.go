package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createGitServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	server, err := services.UnmarshalGitServer(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if rc.IsForced() {
		_, err = client.GitServerClient().UpsertGitServer(ctx, server)
	} else {
		_, err = client.GitServerClient().CreateGitServer(ctx, server)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("git server %q has been created\n", server.GetName())
	return nil
}

func (rc *ResourceCommand) getGitServer(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	var page, servers []types.Server

	// TODO(greedy52) use unified resource request once available.
	if rc.ref.Name != "" {
		server, err := client.GitServerClient().GetGitServer(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewServerCollection([]types.Server{server}), nil
	}
	var err error
	var token string
	for {
		page, token, err = client.GitServerClient().ListGitServers(ctx, 0, token)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers = append(servers, page...)
		if token == "" {
			break
		}
	}
	// TODO(greedy52) consider making dedicated git server collection.
	return collections.NewServerCollection(servers), nil
}

func (rc *ResourceCommand) updateGitServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	server, err := services.UnmarshalGitServer(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.GitServerClient().UpdateGitServer(ctx, server)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("git server %q has been updated\n", server.GetName())
	return nil
}

func (rc *ResourceCommand) deleteGitServer(ctx context.Context, client *authclient.Client) error {
	if err := client.GitServerClient().DeleteGitServer(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("git_server %q has been deleted\n", rc.ref.Name)
	return nil
}
