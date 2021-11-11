package local

import (
	"bytes"
	"context"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// PluginDataService manages PluginData
type PluginDataService struct {
	backend.Backend
}

// NewPluginDataService creates a new PluginDataService
func NewPluginDataService(backend backend.Backend) *PluginDataService {
	return &PluginDataService{backend}
}

// GetPluginData loads all plugin data matching the supplied filter.
func (s *PluginDataService) GetPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error) {
	data, err := s.getPluginData(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// getPluginData returns an array of PluginData matching filter criteria
func (s *PluginDataService) getPluginData(ctx context.Context, filter types.PluginDataFilter) ([]types.PluginData, error) {
	// Filters which specify Resource are a special case since they will match exactly zero or one
	// possible PluginData instances.
	if filter.Resource != "" {
		item, err := s.Get(ctx, pluginDataKey(filter.Kind, filter.Resource))
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

	prefix := backend.Key(pluginDataPrefix, filter.Kind)
	result, err := s.GetRange(ctx, prefix, backend.RangeEnd(prefix), backend.NoLimit)
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

// UpsertsPluginData upserts a per-resource PluginData entry.
func (s *PluginDataService) UpsertPluginData(ctx context.Context, params types.PluginDataUpdateParams, parentExpiresAt *time.Time) error {
	return trace.Wrap(s.upsertPluginData(ctx, params, parentExpiresAt))
}

// updatePluginData updates or creates PluginData using CAS
func (s *PluginDataService) upsertPluginData(ctx context.Context, params types.PluginDataUpdateParams, parentExpiresAt *time.Time) error {
	retryPeriod := retryPeriodMs * time.Millisecond
	retry, err := utils.NewLinear(utils.LinearConfig{
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
		item, err := s.Get(ctx, pluginDataKey(params.Kind, params.Resource))
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
			// In order to prevent orphaned plugin data, we automatically
			// configure new instances to expire shortly after the resource
			// to which they are associated.  This discrepency in expiry gives
			// plugins the ability to use stored data when handling an expiry
			// (OpDelete) event.
			if parentExpiresAt == nil {
				return trace.BadParameter("missing parent resource %q", params.Kind)
			}
			exp := *parentExpiresAt

			data, err = types.NewPluginData(params.Resource, params.Kind)
			if err != nil {
				return trace.Wrap(err)
			}

			// A resource can have empty expiration time by default.
			// In this case, we should leave PluginData expiration time empty as well.
			// TODO: PluginData must be destroyed with parent resource. This needs to be addressed in future.
			if !exp.IsZero() {
				data.SetExpiry(exp.Add(time.Hour))
			}
			create = true
		}
		if err := data.Update(params); err != nil {
			return trace.Wrap(err)
		}
		if err := data.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		newItem, err := itemFromPluginData(data)
		if err != nil {
			return trace.Wrap(err)
		}
		if create {
			if _, err := s.Create(ctx, newItem); err != nil {
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
			if _, err := s.CompareAndSwap(ctx, *item, newItem); err != nil {
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

// pluginDataKey constructs PluginData entry backend key
func pluginDataKey(kind string, name string) []byte {
	return backend.Key(pluginDataPrefix, kind, name, paramsPrefix)
}

func itemFromPluginData(data types.PluginData) (backend.Item, error) {
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
		Key:     pluginDataKey(data.GetSubKind(), data.GetName()),
		Value:   value,
		Expires: data.Expiry(),
		ID:      data.GetResourceID(),
	}, nil
}

func itemToPluginData(item backend.Item) (types.PluginData, error) {
	data, err := services.UnmarshalPluginData(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

const (
	pluginDataPrefix = "plugin_data" // PluginData storage prefix
	maxCmpAttempts   = 7             // Max CAS attempts on plugin save
	retryPeriodMs    = 2048          // CAS retry period
)
