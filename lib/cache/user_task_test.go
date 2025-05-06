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

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
)

// TestUserTasks tests that CRUD operations on user notification resources are
// replicated from the backend to the cache.
func TestUserTasks(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*usertasksv1.UserTask]{
		newResource: func(name string) (*usertasksv1.UserTask, error) {
			return newUserTasks(t), nil
		},
		create: func(ctx context.Context, item *usertasksv1.UserTask) error {
			_, err := p.userTasks.CreateUserTask(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*usertasksv1.UserTask, error) {
			items, _, err := p.userTasks.ListUserTasks(ctx, 0, "", &usertasksv1.ListUserTasksFilters{})
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*usertasksv1.UserTask, error) {
			items, _, err := p.cache.ListUserTasks(ctx, 0, "", &usertasksv1.ListUserTasksFilters{})
			return items, trace.Wrap(err)
		},
		deleteAll: p.userTasks.DeleteAllUserTasks,
	})
}
