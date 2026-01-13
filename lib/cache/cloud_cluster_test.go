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

	cloudclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
)

// TestCloudClusters tests that CRUD operations on cloud clusters resources are
// replicated from the backend to the cache.
func TestCloudClusters(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*cloudclusterv1.CloudCluster]{
		newResource: func(name string) (*cloudclusterv1.CloudCluster, error) {
			return newCloudCluster(t, name), nil
		},
		create: func(ctx context.Context, item *cloudclusterv1.CloudCluster) error {
			_, err := p.cloudClusters.CreateCloudCluster(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*cloudclusterv1.CloudCluster, string, error) {
			return p.cloudClusters.ListCloudClusters(ctx, pageSize, pageToken)
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*cloudclusterv1.CloudCluster, string, error) {
			return p.cloudClusters.ListCloudClusters(ctx, pageSize, pageToken)
		},
		deleteAll: p.cloudClusters.DeleteAllCloudClusters,
	})
}
