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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// Backend handpicks a list of backend functions this service needs.
type Backend interface {
	services.GitServers

	// GetIntegration returns the specified integration resources.
	GetIntegration(ctx context.Context, name string) (types.Integration, error)

	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}

// GitHubAuthRequestCreator creates a new auth request for GitHub OAuth2 flow.
type GitHubAuthRequestCreator func(ctx context.Context, req types.GithubAuthRequest) (*types.GithubAuthRequest, error)

// Config is the config for Service.
type Config struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer
	// Backend is the backend service.
	Backend Backend
	// Log is the slog logger.
	Log *slog.Logger
	// ProxyPublicAddrGetter gets the public proxy address.
	ProxyPublicAddrGetter func() string
	// GitHubAuthRequestCreator is a callback to create the prepared request in
	// the backend.
	GitHubAuthRequestCreator GitHubAuthRequestCreator

	clock clockwork.Clock
}

// Service implements the gRPC service that manages git servers.
type Service struct {
	pb.UnsafeGitServerServiceServer

	cfg Config
}

// NewService creates a new git server service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}
	if cfg.ProxyPublicAddrGetter == nil {
		return nil, trace.BadParameter("ProxyPublicAddrGetter is required")
	}
	if cfg.GitHubAuthRequestCreator == nil {
		return nil, trace.BadParameter("GitHubAuthRequestCreator is required")
	}
	if cfg.Log == nil {
		cfg.Log = slog.With(teleport.ComponentKey, "gitserver.service")
	}
	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}
	return &Service{
		cfg: cfg,
	}, nil

}

func toServerV2(server types.Server) (*types.ServerV2, error) {
	serverV2, ok := server.(*types.ServerV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected server type: %T", serverV2)
	}
	return serverV2, nil
}

func (s *Service) CreateGitServer(ctx context.Context, req *pb.CreateGitServerRequest) (*types.ServerV2, error) {
	if _, err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	server, err := s.cfg.Backend.CreateGitServer(ctx, req.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return toServerV2(server)
}

func (s *Service) GetGitServer(ctx context.Context, req *pb.GetGitServerRequest) (*types.ServerV2, error) {
	authCtx, err := s.authorize(ctx, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	server, err := s.cfg.Backend.GetGitServer(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.checkAccess(authCtx, server); err != nil {
		if trace.IsAccessDenied(err) {
			return nil, trace.NotFound("git server %q not found", req.Name)
		}
		return nil, trace.Wrap(err)
	}
	return toServerV2(server)
}

func (s *Service) ListGitServers(ctx context.Context, req *pb.ListGitServersRequest) (*pb.ListGitServersResponse, error) {
	authCtx, err := s.authorize(ctx, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, next, err := s.cfg.Backend.ListGitServers(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &pb.ListGitServersResponse{
		NextPageToken: next,
	}
	for _, server := range servers {
		err := s.checkAccess(authCtx, server)
		if trace.IsAccessDenied(err) {
			continue
		} else if err != nil {
			return nil, trace.Wrap(err)
		}

		if serverV2, err := toServerV2(server); err != nil {
			return nil, trace.Wrap(err)
		} else {
			resp.Servers = append(resp.Servers, serverV2)
		}
	}
	return resp, nil
}

func (s *Service) UpdateGitServer(ctx context.Context, req *pb.UpdateGitServerRequest) (*types.ServerV2, error) {
	if _, err := s.authorize(ctx, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	server, err := s.cfg.Backend.UpdateGitServer(ctx, req.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return toServerV2(server)
}

func (s *Service) UpsertGitServer(ctx context.Context, req *pb.UpsertGitServerRequest) (*types.ServerV2, error) {
	if _, err := s.authorize(ctx, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	server, err := s.cfg.Backend.UpsertGitServer(ctx, req.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return toServerV2(server)
}

func (s *Service) DeleteGitServer(ctx context.Context, req *pb.DeleteGitServerRequest) (*emptypb.Empty, error) {
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
