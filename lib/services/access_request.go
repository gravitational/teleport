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

package services

import (
	"context"
	"fmt"
	"sort"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/vulcand/predicate"
)

// ValidateAccessRequest validates the AccessRequest and sets default values
func ValidateAccessRequest(ar AccessRequest) error {
	if err := ar.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if uuid.Parse(ar.GetName()) == nil {
		return trace.BadParameter("invalid access request id %q", ar.GetName())
	}
	return nil
}

// NewAccessRequest assembles an AccessRequest resource.
func NewAccessRequest(user string, roles ...string) (AccessRequest, error) {
	req, err := types.NewAccessRequest(uuid.New(), user, roles...)
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
		if uuid.Parse(id) == nil {
			return trace.BadParameter("invalid request id %q", id)
		}
	}
	return nil
}

func (r *RequestIDs) IsEmpty() bool {
	return len(r.AccessRequests) < 1
}

// DynamicAccessCore is the core functionality common to all DynamicAccess implementations.
type DynamicAccessCore interface {
	// CreateAccessRequest stores a new access request.
	CreateAccessRequest(ctx context.Context, req AccessRequest) error
	// SetAccessRequestState updates the state of an existing access request.
	SetAccessRequestState(ctx context.Context, params AccessRequestUpdate) error
	// GetAccessRequests gets all currently active access requests.
	GetAccessRequests(ctx context.Context, filter AccessRequestFilter) ([]AccessRequest, error)
	// DeleteAccessRequest deletes an access request.
	DeleteAccessRequest(ctx context.Context, reqID string) error
	// GetPluginData loads all plugin data matching the supplied filter.
	GetPluginData(ctx context.Context, filter PluginDataFilter) ([]PluginData, error)
	// UpdatePluginData updates a per-resource PluginData entry.
	UpdatePluginData(ctx context.Context, params PluginDataUpdateParams) error
}

// DynamicAccess is a service which manages dynamic RBAC.  Specifically, this is the
// dynamic access interface implemented by remote clients.
type DynamicAccess interface {
	DynamicAccessCore
	// SubmitAccessReview applies a review to a request and returns the post-application state.
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (AccessRequest, error)
}

// DynamicAccessOracle is a service capable of answering questions related
// to the dynamic access API.  Necessary because some information (e.g. the
// list of roles a user is allowed to request) can not be calculated by
// actors with limited privileges.
type DynamicAccessOracle interface {
	GetAccessCapabilities(ctx context.Context, req AccessCapabilitiesRequest) (*AccessCapabilities, error)
}

