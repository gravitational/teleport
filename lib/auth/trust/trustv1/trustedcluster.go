/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package trustv1

import (
	"context"

	"github.com/gravitational/trace"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

// UpsertTrustedCluster upserts a Trusted Cluster.
func (s *Service) UpsertTrustedCluster(ctx context.Context, req *trustpb.UpsertTrustedClusterRequest) (*types.TrustedClusterV2, error) {
	// Don't allow a Cloud tenant to be a leaf cluster.
	if modules.GetModules().Features().Cloud {
		return nil, trace.NotImplemented("cloud tenants cannot be leaf clusters")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindTrustedCluster, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = services.ValidateTrustedCluster(req.GetTrustedCluster()); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := s.authServer.UpsertTrustedClusterV2(ctx, req.GetTrustedCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected Trusted Cluster type: %T", tc)
	}
	return trustedClusterV2, nil
}

// CreateTrustedCluster creates a Trusted Cluster.
func (s *Service) CreateTrustedCluster(ctx context.Context, req *trustpb.CreateTrustedClusterRequest) (*types.TrustedClusterV2, error) {
	// Don't allow a Cloud tenant to be a leaf cluster.
	if modules.GetModules().Features().Cloud {
		return nil, trace.NotImplemented("cloud tenants cannot be leaf clusters")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindTrustedCluster, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = services.ValidateTrustedCluster(req.GetTrustedCluster()); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := s.authServer.CreateTrustedCluster(ctx, req.GetTrustedCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected Trusted Cluster type: %T", tc)
	}
	return trustedClusterV2, nil
}

// UpdateTrustedCluster updates a Trusted Cluster.
func (s *Service) UpdateTrustedCluster(ctx context.Context, req *trustpb.UpdateTrustedClusterRequest) (*types.TrustedClusterV2, error) {
	// Don't allow a Cloud tenant to be a leaf cluster.
	if modules.GetModules().Features().Cloud {
		return nil, trace.NotImplemented("cloud tenants cannot be leaf clusters")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindTrustedCluster, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = services.ValidateTrustedCluster(req.GetTrustedCluster()); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := s.authServer.UpdateTrustedCluster(ctx, req.GetTrustedCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected Trusted Cluster type: %T", tc)
	}
	return trustedClusterV2, nil
}

// ListTrustedClusters returns a page of Trusted Cluster resources.
func (s *Service) ListTrustedClusters(ctx context.Context, req *trustpb.ListTrustedClustersRequest) (*trustpb.ListTrustedClustersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindTrustedCluster, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	tcs, next, err := s.authServer.ListTrustedClusters(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &trustpb.ListTrustedClustersResponse{
		TrustedClusters: make([]*types.TrustedClusterV2, 0, len(tcs)),
		NextPageToken:   next,
	}

	for _, tc := range tcs {
		trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
		if !ok {
			return nil, trace.Errorf("encountered unexpected Trusted Cluster type: %T", tc)
		}
		resp.TrustedClusters = append(resp.TrustedClusters, trustedClusterV2)
	}

	return resp, nil
}
