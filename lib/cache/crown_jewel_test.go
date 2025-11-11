// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
)

// TestCrownJewel tests that CRUD operations on user notification resources are
// replicated from the backend to the cache.
func TestCrownJewel(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*crownjewelv1.CrownJewel]{
		newResource: func(name string) (*crownjewelv1.CrownJewel, error) {
			return newCrownJewel(t, name), nil
		},
		create: func(ctx context.Context, item *crownjewelv1.CrownJewel) error {
			_, err := p.crownJewels.CreateCrownJewel(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*crownjewelv1.CrownJewel, string, error) {
			return p.crownJewels.ListCrownJewels(ctx, int64(pageSize), pageToken)
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]*crownjewelv1.CrownJewel, string, error) {
			return p.crownJewels.ListCrownJewels(ctx, int64(pageSize), pageToken)
		},
		deleteAll: p.crownJewels.DeleteAllCrownJewels,
	}, withSkipPaginationTest())
}
