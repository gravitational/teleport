/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package local

import (
	"context"
	"strings"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	workloadClusterPrefix = "workload_cluster"
)

// WorkloadClusterService manages [workloadclusterv1.WorkloadCluster] resources
// in the backend.
type WorkloadClusterService struct {
	service *generic.ServiceWrapper[*workloadclusterv1.WorkloadCluster]
}

// NewWorkloadClusterService creates a new WorkloadClusterService.
func NewWorkloadClusterService(b backend.Backend) (*WorkloadClusterService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*workloadclusterv1.WorkloadCluster]{
			Backend:       b,
			ResourceKind:  types.KindWorkloadCluster,
			BackendPrefix: backend.NewKey(workloadClusterPrefix),
			MarshalFunc:   services.MarshalWorkloadCluster,
			UnmarshalFunc: services.UnmarshalWorkloadCluster,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &WorkloadClusterService{service: service}, nil
}

// ListWorkloadClusters returns a paginated lists of WorkloadClusters.
func (s *WorkloadClusterService) ListWorkloadClusters(ctx context.Context, pagesize int, lastKey string) ([]*workloadclusterv1.WorkloadCluster, string, error) {
	r, nextToken, err := s.service.ListResources(ctx, pagesize, lastKey)
	return r, nextToken, trace.Wrap(err)
}

// GetWorkloadCluster returns the specified WorkloadCluster.
func (s *WorkloadClusterService) GetWorkloadCluster(ctx context.Context, name string) (*workloadclusterv1.WorkloadCluster, error) {
	r, err := s.service.GetResource(ctx, name)
	return r, trace.Wrap(err)
}

// CreateWorkloadCluster creates a new WorkloadCluster.
func (s *WorkloadClusterService) CreateWorkloadCluster(ctx context.Context, workloadCluster *workloadclusterv1.WorkloadCluster) (*workloadclusterv1.WorkloadCluster, error) {
	r, err := s.service.CreateResource(ctx, workloadCluster)
	return r, trace.Wrap(err)
}

// UpdateWorkloadCluster updates an existing WorkloadCluster.
func (s *WorkloadClusterService) UpdateWorkloadCluster(ctx context.Context, workloadCluster *workloadclusterv1.WorkloadCluster) (*workloadclusterv1.WorkloadCluster, error) {
	r, err := s.service.ConditionalUpdateResource(ctx, workloadCluster)
	return r, trace.Wrap(err)
}

// UpsertWorkloadCluster upserts an existing WorkloadCluster.
func (s *WorkloadClusterService) UpsertWorkloadCluster(ctx context.Context, workloadCluster *workloadclusterv1.WorkloadCluster) (*workloadclusterv1.WorkloadCluster, error) {
	r, err := s.service.UpsertResource(ctx, workloadCluster)
	return r, trace.Wrap(err)
}

// DeleteWorkloadCluster removes the specified WorkloadCluster.
func (s *WorkloadClusterService) DeleteWorkloadCluster(ctx context.Context, name string) error {
	err := s.service.DeleteResource(ctx, name)
	return trace.Wrap(err)
}

// DeleteAllWorkloadClusters removes all WorkloadClusters.
func (s *WorkloadClusterService) DeleteAllWorkloadClusters(ctx context.Context) error {
	err := s.service.DeleteAllResources(ctx)
	return trace.Wrap(err)
}

func newWorkloadClusterParser() *workloadClusterParser {
	return &workloadClusterParser{
		baseParser: newBaseParser(backend.NewKey(workloadClusterPrefix)),
	}
}

type workloadClusterParser struct {
	baseParser
}

func (p *workloadClusterParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(workloadClusterPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}

		return &types.ResourceHeader{
			Kind:    types.KindWorkloadCluster,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      strings.TrimPrefix(name, backend.SeparatorString),
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		r, err := services.UnmarshalWorkloadCluster(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
