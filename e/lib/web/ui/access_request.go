package ui

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
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
	// MaxDuration is the duration for how long the access should be granted.
	// This value can be nil if the request does not have a max duration.
	MaxDuration *time.Time `json:"maxDuration,omitempty"`
	// RequestTTL is the expiration time of the request (how long it will await
	// approval).
	RequestTTL time.Time `json:"requestTTL"`
	// SessionTTL is the duration for how long the generated certificate will be valid.
	SessionTTL time.Time `json:"sessionTTL"`
	// Reviews are reviews applied to this access request.
	Reviews []AccessRequestReview `json:"reviews"`
	// SuggestedReviewers is a list of reviewers suggested.
	SuggestedReviewers []string `json:"suggestedReviewers"`
	// ThresholdNames is a list of threshold names.
	ThresholdNames []string `json:"thresholdNames"`
	// Resources is the list of resources for a Resource Access Request
	Resources []Resource `json:"resources"`
	// PromotedAccessListTitle is the title of the access list that was promoted
	// to a resource access request.
	PromotedAccessListTitle string `json:"promotedAccessListTitle,omitempty"`
	// AssumeStartTime is the time the requested roles can be assumed.
	AssumeStartTime *time.Time `json:"assumeStartTime"`
	// ReasonMode can be either "required" or "optional". Empty string is treated as
	// "optional". If a role has the request reason mode set to "required", then reason is
	// required for this access request.
	ReasonMode string `json:"reasonMode"`
	// ReasonPrompts is a sorted and deduplicated list of reason prompts for this Access
	// Request.
	ReasonPrompts []string `json:"reasonPrompts"`
	// RequestKind indicates the kind (short/long-term) of request.
	RequestKind types.AccessRequestKind `json:"requestKind"`
	// LongTermResourceGrouping contains information about how requested resources
	// can be grouped for long-term access.
	LongTermResourceGrouping *LongTermResourceGrouping `json:"longTermResourceGrouping,omitempty"`
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
	// PromotedAccessListTitle is the title of the access list that the access request
	// was promoted to.
	PromotedAccessListTitle string `json:"promotedAccessListTitle"`
	// AssumeStartTime is the time the requested roles can be assumed.
	AssumeStartTime *time.Time `json:"assumeStartTime"`
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
			// the default value which is empty details. Logic in lib/web relies
			// on the fact that unrelated resource IDs are ignored and may pass
			// in resource detail mappings that contain large numbers of unrealted
			// entries.
			Details: cfg.resourceDetails[types.ResourceIDToString(r)],
		}
	}

	var maxDuration *time.Time
	if !request.GetMaxDuration().IsZero() {
		reqMaxDuration := request.GetMaxDuration()
		maxDuration = &reqMaxDuration
	}

	dryRunEnrichment := request.GetDryRunEnrichment()
	if dryRunEnrichment == nil {
		dryRunEnrichment = &types.AccessRequestDryRunEnrichment{
			ReasonMode: types.RequestReasonModeOptional,
		}
	}

	uiReq := &AccessRequest{
		ID:                      request.GetMetadata().Name,
		State:                   request.GetState().String(),
		ResolveReason:           request.GetResolveReason(),
		RequestReason:           request.GetRequestReason(),
		User:                    request.GetUser(),
		Roles:                   request.GetRoles(),
		Created:                 request.GetCreationTime(),
		MaxDuration:             maxDuration,
		RequestTTL:              request.Expiry(),
		SessionTTL:              request.GetSessionTLL(),
		Expires:                 request.GetAccessExpiry(),
		Reviews:                 reviews,
		SuggestedReviewers:      request.GetSuggestedReviewers(),
		ThresholdNames:          thresholdNames,
		Resources:               resources,
		PromotedAccessListTitle: request.GetPromotedAccessListTitle(),
		AssumeStartTime:         request.GetAssumeStartTime(),
		ReasonMode:              string(dryRunEnrichment.ReasonMode),
		ReasonPrompts:           dryRunEnrichment.ReasonPrompts,
		RequestKind:             request.GetRequestKind(),
	}

	longTermGrouping := request.GetLongTermResourceGrouping()
	if longTermGrouping != nil {
		uiReq.LongTermResourceGrouping = &LongTermResourceGrouping{
			CanProceed:            longTermGrouping.CanProceed,
			ValidationMessage:     longTermGrouping.ValidationMessage,
			RecommendedAccessList: longTermGrouping.RecommendedAccessList,
			AccessListToResources: convertResourceIDs(longTermGrouping.AccessListToResources),
		}
	}

	return uiReq, nil
}

