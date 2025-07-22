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

	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
)

// TestStaticHostUsers tests that CRUD operations on static host user resources are
// replicated from the backend to the cache.
func TestStaticHostUsers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*userprovisioningv2.StaticHostUser]{
		newResource: func(name string) (*userprovisioningv2.StaticHostUser, error) {
			return newStaticHostUser(t, name), nil
		},
		create: func(ctx context.Context, item *userprovisioningv2.StaticHostUser) error {
			_, err := p.staticHostUsers.CreateStaticHostUser(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*userprovisioningv2.StaticHostUser, error) {
			items, _, err := p.staticHostUsers.ListStaticHostUsers(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*userprovisioningv2.StaticHostUser, error) {
			items, _, err := p.cache.ListStaticHostUsers(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheGet:  p.cache.GetStaticHostUser,
		deleteAll: p.staticHostUsers.DeleteAllStaticHostUsers,
	})
}
