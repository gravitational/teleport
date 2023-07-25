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

package local

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

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
	log logrus.FieldLogger
	svc *generic.Service[*userloginstate.UserLoginState]
}

// NewUserLoginStateService creates a new UserLoginStateService.
func NewUserLoginStateService(backend backend.Backend) (*UserLoginStateService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[*userloginstate.UserLoginState]{
		Backend:       backend,
		ResourceKind:  types.KindUserLoginState,
		BackendPrefix: userLoginStatePrefix,
		MarshalFunc:   services.MarshalUserLoginState,
		UnmarshalFunc: services.UnmarshalUserLoginState,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &UserLoginStateService{
		log: logrus.WithFields(logrus.Fields{trace.Component: "user-login-state:local-service"}),
		svc: svc,
	}, nil
}

// GetUserLoginState returns the specified user login state resource.
func (u *UserLoginStateService) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	userLoginState, err := u.svc.GetResource(ctx, name)
	return userLoginState, trace.Wrap(err)
}

// UpsertUserLoginState creates or updates a user login state resource.
func (u *UserLoginStateService) UpsertUserLoginState(ctx context.Context, userLoginState *userloginstate.UserLoginState) (*userloginstate.UserLoginState, error) {
	if err := trace.Wrap(u.svc.UpsertResource(ctx, userLoginState)); err != nil {
		return nil, trace.Wrap(err)
	}
	return userLoginState, nil
}

// DeleteUserLoginState removes the specified user login state resource.
func (u *UserLoginStateService) DeleteUserLoginState(ctx context.Context, name string) error {
	return trace.Wrap(u.svc.DeleteResource(ctx, name))
}
