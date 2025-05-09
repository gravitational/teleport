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

	"github.com/gravitational/teleport/api/types/userloginstate"
)

// TestUserLoginStates tests that CRUD operations on user login state resources are
// replicated from the backend to the cache.
func TestUserLoginStates(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*userloginstate.UserLoginState]{
		newResource: func(name string) (*userloginstate.UserLoginState, error) {
			return newUserLoginState(t, name), nil
		},
		create: func(ctx context.Context, uls *userloginstate.UserLoginState) error {
			_, err := p.userLoginStates.UpsertUserLoginState(ctx, uls)
			return trace.Wrap(err)
		},
		list:      p.userLoginStates.GetUserLoginStates,
		cacheGet:  p.cache.GetUserLoginState,
		cacheList: p.cache.GetUserLoginStates,
		update: func(ctx context.Context, uls *userloginstate.UserLoginState) error {
			_, err := p.userLoginStates.UpsertUserLoginState(ctx, uls)
			return trace.Wrap(err)
		},
		deleteAll: p.userLoginStates.DeleteAllUserLoginStates,
	})
}
