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

package cloudclusterv1

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Cache defines only read-only service methods.
type Cache interface {
	// GetCloudCluster gets the CloudCluster from the backend.
	GetCloudCluster(ctx context.Context, name string) (*cloudcluster.CloudCluster, error)

	// ListCloudClusters lists all CloudCluster from the backend.
	ListCloudClusters(ctx context.Context, pageSize int, pageToken string) ([]*cloudcluster.CloudCluster, string, error)
}

// ServiceConfig holds configuration options for the auto update gRPC service.
type ServiceConfig struct {
	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer
	// Backend is the backend used to store CloudCluster resources.
	Backend services.CloudClusterService
	// Cache is the cache used to store CloudCluster resources.
	Cache Cache
	// Emitter is the event emitter.
	Emitter apievents.Emitter
}

// Backend interface for manipulating CloudCluster resources.
type Backend interface {
	services.CloudClusterService
}

// Service implements the gRPC API layer for the CloudCluster.
type Service struct {
	cloudcluster.UnimplementedCloudClusterServiceServer

	authorizer authz.Authorizer
	backend    services.CloudClusterService
	emitter    apievents.Emitter
	cache      Cache
	clock      clockwork.Clock
}

// NewService returns a new CloudCluster API service using the given storage layer and authorizer.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("Emitter is required")
	}
	return &Service{
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		cache:      cfg.Cache,
		emitter:    cfg.Emitter,
		clock:      clockwork.NewRealClock(),
	}, nil
}

// GetCloudCluster gets the current CloudCluster singleton.
func (s *Service) GetCloudCluster(ctx context.Context, req *cloudcluster.GetCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCloudCluster, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.GetName() == "" {
		return nil, trace.BadParameter("name is required")
	}

	config, err := s.cache.GetCloudCluster(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// CreateCloudCluster creates CloudCluster singleton.
func (s *Service) CreateCloudCluster(ctx context.Context, req *cloudcluster.CreateCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCloudCluster, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateServerSideAgentConfig(req.Cluster); err != nil {
		return nil, trace.Wrap(err)
	}

	fmt.Println("cluster validated")

	config, err := s.backend.CreateCloudCluster(ctx, req.Cluster)
	var errMsg string
	if err != nil {
		fmt.Println(fmt.Sprintf("cluster failed to create: %v", err))
		errMsg = err.Error()
	} else {
		fmt.Println("cluster created successfully")
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.CloudClusterCreate{
		Metadata: apievents.Metadata{
			Type: events.CloudClusterCreateEvent,
			Code: events.CloudClusterCreateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      req.Cluster.Metadata.Name,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
	return config, trace.Wrap(err)
}

// UpdateCloudCluster updates CloudCluster singleton.
func (s *Service) UpdateCloudCluster(ctx context.Context, req *cloudcluster.UpdateCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCloudCluster, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateServerSideAgentConfig(req.Cluster); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.backend.UpdateCloudCluster(ctx, req.Cluster)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.CloudClusterUpdate{
		Metadata: apievents.Metadata{
			Type: events.CloudClusterUpdateEvent,
			Code: events.CloudClusterUpdateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameCloudCluster,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
	return config, trace.Wrap(err)
}

// UpsertCloudCluster updates or creates CloudCluster singleton.
func (s *Service) UpsertCloudCluster(ctx context.Context, req *cloudcluster.UpsertCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCloudCluster, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateServerSideAgentConfig(req.Cluster); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := s.backend.UpsertCloudCluster(ctx, req.Cluster)
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.CloudClusterUpdate{
		Metadata: apievents.Metadata{
			Type: events.CloudClusterUpdateEvent,
			Code: events.CloudClusterUpdateCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameCloudCluster,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
	return config, trace.Wrap(err)
}

// UpsertCloudCluster creates a new CloudCluster or forcefully updates an existing CloudCluster.
// This is a function rather than a method so that it can be used by the gRPC service
// and the auth server init code when dealing with resources to be applied at startup.
func UpsertCloudCluster(
	ctx context.Context,
	backend Backend,
	config *cloudcluster.CloudCluster,
) (*cloudcluster.CloudCluster, error) {
	if err := validateServerSideAgentConfig(config); err != nil {
		return nil, trace.Wrap(err, "validating config")
	}
	out, err := backend.UpsertCloudCluster(ctx, config)
	return out, trace.Wrap(err)
}

// DeleteCloudCluster deletes CloudCluster singleton.
func (s *Service) DeleteCloudCluster(ctx context.Context, req *cloudcluster.DeleteCloudClusterRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCloudCluster, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteCloudCluster(ctx, req.GetName())
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}
	userMetadata := authz.ClientUserMetadata(ctx)
	s.emitEvent(ctx, &apievents.CloudClusterDelete{
		Metadata: apievents.Metadata{
			Type: events.CloudClusterDeleteEvent,
			Code: events.CloudClusterDeleteCode,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.MetaNameCloudCluster,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: err == nil,
			Error:   errMsg,
		},
	})
	return &emptypb.Empty{}, trace.Wrap(err)
}

func (s *Service) getAllReports(ctx context.Context) ([]*cloudcluster.CloudCluster, error) {
	var reports []*cloudcluster.CloudCluster

	// this is an in-memory client, we go for the default page size
	const pageSize = 0
	var pageToken string
	for {
		page, nextToken, err := s.cache.ListCloudClusters(ctx, pageSize, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, page...)
		if nextToken == "" {
			return reports, nil
		}
		pageToken = nextToken
	}
}

func (s *Service) emitEvent(ctx context.Context, e apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, e); err != nil {
		slog.WarnContext(ctx, "Failed to emit audit event",
			"type", e.GetType(),
			"error", err,
		)
	}
}

// validateServerSideAgentConfig validates that the autoupdate_config.agent spec meets the cluster rules.
// Rules may vary based on the cluster, and over time.
//
// This function should not be confused with api/types/autoupdate.ValidateCloudCluster which validates the integrity
// of the resource and does not enforce potentially changing rules.
func validateServerSideAgentConfig(config *cloudcluster.CloudCluster) error {
	return nil
}

// ListCloudClusters returns a list of cloud clusters.
func (s *Service) ListCloudClusters(ctx context.Context, req *cloudcluster.ListCloudClustersRequest) (*cloudcluster.ListCloudClustersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCloudCluster, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, nextToken, err := s.cache.ListCloudClusters(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cloudcluster.ListCloudClustersResponse{
		Clusters:      rsp,
		NextPageToken: nextToken,
	}, nil
}
