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

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	staticHostUserPrefix = "static_host_user"
)

// StaticHostUserService manages host users that should be created on SSH nodes.
type StaticHostUserService struct {
	svc *generic.ServiceWrapper[*userprovisioningpb.StaticHostUser]
}

// NewStaticHostUserService creates a new static host user service.
func NewStaticHostUserService(bk backend.Backend) (*StaticHostUserService, error) {
	svc, err := generic.NewServiceWrapper(
		generic.ServiceWrapperConfig[*userprovisioningpb.StaticHostUser]{
			Backend:       bk,
			ResourceKind:  types.KindStaticHostUser,
			BackendPrefix: backend.NewKey(staticHostUserPrefix),
			MarshalFunc:   services.MarshalProtoResource[*userprovisioningpb.StaticHostUser],
			UnmarshalFunc: services.UnmarshalProtoResource[*userprovisioningpb.StaticHostUser],
			ValidateFunc:  services.ValidateStaticHostUser,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &StaticHostUserService{
		svc: svc,
	}, nil
}

// ListStaticHostUsers lists static host users.
func (s *StaticHostUserService) ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioningpb.StaticHostUser, string, error) {
	out, nextToken, err := s.svc.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return out, nextToken, nil
}

// GetStaticHostUser returns a static host user by name.
func (s *StaticHostUserService) GetStaticHostUser(ctx context.Context, name string) (*userprovisioningpb.StaticHostUser, error) {
	out, err := s.svc.GetResource(ctx, name)
	return out, trace.Wrap(err)
}

// CreateStaticHostUser creates a static host user.
func (s *StaticHostUserService) CreateStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error) {
	out, err := s.svc.CreateResource(ctx, in)
	return out, trace.Wrap(err)
}

// UpdateStaticHostUser updates a static host user.
func (s *StaticHostUserService) UpdateStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error) {
	out, err := s.svc.ConditionalUpdateResource(ctx, in)
	return out, trace.Wrap(err)
}

// UpsertStaticHostUser upserts a static host user.
func (s *StaticHostUserService) UpsertStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error) {
	out, err := s.svc.UpsertResource(ctx, in)
	return out, trace.Wrap(err)
}

// DeleteStaticHostUser deletes a static host user. Note that this does not
// remove any host users created on nodes from the resource.
func (s *StaticHostUserService) DeleteStaticHostUser(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllStaticHostUsers deletes all static host users. Note that this does not
// remove any host users created on nodes from the resources.
func (s *StaticHostUserService) DeleteAllStaticHostUsers(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}
