package ui

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// AccessRequest describes a request's current state.
type AccessRequest struct {
	// ID is the request ID.
	ID string `json:"id"`
	// State is the request state.
	State string `json:"state"`
	// ResolveReason is an optional message on the reason
	// why a request was resolved (approved, denied, etc).
	ResolveReason string `json:"resolveReason"`
	// RequestReason is the reason for request.
	RequestReason string `json:"requestReason"`
	// User is the name of requestor.
	User string `json:"user"`
	// Roles are the list of roles requested.
	Roles []string `json:"roles"`
	// Created is the time the request was made.
	Created time.Time `json:"created"`
	// Expires is when the request will expire.
	Expires time.Time `json:"expires"`
	// Reviews are reviews applied to this access request.
	Reviews []AccessRequestReview `json:"reviews"`
	// SuggestedReviewers is a list of reviewers suggested.
	SuggestedReviewers []string `json:"suggestedReviewers"`
	// ThresholdNames is a list of threshold names.
	ThresholdNames []string `json:"thresholdNames"`
	// Resources is the list of resources for a Resource Access Request
	Resources []Resource `json:"resources"`
}

// AccessRequestReview defines fields of a review applied to a request.
type AccessRequestReview struct {
	// Author is the user who reviewed request.
	Author string `json:"author"`
	// Roles are the list of roles approved.
	Roles []string `json:"roles"`
	// State is either DENIED or APPROVED.
	State string `json:"state"`
	// Reason is the why request was approved or denied.
	Reason string `json:"reason"`
	// Created is the time review was submitted.
	Created time.Time `json:"created"`
}

type Resource struct {
	ID      ResourceID      `json:"id"`
	Details ResourceDetails `json:"details"`
}

type ResourceID struct {
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	ClusterName     string `json:"clusterName"`
	SubResourceName string `json:"subResourceName,omitempty"`
}

type ResourceDetails struct {
	FriendlyName string `json:"friendlyName"`
}

type NewAccessRequestConfig struct {
	resourceDetails map[string]ResourceDetails
}

func defaultNewAccessRequestConfig() *NewAccessRequestConfig {
	return &NewAccessRequestConfig{}
}

type NewAccessRequestOption func(*NewAccessRequestConfig)

func WithResourceDetails(resourceDetails map[string]ResourceDetails) NewAccessRequestOption {
	return func(cfg *NewAccessRequestConfig) {
		cfg.resourceDetails = resourceDetails
	}
}

// NewAccessRequest creates a UI access request object.
func NewAccessRequest(request types.AccessRequest, opts ...NewAccessRequestOption) (*AccessRequest, error) {
	cfg := defaultNewAccessRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if request == nil {
		return nil, trace.BadParameter("nil request")
	}

	// Access Request state NONE is its empty value and the empty value
	// is treated internally as an error, so it should return as an error.
	if request.GetState().IsNone() {
		return nil, trace.BadParameter("request %q, state is set to none", request.GetMetadata().Name)
	}

	reviews := make([]AccessRequestReview, 0, len(request.GetReviews()))
	for _, review := range request.GetReviews() {
		reviews = append(reviews, newAccessReview(review))
	}

	thresholdNames := make([]string, 0, len(request.GetThresholds()))
	for _, threshold := range request.GetThresholds() {
		if threshold.Name != "" {
			thresholdNames = append(thresholdNames, threshold.Name)
		}
	}

	requestedResourceIDs := request.GetRequestedResourceIDs()
	resources := make([]Resource, len(requestedResourceIDs))
	for i, r := range requestedResourceIDs {
		resources[i] = Resource{
			ID: ResourceID{
				ClusterName:     r.ClusterName,
				Kind:            r.Kind,
				Name:            r.Name,
				SubResourceName: r.SubResourceName,
			},
			// If there are no details for this resource, the map lookup returns
			// the default value which is empty details
			Details: cfg.resourceDetails[types.ResourceIDToString(r)],
		}
	}

	return &AccessRequest{
		ID:                 request.GetMetadata().Name,
		State:              request.GetState().String(),
		ResolveReason:      request.GetResolveReason(),
		RequestReason:      request.GetRequestReason(),
		User:               request.GetUser(),
		Roles:              request.GetRoles(),
		Created:            request.GetCreationTime(),
		Expires:            request.GetAccessExpiry(),
		Reviews:            reviews,
		SuggestedReviewers: request.GetSuggestedReviewers(),
		ThresholdNames:     thresholdNames,
		Resources:          resources,
	}, nil
}

func newAccessReview(review types.AccessReview) AccessRequestReview {
	return AccessRequestReview{
		Author:  review.Author,
		Roles:   review.Roles,
		State:   review.ProposedState.String(),
		Reason:  review.Reason,
		Created: review.Created,
	}
}
