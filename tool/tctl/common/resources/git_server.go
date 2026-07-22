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

package resources

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// NewGitServerCollection creates a [Collection] over the provided Git servers.
func NewGitServerCollection(servers []types.Server) Collection {
	// TODO(greedy52) consider making dedicated git server collection.
	return NewServerCollection(servers)
}

func gitServerHandler() Handler {
	return Handler{
		getHandler:    getGitServer,
		createHandler: createGitServer,
		updateHandler: updateGitServer,
		deleteHandler: deleteGitServer,
		singleton:     false,
		mfaRequired:   false,
		description:   "Represents a Git service, such as GitHub, that Teleport can proxy Git connections to.",
	}
}

func getGitServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		server, err := client.GitServerClient().GetGitServer(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewGitServerCollection([]types.Server{server}), nil
	}

	servers, err := stream.Collect(clientutils.Resources(ctx, client.GitServerClient().ListGitServers))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewGitServerCollection(servers), nil
}

func createGitServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	server, err := services.UnmarshalGitServer(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.Force {
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

func updateGitServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
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
