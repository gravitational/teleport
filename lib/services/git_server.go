/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/client/gitserver"
	"github.com/gravitational/teleport/api/types"
)

// GitServerGetter defines interface for fetching git servers.
type GitServerGetter gitserver.ReadOnlyClient

// GitServers defines an interface for managing git servers.
type GitServers interface {
	GitServerGetter

	// CreateGitServer creates a Git server resource.
	CreateGitServer(ctx context.Context, item types.Server) (types.Server, error)
	// UpdateGitServer updates a Git server resource.
	UpdateGitServer(ctx context.Context, item types.Server) (types.Server, error)
	// UpsertGitServer updates a Git server resource, creating it if it doesn't exist.
	UpsertGitServer(ctx context.Context, item types.Server) (types.Server, error)
	// DeleteGitServer removes the specified Git server resource.
	DeleteGitServer(ctx context.Context, name string) error
	// DeleteAllGitServers removes all Git server resources.
	DeleteAllGitServers(ctx context.Context) error
}

// MarshalGitServer marshals the Git Server resource to JSON.
func MarshalGitServer(server types.Server, opts ...MarshalOption) ([]byte, error) {
	return MarshalServer(server, opts...)
}

// UnmarshalGitServer unmarshals the Git Server resource from JSON.
func UnmarshalGitServer(bytes []byte, opts ...MarshalOption) (types.Server, error) {
	return UnmarshalServer(bytes, types.KindGitServer, opts...)
}