func newAccessReview(review types.AccessReview) AccessRequestReview {
	return AccessRequestReview{
		Author:                  review.Author,
		Roles:                   review.Roles,
		State:                   review.ProposedState.String(),
		Reason:                  review.Reason,
		Created:                 review.Created,
		PromotedAccessListTitle: review.GetAccessListTitle(),
		AssumeStartTime:         review.AssumeStartTime,
	}
}

// SuggestedAccessLists is a list of suggested access lists for a given access request.
type SuggestedAccessLists struct {
	AccessLists []*accesslist.AccessList `json:"accessLists,omitempty"`
}

// LongTermResourceGrouping contains information about how resources can be grouped
// based on Access List promotions for long-term Access Requests.
type LongTermResourceGrouping struct {
	// CanProceed represents the validity of the long-term grouping. If all requested
	// resources cannot be grouped together, this will be false.
	CanProceed bool `json:"canProceed"`
	// ValidationMessage is a user-friendly message explaining any grouping error, if CanProceed is false.
	ValidationMessage string `json:"validationMessage,omitempty"`
	// RecommendedAccessList is the name of the Access List that would provide
	// access to the most resources. If multiple Access Lists provide the same
	// number of resources, the first one found will be used.
	RecommendedAccessList string `json:"recommendedAccessList,omitempty"`
	// AccessListToResources maps applicable Access List names to the resources they can grant,
	// including the optimal grouping.
	AccessListToResources map[string][]ResourceID `json:"accessListToResources"`
}

// convertResourceIDs converts a map[string]types.ResourceIDList to map[string[]ui.ResourceID
func convertResourceIDs(groups map[string]types.ResourceIDList) map[string][]ResourceID {
	result := make(map[string][]ResourceID)
	if len(groups) == 0 {
		return result
	}
	for name, group := range groups {
		result[name] = make([]ResourceID, 0, len(group.ResourceIds))
		for _, id := range group.ResourceIds {
			result[name] = append(result[name], convertResourceID(id))
		}
	}
	return result
}

// convertResourceID converts a single types.ResourceID to ui.ResourceID
func convertResourceID(id types.ResourceID) ResourceID {
	return ResourceID{
		Kind:            id.Kind,
		Name:            id.Name,
		ClusterName:     id.ClusterName,
		SubResourceName: id.SubResourceName,
	}
}

// AccessRequestParameters describes parameters for creating or reviewing an access request.
type AccessRequestParameters struct {
	// Reason is the AccessRequest request reason.
	// Used interchangeably between reason why request is made and resolved reason.
	Reason string `json:"reason"`
	// State is the AccessRequest state.
	State string `json:"state"`
	// ID is the request ID.
	ID string `json:"id"`
	// Roles is the list of roles.
	// Used interchangeably between roles requested by user and overriding roles.
	Roles []string `json:"roles"`
	// SuggestedReviewers is a suggested list of reviewers to review a request.
	SuggestedReviewers []string `json:"suggestedReviewers"`
	// ResourceID is a unique identifier for a teleport resource.
	ResourceIDs []ResourceID `json:"resourceIds"`
	// MaxDuration is the maximum duration for which the request is valid.
	MaxDuration time.Time `json:"maxDuration"`
	// RequestTTL is the expiration time of the request (how long it will await
	// approval).
	RequestTTL time.Time `json:"requestTTL"`
	// DryRun is a flag that indicates whether the request is a dry run to check and set defaults,
	// and return before actually creating the request in the backend.
	DryRun bool `json:"dryRun,omitempty"`
	// PromotedAccessListTitle is the title of the access list that this request
	// was promoted to. Used by WebUI to display the title of the access list.
	// This field is only populated when the request is in the PROMOTED state.
	PromotedAccessListTitle string `json:"promotedAccessListTitle,omitempty"`
	// AssumeStartTime is the time the requested roles can be assumed.
	AssumeStartTime *time.Time `json:"assumeStartTime"`
	// RequestKind is the kind of request (short/long-term).
	RequestKind types.AccessRequestKind `json:"requestKind"`
}
