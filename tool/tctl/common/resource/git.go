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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var gitServer = resource{
	getHandler:    getGitServer,
	createHandler: createGitServer,
	updateHandler: updateGitServer,
	deleteHandler: deleteGitServer,
}

func createGitServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	server, err := services.UnmarshalGitServer(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.force {
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

func getGitServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	var page, servers []types.Server

	// TODO(greedy52) use unified resource request once available.
	if ref.Name != "" {
		server, err := client.GitServerClient().GetGitServer(ctx, ref.Name)
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

func updateGitServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteGitServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.GitServerClient().DeleteGitServer(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("git_server %q has been deleted\n", ref.Name)
	return nil
}
