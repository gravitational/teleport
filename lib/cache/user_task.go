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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type userTaskIndex string

const userTaskNameIndex userTaskIndex = "name"

func newUserTaskCollection(upstream services.UserTasks, w types.WatchKind) (*collection[*usertasksv1.UserTask, userTaskIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter UserTasks")
	}

	return &collection[*usertasksv1.UserTask, userTaskIndex]{
		store: newStore(
			proto.CloneOf[*usertasksv1.UserTask],
			map[userTaskIndex]func(*usertasksv1.UserTask) string{
				userTaskNameIndex: func(r *usertasksv1.UserTask) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*usertasksv1.UserTask, error) {
			var resources []*usertasksv1.UserTask
			var nextToken string
			for {
				var page []*usertasksv1.UserTask
				var err error
				page, nextToken, err = upstream.ListUserTasks(ctx, 0 /* page size */, nextToken, &usertasksv1.ListUserTasksFilters{})
				if err != nil {
					return nil, trace.Wrap(err)
				}
				resources = append(resources, page...)

				if nextToken == "" {
					break
				}
			}
			return resources, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *usertasksv1.UserTask {
			return &usertasksv1.UserTask{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// ListUserTasks returns a list of UserTask resources.
func (c *Cache) ListUserTasks(ctx context.Context, pageSize int64, pageToken string, filters *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListUserTasks")
	defer span.End()

	lister := genericLister[*usertasksv1.UserTask, userTaskIndex]{
		cache:      c,
		collection: c.collections.userTasks,
		index:      userTaskNameIndex,
		upstreamList: func(ctx context.Context, i int, s string) ([]*usertasksv1.UserTask, string, error) {
			out, next, err := c.Config.UserTasks.ListUserTasks(ctx, pageSize, pageToken, filters)
			return out, next, trace.Wrap(err)
		},
		nextToken: func(t *usertasksv1.UserTask) string {
			return t.GetMetadata().Name
		},
		filter: func(ut *usertasksv1.UserTask) bool {
			return services.MatchUserTask(ut, filters)
		},
	}
	out, next, err := lister.list(ctx, int(pageSize), pageToken)
	return out, next, trace.Wrap(err)
}

// GetUserTask returns the specified UserTask resource.
func (c *Cache) GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUserTask")
	defer span.End()

	getter := genericGetter[*usertasksv1.UserTask, userTaskIndex]{
		cache:       c,
		collection:  c.collections.userTasks,
		index:       userTaskNameIndex,
		upstreamGet: c.Config.UserTasks.GetUserTask,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
