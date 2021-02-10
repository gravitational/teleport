/*
Copyright 2019 Gravitational, Inc.

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
	"bytes"
	"context"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// DynamicAccessService manages dynamic RBAC
type DynamicAccessService struct {
	backend.Backend
}

// NewDynamicAccessService returns new dynamic access service instance
func NewDynamicAccessService(backend backend.Backend) *DynamicAccessService {
	return &DynamicAccessService{Backend: backend}
}

// CreateAccessRequest stores a new access request.
func (s *DynamicAccessService) CreateAccessRequest(ctx context.Context, req services.AccessRequest) error {
	if err := auth.ValidateAccessRequest(req); err != nil {
		return trace.Wrap(err)
	}
	item, err := itemFromAccessRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := s.Create(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SetAccessRequestState updates the state of an existing access request.
func (s *DynamicAccessService) SetAccessRequestState(ctx context.Context, params services.AccessRequestUpdate) error {
	if err := params.Check(); err != nil {
		return trace.Wrap(err)
	}
	retryPeriod := retryPeriodMs * time.Millisecond
	retry, err := utils.NewLinear(utils.LinearConfig{
		Step: retryPeriod / 7,
		Max:  retryPeriod,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// Setting state is attempted multiple times in the event of concurrent writes.
	// The reason we bother to re-attempt is because state updates aren't meant
	// to be "first come first serve".  Denials should overwrite approvals, but
	// approvals should not overwrite denials.
	for i := 0; i < maxCmpAttempts; i++ {
		item, err := s.Get(ctx, accessRequestKey(params.RequestID))
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.NotFound("cannot set state of access request %q (not found)", params.RequestID)
			}
			return trace.Wrap(err)
		}
		req, err := itemToAccessRequest(*item)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := req.SetState(params.State); err != nil {
			return trace.Wrap(err)
		}
		req.SetResolveReason(params.Reason)
		req.SetResolveAnnotations(params.Annotations)
		if len(params.Roles) > 0 {
			for _, role := range params.Roles {
				if !utils.SliceContainsStr(req.GetRoles(), role) {
					return trace.BadParameter("role %q not in original request, overrides must be a subset of original role list", role)
				}
			}
			req.SetRoles(params.Roles)
		}

		// approved requests should have a resource expiry which matches
		// the underlying access expiry.
		if params.State.IsApproved() {
			req.SetExpiry(req.GetAccessExpiry())
		}
		newItem, err := itemFromAccessRequest(req)
		if err != nil {
			return trace.Wrap(err)
		}
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
		return nil
	}
	return trace.CompareFailed("too many concurrent writes to access request %s, try again later", params.RequestID)
}

// ApplyAccessReview applies a review to a request and returns the post-application state.
func (s *DynamicAccessService) ApplyAccessReview(ctx context.Context, params types.AccessReviewSubmission, checker auth.ReviewPermissionChecker) (services.AccessRequest, error) {
	if err := params.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	retryPeriod := retryPeriodMs * time.Millisecond
	retry, err := utils.NewLinear(utils.LinearConfig{
		Step: retryPeriod / 7,
		Max:  retryPeriod,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Review application is attempted multiple times in the event of concurrent writes.
	for i := 0; i < maxCmpAttempts; i++ {
		item, err := s.Get(ctx, accessRequestKey(params.RequestID))
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("cannot apply review to access request %q (not found)", params.RequestID)
			}
			return nil, trace.Wrap(err)
		}
		req, err := itemToAccessRequest(*item)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// verify review permissions against request details
		if ok, err := checker.CanReviewRequest(req); err != nil || !ok {
			if err == nil {
				err = trace.AccessDenied("user %q cannot review request %q", params.Review.Author, params.RequestID)
			}
			return nil, trace.Wrap(err)
		}

		// run the application logic
		if err := auth.ApplyAccessReview(req, params.Review, checker.User); err != nil {
			return nil, trace.Wrap(err)
		}

		newItem, err := itemFromAccessRequest(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if _, err := s.CompareAndSwap(ctx, *item, newItem); err != nil {
			if trace.IsCompareFailed(err) {
				select {
				case <-retry.After():
					retry.Inc()
					continue
				case <-ctx.Done():
					return nil, trace.Wrap(ctx.Err())
				}
			}
			return nil, trace.Wrap(err)
		}
		return req, nil
	}
	return nil, trace.CompareFailed("too many concurrent writes to access request %s, try again later", params.RequestID)
}

func (s *DynamicAccessService) GetAccessRequest(ctx context.Context, name string) (services.AccessRequest, error) {
	item, err := s.Get(ctx, accessRequestKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("access request %q not found", name)
		}
		return nil, trace.Wrap(err)
	}
	req, err := itemToAccessRequest(*item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// GetAccessRequests gets all currently active access requests.
func (s *DynamicAccessService) GetAccessRequests(ctx context.Context, filter services.AccessRequestFilter) ([]services.AccessRequest, error) {
	// Filters which specify ID are a special case since they will match exactly zero or one
	// possible requests.
	if filter.ID != "" {
		req, err := s.GetAccessRequest(ctx, filter.ID)
		if err != nil {
			// A filter with zero matches is still a success, it just
			// happens to return an empty slice.
			if trace.IsNotFound(err) {
				return nil, nil
			}
			return nil, trace.Wrap(err)
		}
		if !filter.Match(req) {
			// A filter with zero matches is still a success, it just
			// happens to return an empty slice.
			return nil, nil
		}
		return []services.AccessRequest{req}, nil
	}
	result, err := s.GetRange(ctx, backend.Key(accessRequestsPrefix), backend.RangeEnd(backend.Key(accessRequestsPrefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var requests []services.AccessRequest
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			// Item represents a different resource type in the
			// same namespace.
			continue
		}
		req, err := itemToAccessRequest(item, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !filter.Match(req) {
			continue
		}
		requests = append(requests, req)
	}
	return requests, nil
}

// DeleteAccessRequest deletes an access request.
func (s *DynamicAccessService) DeleteAccessRequest(ctx context.Context, name string) error {
	err := s.Delete(ctx, accessRequestKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("cannot delete access request %q (not found)", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (s *DynamicAccessService) DeleteAllAccessRequests(ctx context.Context) error {
	return trace.Wrap(s.DeleteRange(ctx, backend.Key(accessRequestsPrefix), backend.RangeEnd(backend.Key(accessRequestsPrefix))))
}

func (s *DynamicAccessService) UpsertAccessRequest(ctx context.Context, req services.AccessRequest) error {
	if err := auth.ValidateAccessRequest(req); err != nil {
		return trace.Wrap(err)
	}
	item, err := itemFromAccessRequest(req)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPluginData loads all plugin data matching the supplied filter.
func (s *DynamicAccessService) GetPluginData(ctx context.Context, filter services.PluginDataFilter) ([]services.PluginData, error) {
	switch filter.Kind {
	case services.KindAccessRequest:
		data, err := s.getAccessRequestPluginData(ctx, filter)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return data, nil
	default:
		return nil, trace.BadParameter("unsupported resource kind %q", filter.Kind)
	}
}

func (s *DynamicAccessService) getAccessRequestPluginData(ctx context.Context, filter services.PluginDataFilter) ([]services.PluginData, error) {
	// Filters which specify Resource are a special case since they will match exactly zero or one
	// possible PluginData instances.
	if filter.Resource != "" {
		item, err := s.Get(ctx, pluginDataKey(services.KindAccessRequest, filter.Resource))
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
		return []services.PluginData{data}, nil
	}
	prefix := backend.Key(pluginDataPrefix, services.KindAccessRequest)
	result, err := s.GetRange(ctx, prefix, backend.RangeEnd(prefix), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var matches []services.PluginData
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
func (s *DynamicAccessService) UpdatePluginData(ctx context.Context, params services.PluginDataUpdateParams) error {
	switch params.Kind {
	case services.KindAccessRequest:
		return trace.Wrap(s.updateAccessRequestPluginData(ctx, params))
	default:
		return trace.BadParameter("unsupported resource kind %q", params.Kind)
	}
}

func (s *DynamicAccessService) updateAccessRequestPluginData(ctx context.Context, params services.PluginDataUpdateParams) error {
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
		var data services.PluginData
		item, err := s.Get(ctx, pluginDataKey(services.KindAccessRequest, params.Resource))
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
			// configure new instances to expire shortly after the AccessRequest
			// to which they are associated.  This discrepency in expiry gives
			// plugins the ability to use stored data when handling an expiry
			// (OpDelete) event.
			req, err := s.GetAccessRequest(ctx, params.Resource)
			if err != nil {
				return trace.Wrap(err)
			}
			data, err = services.NewPluginData(params.Resource, services.KindAccessRequest)
			if err != nil {
				return trace.Wrap(err)
			}
			data.SetExpiry(req.GetAccessExpiry().Add(time.Hour))
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

func itemFromAccessRequest(req services.AccessRequest) (backend.Item, error) {
	value, err := resource.MarshalAccessRequest(req)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	return backend.Item{
		Key:     accessRequestKey(req.GetName()),
		Value:   value,
		Expires: req.Expiry(),
		ID:      req.GetResourceID(),
	}, nil
}

func itemToAccessRequest(item backend.Item, opts ...auth.MarshalOption) (services.AccessRequest, error) {
	opts = append(
		opts,
		resource.WithResourceID(item.ID),
		resource.WithExpires(item.Expires),
	)
	req, err := resource.UnmarshalAccessRequest(
		item.Value,
		opts...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

func itemFromPluginData(data services.PluginData) (backend.Item, error) {
	value, err := resource.MarshalPluginData(data)
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

func itemToPluginData(item backend.Item) (services.PluginData, error) {
	data, err := resource.UnmarshalPluginData(
		item.Value,
		resource.WithResourceID(item.ID),
		resource.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

func accessRequestKey(name string) []byte {
	return backend.Key(accessRequestsPrefix, name, paramsPrefix)
}

func pluginDataKey(kind string, name string) []byte {
	return backend.Key(pluginDataPrefix, kind, name, paramsPrefix)
}

const (
	accessRequestsPrefix = "access_requests"
	pluginDataPrefix     = "plugin_data"
	maxCmpAttempts       = 7
	retryPeriodMs        = 2048
)
