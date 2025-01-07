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

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	userLoginStatePrefix = "user_login_state"
)

// UserLoginStateService manages user login state resources in the Backend.
type UserLoginStateService struct {
	svc *generic.Service[*userloginstate.UserLoginState]
}

// NewUserLoginStateService creates a new UserLoginStateService.
func NewUserLoginStateService(b backend.Backend) (*UserLoginStateService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[*userloginstate.UserLoginState]{
		Backend:       b,
		ResourceKind:  types.KindUserLoginState,
		BackendPrefix: backend.NewKey(userLoginStatePrefix),
		MarshalFunc:   services.MarshalUserLoginState,
		UnmarshalFunc: services.UnmarshalUserLoginState,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &UserLoginStateService{
		svc: svc,
	}, nil
}

// GetUserLoginStates returns the all user login state resources.
func (u *UserLoginStateService) GetUserLoginStates(ctx context.Context) ([]*userloginstate.UserLoginState, error) {
	states, err := u.svc.GetResources(ctx)
	return states, trace.Wrap(err)
}

// GetUserLoginState returns the specified user login state resource.
func (u *UserLoginStateService) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	userLoginState, err := u.svc.GetResource(ctx, name)
	return userLoginState, trace.Wrap(err)
}

// UpsertUserLoginState creates or updates a user login state resource.
func (u *UserLoginStateService) UpsertUserLoginState(ctx context.Context, userLoginState *userloginstate.UserLoginState) (*userloginstate.UserLoginState, error) {
	upserted, err := u.svc.UpsertResource(ctx, userLoginState)
	return upserted, trace.Wrap(err)
}

// DeleteUserLoginState removes the specified user login state resource.
func (u *UserLoginStateService) DeleteUserLoginState(ctx context.Context, name string) error {
	return trace.Wrap(u.svc.DeleteResource(ctx, name))
}

// DeleteAllUserLoginStates removes all user login state resources.
func (u *UserLoginStateService) DeleteAllUserLoginStates(ctx context.Context) error {
	return trace.Wrap(u.svc.DeleteAllResources(ctx))
}
