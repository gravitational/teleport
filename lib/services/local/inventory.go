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
	"github.com/gravitational/teleport/lib/utils"
)

// GetInstances iterates all teleport instances.
func (s *PresenceService) GetInstances(ctx context.Context, req types.InstanceFilter) stream.Stream[types.Instance] {
	const pageSize = 10_000
	if req.ServerID != "" {
		instance, _, err := s.GetRawInstance(ctx, req.ServerID)
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

	startKey := backend.Key(instancePrefix, "")
	endKey := backend.RangeEnd(startKey)
	items := backend.StreamRange(ctx, s, startKey, endKey, pageSize)
	return stream.FilterMap(items, func(item backend.Item) (types.Instance, bool) {
		var instance types.InstanceV1
		if err := utils.FastUnmarshal(item.Value, &instance); err != nil {
			s.log.Warnf("Skipping instance at %s, failed to unmarshal: %v", item.Key, err)
			return nil, false
		}
		if err := instance.CheckAndSetDefaults(); err != nil {
			s.log.Warnf("Skipping instance at %s: %v", item.Key, err)
			return nil, false
		}
		if !req.Match(&instance) {
			return nil, false
		}
		return &instance, true
	})
}

// GetRawInstance gets an instance resource by server ID.
func (s *PresenceService) GetRawInstance(ctx context.Context, serverID string) (types.Instance, []byte, error) {
	item, err := s.Get(ctx, backend.Key(instancePrefix, serverID))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, nil, trace.NotFound("failed to locate instance %q", serverID)
		}
		return nil, nil, trace.Wrap(err)
	}

	var instance types.InstanceV1
	if err := utils.FastUnmarshal(item.Value, &instance); err != nil {
		return nil, nil, trace.BadParameter("failed to unmarshal instance %q: %v", serverID, err)
	}

	if err := instance.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.BadParameter("instance %q appears malformed: %v", serverID, err)
	}

	return &instance, item.Value, nil
}

// CompareAndSwapInstance creates or updates the underlying instance resource based on the currently
// expected value. The first call to this method should use the value returned by GetRawInstance for the
// 'expect' parameter. Subsequent calls should use the value returned by this method.
func (s *PresenceService) CompareAndSwapInstance(ctx context.Context, instance types.Instance, expect []byte) ([]byte, error) {
	if err := instance.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// instance resource expiry is calculated relative to LastSeen and/or the longest living
	// control log entry (whichever is further in the future).
	if instance.GetLastSeen().IsZero() || instance.Expiry().IsZero() {
		instance.SetLastSeen(s.Clock().Now().UTC())
		instance.SyncLogAndResourceExpiry(apidefaults.ServerAnnounceTTL)
	}

	v1, ok := instance.(*types.InstanceV1)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T, expected %T", instance, v1)
	}

	value, err := utils.FastMarshal(v1)
	if err != nil {
		return nil, trace.Errorf("failed to marshal Instance: %v", err)
	}

	item := backend.Item{
		Key:     backend.Key(instancePrefix, instance.GetName()),
		Value:   value,
		Expires: instance.Expiry(),
	}

	if len(expect) == 0 {
		// empty 'expect' means we expect nonexistence, so we use Create instead of
		// the regular CompareAndSwap.
		_, err = s.Backend.Create(ctx, item)
		if err != nil {
			if trace.IsAlreadyExists(err) {
				return nil, trace.CompareFailed("instance concurrently created")
			}
			return nil, trace.Wrap(err)
		}
		return value, nil
	}

	_, err = s.Backend.CompareAndSwap(ctx, backend.Item{
		Key:   item.Key,
		Value: expect,
	}, item)

	if err != nil {
		if trace.IsCompareFailed(err) {
			return nil, trace.CompareFailed("instance concurrently updated")
		}
		return nil, trace.Wrap(err)
	}

	return value, nil
}

const instancePrefix = "instances"
