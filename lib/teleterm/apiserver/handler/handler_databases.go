/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package handler

import (
	"context"
	"sort"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

// GetDatabases gets databases with filters and returns paginated results
func (s *Handler) GetDatabases(ctx context.Context, req *api.GetDatabasesRequest) (*api.GetDatabasesResponse, error) {
	cluster, _, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.DaemonService.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := cluster.GetDatabases(ctx, proxyClient.CurrentCluster(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.GetDatabasesResponse{
		StartKey:   resp.StartKey,
		TotalCount: int32(resp.TotalCount),
	}

	for _, database := range resp.Databases {
		response.Agents = append(response.Agents, newAPIDatabase(database))
	}

	return response, nil
}

// ListDatabaseUsers is used to list database user suggestions when the user is attempting to
// establish a connection to a database through Teleterm.
//
// The list is based on whatever we can deduce from the role set, so it's similar to the behavior of
// `tsh db ls -v`, with the exception that Teleterm is interested only in the allowed usernames.
func (s *Handler) ListDatabaseUsers(ctx context.Context, req *api.ListDatabaseUsersRequest) (*api.ListDatabaseUsersResponse, error) {
	cluster, _, err := s.DaemonService.ResolveCluster(req.DbUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.DaemonService.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbUsers, err := cluster.GetAllowedDatabaseUsers(ctx, proxyClient.CurrentCluster(), req.DbUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.ListDatabaseUsersResponse{
		Users: dbUsers,
	}, nil
}

func newAPIDatabase(db clusters.Database) *api.Database {
	apiLabels := APILabels{}
	for name, value := range db.GetAllLabels() {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}

	sort.Sort(apiLabels)

	return &api.Database{
		Uri:      db.URI.String(),
		Name:     db.GetName(),
		Desc:     db.GetDescription(),
		Protocol: db.GetProtocol(),
		Type:     db.GetType(),
		Labels:   apiLabels,
	}
}
