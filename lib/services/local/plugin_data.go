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
	"bytes"
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	twoWeeks = 24 * 14 * time.Hour
)

// PluginDataService is the backend service for plugin data.
type PluginDataService struct {
	backend.Backend

	dynamicAccess services.DynamicAccessCore
}

// NewPluginData creates a new plugin data service.
func NewPluginData(backend backend.Backend, dynamicAccess services.DynamicAccessCore) *PluginDataService {
	return &PluginDataService{
		Backend:       backend,
		dynamicAccess: dynamicAccess,
	}
}

// GetPluginData loads all plugin data matching the supplied filter.
func (p *PluginDataService) GetPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error) {
	switch filter.Kind {
	case types.KindAccessRequest, types.KindAccessList:
		data, err := p.getPluginData(ctx, filter)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return data, nil
	default:
		return nil, trace.BadParameter("unsupported resource kind %q", filter.Kind)
	}
}

func (p *PluginDataService) getPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error) {
	// Filters which specify Resource are a special case since they will match exactly zero or one
	// possible PluginData instances.
	if filter.Resource != "" {
		item, err := p.Get(ctx, pluginDataKey(filter.Kind, filter.Resource))
		if err != nil {
			// A filter with zero matches is still a success, it just
			// happens to return an empty slice.
			if trace.IsNotFound(err) {
				return nil, nil
			}
			return nil, trace.Wrap(err)
		}
		data, err := itemToPluginData(*item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !filter.Match(data) {
			// A filter with zero matches is still a success, it just
			// happens to return an empty slice.
			return nil, nil
		}
		return []types.PluginData{data}, nil
	}
	prefix := backend.ExactKey(pluginDataPrefix, filter.Kind)
	result, err := p.GetRange(ctx, prefix, backend.RangeEnd(prefix), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var matches []types.PluginData
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			// Item represents a different resource type in the
			// same namespace.
			continue
		}
		data, err := itemToPluginData(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !filter.Match(data) {
			continue
		}
		matches = append(matches, data)
	}
	return matches, nil
}

// UpdatePluginData updates a per-resource PluginData entry.
func (p *PluginDataService) UpdatePluginData(ctx context.Context, params types.PluginDataUpdateParams) error {
	switch params.Kind {
	case types.KindAccessRequest, types.KindAccessList:
	default:
		return trace.BadParameter("unsupported resource kind %q", params.Kind)
	}

	return trace.Wrap(p.updatePluginData(ctx, params))
}

func (p *PluginDataService) updatePluginData(ctx context.Context, params types.PluginDataUpdateParams) error {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step: retryPeriod / 7,
		Max:  retryPeriod,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// Update is attempted multiple times in the event of concurrent writes.
	for i := 0; i < maxCmpAttempts; i++ {
		var create bool
		var data types.PluginData
		item, err := p.Get(ctx, pluginDataKey(params.Kind, params.Resource))
		if err == nil {
			data, err = itemToPluginData(*item)
			if err != nil {
				return trace.Wrap(err)
			}
			create = false
		} else {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}

			data, err = types.NewPluginData(params.Resource, params.Kind)
			if err != nil {
				return trace.Wrap(err)
			}
			create = true

			if params.Kind == types.KindAccessRequest {
				// In order to prevent orphaned plugin data, we automatically
				// configure new instances to expire shortly after the AccessRequest
				// to which they are associated.  This discrepency in expiry gives
				// plugins the ability to use stored data when handling an expiry
				// (OpDelete) event.
				req, err := services.GetAccessRequest(ctx, p.dynamicAccess, params.Resource)
				if err != nil {
					return trace.Wrap(err)
				}
				data.SetExpiry(req.GetAccessExpiry().Add(time.Hour))
			}
		}

		if params.Kind == types.KindAccessList {
			// Expire access list data two weeks from now for every update, which will
			// make sure that at some point it will get cleaned up if an access list no
			// longer needs notifications.
			data.SetExpiry(p.Clock().Now().Add(twoWeeks))
		}

		if err := data.Update(params); err != nil {
			return trace.Wrap(err)
		}
		if err := services.CheckAndSetDefaults(data); err != nil {
			return trace.Wrap(err)
		}
		newItem, err := itemFromPluginData(data)
		if err != nil {
			return trace.Wrap(err)
		}
		if create {
			if _, err := p.Create(ctx, newItem); err != nil {
				if trace.IsAlreadyExists(err) {
					select {
					case <-retry.After():
						retry.Inc()
						continue
					case <-ctx.Done():
						return trace.Wrap(ctx.Err())
					}
				}
				return trace.Wrap(err)
			}
		} else {
			if _, err := p.CompareAndSwap(ctx, *item, newItem); err != nil {
				if trace.IsCompareFailed(err) {
					select {
					case <-retry.After():
						retry.Inc()
						continue
					case <-ctx.Done():
						return trace.Wrap(ctx.Err())
					}
				}
				return trace.Wrap(err)
			}
		}
		return nil
	}
	return trace.CompareFailed("too many concurrent writes to plugin data %s", params.Resource)
}

func itemFromPluginData(data types.PluginData) (backend.Item, error) {
	rev := data.GetRevision()
	value, err := services.MarshalPluginData(data)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	// enforce explicit limit on resource size in order to prevent PluginData from
	// growing uncontrollably.
	if len(value) > teleport.MaxResourceSize {
		return backend.Item{}, trace.BadParameter("plugin data size limit exceeded")
	}
	return backend.Item{
		Key:      pluginDataKey(data.GetSubKind(), data.GetName()),
		Value:    value,
		Expires:  data.Expiry(),
		ID:       data.GetResourceID(),
		Revision: rev,
	}, nil
}

func itemToPluginData(item backend.Item) (types.PluginData, error) {
	data, err := services.UnmarshalPluginData(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

func pluginDataKey(kind string, name string) []byte {
	return backend.Key(pluginDataPrefix, kind, name, paramsPrefix)
}

const (
	pluginDataPrefix = "plugin_data"
)
