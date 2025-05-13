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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/services"
)

type userLoginStateIndex string

const userLoginStateNameIndex userLoginStateIndex = "name"

func newUserLoginStateCollection(upstream services.UserLoginStates, w types.WatchKind) (*collection[*userloginstate.UserLoginState, userLoginStateIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter UserTasks")
	}

	return &collection[*userloginstate.UserLoginState, userLoginStateIndex]{
		store: newStore(
			(*userloginstate.UserLoginState).Clone,
			map[userLoginStateIndex]func(*userloginstate.UserLoginState) string{
				userLoginStateNameIndex: func(r *userloginstate.UserLoginState) string {
					return r.GetMetadata().Name
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*userloginstate.UserLoginState, error) {
			out, err := upstream.GetUserLoginStates(ctx)
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *userloginstate.UserLoginState {
			return &userloginstate.UserLoginState{
				ResourceHeader: header.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: header.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// GetUserLoginStates returns the all user login state resources.
func (c *Cache) GetUserLoginStates(ctx context.Context) ([]*userloginstate.UserLoginState, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUserLoginStates")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.userLoginStates)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		states, err := c.Config.UserLoginStates.GetUserLoginStates(ctx)
		return states, trace.Wrap(err)
	}

	states := make([]*userloginstate.UserLoginState, 0, rg.store.len())
	for uls := range rg.store.resources(userLoginStateNameIndex, "", "") {
		states = append(states, uls.Clone())
	}

	return states, nil
}

// GetUserLoginState returns the specified user login state resource.
func (c *Cache) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUserLoginState")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[*userloginstate.UserLoginState, userLoginStateIndex]{
		cache:      c,
		collection: c.collections.userLoginStates,
		index:      userLoginStateNameIndex,
		upstreamGet: func(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
			upstreamRead = true
			state, err := c.Config.UserLoginStates.GetUserLoginState(ctx, name)
			return state, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, name)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if uls, err := c.Config.UserLoginStates.GetUserLoginState(ctx, name); err == nil {
			return uls, nil
		}
	}
	return out, trace.Wrap(err)
}
