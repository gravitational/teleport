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

package local

import (
	"context"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const gitServerPrefix = "git_server"

// GitServerService is the local implementation of GitSerever service that is
// using local backend.
type GitServerService struct {
	service *generic.Service[types.Server]
}

func validateKind(server types.Server) error {
	if server.GetKind() != types.KindGitServer {
		return trace.CompareFailed("expecting kind git_server but got %v", server.GetKind())
	}
	return nil
}

// NewGitServerService returns new instance of GitServerService
func NewGitServerService(b backend.Backend) (*GitServerService, error) {
	service, err := generic.NewService(&generic.ServiceConfig[types.Server]{
		Backend:       b,
		ResourceKind:  types.KindGitServer,
		PageLimit:     defaults.MaxIterationLimit,
		BackendPrefix: backend.NewKey(gitServerPrefix),
		MarshalFunc:   services.MarshalGitServer,
		UnmarshalFunc: services.UnmarshalGitServer,
		ValidateFunc:  validateKind,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &GitServerService{
		service: service,
	}, nil
}

// GetGitServer returns Git servers by name.
func (s *GitServerService) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	item, err := s.service.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// CreateGitServer creates a Git server resource.
func (s *GitServerService) CreateGitServer(ctx context.Context, item types.Server) (types.Server, error) {
	created, err := s.service.CreateResource(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return created, nil
}

// UpdateGitServer updates a Git server resource.
func (s *GitServerService) UpdateGitServer(ctx context.Context, item types.Server) (types.Server, error) {
	// ConditionalUpdateResource can return invalid revision instead of not found, so we'll check if resource exists first
	if _, err := s.service.GetResource(ctx, item.GetName()); trace.IsNotFound(err) {
		return nil, err
	}
	updated, err := s.service.ConditionalUpdateResource(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updated, nil
}

// UpsertGitServer updates a Git server resource, creating it if it doesn't exist.
func (s *GitServerService) UpsertGitServer(ctx context.Context, item types.Server) (types.Server, error) {
	upserted, err := s.service.UpsertResource(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return upserted, nil
}

// DeleteGitServer removes the specified Git server resource.
func (s *GitServerService) DeleteGitServer(ctx context.Context, name string) error {
	if err := s.service.DeleteResource(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllGitServers removes all Git server resources.
func (s *GitServerService) DeleteAllGitServers(ctx context.Context) error {
	if err := s.service.DeleteAllResources(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ListGitServers returns all Git servers matching filter.
func (s *GitServerService) ListGitServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	items, next, err := s.service.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return items, next, nil
}

func newGitServerParser() *gitServerParser {
	return &gitServerParser{
		baseParser: newBaseParser(backend.NewKey(gitServerPrefix)),
	}
}

type gitServerParser struct {
	baseParser
}

func (p *gitServerParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		components := event.Item.Key.Components()
		if len(components) < 2 {
			return nil, trace.NotFound("failed parsing %s", event.Item.Key)
		}

		return &types.ResourceHeader{
			Kind:    types.KindGitServer,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      components[1],
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalServer(event.Item.Value,
			types.KindGitServer,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
