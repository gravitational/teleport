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

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const AccessMonitoringRulesPrefix = "accessMonitoringRules"

// AccessMonitoringRulesService manages AccessMonitoringRules in the backend.
type AccessMonitoringRulesService struct {
	backend backend.Backend
}

// NewAccessMonitoringRulesService constructs a new AccessMonitoringRulesService
func NewAccessMonitoringRulesService(backend backend.Backend) *AccessMonitoringRulesService {
	return &AccessMonitoringRulesService{backend: backend}
}

// CreateAccessMonitoringRule implements services.AccessMonitoringRules
func (s *AccessMonitoringRulesService) CreateAccessMonitoringRule(ctx context.Context, AccessMonitoringRule types.AccessMonitoringRule) (types.AccessMonitoringRule, error) {
	value, err := services.MarshalAccessMonitoringRule(AccessMonitoringRule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(AccessMonitoringRulesPrefix, AccessMonitoringRule.GetMetadata().Name),
		Value:   value,
		Expires: *AccessMonitoringRule.GetMetadata().Expires,
		ID:      AccessMonitoringRule.GetMetadata().ID,
	}
	_, err = s.backend.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return AccessMonitoringRule, nil
}

// UpdateAccessMonitoringRule implements services.AccessMonitoringRules
func (s *AccessMonitoringRulesService) UpdateAccessMonitoringRule(ctx context.Context, AccessMonitoringRule types.AccessMonitoringRule) (types.AccessMonitoringRule, error) {
	value, err := services.MarshalAccessMonitoringRule(AccessMonitoringRule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(AccessMonitoringRulesPrefix, AccessMonitoringRule.GetMetadata().Name),
		Value:   value,
		Expires: *AccessMonitoringRule.GetMetadata().Expires,
		ID:      AccessMonitoringRule.GetMetadata().ID,
	}
	_, err = s.backend.Update(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return AccessMonitoringRule, nil
}

// UpsertAccessMonitoringRule implements services.AccessMonitoringRules
func (s *AccessMonitoringRulesService) UpsertAccessMonitoringRule(ctx context.Context, AccessMonitoringRule types.AccessMonitoringRule) (types.AccessMonitoringRule, error) {
	value, err := services.MarshalAccessMonitoringRule(AccessMonitoringRule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(AccessMonitoringRulesPrefix, AccessMonitoringRule.GetMetadata().Name),
		Value:   value,
		Expires: *AccessMonitoringRule.GetMetadata().Expires,
		ID:      AccessMonitoringRule.GetMetadata().ID,
	}
	_, err = s.backend.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return AccessMonitoringRule, nil
}

// DeleteAccessMonitoringRule implements service.AccessMonitoringRules
func (s *AccessMonitoringRulesService) DeleteAccessMonitoringRule(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, backend.Key(AccessMonitoringRulesPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("AccessMonitoringRule %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetAccessMonitoringRule implements services.AccessMonitoringRules
func (s *AccessMonitoringRulesService) GetAccessMonitoringRule(ctx context.Context, name string) (types.AccessMonitoringRule, error) {
	item, err := s.backend.Get(ctx, backend.Key(AccessMonitoringRulesPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("AccessMonitoringRule %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}

	AccessMonitoringRule, err := services.UnmarshalAccessMonitoringRule(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return AccessMonitoringRule, nil
}

// GetAccessMonitoringRules implements services.AccessMonitoringRules
func (s *AccessMonitoringRulesService) GetAccessMonitoringRules(ctx context.Context) ([]types.AccessMonitoringRule, error) {
	const pageSize = apidefaults.DefaultChunkSize
	var results []types.AccessMonitoringRule

	var page []types.AccessMonitoringRule
	var startKey string
	var err error
	for {
		page, startKey, err = s.ListAccessMonitoringRules(ctx, pageSize, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results = append(results, page...)
		if startKey == "" {
			break
		}
	}

	return results, nil
}

// ListAccessMonitoringRules returns a paginated list of AccessMonitoringRule instances.
// StartKey is a resource name, which is the suffix of its key.
func (s *AccessMonitoringRulesService) ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]types.AccessMonitoringRule, string, error) {
	// Get at most limit+1 results to determine if there will be a next key.
	maxLimit := limit + 1

	startKeyBytes := backend.Key(AccessMonitoringRulesPrefix, startKey)
	endKey := backend.RangeEnd(backend.ExactKey(AccessMonitoringRulesPrefix))
	result, err := s.backend.GetRange(ctx, startKeyBytes, endKey, maxLimit)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	AccessMonitoringRules := make([]types.AccessMonitoringRule, 0, len(result.Items))
	for _, item := range result.Items {
		AccessMonitoringRule, err := services.UnmarshalAccessMonitoringRule(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		AccessMonitoringRules = append(AccessMonitoringRules, AccessMonitoringRule)
	}

	var nextKey string
	if len(AccessMonitoringRules) == maxLimit {
		nextKey = backend.GetPaginationKey(AccessMonitoringRules[len(AccessMonitoringRules)-1])
		AccessMonitoringRules = AccessMonitoringRules[:limit]
	}

	return AccessMonitoringRules, nextKey, nil
}

func (s *AccessMonitoringRulesService) updateAndSwap(ctx context.Context, name string, modify func(types.AccessMonitoringRule) error) error {
	key := backend.Key(AccessMonitoringRulesPrefix, name)
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("AccessMonitoringRule %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}

	AccessMonitoringRule, err := services.UnmarshalAccessMonitoringRule(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return trace.Wrap(err)
	}

	newAccessMonitoringRule := AccessMonitoringRule.Clone()

	err = modify(newAccessMonitoringRule)
	if err != nil {
		return trace.Wrap(err)
	}

	rev := newAccessMonitoringRule.GetRevision()
	value, err := services.MarshalAccessMonitoringRule(newAccessMonitoringRule)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.CompareAndSwap(ctx, *item, backend.Item{
		Key:      backend.Key(AccessMonitoringRulesPrefix, AccessMonitoringRule.GetName()),
		Value:    value,
		Expires:  AccessMonitoringRule.Expiry(),
		ID:       AccessMonitoringRule.GetResourceID(),
		Revision: rev,
	})

	return trace.Wrap(err)
}

var _ services.AccessMonitoringRules = (*AccessMonitoringRulesService)(nil)
