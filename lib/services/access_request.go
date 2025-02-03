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

package services

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/parse"
	"github.com/gravitational/teleport/lib/utils/typical"
)

const (
	maxAccessRequestReasonSize = 4096
	maxResourcesPerRequest     = 300
	maxResourcesLength         = 2048

	// A day is sometimes 23 hours, sometimes 25 hours, usually 24 hours.
	day = 24 * time.Hour

	// MaxAccessDuration is the maximum duration that an access request can be
	// granted for.
	MaxAccessDuration = 14 * day

	// requestTTL is the TTL for an access request, i.e. the amount of time that
	// the access request can be reviewed. Defaults to 1 week.
	requestTTL = 7 * day

	// InvalidKubernetesKindAccessRequest is used in part of error messages related to
	// `request.kubernetes_resources` config. It's also used to determine if a returned error
	// contains this string (in tests and tsh) to customize error messages shown to user.
	InvalidKubernetesKindAccessRequest = `your Teleport role's "request.kubernetes_resources" field`
)

// ValidateAccessRequest validates the AccessRequest and sets default values
func ValidateAccessRequest(ar types.AccessRequest) error {
	if err := CheckAndSetDefaults(ar); err != nil {
		return trace.Wrap(err)
	}

	_, err := uuid.Parse(ar.GetName())
	if err != nil {
		return trace.BadParameter("invalid access request ID %q", ar.GetName())
	}
	if len(ar.GetRequestReason()) > maxAccessRequestReasonSize {
		return trace.BadParameter("access request reason is too long, max %v bytes", maxAccessRequestReasonSize)
	}
	if len(ar.GetResolveReason()) > maxAccessRequestReasonSize {
		return trace.BadParameter("access request resolve reason is too long, max %v bytes", maxAccessRequestReasonSize)
	}
	if l := len(ar.GetRequestedResourceIDs()); l > maxResourcesPerRequest {
		return trace.BadParameter("access request contains too many resources (%v), max %v", l, maxResourcesPerRequest)
	}
	return nil
}

// ClusterGetter provides access to the local cluster
type ClusterGetter interface {
	// GetClusterName returns the local cluster name
	GetClusterName(opts ...MarshalOption) (types.ClusterName, error)
	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)
}

// ValidateAccessRequestClusterNames checks that the clusters in the access request exist
func ValidateAccessRequestClusterNames(cg ClusterGetter, ar types.AccessRequest) error {
	localClusterName, err := cg.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	var invalidClusters []string
	for _, resourceID := range ar.GetRequestedResourceIDs() {
		if resourceID.ClusterName == "" {
			continue
		}
		if resourceID.ClusterName == localClusterName.GetClusterName() {
			continue
		}
		_, err := cg.GetRemoteCluster(context.TODO(), resourceID.ClusterName)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "failed to fetch remote cluster %q", resourceID.ClusterName)
		}
		if trace.IsNotFound(err) {
			invalidClusters = append(invalidClusters, resourceID.ClusterName)
		}
	}
	if len(invalidClusters) > 0 {
		return trace.NotFound("access request contains invalid or unknown cluster names: %v",
			strings.Join(apiutils.Deduplicate(invalidClusters), ", "))
	}
	return nil
}

// NewAccessRequest assembles an AccessRequest resource.
func NewAccessRequest(user string, roles ...string) (types.AccessRequest, error) {
	return NewAccessRequestWithResources(user, roles, []types.ResourceID{})
}

// NewAccessRequestWithResources assembles an AccessRequest resource with
// requested resources.
func NewAccessRequestWithResources(user string, roles []string, resourceIDs []types.ResourceID) (types.AccessRequest, error) {
	req, err := types.NewAccessRequestWithResources(uuid.New().String(), user, roles, resourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ValidateAccessRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	return req, nil
}

// RequestIDs is a collection of IDs for privilege escalation requests.
type RequestIDs struct {
	AccessRequests []string `json:"access_requests,omitempty"`
}

func (r *RequestIDs) Marshal() ([]byte, error) {
	data, err := utils.FastMarshal(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

func (r *RequestIDs) Unmarshal(data []byte) error {
	if err := utils.FastUnmarshal(data, r); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.Check())
}

func (r *RequestIDs) Check() error {
	for _, id := range r.AccessRequests {
		_, err := uuid.Parse(id)
		if err != nil {
			return trace.BadParameter("invalid request id %q", id)
		}
	}
	return nil
}

func (r *RequestIDs) IsEmpty() bool {
	return len(r.AccessRequests) < 1
}

// AccessRequestGetter defines the interface for fetching access request resources.
type AccessRequestGetter interface {
	// GetAccessRequests gets all currently active access requests.
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)

	// ListAccessRequests is an access request getter with pagination and sorting options.
	ListAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest) (*proto.ListAccessRequestsResponse, error)
}

// DynamicAccessCore is the core functionality common to all DynamicAccess implementations.
type DynamicAccessCore interface {
	AccessRequestGetter
	// CreateAccessRequestV2 stores a new access request.
	CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error)
	// DeleteAccessRequest deletes an access request.
	DeleteAccessRequest(ctx context.Context, reqID string) error
}

// DynamicAccess is a service which manages dynamic RBAC.  Specifically, this is the
// dynamic access interface implemented by remote clients.
type DynamicAccess interface {
	DynamicAccessCore
	// SetAccessRequestState updates the state of an existing access request.
	SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error
	// SubmitAccessReview applies a review to a request and returns the post-application state.
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
	// GetAccessRequestAllowedPromotions returns suggested access lists for the given access request.
	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
}

// DynamicAccessOracle is a service capable of answering questions related
// to the dynamic access API.  Necessary because some information (e.g. the
// list of roles a user is allowed to request) can not be calculated by
// actors with limited privileges.
type DynamicAccessOracle interface {
	GetAccessCapabilities(ctx context.Context, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error)
}

func shouldFilterRequestableRolesByResource(a RequestValidatorGetter, req types.AccessCapabilitiesRequest) (bool, error) {
	if !req.FilterRequestableRolesByResource {
		return false, nil
	}
	currentCluster, err := a.GetClusterName()
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, resourceID := range req.ResourceIDs {
		if resourceID.ClusterName != currentCluster.GetClusterName() {
			// Requested resource is from another cluster, so we can't know
			// all of the roles which would grant access to it.
			return false, nil
		}
	}
	return true, nil
}

