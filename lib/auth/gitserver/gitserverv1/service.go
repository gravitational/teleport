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

package gitserverv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/teleport"
	gitserverv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Config is the config for Service.
type Config struct {
	Authorizer authz.Authorizer
	Backend    services.Presence
	Cache      services.GitServersGetter
	Log        *slog.Logger
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if c.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if c.Cache == nil {
		return trace.BadParameter("cahe is required")
	}
	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, "gitserver.service")
	}
	return nil
}

type Service struct {
	gitserverv1.UnsafeGitServerServiceServer

	cfg Config
}

func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{
		cfg: cfg,
	}, nil
}

func (s *Service) GetGitServer(ctx context.Context, req *gitserverv1.GetGitServerRequest) (*types.ServerV2, error) {
	authCtx, err := s.authorize(ctx, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server, err := s.cfg.Cache.GetGitServer(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.checkAccess(authCtx, server); err != nil {
		if trace.IsAccessDenied(err) {
			return nil, trace.NotFound("not found")
		}
		return nil, trace.Wrap(err)
	}

	serverV2, ok := server.(*types.ServerV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected server type: %T", serverV2)
	}
	return serverV2, nil
}

func (s *Service) UpsertGitServer(ctx context.Context, req *gitserverv1.UpsertGitServerRequest) (*types.ServerV2, error) {
	_, err := s.authorize(ctx, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server := types.Server(req.Server)
	if err := services.CheckAndSetDefaults(server); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := s.cfg.Backend.UpsertGitServer(ctx, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverV2, ok := upserted.(*types.ServerV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected server type: %T", serverV2)
	}
	return serverV2, nil
}

func (s *Service) DeleteGitServer(ctx context.Context, req *gitserverv1.DeleteGitServerRequest) (*emptypb.Empty, error) {
	_, err := s.authorize(ctx, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.cfg.Backend.DeleteGitServer(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) authorize(ctx context.Context, verb string, additionalVerbs ...string) (*authz.Context, error) {
	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindGitServer, verb, additionalVerbs...); err != nil {
		return nil, trace.Wrap(err)
	}
	return authCtx, nil
}

func (s *Service) checkAccess(authCtx *authz.Context, server types.Server) error {
	// MFA is not required for operations on git resources but will be enforced
	// at the connection time.
	state := services.AccessState{MFAVerified: true}
	return trace.Wrap(authCtx.Checker.CheckAccess(server, state))
}
