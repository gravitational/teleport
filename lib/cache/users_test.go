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

	"github.com/gravitational/teleport/api/types"
)

// TestUsers tests caching of users
func TestUsers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.User]{
		newResource: func(name string) (types.User, error) {
			return types.NewUser("bob")
		},
		create: func(ctx context.Context, user types.User) error {
			_, err := p.usersS.UpsertUser(ctx, user)
			return err
		},
		list: func(ctx context.Context) ([]types.User, error) {
			return p.usersS.GetUsers(ctx, false)
		},
		cacheList: func(ctx context.Context) ([]types.User, error) {
			return p.cache.GetUsers(ctx, false)
		},
		update: func(ctx context.Context, user types.User) error {
			_, err := p.usersS.UpdateUser(ctx, user)
			return err
		},
		deleteAll: func(ctx context.Context) error {
			return p.usersS.DeleteAllUsers(ctx)
		},
	})
}
