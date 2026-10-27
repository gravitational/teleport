/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"iter"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	apistream "github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	libscopes "github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// GetInstances iterates all teleport instances.
func (s *PresenceService) GetInstances(ctx context.Context, req types.InstanceFilter) apistream.Stream[types.Instance] {
	filter := req.ScopeFilter.ToProto()
	if err := libscopes.ValidateFilter(filter); err != nil {
		return apistream.Fail[types.Instance](trace.Wrap(err))
	}

	// When a server id is specified a point read may be performed IFF the request is for an
	// exact scope or an unscoped instance. The backend will search for the instance at
	// the appropriate backend key based on the scope in the filter.
	var instances iter.Seq2[types.Instance, error]
	if req.ServerID != "" && (filter.GetMode() == scopesv1.Mode_MODE_UNSCOPED || filter.GetMode() == scopesv1.Mode_MODE_EXACT) {
		instances = s.getInstance(ctx, scopes.QualifiedName{Name: req.ServerID, Scope: filter.GetScope()})
	} else {
		instances = s.instances.Resources(ctx, "", "")
	}

	return stream.IntoLegacy(stream.FilterMap(
		instances,
		func(instance types.Instance) (types.Instance, bool) {
			if !libscopes.MatchScope(filter, instance.GetScope()) {
				return nil, false
			}

			if !req.Match(instance) {
				return nil, false
			}

			return instance, true
		},
	))
}

func (s *PresenceService) getInstance(ctx context.Context, sqn scopes.QualifiedName) iter.Seq2[types.Instance, error] {
	return func(yield func(types.Instance, error) bool) {
		instance, err := s.instances.GetResource(ctx, sqn)
		if err != nil {
			if trace.IsNotFound(err) {
				return
			}

			yield(nil, trace.Wrap(err))
			return
		}

		yield(instance, nil)
	}
}

// UpsertInstance creates or updates an instance resource.
func (s *PresenceService) UpsertInstance(ctx context.Context, instance types.Instance) error {
	if err := services.CheckAndSetDefaults(instance); err != nil {
		return trace.Wrap(err)
	}

	// instance resource expiry is calculated relative to LastSeen and/or the longest living
	// control log entry (whichever is further in the future).
	if instance.GetLastSeen().IsZero() || instance.Expiry().IsZero() {
		instance.SetLastSeen(s.Clock().Now().UTC())
		instance.SyncLogAndResourceExpiry(apidefaults.ServerAnnounceTTL)
	}

	_, err := s.instances.UpsertResource(ctx, instance)
	return trace.Wrap(err)
}

const instancePrefix = "instances"
