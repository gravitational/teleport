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
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
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
func (s *DynamicAccessService) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	_, err := s.CreateAccessRequestV2(ctx, req)
	return trace.Wrap(err)
}

// CreateAccessRequestV2 stores a new access request.
func (s *DynamicAccessService) CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error) {
	if err := services.ValidateAccessRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.GetDryRun() {
		return nil, trace.BadParameter("dry run access request made it to DynamicAccessService, this is a bug")
	}
	item, err := itemFromAccessRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := s.Create(ctx, item); err != nil {
		return nil, trace.Wrap(err)
	}

	return req, nil
}

// SetAccessRequestState updates the state of an existing access request.
func (s *DynamicAccessService) SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) (types.AccessRequest, error) {
	if err := params.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		Step: retryPeriod / 7,
		Max:  retryPeriod,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Setting state is attempted multiple times in the event of concurrent writes.
	// The reason we bother to re-attempt is because state updates aren't meant
	// to be "first come first serve".  Denials should overwrite approvals, but
	// approvals should not overwrite denials.
	for i := 0; i < maxCmpAttempts; i++ {
		item, err := s.Get(ctx, accessRequestKey(params.RequestID))
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("cannot set state of access request %q (not found)", params.RequestID)
			}
			return nil, trace.Wrap(err)
		}
		req, err := itemToAccessRequest(*item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := req.SetState(params.State); err != nil {
			return nil, trace.Wrap(err)
		}
		req.SetResolveReason(params.Reason)
		req.SetResolveAnnotations(params.Annotations)
		if len(params.Roles) > 0 {
			for _, role := range params.Roles {
				if !slices.Contains(req.GetRoles(), role) {
					return nil, trace.BadParameter("role %q not in original request, overrides must be a subset of original role list", role)
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

// ApplyAccessReview applies a review to a request and returns the post-application state.
func (s *DynamicAccessService) ApplyAccessReview(ctx context.Context, params types.AccessReviewSubmission, checker services.ReviewPermissionChecker) (types.AccessRequest, error) {
	if err := params.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
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
		if err := services.ApplyAccessReview(req, params.Review, checker.UserState); err != nil {
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

func (s *DynamicAccessService) GetAccessRequest(ctx context.Context, name string) (types.AccessRequest, error) {
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
func (s *DynamicAccessService) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
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
		return []types.AccessRequest{req}, nil
	}
	startKey := backend.ExactKey(accessRequestsPrefix)
	endKey := backend.RangeEnd(startKey)
	result, err := s.GetRange(ctx, startKey, endKey, backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var requests []types.AccessRequest
	for _, item := range result.Items {
		if !bytes.HasSuffix(item.Key, []byte(paramsPrefix)) {
			// Item represents a different resource type in the
			// same namespace.
			continue
		}
		req, err := itemToAccessRequest(item)
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
	startKey := backend.ExactKey(accessRequestsPrefix)
	endKey := backend.RangeEnd(startKey)
	return trace.Wrap(s.DeleteRange(ctx, startKey, endKey))
}

func (s *DynamicAccessService) UpsertAccessRequest(ctx context.Context, req types.AccessRequest) error {
	if err := services.ValidateAccessRequest(req); err != nil {
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

// CreateAccessRequestAllowedPromotions creates AccessRequestAllowedPromotions object.
func (s *DynamicAccessService) CreateAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest, accessLists *types.AccessRequestAllowedPromotions) error {
	// create the new access request promotion object
	item, err := itemFromAccessListPromotions(req, accessLists)
	if err != nil {
		return trace.Wrap(err)
	}
	// Currently, this logic is used only internally (no API exposed), and
	// there is only one place that calls it. If this ever changes, we will
	// need to do a CompareAndSwap here.
	if _, err := s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAccessRequestAllowedPromotions returns AccessRequestAllowedPromotions object.
func (s *DynamicAccessService) GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	// get the access request promotions from the backend
	item, err := s.Get(ctx, AccessRequestAllowedPromotionKey(req.GetName()))
	if err != nil {
		if trace.IsNotFound(err) {
			// do not return nil as the caller will assume that nil error
			// means that there are some promotions
			return types.NewAccessRequestAllowedPromotions(nil), nil
		}
		return nil, trace.Wrap(err)
	}
	// unmarshal the access request promotions
	promotions, err := services.UnmarshalAccessRequestAllowedPromotion(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return promotions, nil
}

func itemFromAccessRequest(req types.AccessRequest) (backend.Item, error) {
	rev := req.GetRevision()
	value, err := services.MarshalAccessRequest(req)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	return backend.Item{
		Key:      accessRequestKey(req.GetName()),
		Value:    value,
		Expires:  req.Expiry(),
		ID:       req.GetResourceID(),
		Revision: rev,
	}, nil
}

func itemFromAccessListPromotions(req types.AccessRequest, suggestedItems *types.AccessRequestAllowedPromotions) (backend.Item, error) {
	value, err := services.MarshalAccessRequestAllowedPromotion(suggestedItems)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	return backend.Item{
		Key:      AccessRequestAllowedPromotionKey(req.GetName()),
		Value:    value,
		Expires:  req.Expiry(), // expire the promotion at the same time as the access request
		ID:       req.GetResourceID(),
		Revision: req.GetRevision(),
	}, nil
}

func itemToAccessRequest(item backend.Item, opts ...services.MarshalOption) (types.AccessRequest, error) {
	opts = append(
		opts,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
	req, err := services.UnmarshalAccessRequest(
		item.Value,
		opts...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

func accessRequestKey(name string) []byte {
	return backend.Key(accessRequestsPrefix, name, paramsPrefix)
}

func AccessRequestAllowedPromotionKey(name string) []byte {
	return backend.Key(accessRequestPromotionPrefix, name, paramsPrefix)
}

const (
	accessRequestsPrefix         = "access_requests"
	accessRequestPromotionPrefix = "access_request_promotions"
	maxCmpAttempts               = 7
	retryPeriod                  = 2048 * time.Millisecond
)
