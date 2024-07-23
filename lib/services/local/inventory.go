/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// GetInstances iterates all teleport instances.
func (s *PresenceService) GetInstances(ctx context.Context, req types.InstanceFilter) stream.Stream[types.Instance] {
	const pageSize = 1_000
	if req.ServerID != "" {
		instance, err := s.getInstance(ctx, req.ServerID)
		if err != nil {
			if trace.IsNotFound(err) {
				return stream.Empty[types.Instance]()
			}
			return stream.Fail[types.Instance](trace.Wrap(err))
		}
		if !req.Match(instance) {
			return stream.Empty[types.Instance]()
		}
		return stream.Once(instance)
	}

	startKey := backend.ExactKey(instancePrefix)
	endKey := backend.RangeEnd(startKey)
	items := backend.StreamRange(ctx, s, startKey, endKey, pageSize)
	return stream.FilterMap(items, func(item backend.Item) (types.Instance, bool) {
		instance, err := generic.FastUnmarshal[*types.InstanceV1](item)
		if err != nil {
			s.log.Warnf("Skipping instance at %s, failed to unmarshal: %v", item.Key, err)
			return nil, false
		}
		if err := instance.CheckAndSetDefaults(); err != nil {
			s.log.Warnf("Skipping instance at %s: %v", item.Key, err)
			return nil, false
		}
		if !req.Match(instance) {
			return nil, false
		}
		return instance, true
	})
}

// getInstance gets an instance resource by server ID.
func (s *PresenceService) getInstance(ctx context.Context, serverID string) (types.Instance, error) {
	item, err := s.Get(ctx, backend.Key(instancePrefix, serverID))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	instance, err := generic.FastUnmarshal[*types.InstanceV1](*item)
	if err != nil {
		return nil, trace.BadParameter("failed to unmarshal instance %q: %v", serverID, err)
	}

	if err := instance.CheckAndSetDefaults(); err != nil {
		return nil, trace.BadParameter("instance %q appears malformed: %v", serverID, err)
	}

	return instance, nil
}

// UpsertInstance creates or updates an instance resource.
func (s *PresenceService) UpsertInstance(ctx context.Context, instance types.Instance) error {
	if err := instance.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// instance resource expiry is calculated relative to LastSeen and/or the longest living
	// control log entry (whichever is further in the future).
	if instance.GetLastSeen().IsZero() || instance.Expiry().IsZero() {
		instance.SetLastSeen(s.Clock().Now().UTC())
		instance.SyncLogAndResourceExpiry(apidefaults.ServerAnnounceTTL)
	}

	v1, ok := instance.(*types.InstanceV1)
	if !ok {
		return trace.BadParameter("unexpected type %T, expected %T", instance, v1)
	}

	item, err := generic.FastMarshal(backend.Key(instancePrefix, instance.GetName()), v1)
	if err != nil {
		return trace.Errorf("failed to marshal Instance: %v", err)
	}

	_, err = s.Backend.Put(ctx, item)

	return trace.Wrap(err)
}

const instancePrefix = "instances"