// CalculateAccessCapabilities aggregates the requested capabilities using the supplied getter
// to load relevant resources.
func CalculateAccessCapabilities(ctx context.Context, clt UserAndRoleGetter, req AccessCapabilitiesRequest) (*AccessCapabilities, error) {
	var caps AccessCapabilities
	// all capabilities require use of a request validator.  calculating suggested reviewers
	// requires that the validator be configured for variable expansion.
	v, err := NewRequestValidator(clt, req.User, ExpandVars(req.SuggestedReviewers))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.RequestableRoles {
		caps.RequestableRoles, err = v.GetRequestableRoles()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if req.SuggestedReviewers {
		caps.SuggestedReviewers = v.SuggestedReviewers
	}
	return &caps, nil
}

// DynamicAccessExt is an extended dynamic access interface
// used to implement some auth server internals.
type DynamicAccessExt interface {
	DynamicAccessCore
	// ApplyAccessReview applies a review to a request in the backend and returns the post-application state.
	ApplyAccessReview(ctx context.Context, params types.AccessReviewSubmission, checker ReviewPermissionChecker) (AccessRequest, error)
	// UpsertAccessRequest creates or updates an access request.
	UpsertAccessRequest(ctx context.Context, req AccessRequest) error
	// DeleteAllAccessRequests deletes all existent access requests.
	DeleteAllAccessRequests(ctx context.Context) error
}

// reviewParamsContext is a simplified view of an access review
// which represents the incoming review during review threshold
// filter evaluation.
type reviewParamsContext struct {
	Reason      string              `json:"reason"`
	Annotations map[string][]string `json:"annotations"`
}

// reviewAuthorContext is a simplified view of a user
// resource which represents the author of a review during
// review threshold filter evaluation.
type reviewAuthorContext struct {
	Roles  []string            `json:"roles"`
	Traits map[string][]string `json:"traits"`
}

// reviewRequestContext is a simplified view of an access request
// resource which represents the request parameters which are in-scope
// during review threshold filter evaluation.
type reviewRequestContext struct {
	Roles             []string            `json:"roles"`
	Reason            string              `json:"reason"`
	SystemAnnotations map[string][]string `json:"system_annotations"`
}

// thresholdFilterContext is the top-level context used to evaluate
// review threshold filters.
type thresholdFilterContext struct {
	Reviewer reviewAuthorContext  `json:"reviewer"`
	Review   reviewParamsContext  `json:"review"`
	Request  reviewRequestContext `json:"request"`
}

// reviewPermissionContext is the top-level context used to evaluate
// a user's review permissions.  It is functionally identical to the
// thresholdFilterContext except that it does not expose review parameters.
// this is because review permissions are used to determine which requests
// a user is allowed to see, and therefore needs to be calculable prior
// to construction of review parameters.
type reviewPermissionContext struct {
	Reviewer reviewAuthorContext  `json:"reviewer"`
	Request  reviewRequestContext `json:"request"`
}

// ValidateAccessPredicates checks request & review permission predicates for
// syntax errors.  Used to help prevent users from accidentally writing incorrect
// predicates.  This function should only be called by the auth server prior to
// storing new/updated roles.  Normal role validation deliberately omits these
// checks in order to allow us to extend the available namespaces without breaking
// backwards compatibility with older nodes/proxies (which never need to evaluate
// these predicates).
func ValidateAccessPredicates(role Role) error {
	tp, err := NewJSONBoolParser(thresholdFilterContext{})
	if err != nil {
		return trace.Wrap(err, "failed to build empty threshold predicate parser (this is a bug)")
	}

	if len(role.GetAccessRequestConditions(Deny).Thresholds) != 0 {
		// deny blocks never contain thresholds.  a threshold which happens to describe a *denial condition* is
		// still part of the "allow" block.  thresholds are not part of deny blocks because thresholds describe the
		// state-transition scenarios supported by a request (including potentially being denied).  deny.request blocks match
		// requests which are *never* allowable, and therefore will never reach the point of needing to encode thresholds.
		return trace.BadParameter("deny.request cannot contain thresholds, set denial counts in allow.request.thresholds instead")
	}

	for _, t := range role.GetAccessRequestConditions(Allow).Thresholds {
		if t.Filter == "" {
			continue
		}
		if _, err := tp.EvalBoolPredicate(t.Filter); err != nil {
			return trace.BadParameter("invalid threshold predicate: %q, %v", t.Filter, err)
		}
	}

	rp, err := NewJSONBoolParser(reviewPermissionContext{})
	if err != nil {
		return trace.Wrap(err, "failed to build empty review predicate parser (this is a bug)")
	}

	if w := role.GetAccessReviewConditions(Deny).Where; w != "" {
		if _, err := rp.EvalBoolPredicate(w); err != nil {
			return trace.BadParameter("invalid review predicate: %q, %v", w, err)
		}
	}

	if w := role.GetAccessReviewConditions(Allow).Where; w != "" {
		if _, err := rp.EvalBoolPredicate(w); err != nil {
			return trace.BadParameter("invalid review predicate: %q, %v", w, err)
		}
	}

	return nil
}

// ApplyAccessReview attempts to apply the specified access review to the specified request.
func ApplyAccessReview(req AccessRequest, rev types.AccessReview, author User) error {
	if rev.Author != author.GetName() {
		return trace.BadParameter("mismatched review author (expected %q, got %q)", rev.Author, author)
	}

	// role lists must be deduplicated and sorted
	rev.Roles = utils.Deduplicate(rev.Roles)
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

	// set threshold indexes and store the review
	rev.ThresholdIndexes = tids
	req.SetReviews(append(req.GetReviews(), rev))

	// if request has already exited the pending state, then no further work
	// needs to be done (subsequent reviews have no effect after initial
	// state-transition).
	if !req.GetState().IsPending() {
		return nil
	}

	// request is still pending, so check to see if this
	// review introduces a state-transition.
	res, err := calculateReviewBasedResolution(req)
	if err != nil || res == nil {
		return trace.Wrap(err)
	}

	// state-transition was triggered.  update the appropriate fields.
	req.SetState(res.state)
	req.SetResolveReason(res.reason)
	req.SetExpiry(req.GetAccessExpiry())
	return nil
}

// checkReviewCompat performs basic checks to ensure that the specified review can be
// applied to the specified request (part of review application logic).
func checkReviewCompat(req AccessRequest, rev types.AccessReview) error {
	// we currently only support reviews that propose approval/denial.  future iterations
	// may support additional states (e.g. None for comment-only reviews).
	if !rev.ProposedState.IsApproved() && !rev.ProposedState.IsDenied() {
		return trace.BadParameter("invalid state proposal: %s (expected approval/denial)", rev.ProposedState)
	}

	// the default threshold should exist. if it does not, the request either is not fully
	// initialized (i.e. variable expansion has not been run yet) or the request was inserted into
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
func collectReviewThresholdIndexes(req AccessRequest, rev types.AccessReview, author User) ([]uint32, error) {
	parser, err := newThresholdFilterParser(req, rev, author)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var tids []uint32

	for i, t := range req.GetThresholds() {
		match, err := accessReviewThresholdMatchesFilter(t, parser)
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
func accessReviewThresholdMatchesFilter(t types.AccessReviewThreshold, parser predicate.Parser) (bool, error) {
	if t.Filter == "" {
		return true, nil
	}
	ifn, err := parser.Parse(t.Filter)
	if err != nil {
		return false, trace.Wrap(err)
	}
	fn, ok := ifn.(predicate.BoolPredicate)
	if !ok {
		return false, trace.BadParameter("unsupported type: %T", ifn)
	}
	return fn(), nil
}

// newThresholdFilterParser creates a custom parser context which exposes a simplified view of the review author
// and the request for evaluation of review threshold filters.
func newThresholdFilterParser(req AccessRequest, rev types.AccessReview, author User) (BoolPredicateParser, error) {
	return NewJSONBoolParser(thresholdFilterContext{
		Reviewer: reviewAuthorContext{
			Roles:  author.GetRoles(),
			Traits: author.GetTraits(),
		},
		Review: reviewParamsContext{
			Reason:      rev.Reason,
			Annotations: rev.Annotations,
		},
		Request: reviewRequestContext{
			Roles:             req.GetOriginalRoles(),
			Reason:            req.GetRequestReason(),
			SystemAnnotations: req.GetSystemAnnotations(),
		},
	})
}

// requestResolution describes a request state-transition from
// PENDING to some other state.
type requestResolution struct {
	state  RequestState
	reason string
}

// calculateReviewBasedResolution calculates the request resolution based upon
// a request's reviews.  Returns (nil,nil) in the event no resolution has been reached.
func calculateReviewBasedResolution(req AccessRequest) (*requestResolution, error) {
	// thresholds and reviews must be populated before state-transitions are possible
	thresholds, reviews := req.GetThresholds(), req.GetReviews()
	if len(thresholds) == 0 || len(reviews) == 0 {
		return nil, nil
	}

	// approved keeps track of roles that have hit at least one
	// of their approval thresholds.
	approved := make(map[string]struct{})

	// denied keeps track of whether or not we've seen *any* role get denied
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
				// role was marked approved during a previous iteration
				continue CheckRoleApprovals
			}

			// iterate through all threshold sets.  All sets must have at least
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
				// which did meet their requirements this iteration, but there is no point
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
func GetAccessRequest(ctx context.Context, acc DynamicAccess, reqID string) (AccessRequest, error) {
	reqs, err := acc.GetAccessRequests(ctx, AccessRequestFilter{
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
func GetTraitMappings(cms []types.ClaimMapping) TraitMappingSet {
	tm := make([]TraitMapping, 0, len(cms))
	for _, mapping := range cms {
		tm = append(tm, TraitMapping{
			Trait: mapping.Claim,
			Value: mapping.Value,
			Roles: mapping.Roles,
		})
	}
	return TraitMappingSet(tm)
}

type UserAndRoleGetter interface {
	UserGetter
	RoleGetter
	GetRoles(ctx context.Context) ([]Role, error)
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

// insertAnnotations constructs all annotations for a given
// AccessRequestConditions instance and adds them to the
// supplied annotations mapping.
func insertAnnotations(annotations map[string][]string, conditions AccessRequestConditions, traits map[string][]string) {
	for key, vals := range conditions.Annotations {
		// get any previous values at key
		allVals := annotations[key]

		// iterate through all new values and expand any
		// variable interpolation syntax they contain.
	ApplyTraits:
		for _, v := range vals {
			applied, err := applyValueTraits(v, traits)
			if err != nil {
				// skip values that failed variable expansion
				continue ApplyTraits
			}
			allVals = append(allVals, applied...)
		}

		annotations[key] = allVals
	}
}

// ReviewPermissionChecker is a helper for validating whether or not a user
// is allowed to review specific access requests.
type ReviewPermissionChecker struct {
	User  User
	Roles struct {
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
// note that the ability to review a request does not necessarily imply that any specific
// approval/denial thresholds will actually match the user's review.  Matching one or more
// thresholds is not a pre-requisite for review submission.
func (c *ReviewPermissionChecker) CanReviewRequest(req AccessRequest) (bool, error) {
	// TODO(fspmarshall): Refactor this to improve readability when
	// adding role subselection support.

	// user cannot review their own request
	if c.User.GetName() == req.GetUser() {
		return false, nil
	}

	// method allocates new array if an override has already been
	// called, so get the role list once in advance.
	requestedRoles := req.GetOriginalRoles()

	parser, err := NewJSONBoolParser(reviewPermissionContext{
		Reviewer: reviewAuthorContext{
			Roles:  c.User.GetRoles(),
			Traits: c.User.GetTraits(),
		},
		Request: reviewRequestContext{
			Roles:             requestedRoles,
			Reason:            req.GetRequestReason(),
			SystemAnnotations: req.GetSystemAnnotations(),
		},
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	// check all denial rules first.
	for expr, denyMatchers := range c.Roles.DenyReview {
		// if predicate is non-empty, it must match
		if expr != "" {
			match, err := parser.EvalBoolPredicate(expr)
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
			match, err := parser.EvalBoolPredicate(expr)
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

func NewReviewPermissionChecker(ctx context.Context, getter UserAndRoleGetter, username string) (ReviewPermissionChecker, error) {
	user, err := getter.GetUser(username, false)
	if err != nil {
		return ReviewPermissionChecker{}, trace.Wrap(err)
	}

	c := ReviewPermissionChecker{
		User: user,
	}

	c.Roles.AllowReview = make(map[string][]parse.Matcher)
	c.Roles.DenyReview = make(map[string][]parse.Matcher)

	// load all statically assigned roles for the user and
	// use them to build our checker state.
	for _, roleName := range c.User.GetRoles() {
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

func (c *ReviewPermissionChecker) push(role Role) error {

	allow, deny := role.GetAccessReviewConditions(Allow), role.GetAccessReviewConditions(Deny)

	var err error

	c.Roles.DenyReview[deny.Where], err = appendRoleMatchers(c.Roles.DenyReview[deny.Where], deny.Roles, deny.ClaimsToRoles, c.User.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	c.Roles.AllowReview[allow.Where], err = appendRoleMatchers(c.Roles.AllowReview[allow.Where], allow.Roles, allow.ClaimsToRoles, c.User.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// RequestValidator a helper for validating access requests.
// a user's statically assigned roles are are "added" to the
// validator via the push() method, which extracts all the
// relevant rules, peforms variable substitutions, and builds
// a set of simple Allow/Deny datastructures.  These, in turn,
// are used to validate and expand the access request.
type RequestValidator struct {
	getter        UserAndRoleGetter
	user          User
	requireReason bool
	opts          struct {
		expandVars bool
	}
	Roles struct {
		AllowRequest, DenyRequest []parse.Matcher
	}
	Annotations struct {
		Allow, Deny map[string][]string
	}
	ThresholdMatchers []struct {
		Matchers   []parse.Matcher
		Thresholds []types.AccessReviewThreshold
	}
	SuggestedReviewers []string
}

// NewRequestValidator configures a new RequestValidor for the specified user.
func NewRequestValidator(getter UserAndRoleGetter, username string, opts ...ValidateRequestOption) (RequestValidator, error) {
	user, err := getter.GetUser(username, false)
	if err != nil {
		return RequestValidator{}, trace.Wrap(err)
	}

	m := RequestValidator{
		getter: getter,
		user:   user,
	}
	for _, opt := range opts {
		opt(&m)
	}
	if m.opts.expandVars {
		// validation process for incoming access requests requires
		// generating system annotations to be attached to the request
		// before it is inserted into the backend.
		m.Annotations.Allow = make(map[string][]string)
		m.Annotations.Deny = make(map[string][]string)
	}

	// load all statically assigned roles for the user and
	// use them to build our validation state.
	for _, roleName := range m.user.GetRoles() {
		role, err := m.getter.GetRole(context.TODO(), roleName)
		if err != nil {
			return RequestValidator{}, trace.Wrap(err)
		}
		if err := m.push(role); err != nil {
			return RequestValidator{}, trace.Wrap(err)
		}
	}
	return m, nil
}

// Validate validates an access request and potentially modifies it depending on how
// the validator was configured.
func (m *RequestValidator) Validate(req AccessRequest) error {
	if m.user.GetName() != req.GetUser() {
		return trace.BadParameter("request validator configured for different user (this is a bug)")
	}

	if m.requireReason && req.GetRequestReason() == "" {
		return trace.BadParameter("request reason must be specified (required by static role configuration)")
	}

	// check for "wildcard request" (`roles=*`).  wildcard requests
	// need to be expanded into a list consisting of all existing roles
	// that the user does not hold and is allowed to request.
	if r := req.GetRoles(); len(r) == 1 && r[0] == Wildcard {

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

		requestable, err := m.GetRequestableRoles()
		if err != nil {
			return trace.Wrap(err)
		}

		if len(requestable) == 0 {
			return trace.BadParameter("no requestable roles, please verify static RBAC configuration")
		}
		req.SetRoles(requestable)
	}

	// verify that all requested roles are permissible
	for _, roleName := range req.GetRoles() {
		if !m.CanRequestRole(roleName) {
			return trace.BadParameter("user %q can not request role %q", req.GetUser(), roleName)
		}
	}

	if m.opts.expandVars {
		// build the thresholds array and role-threshold-mapping.  the rtm encodes the
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
		req.SetSystemAnnotations(m.SystemAnnotations())

		// if no suggested reviewers were provided by the user then
		// use the defaults suggested by the user's static roles.
		if len(req.GetSuggestedReviewers()) == 0 {
			req.SetSuggestedReviewers(utils.Deduplicate(m.SuggestedReviewers))
		}
	}
	return nil
}

// GetRequestableRoles gets the list of all existent roles which the user is
// able to request.  This operation is expensive since it loads all existent
// roles in order to determine the role list.  Prefer calling CanRequestRole
// when checking againt a known role list.
func (m *RequestValidator) GetRequestableRoles() ([]string, error) {
	allRoles, err := m.getter.GetRoles(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var expanded []string
	for _, role := range allRoles {
		if n := role.GetName(); !utils.SliceContainsStr(m.user.GetRoles(), n) && m.CanRequestRole(n) {
			// user does not currently hold this role, and is allowed to request it.
			expanded = append(expanded, n)
		}
	}
	return expanded, nil
}

// push compiles a role's configuration into the request validator.
// All of the requesint user's statically assigned roles must be pushed
// before validation begins.
func (m *RequestValidator) push(role Role) error {
	var err error

	m.requireReason = m.requireReason || role.GetOptions().RequestAccess.RequireReason()

	allow, deny := role.GetAccessRequestConditions(Allow), role.GetAccessRequestConditions(Deny)

	m.Roles.DenyRequest, err = appendRoleMatchers(m.Roles.DenyRequest, deny.Roles, deny.ClaimsToRoles, m.user.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	// record what will be the starting index of the allow
	// matchers for this role, if it applies any.
	astart := len(m.Roles.AllowRequest)

	m.Roles.AllowRequest, err = appendRoleMatchers(m.Roles.AllowRequest, allow.Roles, allow.ClaimsToRoles, m.user.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	if m.opts.expandVars {
		// if this role added additional allow matchers, then we need to record the relationship
		// between its matchers and its thresholds.  this information is used later to calculate
		// the rtm and threshold list.
		if len(m.Roles.AllowRequest) > astart {
			m.ThresholdMatchers = append(m.ThresholdMatchers, struct {
				Matchers   []parse.Matcher
				Thresholds []types.AccessReviewThreshold
			}{
				Matchers:   m.Roles.AllowRequest[astart:],
				Thresholds: allow.Thresholds,
			})
		}

		// validation process for incoming access requests requires
		// generating system annotations to be attached to the request
		// before it is inserted into the backend.
		insertAnnotations(m.Annotations.Deny, deny, m.user.GetTraits())
		insertAnnotations(m.Annotations.Allow, allow, m.user.GetTraits())

		m.SuggestedReviewers = append(m.SuggestedReviewers, allow.SuggestedReviewers...)
	}
	return nil
}

// thresholdCollector is a helper which assembles the Thresholds array for a request.
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
		if t.Equals(threshold) {
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

// SystemAnnotations calculates the system annotations for a pending
// access request.
func (m *RequestValidator) SystemAnnotations() map[string][]string {
	annotations := make(map[string][]string)
	for k, va := range m.Annotations.Allow {
		var filtered []string
		for _, v := range va {
			if !utils.SliceContainsStr(m.Annotations.Deny[k], v) {
				filtered = append(filtered, v)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		annotations[k] = filtered
	}
	return annotations
}

type ValidateRequestOption func(*RequestValidator)

// ExpandVars toggles variable expansion during request validation.  Variable expansion
// includes expanding wildcard requests, setting system annotations, and gathering
// threshold information.  Variable expansion should be run by the auth server prior
// to storing an access request for the first time.
func ExpandVars(expand bool) ValidateRequestOption {
	return func(v *RequestValidator) {
		v.opts.expandVars = expand
	}
}

// ValidateAccessRequestForUser validates an access request against the associated users's
// *statically assigned* roles. If expandRoles is true, it will also expand wildcard
// requests, setting their role list to include all roles the user is allowed to request.
// Expansion should be performed before an access request is initially placed in the backend.
func ValidateAccessRequestForUser(getter UserAndRoleGetter, req AccessRequest, opts ...ValidateRequestOption) error {
	v, err := NewRequestValidator(getter, req.GetUser(), opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(v.Validate(req))
}

// AccessRequestSpecSchema is JSON schema for AccessRequestSpec
const AccessRequestSpecSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"user": { "type": "string" },
		"roles": {
			"type": "array",
			"items": { "type": "string" }
		},
		"state": { "type": "integer" },
		"created": { "type": "string" },
		"expires": { "type": "string" },
		"request_reason": { "type": "string" },
		"resolve_reason": { "type": "string" },
		"resolve_annotations": { "type": "object" },
		"system_annotations": { "type": "object" },
		"thresholds": { "type": "array" },
		"rtm": { "type": "object" },
		"reviews": { "type": "array" },
		"suggested_reviewers": { "type": "array" }
	}
}`

// GetAccessRequestSchema gets the full AccessRequest JSON schema
func GetAccessRequestSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, AccessRequestSpecSchema, DefaultDefinitions)
}

// UnmarshalAccessRequest unmarshals the AccessRequest resource from JSON.
func UnmarshalAccessRequest(data []byte, opts ...MarshalOption) (AccessRequest, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req AccessRequestV3
	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(data, &req); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := utils.UnmarshalWithSchema(GetAccessRequestSchema(), &req, data); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := ValidateAccessRequest(&req); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		req.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		req.SetExpiry(cfg.Expires)
	}
	return &req, nil
}

// MarshalAccessRequest marshals the AccessRequest resource to JSON.
func MarshalAccessRequest(accessRequest AccessRequest, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch accessRequest := accessRequest.(type) {
	case *AccessRequestV3:
		if version := accessRequest.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched access request version %v and type %T", version, accessRequest)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *accessRequest
			copy.SetResourceID(0)
			accessRequest = &copy
		}
		return utils.FastMarshal(accessRequest)
	default:
		return nil, trace.BadParameter("unrecognized access request type: %T", accessRequest)
	}
}
