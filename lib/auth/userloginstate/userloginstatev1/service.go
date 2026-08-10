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

package userloginstatev1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	userloginstatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userloginstate/v1"
	"github.com/gravitational/teleport/api/types"
	conv "github.com/gravitational/teleport/api/types/userloginstate/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig is the service config for the Access Lists gRPC service.
type ServiceConfig struct {

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// UserLoginStates is the user login state service to use.
	UserLoginStates services.UserLoginStates

	// Clock is the clock.
	Clock clockwork.Clock
}

func (c *ServiceConfig) checkAndSetDefaults() error {
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}

	if c.UserLoginStates == nil {
		return trace.BadParameter("user login states service is missing")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

type Service struct {
	userloginstatev1.UnimplementedUserLoginStateServiceServer

	authorizer      authz.Authorizer
	userLoginStates services.UserLoginStates
	clock           clockwork.Clock
}

// NewService creates a new User Login State gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		authorizer:      cfg.Authorizer,
		userLoginStates: cfg.UserLoginStates,
		clock:           cfg.Clock,
	}, nil
}

// GetUserLoginStates returns a list of all user login states.
func (s *Service) GetUserLoginStates(ctx context.Context, _ *userloginstatev1.GetUserLoginStatesRequest) (*userloginstatev1.GetUserLoginStatesResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserLoginState, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	results, err := s.userLoginStates.GetUserLoginStates(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ulsList := make([]*userloginstatev1.UserLoginState, len(results))
	for i, r := range results {
		ulsList[i] = conv.ToProto(r)
	}

	return &userloginstatev1.GetUserLoginStatesResponse{
		UserLoginStates: ulsList,
	}, nil
}

// GetUserLoginState returns the specified user login state resource.
func (s *Service) GetUserLoginState(ctx context.Context, req *userloginstatev1.GetUserLoginStateRequest) (*userloginstatev1.UserLoginState, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserLoginState, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := s.userLoginStates.GetUserLoginState(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(result), nil
}

// UpsertUserLoginState creates or updates a user login state resource.
func (s *Service) UpsertUserLoginState(ctx context.Context, req *userloginstatev1.UpsertUserLoginStateRequest) (*userloginstatev1.UserLoginState, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserLoginState, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	newUls, err := conv.FromProto(req.GetUserLoginState())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	responseUls, err := s.userLoginStates.UpsertUserLoginState(ctx, newUls)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conv.ToProto(responseUls), nil
}

// DeleteUserLoginState removes the specified user login state resource.
func (s *Service) DeleteUserLoginState(ctx context.Context, req *userloginstatev1.DeleteUserLoginStateRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserLoginState, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.userLoginStates.DeleteUserLoginState(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllUserLoginStates removes all user login states.
func (s *Service) DeleteAllUserLoginStates(ctx context.Context, _ *userloginstatev1.DeleteAllUserLoginStatesRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindUserLoginState, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.userLoginStates.DeleteAllUserLoginStates(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