// CalculateAccessCapabilities aggregates the requested capabilities using the supplied getter
// to load relevant resources.
func CalculateAccessCapabilities(ctx context.Context, clock clockwork.Clock, clt RequestValidatorGetter, identity tlsca.Identity, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error) {
	shouldFilter, err := shouldFilterRequestableRolesByResource(clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !shouldFilter && req.FilterRequestableRolesByResource {
		req.ResourceIDs = nil
	}

	var caps types.AccessCapabilities
	// all capabilities require use of a request validator.  calculating suggested reviewers
	// requires that the validator be configured for variable expansion.
	v, err := NewRequestValidator(ctx, clock, clt, req.User, ExpandVars(req.SuggestedReviewers))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.ResourceIDs) != 0 && !req.FilterRequestableRolesByResource {
		caps.ApplicableRolesForResources, err = v.applicableSearchAsRoles(ctx, req.ResourceIDs, req.Login)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if req.RequestableRoles {
		var resourceIDs []types.ResourceID
		if req.FilterRequestableRolesByResource {
			resourceIDs = req.ResourceIDs
		}
		caps.RequestableRoles, err = v.GetRequestableRoles(ctx, identity, resourceIDs, req.Login)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if req.SuggestedReviewers {
		caps.SuggestedReviewers = v.SuggestedReviewers
	}

	caps.RequireReason, err = v.calcRequireReasonCap(ctx, req, caps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caps.RequestPrompt = v.prompt
	caps.AutoRequest = v.autoRequest

	return &caps, nil
}

func (v *RequestValidator) calcRequireReasonCap(ctx context.Context, req types.AccessCapabilitiesRequest, caps types.AccessCapabilities) (requireReason bool, err error) {
	var roles []string
	if req.RequestableRoles {
		roles = caps.RequestableRoles
	} else {
		roles = caps.ApplicableRolesForResources
	}

	requireReason, _, err = v.isReasonRequired(ctx, roles, nil)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return requireReason, nil
}

// allowedSearchAsRoles returns all allowed `allow.request.search_as_roles` for the user that are
// not in the `deny.request.search_as_roles`. It does not filter out any roles that should not be
// allowed based on requests.
func (m *RequestValidator) allowedSearchAsRoles() ([]string, error) {
	var rolesToRequest []string
	for _, roleName := range m.Roles.AllowSearch {
		if !m.CanSearchAsRole(roleName) {
			continue
		}
		rolesToRequest = append(rolesToRequest, roleName)
	}
	if len(rolesToRequest) == 0 {
		return nil, trace.AccessDenied(`Resource Access Requests require usable "search_as_roles", none found for user %q`, m.userState.GetName())
	}

	return rolesToRequest, nil
}

// applicableSearchAsRoles prunes the search_as_roles and only returns those
// applicable for the given list of resourceIDs.
//
// If loginHint is provided, it will attempt to prune the list to a single role.
func (m *RequestValidator) applicableSearchAsRoles(ctx context.Context, resourceIDs []types.ResourceID, loginHint string) ([]string, error) {
	rolesToRequest, err := m.allowedSearchAsRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Prune the list of roles to request to only those which may be necessary
	// to access the requested resources.
	rolesToRequest, err = m.pruneResourceRequestRoles(ctx, resourceIDs, loginHint, rolesToRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rolesToRequest, nil
}

// DynamicAccessExt is an extended dynamic access interface
// used to implement some auth server internals.
type DynamicAccessExt interface {
	DynamicAccessCore
	// CreateAccessRequest stores a new access request.
	CreateAccessRequest(ctx context.Context, req types.AccessRequest) error
	// ApplyAccessReview applies a review to a request in the backend and returns the post-application state.
	ApplyAccessReview(ctx context.Context, params types.AccessReviewSubmission, checker ReviewPermissionChecker) (types.AccessRequest, error)
	// UpsertAccessRequest creates or updates an access request.
	UpsertAccessRequest(ctx context.Context, req types.AccessRequest) error
	// DeleteAllAccessRequests deletes all existent access requests.
	DeleteAllAccessRequests(ctx context.Context) error
	// SetAccessRequestState updates the state of an existing access request.
	SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) (types.AccessRequest, error)
	// CreateAccessRequestAllowedPromotions creates a list of allowed access list promotions for the given access request.
	CreateAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest, accessLists *types.AccessRequestAllowedPromotions) error
	// GetAccessRequestAllowedPromotions returns a lists of allowed access list promotions for the given access request.
	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
}

// reviewParamsContext is a simplified view of an access review
// which represents the incoming review during review threshold
// filter evaluation.
type reviewParamsContext struct {
	reason      string
	annotations map[string][]string
}

// reviewAuthorContext is a simplified view of a user
// resource which represents the author of a review during
// review threshold filter evaluation.
type reviewAuthorContext struct {
	roles  []string
	traits map[string][]string
}

// reviewRequestContext is a simplified view of an access request
// resource which represents the request parameters which are in-scope
// during review threshold filter evaluation.
type reviewRequestContext struct {
	roles             []string
	reason            string
	systemAnnotations map[string][]string
}

// thresholdFilterContext is the top-level context used to evaluate
// review threshold filters.
type thresholdFilterContext struct {
	reviewer reviewAuthorContext
	review   reviewParamsContext
	request  reviewRequestContext
}

// reviewPermissionContext is the top-level context used to evaluate
// a user's review permissions. It is functionally identical to the
// thresholdFilterContext except that it does not expose review parameters.
// This is because review permissions are used to determine which requests
// a user is allowed to see, and therefore needs to be calculable prior
// to construction of review parameters.
type reviewPermissionContext struct {
	reviewer reviewAuthorContext
	request  reviewRequestContext
}

// ValidateAccessPredicates checks request & review permission predicates for
// syntax errors.  Used to help prevent users from accidentally writing incorrect
// predicates.  This function should only be called by the auth server prior to
// storing new/updated roles.  Normal role validation deliberately omits these
// checks to allow us to extend the available namespaces without breaking
// backwards compatibility with older nodes/proxies (which never need to evaluate
// these predicates).
func ValidateAccessPredicates(role types.Role) error {
	if len(role.GetAccessRequestConditions(types.Deny).Thresholds) != 0 {
		// deny blocks never contain thresholds.  a threshold which happens to describe a *denial condition* is
		// still part of the "allow" block.  thresholds are not part of deny blocks because thresholds describe the
		// state-transition scenarios supported by a request (including potentially being denied).  deny.request blocks match
		// requests which are *never* allowable, and therefore will never reach the point of needing to encode thresholds.
		return trace.BadParameter("deny.request cannot contain thresholds, set denial counts in allow.request.thresholds instead")
	}

	for _, t := range role.GetAccessRequestConditions(types.Allow).Thresholds {
		if t.Filter == "" {
			continue
		}
		if _, err := parseThresholdFilterExpression(t.Filter); err != nil {
			return trace.BadParameter("invalid threshold predicate: %q, %v", t.Filter, err)
		}
	}

	if w := role.GetAccessReviewConditions(types.Deny).Where; w != "" {
		if _, err := parseReviewPermissionExpression(w); err != nil {
			return trace.BadParameter("invalid review predicate: %q, %v", w, err)
		}
	}

	if w := role.GetAccessReviewConditions(types.Allow).Where; w != "" {
		if _, err := parseReviewPermissionExpression(w); err != nil {
			return trace.BadParameter("invalid review predicate: %q, %v", w, err)
		}
	}

	if maxDuration := role.GetAccessRequestConditions(types.Allow).MaxDuration; maxDuration.Duration() != 0 &&
		maxDuration.Duration() > MaxAccessDuration {
		return trace.BadParameter("max access duration must be less than or equal to %v", MaxAccessDuration)
	}

	return nil
}

// ApplyAccessReview attempts to apply the specified access review to the specified request.
func ApplyAccessReview(req types.AccessRequest, rev types.AccessReview, author UserState) error {
	if rev.Author != author.GetName() {
		return trace.BadParameter("mismatched review author (expected %q, got %q)", rev.Author, author)
	}

	// role lists must be deduplicated and sorted
	rev.Roles = apiutils.Deduplicate(rev.Roles)
	sort.Strings(rev.Roles)

	// basic compatibility/sanity checks
	if err := checkReviewCompat(req, rev); err != nil {
		return trace.Wrap(err)
	}

	// aggregate the threshold indexes for this review
	tids, err := collectReviewThresholdIndexes(req, rev, author)
	if err != nil {
		return trace.Wrap(err)
	}

	// set a review created time if not already set
	if rev.Created.IsZero() {
		rev.Created = time.Now()
	}

	// set threshold indexes
	rev.ThresholdIndexes = tids

	// Resolved requests should not be updated.
	switch {
	case req.GetState().IsApproved():
		return trace.AccessDenied("the access request has been already approved")
	case req.GetState().IsDenied():
		return trace.AccessDenied("the access request has been already denied")
	case req.GetState().IsPromoted():
		return trace.AccessDenied("the access request has been already promoted")
	}

	req.SetReviews(append(req.GetReviews(), rev))

	if rev.AssumeStartTime != nil {
		if err := types.ValidateAssumeStartTime(*rev.AssumeStartTime, req.GetAccessExpiry(), req.GetCreationTime()); err != nil {
			return trace.Wrap(err)
		}
		req.SetAssumeStartTime(*rev.AssumeStartTime)
	}

	// the request is still pending, so check to see if this
	// review introduces a state-transition.
	res, err := calculateReviewBasedResolution(req)
	if err != nil || res == nil {
		return trace.Wrap(err)
	}

	// state-transition was triggered. update the appropriate fields.
	if err := req.SetState(res.state); err != nil {
		return trace.Wrap(err)
	}
	req.SetResolveReason(res.reason)
	if req.GetPromotedAccessListName() == "" {
		// Set the title only if it's not set yet. This is to prevent
		// overwriting the title by another promotion review.
		req.SetPromotedAccessListName(rev.GetAccessListName())
		req.SetPromotedAccessListTitle(rev.GetAccessListTitle())
	}
	req.SetExpiry(req.GetAccessExpiry())
	return nil
}

// checkReviewCompat performs basic checks to ensure that the specified review can be
// applied to the specified request (part of review application logic).
func checkReviewCompat(req types.AccessRequest, rev types.AccessReview) error {
	// The Proposal cannot be yet resolved.
	if !rev.ProposedState.IsResolved() {
		// Skip the promoted state in the error message. It's not a state that most people
		// should be concerned with.
		return trace.BadParameter("invalid state proposal: %s (expected approval/denial)", rev.ProposedState)
	}

	// the default threshold should exist. if it does not, the request either is not fully
	// initialized (i.e., variable expansion has not been run yet), or the request was inserted into
	// the backend by a teleport instance which does not support the review feature.
	if len(req.GetThresholds()) == 0 {
		return trace.BadParameter("request is uninitialized or does not support reviews")
	}

	// user must not have previously reviewed this request
	for _, existingReview := range req.GetReviews() {
		if existingReview.Author == rev.Author {
			return trace.AccessDenied("user %q has already reviewed this request", rev.Author)
		}
	}

	rtm := req.GetRoleThresholdMapping()

	// TODO(fspmarshall): Remove this restriction once role overrides
	// in reviews are fully supported.
	if len(rev.Roles) != 0 && len(rev.Roles) != len(rtm) {
		return trace.NotImplemented("role subselection is not yet supported in reviews, try omitting role list")
	}

	// TODO(fspmarhsall): Remove this restriction once annotations
	// in reviews are fully supported.
	if len(rev.Annotations) != 0 {
		return trace.NotImplemented("annotations are not yet supported in reviews, try omitting annotations field")
	}

	// verify that all roles are present within the request
	for _, role := range rev.Roles {
		if _, ok := rtm[role]; !ok {
			return trace.BadParameter("role %q is not a member of this request", role)
		}
	}

	return nil
}

// collectReviewThresholdIndexes aggregates the indexes of all thresholds whose filters match
// the supplied review (part of review application logic).
func collectReviewThresholdIndexes(req types.AccessRequest, rev types.AccessReview, author UserState) ([]uint32, error) {
	var tids []uint32
	ctx := newThresholdFilterContext(req, rev, author)
	for i, t := range req.GetThresholds() {
		match, err := accessReviewThresholdMatchesFilter(t, ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !match {
			continue
		}

		tid := uint32(i)
		if int(tid) != i {
			// sanity-check.  we disallow extremely large threshold lists elsewhere, but it's always
			// best to double-check these things.
			return nil, trace.Errorf("threshold index %d out of supported range (this is a bug)", i)
		}
		tids = append(tids, tid)
	}

	return tids, nil
}

// accessReviewThresholdMatchesFilter returns true if Filter rule matches
// Empty Filter block always matches
func accessReviewThresholdMatchesFilter(t types.AccessReviewThreshold, ctx thresholdFilterContext) (bool, error) {
	if t.Filter == "" {
		return true, nil
	}
	expr, err := parseThresholdFilterExpression(t.Filter)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return expr.Evaluate(ctx)
}

// newThresholdFilterContext creates a custom parser context which exposes a simplified view of the review author
// and the request for evaluation of review threshold filters.
func newThresholdFilterContext(req types.AccessRequest, rev types.AccessReview, author UserState) thresholdFilterContext {
	return thresholdFilterContext{
		reviewer: reviewAuthorContext{
			roles:  author.GetRoles(),
			traits: author.GetTraits(),
		},
		review: reviewParamsContext{
			reason:      rev.Reason,
			annotations: rev.Annotations,
		},
		request: reviewRequestContext{
			roles:             req.GetOriginalRoles(),
			reason:            req.GetRequestReason(),
			systemAnnotations: req.GetSystemAnnotations(),
		},
	}
}

// requestResolution describes a request state-transition from
// PENDING to some other state.
type requestResolution struct {
	state  types.RequestState
	reason string
}

// calculateReviewBasedResolution calculates the request resolution based upon
// a request's reviews. Returns (nil,nil) in the event no resolution has been reached.
func calculateReviewBasedResolution(req types.AccessRequest) (*requestResolution, error) {
	// thresholds and reviews must be populated before state-transitions are possible
	thresholds, reviews := req.GetThresholds(), req.GetReviews()
	if len(thresholds) == 0 || len(reviews) == 0 {
		return nil, nil
	}

	// approved keeps track of roles that have hit at least one
	// of their approval thresholds.
	approved := make(map[string]struct{})

	// denied keeps track of whether we've seen *any* role get denied
	// (which role does not currently matter since we short-circuit on the
	// first denial to be triggered).
	denied := false

	// counts keeps track of the approval and denial counts for all thresholds.
	counts := make([]struct{ approval, denial uint32 }, len(thresholds))

	// lastReview stores the most recently processed review.  Since processing halts
	// once we hit our first approval/denial condition, this review represents the
	// triggering review for the approval/denial state-transition.
	var lastReview types.AccessReview

	// Iterate through all reviews and aggregate them against `counts`.
ProcessReviews:
	for _, rev := range reviews {
		lastReview = rev
		for _, tid := range rev.ThresholdIndexes {
			idx := int(tid)
			if len(thresholds) <= idx {
				return nil, trace.Errorf("threshold index '%d' out of range (this is a bug)", idx)
			}
			switch {
			case rev.ProposedState.IsApproved():
				counts[idx].approval++
			case rev.ProposedState.IsDenied():
				counts[idx].denial++
			case rev.ProposedState.IsPromoted():
				// Promote skips the threshold check.
				break ProcessReviews
			default:
				return nil, trace.BadParameter("cannot calculate state-transition, unexpected proposal: %s", rev.ProposedState)
			}
		}

		// If we hit any denial thresholds, short-circuit immediately
		for i, t := range thresholds {
			if counts[i].denial >= t.Deny && t.Deny != 0 {
				denied = true
				break ProcessReviews
			}
		}

		// check for roles that can be transitioned to an approved state
	CheckRoleApprovals:
		for role, thresholdSets := range req.GetRoleThresholdMapping() {
			if _, ok := approved[role]; ok {
				// the role was marked approved during a previous iteration
				continue CheckRoleApprovals
			}

			// iterate through all threshold sets. All sets must have at least
			// one threshold which has hit its approval count in order for the
			// role to be considered approved.
		CheckThresholdSets:
			for _, tset := range thresholdSets.Sets {

				for _, tid := range tset.Indexes {
					idx := int(tid)
					if len(thresholds) <= idx {
						return nil, trace.Errorf("threshold index out of range %s/%d (this is a bug)", role, tid)
					}
					t := thresholds[idx]

					if counts[idx].approval >= t.Approve && t.Approve != 0 {
						// this set contains a threshold which has met its approval condition.
						// skip to the next set.
						continue CheckThresholdSets
					}
				}

				// no thresholds met for this set. there may be additional roles/thresholds
				// that did meet their requirements this iteration, but there is no point in
				// processing them unless this set has also hit its requirements.  we therefore
				// move immediately to processing the next review.
				continue ProcessReviews
			}

			// since we skip to the next review as soon as we see a set which has not hit any of its
			// approval scenarios, we know that if we get to this point the role must be approved.
			approved[role] = struct{}{}
		}
		// If we got here, then we iterated across all roles in the rtm without hitting any that
		// had not met their approval scenario.  The request has hit an approved state and further
		// reviews will not be processed.
		break ProcessReviews
	}

	switch {
	case lastReview.ProposedState.IsApproved():
		if len(approved) != len(req.GetRoleThresholdMapping()) {
			// processing halted on approval, but not all roles have
			// hit their approval thresholds; no state-transition.
			return nil, nil
		}
	case lastReview.ProposedState.IsDenied():
		if !denied {
			// processing halted on denial, but no roles have hit
			// their denial thresholds; no state-transition.
			return nil, nil
		}
	case lastReview.ProposedState.IsPromoted():
		// Let the state change. Promoted won't grant any access, meaning it is roughly equivalent to denial.
		// But we want to be able to distinguish between promoted and denied in audit logs/UI.
	default:
		return nil, trace.BadParameter("cannot calculate state-transition, unexpected proposal: %s", lastReview.ProposedState)
	}

	// processing halted on valid state-transition; return resolution
	// based on last review
	return &requestResolution{
		state:  lastReview.ProposedState,
		reason: lastReview.Reason,
	}, nil
}

// GetAccessRequest is a helper function assists with loading a specific request by ID.
func GetAccessRequest(ctx context.Context, acc DynamicAccessCore, reqID string) (types.AccessRequest, error) {
	reqs, err := acc.GetAccessRequests(ctx, types.AccessRequestFilter{
		ID: reqID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(reqs) < 1 {
		return nil, trace.NotFound("no access request matching %q", reqID)
	}
	return reqs[0], nil
}

// GetTraitMappings gets the AccessRequestConditions' claims as a TraitMappingsSet
func GetTraitMappings(cms []types.ClaimMapping) types.TraitMappingSet {
	tm := make([]types.TraitMapping, 0, len(cms))
	for _, mapping := range cms {
		tm = append(tm, types.TraitMapping{
			Trait: mapping.Claim,
			Value: mapping.Value,
			Roles: mapping.Roles,
		})
	}
	return types.TraitMappingSet(tm)
}

// RequestValidatorGetter is the interface required by the request validation
// functions used to get the necessary resources.
type RequestValidatorGetter interface {
	UserLoginStatesGetter
	UserGetter
	RoleGetter
	client.ListResourcesClient
	GetRoles(ctx context.Context) ([]types.Role, error)
	GetClusterName(opts ...MarshalOption) (types.ClusterName, error)
}

// appendRoleMatchers constructs all role matchers for a given
// AccessRequestConditions instance and appends them to the
// supplied matcher slice.
func appendRoleMatchers(matchers []parse.Matcher, roles []string, cms []types.ClaimMapping, traits map[string][]string) ([]parse.Matcher, error) {
	// build matchers for the role list
	for _, r := range roles {
		m, err := parse.NewMatcher(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		matchers = append(matchers, m)
	}

	// build matchers for all role mappings
	ms, err := TraitsToRoleMatchers(GetTraitMappings(cms), traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return append(matchers, ms...), nil
}

// ReviewPermissionChecker is a helper for validating whether a user
// is allowed to review specific access requests.
type ReviewPermissionChecker struct {
	UserState UserState
	Roles     struct {
		// allow/deny mappings sort role matches into lists based on their
		// constraining predicate (where) expression.
		AllowReview, DenyReview map[string][]parse.Matcher
	}
}

// HasAllowDirectives checks if any allow directives exist.  A user with
// no allow directives will never be able to review any requests.
func (c *ReviewPermissionChecker) HasAllowDirectives() bool {
	for _, allowMatchers := range c.Roles.AllowReview {
		if len(allowMatchers) > 0 {
			return true
		}
	}
	return false
}

// CanReviewRequest checks if the user is allowed to review the specified request.
// Note that the ability to review a request does not necessarily imply that any specific
// approval/denial thresholds will actually match the user's review.  Matching one or more
// thresholds is not a pre-requisite for review submission.
func (c *ReviewPermissionChecker) CanReviewRequest(req types.AccessRequest) (bool, error) {
	// TODO(fspmarshall): Refactor this to improve readability when
	// adding role subselection support.

	// user cannot review their own request
	if c.UserState.GetName() == req.GetUser() {
		return false, nil
	}

	// method allocates a new array if an override has already been
	// called, so get the role list once in advance.
	requestedRoles := req.GetOriginalRoles()

	rpc := reviewPermissionContext{
		reviewer: reviewAuthorContext{
			roles:  c.UserState.GetRoles(),
			traits: c.UserState.GetTraits(),
		},
		request: reviewRequestContext{
			roles:             requestedRoles,
			reason:            req.GetRequestReason(),
			systemAnnotations: req.GetSystemAnnotations(),
		},
	}

	// check all denial rules first.
	for expr, denyMatchers := range c.Roles.DenyReview {
		// if predicate is non-empty, it must match
		if expr != "" {
			parsed, err := parseReviewPermissionExpression(expr)
			if err != nil {
				return false, trace.Wrap(err)
			}
			match, err := parsed.Evaluate(rpc)
			if err != nil {
				return false, trace.Wrap(err)
			}
			if !match {
				continue
			}
		}

		for _, role := range requestedRoles {
			for _, deny := range denyMatchers {
				if deny.Match(role) {
					// short-circuit on first denial
					return false, nil
				}
			}
		}
	}

	// needsAllow tracks the list of roles which still need to match an allow directive
	// in order for the request to be reviewable.  we need to perform a deep copy here
	// since we perform a filter-in-place when we find a matching allow directive.
	needsAllow := make([]string, len(requestedRoles))
	copy(needsAllow, requestedRoles)

Outer:
	for expr, allowMatchers := range c.Roles.AllowReview {
		// if predicate is non-empty, it must match.
		if expr != "" {
			parsed, err := parseReviewPermissionExpression(expr)
			if err != nil {
				return false, trace.Wrap(err)
			}
			match, err := parsed.Evaluate(rpc)
			if err != nil {
				return false, trace.Wrap(err)
			}
			if !match {
				continue Outer
			}
		}

		// unmatched collects unmatched roles for our filter-in-place operation.
		unmatched := needsAllow[:0]

	MatchRoles:
		for _, role := range needsAllow {
			for _, allow := range allowMatchers {
				if allow.Match(role) {
					// role matched this allow directive, and will be filtered out
					continue MatchRoles
				}
			}

			// still unmatched, this role will continue to be part of
			// the needsAllow list next iteration.
			unmatched = append(unmatched, role)
		}

		// finalize our filter-in-place
		needsAllow = unmatched

		if len(needsAllow) == 0 {
			// all roles have matched an allow directive, no further
			// processing is required.
			break Outer
		}
	}

	return len(needsAllow) == 0, nil
}

type userStateRoleOverride struct {
	UserState
	Roles []string
}

func (u userStateRoleOverride) GetRoles() []string {
	return u.Roles
}

func NewReviewPermissionChecker(
	ctx context.Context,
	getter RequestValidatorGetter,
	username string,
	identity *tlsca.Identity,
) (ReviewPermissionChecker, error) {
	uls, err := GetUserOrLoginState(ctx, getter, username)
	if err != nil {
		return ReviewPermissionChecker{}, trace.Wrap(err)
	}

	// By default, the users freshly fetched roles are used rather than the
	// roles on the x509 identity. This prevents recursive access request
	// review.
	//
	// For bots, however, the roles on the identity must be used. This is
	// because the certs output by a bot always use role impersonation and the
	// role directly assigned to a bot has minimal permissions.
	if uls.IsBot() {
		if identity == nil {
			// Handle an edge case where SubmitAccessReview is being invoked
			// in-memory but as a bot user.
			return ReviewPermissionChecker{}, trace.BadParameter(
				"bot user provided but identity parameter is nil",
			)
		}
		if identity.Username != username {
			// It should not be possible for these to be different as a
			// guard in AuthorizeAccessReviewRequest prevents submitting a
			// request as another user unless you have the admin role. This
			// safeguard protects against that regressing and creating an
			// inconsistent state.
			return ReviewPermissionChecker{}, trace.BadParameter(
				"bot identity username and review author mismatch",
			)
		}
		if len(identity.ActiveRequests) > 0 {
			// It should not be possible for a bot's output certificates to
			// have active requests - but this additional check safeguards us
			// against a regression elsewhere and prevents recursive access
			// requests occurring.
			return ReviewPermissionChecker{}, trace.BadParameter(
				"bot should not have active requests",
			)
		}

		// Override list of roles to roles currently present on the x509 ident.
		uls = userStateRoleOverride{
			UserState: uls,
			Roles:     identity.Groups,
		}
	}

	c := ReviewPermissionChecker{
		UserState: uls,
	}

	c.Roles.AllowReview = make(map[string][]parse.Matcher)
	c.Roles.DenyReview = make(map[string][]parse.Matcher)

	// load all statically assigned roles for the user and
	// use them to build our checker state.
	for _, roleName := range c.UserState.GetRoles() {
		role, err := getter.GetRole(ctx, roleName)
		if err != nil {
			return ReviewPermissionChecker{}, trace.Wrap(err)
		}
		if err := c.push(role); err != nil {
			return ReviewPermissionChecker{}, trace.Wrap(err)
		}
	}

	return c, nil
}

func (c *ReviewPermissionChecker) push(role types.Role) error {
	allow, deny := role.GetAccessReviewConditions(types.Allow), role.GetAccessReviewConditions(types.Deny)

	var err error

	c.Roles.DenyReview[deny.Where], err = appendRoleMatchers(c.Roles.DenyReview[deny.Where], deny.Roles, deny.ClaimsToRoles, c.UserState.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	c.Roles.AllowReview[allow.Where], err = appendRoleMatchers(c.Roles.AllowReview[allow.Where], allow.Roles, allow.ClaimsToRoles, c.UserState.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// RequestValidator a helper for validating access requests.
// a user's statically assigned roles are "added" to the
// validator via the push() method, which extracts all the
// relevant rules, performs variable substitutions, and builds
// a set of simple Allow/Deny datastructures.  These, in turn,
// are used to validate and expand the access request.
type RequestValidator struct {
	clock     clockwork.Clock
	getter    RequestValidatorGetter
	userState UserState
	// requireReasonForAllRoles indicates that non-empty reason is required for all access
	// requests. This happens if any of the user roles has options.request_access "always" or
	// "reason".
	requireReasonForAllRoles bool
	// requiringReasonRoles is a set of role names, which require non-empty reason to be
	// specified when requested. The same applies to all requested resources allowed by those
	// roles. Such roles are all requestable roles and search_as_roles allowed by a role
	// assigned to a user and having spec.allow.request.reason.mode="required" set.
	//
	// Please note this means, roles having spec.allow.request.reason.mode="required" don't
	// necessarily require reason when they are requested themselves. Instead they mark roles
	// in spec.allow.request.roles and spec.allow.request.search_as_roles as roles requiring
	// reason.
	requiringReasonRoles map[string]struct{}
	// Used to enforce that the configuration found in the static
	// role that defined the search_as_role, is respected.
	// An empty map or list means nothing was configured.
	kubernetesResource struct {
		// allow is a map from the user's allowed search_as_roles to the list of
		// kubernetes resource kinds the user is allowed to request with that role.
		allow map[string][]types.RequestKubernetesResource
		// deny is the list of kubernetes resource kinds the user is explicitly
		// denied from requesting.
		deny []types.RequestKubernetesResource
	}
	autoRequest bool
	prompt      string
	opts        struct {
		expandVars bool
	}
	Roles struct {
		AllowRequest, DenyRequest []parse.Matcher
		AllowSearch, DenySearch   []string
	}
	Annotations struct {
		// Allowed annotations are not greedy, the role that defines the annotation must allow requesting one
		// of the roles that are being requested in order for the annotation to be applied.
		Allow map[singleAnnotation]annotationMatcher
		// Denied annotations match greedily, if a user has any role that denies a specific annotation it will
		// always be denied.
		Deny map[singleAnnotation]struct{}
	}
	ThresholdMatchers []struct {
		Matchers   []parse.Matcher
		Thresholds []types.AccessReviewThreshold
	}
	SuggestedReviewers  []string
	MaxDurationMatchers []struct {
		Matchers    []parse.Matcher
		MaxDuration time.Duration
	}
	logger *slog.Logger
}

// NewRequestValidator configures a new RequestValidator for the specified user.
func NewRequestValidator(ctx context.Context, clock clockwork.Clock, getter RequestValidatorGetter, username string, opts ...ValidateRequestOption) (RequestValidator, error) {
	uls, err := GetUserOrLoginState(ctx, getter, username)
	if err != nil {
		return RequestValidator{}, trace.Wrap(err)
	}

	m := RequestValidator{
		clock:     clock,
		getter:    getter,
		userState: uls,
		logger:    slog.With(teleport.ComponentKey, "request.validator"),

		requiringReasonRoles: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(&m)
	}
	if m.opts.expandVars {
		// validation process for incoming access requests requires
		// generating system annotations to be attached to the request
		// before it is inserted into the backend.
		m.Annotations.Allow = make(map[singleAnnotation]annotationMatcher)
		m.Annotations.Deny = make(map[singleAnnotation]struct{})
	}

	m.kubernetesResource.allow = make(map[string][]types.RequestKubernetesResource)

	// load all statically assigned roles for the user and
	// use them to build our validation state.
	for _, roleName := range m.userState.GetRoles() {
		role, err := m.getter.GetRole(ctx, roleName)
		if err != nil {
			return RequestValidator{}, trace.Wrap(err)
		}
		if err := m.push(ctx, role); err != nil {
			return RequestValidator{}, trace.Wrap(err)
		}
	}
	return m, nil
}

// Validate validates an access request and potentially modifies it depending on how
// the validator was configured.
func (m *RequestValidator) Validate(ctx context.Context, req types.AccessRequest, identity tlsca.Identity) error {
	if m.userState.GetName() != req.GetUser() {
		return trace.BadParameter("request validator configured for different user (this is a bug)")
	}

	if !req.GetState().IsPromoted() && req.GetPromotedAccessListTitle() != "" {
		return trace.BadParameter("only promoted requests can set the promoted access list title")
	}

	// check for "wildcard request" (`roles=*`).  wildcard requests
	// need to be expanded into a list consisting of all existing roles
	// that the user does not hold and is allowed to request.
	if r := req.GetRoles(); len(r) == 1 && r[0] == types.Wildcard {
		if !req.GetState().IsPending() {
			// expansion is only permitted in pending requests.  once resolved,
			// a request's role list must be immutable.
			return trace.BadParameter("wildcard requests are not permitted in state %s", req.GetState())
		}

		if !m.opts.expandVars {
			// teleport always validates new incoming pending access requests
			// with ExpandVars(true). after that, it should be impossible to
			// add new values to the role list.
			return trace.BadParameter("unexpected wildcard request (this is a bug)")
		}

		requestable, err := m.GetRequestableRoles(ctx, identity, nil, "")
		if err != nil {
			return trace.Wrap(err)
		}

		if len(requestable) == 0 {
			return trace.BadParameter("no requestable roles, please verify static RBAC configuration")
		}
		req.SetRoles(requestable)
	}

	// If the reason is provided, don't check if it's required. It has to happen after wildcard
	// role expansion.
	if len(strings.TrimSpace(req.GetRequestReason())) == 0 {
		required, explanation, err := m.isReasonRequired(ctx, req.GetRoles(), req.GetRequestedResourceIDs())
		if err != nil {
			return trace.Wrap(err)
		}
		if required {
			return trace.BadParameter(explanation)
		}
	}

	// verify that all requested roles are permissible
	for _, roleName := range req.GetRoles() {
		if len(req.GetRequestedResourceIDs()) > 0 {
			if !m.CanSearchAsRole(roleName) {
				// Roles are normally determined automatically for resource
				// access requests, this role must have been explicitly
				// requested, or a new deny rule has since been added.
				return trace.BadParameter("user %q can not request role %q", req.GetUser(), roleName)
			}
		} else {
			if !m.CanRequestRole(roleName) {
				return trace.BadParameter("user %q can not request role %q", req.GetUser(), roleName)
			}
		}
	}

	// Verify that each requested role allows requesting every requested kube resource kind.
	if len(req.GetRequestedResourceIDs()) > 0 && len(req.GetRoles()) > 0 {
		// If there were pruned roles, then the request will be rejected.
		// A pruned role meant that role did not allow requesting to all of requested kube resource.
		prunedRoles, mappedRequestedRolesToAllowedKinds := m.pruneRequestedRolesNotMatchingKubernetesResourceKinds(req.GetRequestedResourceIDs(), req.GetRoles())
		if len(prunedRoles) != len(req.GetRoles()) {
			return getInvalidKubeKindAccessRequestsError(mappedRequestedRolesToAllowedKinds, true /* requestedRoles */)
		}

	}

	if m.opts.expandVars {
		// deduplicate requested resource IDs
		var deduplicated []types.ResourceID
		seen := make(map[string]struct{})
		resourcesLen := 0
		for _, resource := range req.GetRequestedResourceIDs() {
			id := types.ResourceIDToString(resource)
			if _, isDuplicate := seen[id]; isDuplicate {
				continue
			}
			seen[id] = struct{}{}
			deduplicated = append(deduplicated, resource)
			resourcesLen += len(id)
		}
		req.SetRequestedResourceIDs(deduplicated)

		// In addition to capping the maximum number of resources in a single request,
		// we also need to ensure that the sum of the resource IDs in the request doesn't
		// get too big.
		if resourcesLen > maxResourcesLength {
			return trace.BadParameter("access request exceeds maximum length: try reducing the number of resources in the request")
		}

		// determine the roles which should be requested for a resource access
		// request, and write them to the request
		if err := m.setRolesForResourceRequest(ctx, req); err != nil {
			return trace.Wrap(err)
		}

		// build the threshold array and role-threshold-mapping.  the rtm encodes the
		// relationship between a role, and the thresholds which must pass in order
		// for that role to be considered approved.  when building the validator we
		// recorded the relationship between the various allow matchers and their associated
		// threshold groups.
		rtm := make(map[string]types.ThresholdIndexSets)
		var tc thresholdCollector
		for _, role := range req.GetRoles() {
			sets, err := m.collectSetsForRole(&tc, role)
			if err != nil {
				return trace.Wrap(err)
			}
			rtm[role] = types.ThresholdIndexSets{
				Sets: sets,
			}
		}
		req.SetThresholds(tc.Thresholds)
		req.SetRoleThresholdMapping(rtm)

		// incoming requests must have system annotations attached
		// before being inserted into the backend. this is how the
		// RBAC system propagates sideband information to plugins.
		systemAnnotations, err := m.SystemAnnotations(req)
		if err != nil {
			return trace.Wrap(err)
		}
		req.SetSystemAnnotations(systemAnnotations)

		// if no suggested reviewers were provided by the user, then
		// use the defaults suggested by the user's static roles.
		if len(req.GetSuggestedReviewers()) == 0 {
			req.SetSuggestedReviewers(apiutils.Deduplicate(m.SuggestedReviewers))
		}

		// Pin the time to the current time to prevent time drift.
		now := m.clock.Now().UTC()

		// Calculate the expiration time of the elevated certificate that will
		// be issued if the Access Request is approved.
		sessionTTL, err := m.sessionTTL(ctx, identity, req, now)
		if err != nil {
			return trace.Wrap(err)
		}

		maxDuration, err := m.calculateMaxAccessDuration(req, sessionTTL)
		if err != nil {
			return trace.Wrap(err)
		}

		// If the maxDuration flag is set, consider it instead of only using the session TTL.
		var maxAccessDuration time.Duration

		if maxDuration > 0 {
			req.SetSessionTLL(now.Add(min(sessionTTL, maxDuration)))
			maxAccessDuration = maxDuration
		} else {
			req.SetSessionTLL(now.Add(sessionTTL))
			maxAccessDuration = sessionTTL
		}

		// This is the final adjusted access expiry where both max duration
		// and session TTL were taken into consideration.
		accessExpiry := now.Add(maxAccessDuration)
		// Adjusted max access duration is equal to the access expiry time.
		req.SetMaxDuration(accessExpiry)

		// Setting access expiry before calling `calculatePendingRequestTTL`
		// matters since the func relies on this adjusted expiry.
		req.SetAccessExpiry(accessExpiry)

		// Calculate the expiration time of the Access Request (how long it
		// will await approval).
		requestTTL, err := m.calculatePendingRequestTTL(req, now)
		if err != nil {
			return trace.Wrap(err)
		}
		req.SetExpiry(now.Add(requestTTL))

		if req.GetAssumeStartTime() != nil {
			assumeStartTime := *req.GetAssumeStartTime()
			if err := types.ValidateAssumeStartTime(assumeStartTime, accessExpiry, req.GetCreationTime()); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// isReasonRequired checks if the reason is required for the given roles and resource IDs.
func (v *RequestValidator) isReasonRequired(ctx context.Context, requestedRoles []string, requestedResourceIDs []types.ResourceID) (required bool, explanation string, err error) {
	if v.requireReasonForAllRoles {
		return true, "request reason must be specified (required request_access option in one of the roles)", nil
	}

	allApplicableRoles := requestedRoles
	if len(requestedResourceIDs) > 0 {
		// Do not provide loginHint. We want all matching search_as_roles for those resources.
		roles, err := v.applicableSearchAsRoles(ctx, requestedResourceIDs, "")
		if err != nil {
			return false, "", trace.Wrap(err)
		}
		if len(allApplicableRoles) == 0 {
			allApplicableRoles = roles
		} else {
			allApplicableRoles = append(allApplicableRoles, roles...)
		}
	}

	for _, r := range allApplicableRoles {
		if _, ok := v.requiringReasonRoles[r]; ok {
			return true, fmt.Sprintf("request reason must be specified (required for role %q)", r), nil
		}
	}

	return false, "", nil
}

// calculateMaxAccessDuration calculates the maximum time for the access request.
// The max duration time is the minimum of the max_duration time set on the request
// and the max_duration time set on the request role.
func (m *RequestValidator) calculateMaxAccessDuration(req types.AccessRequest, sessionTTL time.Duration) (time.Duration, error) {
	// Check if the maxDuration time is set.
	maxDurationTime := req.GetMaxDuration()
	maxDuration := maxDurationTime.Sub(req.GetCreationTime())

	// For dry run requests, use the maximum possible duration.
	// This prevents the time drift that can occur as the value is set on the client side.
	if req.GetDryRun() {
		maxDuration = MaxAccessDuration
		// maxDuration may end up < 0 even if maxDurationTime is set
	} else if !maxDurationTime.IsZero() && maxDuration < 0 {
		return 0, trace.BadParameter("invalid maxDuration: must be greater than creation time")
	}

	if maxDuration > MaxAccessDuration {
		return 0, trace.BadParameter("max_duration must be less than or equal to %v", MaxAccessDuration)
	}

	var minAdjDuration time.Duration
	// Adjust the expiration time if the max_duration value is set on the request role.
	for _, roleName := range req.GetRoles() {
		maxDurationForRole := m.maxDurationForRole(roleName)
		if minAdjDuration == 0 || maxDurationForRole < minAdjDuration {
			minAdjDuration = maxDurationForRole
		}
	}
	if !maxDurationTime.IsZero() && maxDuration < minAdjDuration {
		minAdjDuration = maxDuration
	}

	// minAdjDuration can end up being 0, if no role has a
	// field `max_duration` defined.
	// In this case, return the smaller value between the sessionTTL
	// and the requested max duration.
	if minAdjDuration == 0 && maxDuration < sessionTTL {
		return maxDuration, nil
	}

	return minAdjDuration, nil
}

func (m *RequestValidator) maxDurationForRole(roleName string) time.Duration {
	var maxDurationForRole time.Duration
	for _, tms := range m.MaxDurationMatchers {
		for _, matcher := range tms.Matchers {
			if matcher.Match(roleName) {
				if tms.MaxDuration > maxDurationForRole {
					maxDurationForRole = tms.MaxDuration
				}
			}
		}
	}
	return maxDurationForRole
}

// calculatePendingRequestTTL calculates the TTL of the Access Request (how long it will await
// approval). request TTL is capped to the smaller value between the const requestTTL and the
// access request access expiry.
func (m *RequestValidator) calculatePendingRequestTTL(r types.AccessRequest, now time.Time) (time.Duration, error) {
	accessExpiryTTL := r.GetAccessExpiry().Sub(now)

	// If no expiration provided, use default.
	expiry := r.Expiry()
	if expiry.IsZero() {
		// Guard against the default expiry being greater than access expiry.
		if requestTTL < accessExpiryTTL {
			expiry = now.Add(requestTTL)
		} else {
			expiry = now.Add(accessExpiryTTL)
		}
	}

	if expiry.Before(now) {
		return 0, trace.BadParameter("invalid request TTL: Access Request can not be created in the past")
	}

	// Before returning the TTL, validate that the value requested was smaller
	// than the maximum value allowed. Used to return a sensible error to the
	// user.
	requestedTTL := expiry.Sub(now)
	if !r.Expiry().IsZero() {
		if requestedTTL > requestTTL {
			return 0, trace.BadParameter("invalid request TTL: %v greater than maximum allowed (%v)", requestedTTL, requestTTL)
		}
		if requestedTTL > accessExpiryTTL {
			return 0, trace.BadParameter("invalid request TTL: %v greater than maximum allowed (%v)", requestedTTL, accessExpiryTTL)
		}
	}

	return requestedTTL, nil
}

// sessionTTL calculates the TTL of the elevated certificate that will be issued
// if the Access Request is approved.
func (m *RequestValidator) sessionTTL(ctx context.Context, identity tlsca.Identity, r types.AccessRequest, now time.Time) (time.Duration, error) {
	ttl, err := m.truncateTTL(ctx, identity, r.GetAccessExpiry(), r.GetRoles(), now)
	if err != nil {
		return 0, trace.BadParameter("invalid session TTL: %v", err)
	}

	// Before returning the TTL, validate that the value requested was smaller
	// than the maximum value allowed. Used to return a sensible error to the
	// user.
	requestedTTL := r.GetAccessExpiry().Sub(now)
	if !r.GetAccessExpiry().IsZero() && requestedTTL > ttl {
		return 0, trace.BadParameter("invalid session TTL: %v greater than maximum allowed (%v)", requestedTTL, ttl)
	}

	return ttl, nil
}

// truncateTTL will truncate given expiration by identity expiration and
// shortest session TTL of any role.
func (m *RequestValidator) truncateTTL(ctx context.Context, identity tlsca.Identity, expiry time.Time, roles []string, now time.Time) (time.Duration, error) {
	ttl := apidefaults.MaxCertDuration

	// Reduce by remaining TTL on requesting certificate (identity).
	identityTTL := identity.Expires.Sub(now)
	if identityTTL > 0 && identityTTL < ttl {
		ttl = identityTTL
	}

	// Reduce TTL further if expiration time requested is shorter than that
	// identity.
	expiryTTL := expiry.Sub(now)
	if expiryTTL > 0 && expiryTTL < ttl {
		ttl = expiryTTL
	}

	// Loop over the roles requested by the user and reduce certificate TTL
	// further. Follow the typical Teleport RBAC pattern of strictest setting
	// wins.
	for _, roleName := range roles {
		role, err := m.getter.GetRole(ctx, roleName)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		roleTTL := time.Duration(role.GetOptions().MaxSessionTTL)
		if roleTTL > 0 && roleTTL < ttl {
			ttl = roleTTL
		}
	}

	return ttl, nil
}

// getResourceViewingRoles gets the subset of the user's roles that could be used
// to view resources (i.e., base roles + search as roles).
func (m *RequestValidator) getResourceViewingRoles() []string {
	roles := slices.Clone(m.userState.GetRoles())
	for _, role := range m.Roles.AllowSearch {
		if m.CanSearchAsRole(role) {
			roles = append(roles, role)
		}
	}
	return apiutils.Deduplicate(roles)
}

// GetRequestableRoles gets the list of all existent roles which the user is
// able to request.  This operation is expensive since it loads all existent
// roles to determine the role list.  Prefer calling CanRequestRole
// when checking against a known role list. If resource IDs or a login hints
// are provided, roles will be filtered to only include those that would
// allow access to the given resource with the given login.
func (m *RequestValidator) GetRequestableRoles(ctx context.Context, identity tlsca.Identity, resourceIDs []types.ResourceID, loginHint string) ([]string, error) {
	allRoles, err := m.getter.GetRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources, err := m.getUnderlyingResourcesByResourceIDs(ctx, resourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := m.getter.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessChecker, err := NewAccessChecker(&AccessInfo{
		Roles:              m.getResourceViewingRoles(),
		Traits:             m.userState.GetTraits(),
		Username:           m.userState.GetName(),
		AllowedResourceIDs: identity.AllowedResourceIDs,
	}, cluster.GetClusterName(), m.getter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Filter out resources the user requested but doesn't have access to.
	filteredResources := make([]types.ResourceWithLabels, 0, len(resources))
	for _, resource := range resources {
		if err := accessChecker.CheckAccess(resource, AccessState{MFAVerified: true}); err == nil {
			filteredResources = append(filteredResources, resource)
		}
	}

	var expanded []string
	for _, role := range allRoles {
		n := role.GetName()
		if slices.Contains(m.userState.GetRoles(), n) || !m.CanRequestRole(n) {
			continue
		}

		roleAllowsAccess := true
		for _, resource := range filteredResources {
			access, err := m.roleAllowsResource(role, resource, loginHint)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !access {
				roleAllowsAccess = false
			}
		}

		// user does not currently hold this role, and is allowed to request it.
		if roleAllowsAccess {
			expanded = append(expanded, n)
		}
	}
	return expanded, nil
}

// setAllowRequestKubeResourceLookup goes through each search as roles and sets it with the allowed roles.
// Multiple allow request.kubernetes_resources found for a role will be merged, except when an empty configuration
// is encountered. In this case, empty configuration will override configured request field
// (which results in allowing anything).
func setAllowRequestKubeResourceLookup(allowKubernetesResources []types.RequestKubernetesResource, searchAsRoles []string, lookup map[string][]types.RequestKubernetesResource) {
	if len(allowKubernetesResources) == 0 {
		// Empty configuration overrides any configured request.kubernetes_resources field.
		for _, searchAsRoles := range searchAsRoles {
			lookup[searchAsRoles] = []types.RequestKubernetesResource{}
		}
		return
	}

	for _, searchAsRole := range searchAsRoles {
		currentAllowedResources, exists := lookup[searchAsRole]
		if exists && len(currentAllowedResources) == 0 {
			// Already allowed to access all kube resource kinds.
			continue
		}
		lookup[searchAsRole] = append(currentAllowedResources, allowKubernetesResources...)
	}
}

// push compiles a role's configuration into the request validator.
// All of the requesting user's statically assigned roles must be pushed
// before validation begins.
func (m *RequestValidator) push(ctx context.Context, role types.Role) error {
	var err error

	m.requireReasonForAllRoles = m.requireReasonForAllRoles || role.GetOptions().RequestAccess.RequireReason()
	m.autoRequest = m.autoRequest || role.GetOptions().RequestAccess.ShouldAutoRequest()
	if m.prompt == "" {
		m.prompt = role.GetOptions().RequestPrompt
	}

	allow, deny := role.GetAccessRequestConditions(types.Allow), role.GetAccessRequestConditions(types.Deny)

	if allow.Reason != nil && allow.Reason.Mode.Required() {
		for _, r := range allow.Roles {
			m.requiringReasonRoles[r] = struct{}{}
		}
		for _, r := range allow.SearchAsRoles {
			m.requiringReasonRoles[r] = struct{}{}
		}
	}

	setAllowRequestKubeResourceLookup(allow.KubernetesResources, allow.SearchAsRoles, m.kubernetesResource.allow)

	if len(deny.KubernetesResources) > 0 {
		m.kubernetesResource.deny = append(m.kubernetesResource.deny, deny.KubernetesResources...)
	}

	m.Roles.DenyRequest, err = appendRoleMatchers(m.Roles.DenyRequest, deny.Roles, deny.ClaimsToRoles, m.userState.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	// record what will be the starting index of the allow and deny matchers for this role, if it applies any.
	astart := len(m.Roles.AllowRequest)

	m.Roles.AllowRequest, err = appendRoleMatchers(m.Roles.AllowRequest, allow.Roles, allow.ClaimsToRoles, m.userState.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	m.Roles.AllowSearch = apiutils.Deduplicate(append(m.Roles.AllowSearch, allow.SearchAsRoles...))
	m.Roles.DenySearch = apiutils.Deduplicate(append(m.Roles.DenySearch, deny.SearchAsRoles...))

	if m.opts.expandVars {
		// if this role added additional allow matchers, then we need to record the relationship
		// between its matchers and its thresholds. This information is used later to calculate
		// the rtm and threshold list.
		newAllowRequestMatchers := m.Roles.AllowRequest[astart:]
		newAllowSearchMatchers := literalMatchers(allow.SearchAsRoles)

		allNewAllowMatchers := make([]parse.Matcher, 0, len(newAllowRequestMatchers)+len(newAllowSearchMatchers))
		allNewAllowMatchers = append(allNewAllowMatchers, newAllowRequestMatchers...)
		allNewAllowMatchers = append(allNewAllowMatchers, newAllowSearchMatchers...)

		if len(allNewAllowMatchers) > 0 {
			m.ThresholdMatchers = append(m.ThresholdMatchers, struct {
				Matchers   []parse.Matcher
				Thresholds []types.AccessReviewThreshold
			}{
				Matchers:   allNewAllowMatchers,
				Thresholds: allow.Thresholds,
			})
		}

		if allow.MaxDuration != 0 {
			m.MaxDurationMatchers = append(m.MaxDurationMatchers, struct {
				Matchers    []parse.Matcher
				MaxDuration time.Duration
			}{
				Matchers:    allNewAllowMatchers,
				MaxDuration: allow.MaxDuration.Duration(),
			})
		}

		// validation process for incoming access requests requires
		// generating system annotations to be attached to the request
		// before it is inserted into the backend.
		m.insertAllowedAnnotations(ctx, allow, newAllowRequestMatchers, newAllowSearchMatchers)
		m.insertDeniedAnnotations(ctx, deny)

		m.SuggestedReviewers = append(m.SuggestedReviewers, allow.SuggestedReviewers...)
	}
	return nil
}

// setRolesForResourceRequest determines if the given access request is
// resource-based, and if so, it determines which underlying roles are necessary
// and adds them to the request.
func (m *RequestValidator) setRolesForResourceRequest(ctx context.Context, req types.AccessRequest) error {
	if !m.opts.expandVars {
		// Don't set the roles if expandVars is not set, they have probably
		// already been set and we are just validating the request.
		return nil
	}
	if len(req.GetRequestedResourceIDs()) == 0 {
		// This is not a resource request.
		return nil
	}
	if len(req.GetRoles()) > 0 {
		// Roles were explicitly requested, don't change them.
		return nil
	}

	rolesToRequest, err := m.applicableSearchAsRoles(ctx, req.GetRequestedResourceIDs(), req.GetLoginHint())
	if err != nil {
		return trace.Wrap(err)
	}

	req.SetRoles(rolesToRequest)
	return nil
}

// pruneRequestedRolesNotMatchingKubernetesResourceKinds will filter out the kubernetes kinds from the requested resource IDs (kube_cluster and its subresources)
// disregarding whether it's leaf or root cluster request, and for each requested role, ensures that all requested kube resource kind are allowed by the role.
// Roles not matching with every kind requested, will be pruned from the requested roles.
//
// Returns pruned roles, and a map of requested roles with allowed kinds (with denied applied), used to help aid user in case a request gets rejected,
// lets user know which kinds are allowed for each requested roles.
func (m *RequestValidator) pruneRequestedRolesNotMatchingKubernetesResourceKinds(requestedResourceIDs []types.ResourceID, requestedRoles []string) ([]string, map[string][]string) {
	// Filter for the kube_cluster and its subresource kinds.
	requestedKubeKinds := make(map[string]struct{})
	for _, resourceID := range requestedResourceIDs {
		if resourceID.Kind == types.KindKubernetesCluster || slices.Contains(types.KubernetesResourcesKinds, resourceID.Kind) {
			requestedKubeKinds[resourceID.Kind] = struct{}{}
		}
	}

	if len(requestedKubeKinds) == 0 {
		return requestedRoles, nil
	}

	goodRoles := make(map[string]struct{})
	mappedRequestedRolesToAllowedKinds := make(map[string][]string)
	for _, requestedRoleName := range requestedRoles {
		allowedKinds, deniedKinds := getKubeResourceKinds(m.kubernetesResource.allow[requestedRoleName]), getKubeResourceKinds(m.kubernetesResource.deny)

		// Any resource is allowed.
		if len(allowedKinds) == 0 && len(deniedKinds) == 0 {
			goodRoles[requestedRoleName] = struct{}{}
			continue
		}

		// All supported kube kinds are allowed when there was nothing configured.
		if len(allowedKinds) == 0 {
			allowedKinds = types.KubernetesResourcesKinds
			allowedKinds = append(allowedKinds, types.KindKubernetesCluster)
		}

		// Filter out denied kinds from the allowed kinds
		if len(deniedKinds) > 0 && len(allowedKinds) > 0 {
			allowedKinds = getAllowedKubeResourceKinds(allowedKinds, deniedKinds)
		}

		mappedRequestedRolesToAllowedKinds[requestedRoleName] = allowedKinds

		roleIsDenied := false
		for requestedKubeKind := range requestedKubeKinds {
			if !slices.Contains(allowedKinds, requestedKubeKind) {
				roleIsDenied = true
				continue
			}
		}

		if !roleIsDenied {
			goodRoles[requestedRoleName] = struct{}{}
		}
	}

	return slices.Collect(maps.Keys(goodRoles)), mappedRequestedRolesToAllowedKinds
}

// thresholdCollector is a helper that assembles the Thresholds array for a request.
// the push() method is used to insert groups of related thresholds and calculate their
// corresponding index set.
type thresholdCollector struct {
	Thresholds []types.AccessReviewThreshold
}

// push pushes a set of related thresholds and returns the associated indexes.  each set of indexes represents
// an "or" operator, indicating that one of the referenced thresholds must reach its approval condition in order
// for the set as a whole to be considered approved.
func (c *thresholdCollector) push(s []types.AccessReviewThreshold) ([]uint32, error) {
	if len(s) == 0 {
		// empty threshold sets are equivalent to the default threshold
		s = []types.AccessReviewThreshold{
			{
				Name:    "default",
				Approve: 1,
				Deny:    1,
			},
		}
	}

	var indexes []uint32

	for _, t := range s {
		tid, err := c.pushThreshold(t)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		indexes = append(indexes, tid)
	}

	return indexes, nil
}

// pushThreshold pushes a threshold to the main threshold list and returns its index
// as a uint32 for compatibility with grpc types.
func (c *thresholdCollector) pushThreshold(t types.AccessReviewThreshold) (uint32, error) {
	// maxThresholds is an arbitrary large number that serves as a guard against
	// odd errors due to casting between int and uint32.  This is probably unnecessary
	// since we'd likely hit other limitations *well* before wrapping became a concern,
	// but its best to have explicit guard rails.
	const maxThresholds = 4096

	// don't bother double-storing equivalent thresholds
	for i, threshold := range c.Thresholds {
		if t.IsEqual(&threshold) {
			return uint32(i), nil
		}
	}

	if len(c.Thresholds) >= maxThresholds {
		return 0, trace.LimitExceeded("max review thresholds exceeded (max=%d)", maxThresholds)
	}

	c.Thresholds = append(c.Thresholds, t)

	return uint32(len(c.Thresholds) - 1), nil
}

// CanRequestRole checks if a given role can be requested.
func (m *RequestValidator) CanRequestRole(name string) bool {
	for _, deny := range m.Roles.DenyRequest {
		if deny.Match(name) {
			return false
		}
	}
	for _, allow := range m.Roles.AllowRequest {
		if allow.Match(name) {
			return true
		}
	}
	return false
}

// CanSearchAsRole check if a given role can be requested through a search-based
// access request
func (m *RequestValidator) CanSearchAsRole(name string) bool {
	if slices.Contains(m.Roles.DenySearch, name) {
		return false
	}
	for _, deny := range m.Roles.DenyRequest {
		if deny.Match(name) {
			return false
		}
	}
	return slices.Contains(m.Roles.AllowSearch, name)
}

// collectSetsForRole collects the threshold index sets which describe the various groups of
// thresholds which must pass in order for a request for the given role to be approved.
func (m *RequestValidator) collectSetsForRole(c *thresholdCollector, role string) ([]types.ThresholdIndexSet, error) {
	var sets []types.ThresholdIndexSet

Outer:
	for _, tms := range m.ThresholdMatchers {
		for _, matcher := range tms.Matchers {
			if matcher.Match(role) {
				set, err := c.push(tms.Thresholds)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				sets = append(sets, types.ThresholdIndexSet{
					Indexes: set,
				})
				continue Outer
			}
		}
	}

	if len(sets) == 0 {
		// this should never happen since every allow directive is associated with at least one
		// threshold, and this operation happens after requested roles have been validated to match at
		// least one allow directive.
		return nil, trace.BadParameter("role %q matches no threshold sets (this is a bug)", role)
	}

	return sets, nil
}

// singleAnnotation holds a single annotation key/value pair. The value must already have been expanded with
// ApplyValueTraits.
type singleAnnotation struct {
	key, value string
}

// annotationsMatcher holds a set of role matchers used to decide if an annotations should be added to an
// access request when one of the requested roles matches.
type annotationMatcher struct {
	roleRequestMatchers     []parse.Matcher
	resourceRequestMatchers []parse.Matcher
}

// matchesRequest returns true if either:
// - req is a role access request and one of [m.roleRequestMatchers] matches one of the requested roles
// - req is a resource access request and one of [m.resourceRequestMatchers] matches one of the requested roles
func (m *annotationMatcher) matchesRequest(req types.AccessRequest) bool {
	matchers := m.roleRequestMatchers
	if len(req.GetRequestedResourceIDs()) > 0 {
		matchers = m.resourceRequestMatchers
	}
	for _, matcher := range matchers {
		for _, role := range req.GetRoles() {
			if matcher.Match(role) {
				return true
			}
		}
	}
	return false
}

// insertAllowedAnnotations constructs all allowed annotations for a given AccessRequestConditions instance
// from one of the users current roles and adds them to the annotation matchers mapping.
//
// Annotations are only applied to access requests requests when one of the requested roles matches one of the
// role matchers.
func (m *RequestValidator) insertAllowedAnnotations(ctx context.Context, conditions types.AccessRequestConditions, roleRequestMatchers, resourceRequestMatchers []parse.Matcher) {
	for annotationKey, annotationValueTemplates := range conditions.Annotations {
		// iterate through all new values and expand any
		// variable interpolation syntax they contain.
		for _, template := range annotationValueTemplates {
			expandedValues, err := ApplyValueTraits(template, m.userState.GetTraits())
			if err != nil {
				// skip values that failed variable expansion
				m.logger.WarnContext(ctx, "Failed to expand trait template in access request annotation",
					"key", annotationKey, "template", template, "error", err)
				continue
			}
			for _, expanded := range expandedValues {
				annotation := singleAnnotation{annotationKey, expanded}
				matchers := m.Annotations.Allow[annotation]
				matchers.roleRequestMatchers = append(matchers.roleRequestMatchers, roleRequestMatchers...)
				matchers.resourceRequestMatchers = append(matchers.resourceRequestMatchers, resourceRequestMatchers...)
				m.Annotations.Allow[annotation] = matchers
			}
		}
	}
}

// insertDeniedAnnotations constructs all denied annotations for a given AccessRequestConditions instance
// from one of the users current roles and adds them to the denied annotations set.
func (m *RequestValidator) insertDeniedAnnotations(ctx context.Context, conditions types.AccessRequestConditions) {
	for annotationKey, annotationValueTemplates := range conditions.Annotations {
		// iterate through all new values and expand any
		// variable interpolation syntax they contain.
		for _, template := range annotationValueTemplates {
			expandedValues, err := ApplyValueTraits(template, m.userState.GetTraits())
			if err != nil {
				// skip values that failed variable expansion
				m.logger.WarnContext(ctx, "Failed to expand trait template in access request annotation",
					"key", annotationKey, "template", template, "error", err)
				continue
			}
			for _, expanded := range expandedValues {
				annotation := singleAnnotation{annotationKey, expanded}
				m.Annotations.Deny[annotation] = struct{}{}
			}
		}
	}
}

// SystemAnnotations calculates the system annotations for a pending
// access request.
func (m *RequestValidator) SystemAnnotations(req types.AccessRequest) (map[string][]string, error) {
	annotations := make(map[string][]string)

	for annotation, allowMatchers := range m.Annotations.Allow {
		if _, denied := m.Annotations.Deny[annotation]; denied {
			// Deny matches are greedy, if any of the users roles denies this annotation it is filtered out.
			continue
		}
		if !allowMatchers.matchesRequest(req) {
			// Annotations are filtered out unless this request matches one of the role matchers for this
			// annotation.
			continue
		}
		annotations[annotation.key] = append(annotations[annotation.key], annotation.value)
	}

	// Sort and deduplicate.
	for k := range annotations {
		slices.Sort(annotations[k])
		annotations[k] = slices.Compact(annotations[k])
	}
	return annotations, nil
}

type ValidateRequestOption func(*RequestValidator)

// ExpandVars toggles variable expansion during request validation.  Variable expansion includes
// expanding wildcard requests, setting system annotations, finding applicable roles for
// resource-based requests and gathering threshold information.  Variable expansion should be run
// by the auth server prior to storing an access request for the first time.
func ExpandVars(expand bool) ValidateRequestOption {
	return func(v *RequestValidator) {
		v.opts.expandVars = expand
	}
}

// ValidateAccessRequestForUser validates an access request against the associated users's
// *statically assigned* roles. If [[ExpandVars]] is set to true, it will also expand wildcard
// requests, setting their role list to include all roles the user is allowed to request.
// Expansion should be performed before an access request is initially placed in the backend.
func ValidateAccessRequestForUser(ctx context.Context, clock clockwork.Clock, getter RequestValidatorGetter, req types.AccessRequest, identity tlsca.Identity, opts ...ValidateRequestOption) error {
	v, err := NewRequestValidator(ctx, clock, getter, req.GetUser(), opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(v.Validate(ctx, req, identity))
}

// UnmarshalAccessRequest unmarshals the AccessRequest resource from JSON.
func UnmarshalAccessRequest(data []byte, opts ...MarshalOption) (*types.AccessRequestV3, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req types.AccessRequestV3
	if err := utils.FastUnmarshal(data, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ValidateAccessRequest(&req); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Revision != "" {
		req.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		req.SetExpiry(cfg.Expires)
	}
	return &req, nil
}

// MarshalAccessRequest marshals the AccessRequest resource to JSON.
func MarshalAccessRequest(accessRequest types.AccessRequest, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateAccessRequest(accessRequest); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch accessRequest := accessRequest.(type) {
	case *types.AccessRequestV3:
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, accessRequest))
	default:
		return nil, trace.BadParameter("unrecognized access request type: %T", accessRequest)
	}
}

// MarshalAccessRequestAllowedPromotion marshals the list of access list IDs to JSON.
func MarshalAccessRequestAllowedPromotion(accessListIDs *types.AccessRequestAllowedPromotions) ([]byte, error) {
	payload, err := utils.FastMarshal(accessListIDs)
	return payload, trace.Wrap(err)
}

// UnmarshalAccessRequestAllowedPromotion unmarshals the list of access list IDs from JSON.
func UnmarshalAccessRequestAllowedPromotion(data []byte) (*types.AccessRequestAllowedPromotions, error) {
	var accessListIDs types.AccessRequestAllowedPromotions
	if err := utils.FastUnmarshal(data, &accessListIDs); err != nil {
		return nil, trace.Wrap(err)
	}
	return &accessListIDs, nil
}

func getInvalidKubeKindAccessRequestsError(mappedRequestedRolesToAllowedKinds map[string][]string, requestedRoles bool) error {
	allowedStr := ""
	for roleName, allowedKinds := range mappedRequestedRolesToAllowedKinds {
		if len(allowedStr) > 0 {
			allowedStr = fmt.Sprintf("%s, %s: %v", allowedStr, roleName, allowedKinds)
		} else {
			allowedStr = fmt.Sprintf("%s: %v", roleName, allowedKinds)
		}
	}

	requestWord := "requestable"
	if requestedRoles {
		requestWord = "requested"
	}

	// This error must be in sync with web UI's RequestCheckout.tsx ("checkSupportForKubeResources").
	// Web UI relies on the exact format of this error message to determine what kube kinds are
	// supported since web UI does not support all kube resources at this time.
	return trace.BadParameter(`%s did not allow requesting to some or all of the requested `+
		`Kubernetes resources. allowed kinds for each %s roles: %v`,
		InvalidKubernetesKindAccessRequest, requestWord, allowedStr)
}

// pruneResourceRequestRoles takes a list of requested resource IDs and
// a list of candidate roles to request, and returns a "pruned" list of roles.
//
// Candidate roles are *always* pruned when the user is not allowed to
// request the role with all requested resources.
//
// A best-effort attempt is made to prune roles that would not allow
// access to any of the requested resources, this is skipped when any
// resource is in a leaf cluster.
//
// If loginHint is provided, it will attempt to prune the list to a single role.
func (m *RequestValidator) pruneResourceRequestRoles(
	ctx context.Context,
	resourceIDs []types.ResourceID,
	loginHint string,
	roles []string,
) ([]string, error) {
	if len(resourceIDs) == 0 {
		// This is not a resource request, nothing to do
		return roles, nil
	}

	var mappedRequestedRolesToAllowedKinds map[string][]string
	roles, mappedRequestedRolesToAllowedKinds = m.pruneRequestedRolesNotMatchingKubernetesResourceKinds(resourceIDs, roles)
	if len(roles) == 0 { // all roles got pruned from not matching every kube requested kind.
		return nil, getInvalidKubeKindAccessRequestsError(mappedRequestedRolesToAllowedKinds, false /* requestedRoles */)
	}

	clusterNameResource, err := m.getter.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localClusterName := clusterNameResource.GetClusterName()

	for _, resourceID := range resourceIDs {
		if resourceID.ClusterName != localClusterName {
			rbacLogger.LogAttrs(ctx, logutils.TraceLevel, `Requested resource is in a foreign cluster, unable to prune roles - All available "search_as_roles" will be requested`,
				slog.Any("requested_resources", types.ResourceIDToString(resourceID)),
			)
			return roles, nil
		}
	}

	allRoles, err := FetchRoles(roles, m.getter, m.userState.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources, err := m.getUnderlyingResourcesByResourceIDs(ctx, resourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	necessaryRoles := make(map[string]struct{})
	for _, resource := range resources {
		var (
			rolesForResource    []types.Role
			matchers            []RoleMatcher
			kubeResourceMatcher *KubeResourcesMatcher
		)
		kubernetesResources, err := getKubeResourcesFromResourceIDs(resourceIDs, resource.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(kubernetesResources) > 0 {
			kubeResourceMatcher = NewKubeResourcesMatcher(kubernetesResources)
			matchers = append(matchers, kubeResourceMatcher)
		}

		switch rr := resource.(type) {
		case types.Resource153Unwrapper:
			switch urr := rr.Unwrap().(type) {
			case IdentityCenterAccount:
				matchers = append(matchers, NewIdentityCenterAccountMatcher(urr))

			case IdentityCenterAccountAssignment:
				matchers = append(matchers, NewIdentityCenterAccountAssignmentMatcher(urr))
			}
		}

		for _, role := range allRoles {
			roleAllowsAccess, err := m.roleAllowsResource(role, resource, loginHint, matchers...)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if !roleAllowsAccess {
				// Role does not allow access to this resource. We will prune it
				// unless it allows access to another resource.
				continue
			}
			rolesForResource = append(rolesForResource, role)
		}
		// If any of the requested resources didn't match with the provided roles,
		// we deny the request because the user is trying to request more access
		// than what is allowed by its search_as_roles.
		if kubeResourceMatcher != nil && len(kubeResourceMatcher.Unmatched()) > 0 {
			resourcesStr, err := types.ResourceIDsToString(resourceIDs)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return nil, trace.BadParameter(
				`no roles configured in the "search_as_roles" for this user allow `+
					`access to at least one requested resources. `+
					`resources: %s roles: %v unmatched resources: %v`,
				resourcesStr, roles, kubeResourceMatcher.Unmatched())
		}
		if len(loginHint) > 0 {
			// If we have a login hint, request the single role with the fewest
			// allowed logins. All roles at this point have already matched the
			// requested login and will include it.
			rolesForResource = fewestLogins(rolesForResource)
		}
		for _, role := range rolesForResource {
			necessaryRoles[role.GetName()] = struct{}{}
		}
	}

	if len(necessaryRoles) == 0 {
		resourcesStr, err := types.ResourceIDsToString(resourceIDs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, trace.BadParameter(
			`no roles configured in the "search_as_roles" for this user allow `+
				`access to any requested resources. The user may already have `+
				`access to all requested resources with their existing roles. `+
				`resources: %s roles: %v login: %q`,
			resourcesStr, roles, loginHint)
	}
	prunedRoles := make([]string, 0, len(necessaryRoles))
	for role := range necessaryRoles {
		prunedRoles = append(prunedRoles, role)
	}
	return prunedRoles, nil
}

func fewestLogins(roles []types.Role) []types.Role {
	if len(roles) == 0 {
		return roles
	}
	fewest := roles[0]
	fewestCount := countAllowedLogins(fewest)
	for _, role := range roles[1:] {
		if countAllowedLogins(role) < fewestCount {
			fewest = role
		}
	}
	return []types.Role{fewest}
}

func countAllowedLogins(role types.Role) int {
	allowed := make(map[string]struct{})
	for _, a := range role.GetLogins(types.Allow) {
		allowed[a] = struct{}{}
	}
	for _, d := range role.GetLogins(types.Deny) {
		delete(allowed, d)
	}
	return len(allowed)
}

// getKubeResourceKinds just extracts the kinds from the list.
// If a wildcard is present, then all supported resource types are returned.
func getKubeResourceKinds(kubernetesResources []types.RequestKubernetesResource) []string {
	var kinds []string
	for _, rm := range kubernetesResources {
		if rm.Kind == types.Wildcard {
			return types.KubernetesResourcesKinds
		}
		kinds = append(kinds, rm.Kind)
	}
	return kinds
}

// getAllowedKubeResourceKinds returns only the allowed kinds that were not in the
// denied list.
func getAllowedKubeResourceKinds(allowedKinds []string, deniedKinds []string) []string {
	allowed := make(map[string]struct{}, len(allowedKinds))
	for _, kind := range allowedKinds {
		allowed[kind] = struct{}{}
	}
	for _, kind := range deniedKinds {
		delete(allowed, kind)
	}
	return slices.Collect(maps.Keys(allowed))
}

func (m *RequestValidator) roleAllowsResource(
	role types.Role,
	resource types.ResourceWithLabels,
	loginHint string,
	extraMatchers ...RoleMatcher,
) (bool, error) {
	roleSet := RoleSet{role}
	var matchers []RoleMatcher
	if len(loginHint) > 0 {
		matchers = append(matchers, NewLoginMatcher(loginHint))
	}
	matchers = append(matchers, extraMatchers...)
	err := roleSet.checkAccess(resource, m.userState.GetTraits(), AccessState{MFAVerified: true}, matchers...)
	if trace.IsAccessDenied(err) {
		// Access denied, this role does not allow access to this resource, no
		// unexpected error to report.
		return false, nil
	}
	if err != nil {
		// Unexpected error, return it.
		return false, trace.Wrap(err)
	}
	// Role allows access to this resource.
	return true, nil
}

// getUnderlyingResourcesByResourceIDs gets the underlying resources the user
// requested access. Except for resource Kinds present in types.KubernetesResourcesKinds,
// the underlying resources are the same as requested. If the resource requested
// is a Kubernetes resource, we return the underlying Kubernetes cluster.
func (m *RequestValidator) getUnderlyingResourcesByResourceIDs(ctx context.Context, resourceIDs []types.ResourceID) ([]types.ResourceWithLabels, error) {
	if len(resourceIDs) == 0 {
		return []types.ResourceWithLabels{}, nil
	}
	// When searching for Kube Resources, we change the resource Kind to the Kubernetes
	// Cluster in order to load the roles that grant access to it and to verify
	// if the access to it is allowed. We later verify if every Kubernetes Resource
	// requested is fulfilled by at least one role.
	searchableResourcesIDs := slices.Clone(resourceIDs)
	for i := range searchableResourcesIDs {
		if slices.Contains(types.KubernetesResourcesKinds, searchableResourcesIDs[i].Kind) {
			searchableResourcesIDs[i].Kind = types.KindKubernetesCluster
		}
	}
	// load the underlying resources.
	resources, err := accessrequest.GetResourcesByResourceIDs(ctx, m.getter, searchableResourcesIDs)
	return resources, trace.Wrap(err)
}

// getKubeResourcesFromResourceIDs returns the Kubernetes Resources requested for
// the configured cluster.
func getKubeResourcesFromResourceIDs(resourceIDs []types.ResourceID, clusterName string) ([]types.KubernetesResource, error) {
	kubernetesResources := make([]types.KubernetesResource, 0, len(resourceIDs))

	for _, resourceID := range resourceIDs {
		if slices.Contains(types.KubernetesResourcesKinds, resourceID.Kind) && resourceID.Name == clusterName {
			switch {
			case slices.Contains(types.KubernetesClusterWideResourceKinds, resourceID.Kind):
				kubernetesResources = append(kubernetesResources, types.KubernetesResource{
					Kind: resourceID.Kind,
					Name: resourceID.SubResourceName,
				})
			default:
				splits := strings.Split(resourceID.SubResourceName, "/")
				if len(splits) != 2 {
					return nil, trace.BadParameter("subresource name %q does not follow <namespace>/<name> format", resourceID.SubResourceName)
				}
				kubernetesResources = append(kubernetesResources, types.KubernetesResource{
					Kind:      resourceID.Kind,
					Namespace: splits[0],
					Name:      splits[1],
				})
			}
		}
	}
	return kubernetesResources, nil
}

func newReviewPermissionParser() (*typical.Parser[reviewPermissionContext, bool], error) {
	return typical.NewParser[reviewPermissionContext, bool](typical.ParserSpec[reviewPermissionContext]{
		Variables: map[string]typical.Variable{
			"reviewer.roles": typical.DynamicVariable(func(ctx reviewPermissionContext) ([]string, error) {
				return ctx.reviewer.roles, nil
			}),
			"reviewer.traits": typical.DynamicVariable(func(ctx reviewPermissionContext) (map[string][]string, error) {
				return ctx.reviewer.traits, nil
			}),
			"request.roles": typical.DynamicVariable(func(ctx reviewPermissionContext) ([]string, error) {
				return ctx.request.roles, nil
			}),
			"request.reason": typical.DynamicVariable(func(ctx reviewPermissionContext) (string, error) {
				return ctx.request.reason, nil
			}),
			"request.system_annotations": typical.DynamicVariable(func(ctx reviewPermissionContext) (map[string][]string, error) {
				return ctx.request.systemAnnotations, nil
			}),
		},
		Functions: map[string]typical.Function{
			"equals":       typical.BinaryFunction[reviewPermissionContext](equalsFunc),
			"contains":     typical.BinaryFunction[reviewPermissionContext](containsFunc),
			"regexp.match": typical.BinaryFunction[reviewPermissionContext](regexpMatchFunc),
		},
	})
}

func mustNewReviewPermissionParser() *typical.Parser[reviewPermissionContext, bool] {
	parser, err := newReviewPermissionParser()
	if err != nil {
		panic(err)
	}
	return parser
}

var (
	reviewPermissionParser = mustNewReviewPermissionParser()
)

func parseReviewPermissionExpression(expr string) (typical.Expression[reviewPermissionContext, bool], error) {
	parsed, err := reviewPermissionParser.Parse(expr)
	return parsed, trace.Wrap(err, "parsing review.where expression")
}

func newThresholdFilterParser() (*typical.Parser[thresholdFilterContext, bool], error) {
	return typical.NewParser[thresholdFilterContext, bool](typical.ParserSpec[thresholdFilterContext]{
		Variables: map[string]typical.Variable{
			"reviewer.roles": typical.DynamicVariable(func(ctx thresholdFilterContext) ([]string, error) {
				return ctx.reviewer.roles, nil
			}),
			"reviewer.traits": typical.DynamicVariable(func(ctx thresholdFilterContext) (map[string][]string, error) {
				return ctx.reviewer.traits, nil
			}),
			"review.reason": typical.DynamicVariable(func(ctx thresholdFilterContext) (string, error) {
				return ctx.review.reason, nil
			}),
			"review.annotations": typical.DynamicVariable(func(ctx thresholdFilterContext) (map[string][]string, error) {
				return ctx.review.annotations, nil
			}),
			"request.roles": typical.DynamicVariable(func(ctx thresholdFilterContext) ([]string, error) {
				return ctx.request.roles, nil
			}),
			"request.reason": typical.DynamicVariable(func(ctx thresholdFilterContext) (string, error) {
				return ctx.request.reason, nil
			}),
			"request.system_annotations": typical.DynamicVariable(func(ctx thresholdFilterContext) (map[string][]string, error) {
				return ctx.request.systemAnnotations, nil
			}),
		},
		Functions: map[string]typical.Function{
			"equals":       typical.BinaryFunction[thresholdFilterContext](equalsFunc),
			"contains":     typical.BinaryFunction[thresholdFilterContext](containsFunc),
			"regexp.match": typical.BinaryFunction[thresholdFilterContext](regexpMatchFunc),
		},
	})
}

func mustNewThresholdFilterParser() *typical.Parser[thresholdFilterContext, bool] {
	parser, err := newThresholdFilterParser()
	if err != nil {
		panic(err)
	}
	return parser
}

var (
	thresholdFilterParser = mustNewThresholdFilterParser()
)

func parseThresholdFilterExpression(expr string) (typical.Expression[thresholdFilterContext, bool], error) {
	parsed, err := thresholdFilterParser.Parse(expr)
	return parsed, trace.Wrap(err, "parsing threshold filter expression")
}

func equalsFunc(a, b any) (bool, error) {
	switch aval := a.(type) {
	case string:
		bval, ok := b.(string)
		if ok {
			return aval == bval, nil
		}
	case []string:
		bval, ok := b.([]string)
		if ok {
			return slices.Equal(aval, bval), nil
		}
	}
	return false, trace.BadParameter("parameter types must match and be string or []string, got (%T, %T)", a, b)
}

func containsFunc(s []string, v string) (bool, error) {
	return slices.Contains(s, v), nil
}

func regexpMatchFunc(list []string, re string) (bool, error) {
	match, err := utils.RegexMatchesAny(list, re)
	if err != nil {
		return false, trace.Wrap(err, "invalid regular expression %q", re)
	}
	return match, nil
}
