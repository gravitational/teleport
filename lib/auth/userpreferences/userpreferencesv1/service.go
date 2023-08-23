/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package userpreferencesv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	userpreferences "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the user preferences service.
type ServiceConfig struct {
	Backend    services.UserPreferences
	Authorizer authz.Authorizer
	Logger     *logrus.Entry
}

// Service implements the teleport.userpreferences.v1.UserPreferencesService RPC service.
type Service struct {
	userpreferences.UnimplementedUserPreferencesServiceServer

	backend    services.UserPreferences
	authorizer authz.Authorizer
	log        *logrus.Entry
}

// NewService returns a new user preferences gRPC service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Logger == nil:
		cfg.Logger = logrus.WithField(trace.Component, "userpreferences.service")
	}

	return &Service{
		backend:    cfg.Backend,
		authorizer: cfg.Authorizer,
		log:        cfg.Logger,
	}, nil
}

// GetUserPreferences returns the user preferences for a given user.
func (a *Service) GetUserPreferences(ctx context.Context, _ *userpreferences.GetUserPreferencesRequest) (*userpreferences.GetUserPreferencesResponse, error) {
	authCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.IsLocalUser(*authCtx) {
		return nil, trace.AccessDenied("Non-local user cannot get user preferences")
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
	if !authz.IsLocalUser(*authCtx) {
		return nil, trace.AccessDenied("Non-local user cannot upsert user preferences")
	}

	username := authCtx.User.GetName()

	return &emptypb.Empty{}, trace.Wrap(a.backend.UpsertUserPreferences(ctx, username, req.Preferences))
}
