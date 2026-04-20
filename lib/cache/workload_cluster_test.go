// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/trace"

	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
)

// TestWorkloadClusters tests that CRUD operations on workload clusters resources are
// replicated from the backend to the cache.
func TestWorkloadClusters(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*workloadclusterv1.WorkloadCluster]{
		newResource: func(name string) (*workloadclusterv1.WorkloadCluster, error) {
			return newWorkloadCluster(t, name), nil
		},
		create: func(ctx context.Context, item *workloadclusterv1.WorkloadCluster) error {
			_, err := p.workloadClusters.CreateWorkloadCluster(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*workloadclusterv1.WorkloadCluster, string, error) {
			return p.workloadClusters.ListWorkloadClusters(ctx, pageSize, pageToken)
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*workloadclusterv1.WorkloadCluster, string, error) {
			return p.workloadClusters.ListWorkloadClusters(ctx, pageSize, pageToken)
		},
		deleteAll: p.workloadClusters.DeleteAllWorkloadClusters,
	})
}
