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

package userpreferencesv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	userpreferences "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the user preferences service.
type ServiceConfig struct {
	Backend    services.UserPreferences
	Authorizer authz.Authorizer
}

// Service implements the teleport.userpreferences.v1.UserPreferencesService RPC service.
type Service struct {
	userpreferences.UnimplementedUserPreferencesServiceServer

	backend    services.UserPreferences
	authorizer authz.Authorizer
}

// NewService returns a new user preferences gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	}

	return &Service{
		backend:    cfg.Backend,
		authorizer: cfg.Authorizer,
	}, nil
}

// GetUserPreferences returns the user preferences for a given user.
func (a *Service) GetUserPreferences(ctx context.Context, _ *userpreferences.GetUserPreferencesRequest) (*userpreferences.GetUserPreferencesResponse, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()

	prefs, err := a.backend.GetUserPreferences(ctx, username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &userpreferences.GetUserPreferencesResponse{
		Preferences: prefs,
	}, nil
}

// UpsertUserPreferences creates or updates user preferences for a given username.
func (a *Service) UpsertUserPreferences(ctx context.Context, req *userpreferences.UpsertUserPreferencesRequest) (*emptypb.Empty, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()

	return &emptypb.Empty{}, trace.Wrap(a.backend.UpsertUserPreferences(ctx, username, req.Preferences))
}
