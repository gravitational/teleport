/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/utils"
)

// UserLoginStateGetter defines an interface for reading user login states.
type UserLoginStateGetter interface {
	// GetUserLoginState returns the specified user login state resource.
	GetUserLoginState(context.Context, string) (*userloginstate.UserLoginState, error)
}

// UserLoginStates defines an interface for managing user login states.
type UserLoginStates interface {
	UserLoginStateGetter

	// UpsertUserLoginState creates or updates a user login state resource.
	UpsertUserLoginState(context.Context, *userloginstate.UserLoginState) (*userloginstate.UserLoginState, error)

	// DeletetUserLoginState deletes a user login state resource.
	DeleteUserLoginState(context.Context, string) error
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
		copy := *userLoginState
		copy.SetResourceID(0)
		userLoginState = &copy
	}
	return utils.FastMarshal(userLoginState)
}

// UnmarshalUserLoginState unmarshals the user login state resource from JSON.
func UnmarshalUserLoginState(data []byte, opts ...MarshalOption) (*userloginstate.UserLoginState, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing user login state data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var userLoginState *userloginstate.UserLoginState
	if err := utils.FastUnmarshal(data, &userLoginState); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := userLoginState.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		userLoginState.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		userLoginState.SetExpiry(cfg.Expires)
	}
	return userLoginState, nil
}
