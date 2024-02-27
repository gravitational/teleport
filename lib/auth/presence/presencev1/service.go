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

package presencev1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	presencepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// Cache is the subset of the cached resources that the Service queries.
type Cache interface{}

// Backend is the subset of the backend resources that the Service modifies.
type Backend interface {
	GetRemoteCluster(clusterName string) (types.RemoteCluster, error)
	CreateRemoteCluster(rc types.RemoteCluster) error
	UpdateRemoteCluster(rc types.RemoteCluster) error
}

type AuthServer interface {
	// Special!
	DeleteRemoteCluster(ctx context.Context, clusterName string) error
}

// ServiceConfig holds configuration options for
// the presence gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	AuthServer AuthServer
	Cache      Cache
	Backend    Backend
	Logger     logrus.FieldLogger
	Emitter    apievents.Emitter
	Reporter   usagereporter.UsageReporter
	Clock      clockwork.Clock
}

// Service implements the teleport.presence.v1.PresenceService RPC service.
type Service struct {
	presencepb.UnimplementedPresenceServiceServer

	authorizer authz.Authorizer
	authServer AuthServer
	cache      Cache
	backend    Backend
	logger     logrus.FieldLogger
	emitter    apievents.Emitter
	reporter   usagereporter.UsageReporter
	clock      clockwork.Clock
}

// NewService returns a new presence gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	case cfg.AuthServer == nil:
		return nil, trace.BadParameter("auth server is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = logrus.WithField(trace.Component, "presence.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &Service{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		authServer: cfg.AuthServer,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		emitter:    cfg.Emitter,
		reporter:   cfg.Reporter,
		clock:      cfg.Clock,
	}, nil
}

func (s *Service) GetRemoteCluster(
	ctx context.Context, req *presencepb.GetRemoteClusterRequest,
) (*types.RemoteClusterV3, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("name: must be specified")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRemoteCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := s.backend.GetRemoteCluster(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.Checker.CheckAccessToRemoteCluster(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}

	v3, ok := rc.(*types.RemoteClusterV3)
	if !ok {
		s.logger.Warnf("expected type RemoteClusterV3, got %T for user %q", rc, rc.GetName())
		return nil, trace.BadParameter("encountered unexpected remote cluster type")
	}

	return v3, nil
}

func (s *Service) ListRemoteClusters(
	ctx context.Context, req *presencepb.ListRemoteClustersRequest,
) (*presencepb.ListRemoteClustersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRemoteCluster, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: implement ListRemoteClusters in the backend
	rcs := make([]types.RemoteCluster, 0)

	// TODO: Filter on their access

	return &presencepb.ListRemoteClustersResponse{
		RemoteClusters: nil,
	}, nil
}

func (s *Service) CreateRemoteCluster(
	ctx context.Context, req *presencepb.CreateRemoteClusterRequest,
) (*types.RemoteClusterV3, error) {
	if req.RemoteCluster == nil {
		return nil, trace.BadParameter("remote_cluster: must not be nil")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRemoteCluster, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: return the created remote cluster
	if err := s.backend.CreateRemoteCluster(req.RemoteCluster); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func (s *Service) UpdateRemoteCluster(
	ctx context.Context, req *presencepb.UpdateRemoteClusterRequest,
) (*types.RemoteClusterV3, error) {
	if req.RemoteCluster == nil {
		return nil, trace.BadParameter("remote_cluster: must not be nil")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindRemoteCluster, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: return the updated remote cluster
	if err := s.backend.UpdateRemoteCluster(req.RemoteCluster); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func (s *Service) UpsertRemoteCluster(
	ctx context.Context, req *presencepb.UpsertRemoteClusterRequest,
) (*types.RemoteClusterV3, error) {
	if req.RemoteCluster == nil {
		return nil, trace.BadParameter("remote_cluster: must not be nil")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(
		types.KindRemoteCluster, types.VerbCreate, types.VerbUpdate,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Implement Upsert
	return nil, nil
}

func (s *Service) DeleteRemoteCluster(
	ctx context.Context, req *presencepb.DeleteRemoteClusterRequest,
) (*types.RemoteClusterV3, error) {
	if req.Name == "" {
		return nil, trace.BadParameter("name: must be specified")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(
		types.KindRemoteCluster, types.VerbDelete,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authServer.DeleteRemoteCluster(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}
