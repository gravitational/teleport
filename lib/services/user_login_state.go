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

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/utils"
)

// UserLoginStatesGetter is the interface for reading user login states.
type UserLoginStatesGetter interface {
	// GetUserLoginStates returns the all user login state resources.
	GetUserLoginStates(context.Context) ([]*userloginstate.UserLoginState, error)

	// GetUserLoginState returns the specified user login state resource.
	GetUserLoginState(context.Context, string) (*userloginstate.UserLoginState, error)
}

// UserLoginStates is the interface for managing with user login states.
type UserLoginStates interface {
	UserLoginStatesGetter

	// UpsertUserLoginState creates or updates a user login state resource.
	UpsertUserLoginState(context.Context, *userloginstate.UserLoginState) (*userloginstate.UserLoginState, error)

	// DeleteUserLoginState removes the specified user login state resource.
	DeleteUserLoginState(context.Context, string) error

	// DeleteAllUserLoginStates removes all user login state resources.
	DeleteAllUserLoginStates(context.Context) error
}

// MarshalUserLoginState marshals the user login state resource to JSON.
func MarshalUserLoginState(userLoginState *userloginstate.UserLoginState, opts ...MarshalOption) ([]byte, error) {
	if err := userLoginState.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		prevID := userLoginState.GetResourceID()
		prevRev := userLoginState.GetRevision()
		defer func() {
			userLoginState.SetResourceID(prevID)
			userLoginState.SetRevision(prevRev)
		}()
		userLoginState.SetResourceID(0)
		userLoginState.SetRevision("")
	}
	return utils.FastMarshal(userLoginState)
}

// UnmarshalUserLoginState unmarshals the user login state resource from JSON.
func UnmarshalUserLoginState(data []byte, opts ...MarshalOption) (*userloginstate.UserLoginState, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uls := &userloginstate.UserLoginState{}
	if err := utils.FastUnmarshal(data, &uls); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := uls.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		uls.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		uls.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		uls.SetExpiry(cfg.Expires)
	}

	return uls, nil
}

// UserOrLoginStateGetter defines an interface that can get user login states or users.
type UserOrLoginStateGetter interface {
	UserLoginStatesGetter
	UserGetter
}

// GetUserOrLoginState will return the given user or the login state associated with the user.
func GetUserOrLoginState(ctx context.Context, getter UserOrLoginStateGetter, username string) (UserState, error) {
	uls, err := getter.GetUserLoginState(ctx, username)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if err == nil {
		return uls, nil
	}

	user, err := getter.GetUser(ctx, username, false)
	return user, trace.Wrap(err)
}
