/*
Copyright 2020 Gravitational, Inc.

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

// Package types contains all types and logic required by the Teleport API.
package types

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"
)

// AccessRequest is a request for temporarily granted roles
type AccessRequest interface {
	ResourceWithLabels
	// GetUser gets the name of the requesting user
	GetUser() string
	// GetRoles gets the roles being requested by the user
	GetRoles() []string
	// SetRoles overrides the roles being requested by the user
	SetRoles([]string)
	// GetState gets the current state of the request
	GetState() RequestState
	// SetState sets the approval state of the request
	SetState(RequestState) error
	// GetCreationTime gets the time at which the request was
	// originally registered with the auth server.
	GetCreationTime() time.Time
	// SetCreationTime sets the creation time of the request.
	SetCreationTime(time.Time)
	// GetAccessExpiry gets the expiration time for the elevated certificate
	// that will be issued if the Access Request is approved.
	GetAccessExpiry() time.Time
	// GetAssumeStartTime gets the time the roles can be assumed
	// if the Access Request is approved.
	GetAssumeStartTime() *time.Time
	// SetAssumeStartTime sets the time the roles can be assumed
	// if the Access Request is approved.
	SetAssumeStartTime(time.Time)
	// SetAccessExpiry sets the expiration time for the elevated certificate
	// that will be issued if the Access Request is approved.
	SetAccessExpiry(time.Time)
	// GetSessionTLL gets the session TTL for generated certificates.
	GetSessionTLL() time.Time
	// SetSessionTLL sets the session TTL for generated certificates.
	SetSessionTLL(time.Time)
	// GetRequestReason gets the reason for the request's creation.
	GetRequestReason() string
	// SetRequestReason sets the reason for the request's creation.
	SetRequestReason(string)
	// GetResolveReason gets the reason for the request's resolution.
	GetResolveReason() string
	// SetResolveReason sets the reason for the request's resolution.
	SetResolveReason(string)
	// GetResolveAnnotations gets the annotations associated with
	// the request's resolution.
	GetResolveAnnotations() map[string][]string
	// SetResolveAnnotations sets the annotations associated with
	// the request's resolution.
	SetResolveAnnotations(map[string][]string)
	// GetSystemAnnotations gets the teleport-applied annotations.
	GetSystemAnnotations() map[string][]string
	// SetSystemAnnotations sets the teleport-applied annotations.
	SetSystemAnnotations(map[string][]string)
	// GetOriginalRoles gets the original (pre-override) role list.
	GetOriginalRoles() []string
	// GetThresholds gets the review thresholds.
	GetThresholds() []AccessReviewThreshold
	// SetThresholds sets the review thresholds (internal use only).
	SetThresholds([]AccessReviewThreshold)
	// GetRoleThresholdMapping gets the rtm.  See documentation of the
	// AccessRequestSpecV3.RoleThresholdMapping field for details.
	GetRoleThresholdMapping() map[string]ThresholdIndexSets
	// SetRoleThresholdMapping sets the rtm (internal use only).  See documentation
	// of the AccessRequestSpecV3.RoleThresholdMapping field for details.
	SetRoleThresholdMapping(map[string]ThresholdIndexSets)
	// GetReviews gets the list of currently applied access reviews.
	GetReviews() []AccessReview
	// SetReviews sets the list of currently applied access reviews (internal use only).
	SetReviews([]AccessReview)
	// GetPromotedAccessListName returns the access list name that this access request
	// was promoted to.
	GetPromotedAccessListName() string
	// SetPromotedAccessListName sets the access list name that this access request
	// was promoted to.
	SetPromotedAccessListName(name string)
	// GetPromotedAccessListTitle returns the access list title that this access request
	// was promoted to.
	GetPromotedAccessListTitle() string
	// SetPromotedAccessListTitle sets the access list title that this access request
	// was promoted to.
	SetPromotedAccessListTitle(string)
	// GetSuggestedReviewers gets the suggested reviewer list.
	GetSuggestedReviewers() []string
	// SetSuggestedReviewers sets the suggested reviewer list.
	SetSuggestedReviewers([]string)
	// GetRequestedResourceIDs gets the resource IDs to which access is being requested.
	GetRequestedResourceIDs() []ResourceID
	// SetRequestedResourceIDs sets the resource IDs to which access is being requested.
	SetRequestedResourceIDs([]ResourceID)
	// GetLoginHint gets the requested login hint.
	GetLoginHint() string
	// SetLoginHint sets the requested login hint.
	SetLoginHint(string)
	// GetMaxDuration gets the maximum time at which the access should be approved for.
	GetMaxDuration() time.Time
	// SetMaxDuration sets the maximum time at which the access should be approved for.
	SetMaxDuration(time.Time)
	// GetDryRun returns true if this request should not be created and is only
	// a dry run to validate request capabilities.
	GetDryRun() bool
	// SetDryRun sets the dry run flag on the request.
	SetDryRun(bool)
	// GetDryRunEnrichment gets the dry run enrichment data.
	GetDryRunEnrichment() *AccessRequestDryRunEnrichment
	// SetDryRunEnrichment sets the dry run enrichment data.
	SetDryRunEnrichment(*AccessRequestDryRunEnrichment)
	// GetRequestKind gets the kind of request.
	GetRequestKind() AccessRequestKind
	// SetRequestKind sets the kind (short/long-term) of request.
	SetRequestKind(AccessRequestKind)
	// Copy returns a copy of the access request resource.
	Copy() AccessRequest
}

// NewAccessRequest assembles an AccessRequest resource.
func NewAccessRequest(name string, user string, roles ...string) (AccessRequest, error) {
	return NewAccessRequestWithResources(name, user, roles, []ResourceID{})
}

// NewAccessRequestWithResources assembles an AccessRequest resource with
// requested resources.
func NewAccessRequestWithResources(name string, user string, roles []string, resourceIDs []ResourceID) (AccessRequest, error) {
	req := AccessRequestV3{
		Metadata: Metadata{
			Name: name,
		},
		Spec: AccessRequestSpecV3{
			User:                 user,
			Roles:                utils.CopyStrings(roles),
			RequestedResourceIDs: append([]ResourceID{}, resourceIDs...),
		},
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// GetUser gets User
func (r *AccessRequestV3) GetUser() string {
	return r.Spec.User
}

// GetRoles gets Roles
func (r *AccessRequestV3) GetRoles() []string {
	return r.Spec.Roles
}

// SetRoles sets Roles
func (r *AccessRequestV3) SetRoles(roles []string) {
	r.Spec.Roles = roles
}

// GetState gets State
func (r *AccessRequestV3) GetState() RequestState {
	return r.Spec.State
}

// SetState sets State
func (r *AccessRequestV3) SetState(state RequestState) error {
	if r.Spec.State.IsDenied() {
		if state.IsDenied() {
			return nil
		}
		return trace.BadParameter("cannot set request-state %q (already denied)", state.String())
	}
	r.Spec.State = state
	return nil
}

// GetCreationTime gets CreationTime
func (r *AccessRequestV3) GetCreationTime() time.Time {
	return r.Spec.Created
}

// SetCreationTime sets CreationTime
func (r *AccessRequestV3) SetCreationTime(t time.Time) {
	r.Spec.Created = t.UTC()
}

// GetAccessExpiry gets AccessExpiry
func (r *AccessRequestV3) GetAccessExpiry() time.Time {
	return r.Spec.Expires
}

// GetAssumeStartTime gets AssumeStartTime
func (r *AccessRequestV3) GetAssumeStartTime() *time.Time {
	return r.Spec.AssumeStartTime
}

// SetAssumeStartTime sets AssumeStartTime
func (r *AccessRequestV3) SetAssumeStartTime(t time.Time) {
	r.Spec.AssumeStartTime = &t
}

// SetAccessExpiry sets AccessExpiry
func (r *AccessRequestV3) SetAccessExpiry(expiry time.Time) {
	r.Spec.Expires = expiry.UTC()
}

// GetSessionTLL gets SessionTLL
func (r *AccessRequestV3) GetSessionTLL() time.Time {
	return r.Spec.SessionTTL
}

// SetSessionTLL sets SessionTLL
func (r *AccessRequestV3) SetSessionTLL(t time.Time) {
	r.Spec.SessionTTL = t.UTC()
}

// GetRequestReason gets RequestReason
func (r *AccessRequestV3) GetRequestReason() string {
	return r.Spec.RequestReason
}

// SetRequestReason sets RequestReason
func (r *AccessRequestV3) SetRequestReason(reason string) {
	r.Spec.RequestReason = reason
}

// GetResolveReason gets ResolveReason
func (r *AccessRequestV3) GetResolveReason() string {
	return r.Spec.ResolveReason
}

// SetResolveReason sets ResolveReason
func (r *AccessRequestV3) SetResolveReason(reason string) {
	r.Spec.ResolveReason = reason
}

// GetResolveAnnotations gets ResolveAnnotations
func (r *AccessRequestV3) GetResolveAnnotations() map[string][]string {
	return r.Spec.ResolveAnnotations
}

// SetResolveAnnotations sets ResolveAnnotations
func (r *AccessRequestV3) SetResolveAnnotations(annotations map[string][]string) {
	r.Spec.ResolveAnnotations = annotations
}

// GetSystemAnnotations gets SystemAnnotations
func (r *AccessRequestV3) GetSystemAnnotations() map[string][]string {
	return r.Spec.SystemAnnotations
}

// SetSystemAnnotations sets SystemAnnotations
func (r *AccessRequestV3) SetSystemAnnotations(annotations map[string][]string) {
	r.Spec.SystemAnnotations = annotations
}

func (r *AccessRequestV3) GetOriginalRoles() []string {
	if l := len(r.Spec.RoleThresholdMapping); l == 0 || l == len(r.Spec.Roles) {
		// rtm is unspecified or original role list is unmodified.  since the rtm
		// keys and role list are identical until role subselection is applied,
		// we can return the role list directly.
		return r.Spec.Roles
	}

	// role subselection has been applied.  calculate original roles
	// by collecting the keys of the rtm.
	roles := make([]string, 0, len(r.Spec.RoleThresholdMapping))
	for role := range r.Spec.RoleThresholdMapping {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

// GetThresholds gets the review thresholds.
func (r *AccessRequestV3) GetThresholds() []AccessReviewThreshold {
	return r.Spec.Thresholds
}

// SetThresholds sets the review thresholds.
func (r *AccessRequestV3) SetThresholds(thresholds []AccessReviewThreshold) {
	r.Spec.Thresholds = thresholds
}

// GetRoleThresholdMapping gets the rtm.
func (r *AccessRequestV3) GetRoleThresholdMapping() map[string]ThresholdIndexSets {
	return r.Spec.RoleThresholdMapping
}

// SetRoleThresholdMapping sets the rtm (internal use only).
func (r *AccessRequestV3) SetRoleThresholdMapping(rtm map[string]ThresholdIndexSets) {
	r.Spec.RoleThresholdMapping = rtm
}

// SetReviews sets the list of currently applied access reviews.
func (r *AccessRequestV3) SetReviews(revs []AccessReview) {
	utcRevs := make([]AccessReview, len(revs))
	for i, rev := range revs {
		utcRevs[i] = rev
		utcRevs[i].Created = rev.Created.UTC()
	}
	r.Spec.Reviews = utcRevs
}

// GetReviews gets the list of currently applied access reviews.
func (r *AccessRequestV3) GetReviews() []AccessReview {
	return r.Spec.Reviews
}

// GetSuggestedReviewers gets the suggested reviewer list.
func (r *AccessRequestV3) GetSuggestedReviewers() []string {
	return r.Spec.SuggestedReviewers
}

// SetSuggestedReviewers sets the suggested reviewer list.
func (r *AccessRequestV3) SetSuggestedReviewers(reviewers []string) {
	r.Spec.SuggestedReviewers = reviewers
}

// GetPromotedAccessListName returns PromotedAccessListName.
func (r *AccessRequestV3) GetPromotedAccessListName() string {
	if r.Spec.AccessList == nil {
		return ""
	}
	return r.Spec.AccessList.Name
}

// SetPromotedAccessListName sets PromotedAccessListName.
func (r *AccessRequestV3) SetPromotedAccessListName(name string) {
	if r.Spec.AccessList == nil {
		r.Spec.AccessList = &PromotedAccessList{}
	}
	r.Spec.AccessList.Name = name
}

// GetPromotedAccessListTitle returns PromotedAccessListTitle.
func (r *AccessRequestV3) GetPromotedAccessListTitle() string {
	if r.Spec.AccessList == nil {
		return ""
	}
	return r.Spec.AccessList.Title
}

// SetPromotedAccessListTitle sets PromotedAccessListTitle.
func (r *AccessRequestV3) SetPromotedAccessListTitle(title string) {
	if r.Spec.AccessList == nil {
		r.Spec.AccessList = &PromotedAccessList{}
	}
	r.Spec.AccessList.Title = title
}

// setStaticFields sets static resource header and metadata fields.
func (r *AccessRequestV3) setStaticFields() {
	r.Kind = KindAccessRequest
	r.Version = V3
}

// CheckAndSetDefaults validates set values and sets default values
func (r *AccessRequestV3) CheckAndSetDefaults() error {
	r.setStaticFields()
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if r.Spec.State.IsNone() {
		r.Spec.State = RequestState_PENDING
	}

	if r.GetState().IsPending() {
		if r.GetResolveReason() != "" {
			return trace.BadParameter("pending requests cannot include resolve reason")
		}
		if len(r.GetResolveAnnotations()) != 0 {
			return trace.BadParameter("pending requests cannot include resolve annotations")
		}
	}

	if r.GetUser() == "" {
		return trace.BadParameter("access request user name not set")
	}

	if r.Spec.Roles == nil {
		r.Spec.Roles = []string{}
	}
	if r.Spec.RequestedResourceIDs == nil {
		r.Spec.RequestedResourceIDs = []ResourceID{}
	}
	if len(r.GetRoles()) == 0 && len(r.GetRequestedResourceIDs()) == 0 {
		return trace.BadParameter("access request does not specify any roles or resources")
	}

	// dedupe and sort roles to simplify comparing role lists
	r.Spec.Roles = utils.Deduplicate(r.Spec.Roles)
	sort.Strings(r.Spec.Roles)

	return nil
}

// GetKind gets Kind
func (r *AccessRequestV3) GetKind() string {
	return r.Kind
}

// GetSubKind gets SubKind
func (r *AccessRequestV3) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets SubKind
func (r *AccessRequestV3) SetSubKind(subKind string) {
	r.SubKind = subKind
}

// GetVersion gets Version
func (r *AccessRequestV3) GetVersion() string {
	return r.Version
}

// GetName gets Name
func (r *AccessRequestV3) GetName() string {
	return r.Metadata.Name
}

// SetName sets Name
func (r *AccessRequestV3) SetName(name string) {
	r.Metadata.Name = name
}

// Expiry gets Expiry
func (r *AccessRequestV3) Expiry() time.Time {
	// Fallback on existing expiry in metadata if not set in spec.
	if r.Spec.ResourceExpiry != nil {
		return *r.Spec.ResourceExpiry
	}
	return r.Metadata.Expiry()
}

// SetExpiry sets Expiry
func (r *AccessRequestV3) SetExpiry(expiry time.Time) {
	t := expiry.UTC()
	r.Spec.ResourceExpiry = &t
}

// GetMetadata gets Metadata
func (r *AccessRequestV3) GetMetadata() Metadata {
	return r.Metadata
}

// GetRevision returns the revision
func (r *AccessRequestV3) GetRevision() string {
	return r.Metadata.GetRevision()
}

// SetRevision sets the revision
func (r *AccessRequestV3) SetRevision(rev string) {
	r.Metadata.SetRevision(rev)
}

// GetRequestedResourceIDs gets the resource IDs to which access is being requested.
func (r *AccessRequestV3) GetRequestedResourceIDs() []ResourceID {
	return append([]ResourceID{}, r.Spec.RequestedResourceIDs...)
}

// SetRequestedResourceIDs sets the resource IDs to which access is being requested.
func (r *AccessRequestV3) SetRequestedResourceIDs(ids []ResourceID) {
	r.Spec.RequestedResourceIDs = append([]ResourceID{}, ids...)
}

// GetLoginHint gets the requested login hint.
func (r *AccessRequestV3) GetLoginHint() string {
	return r.Spec.LoginHint
}

// SetLoginHint sets the requested login hint.
func (r *AccessRequestV3) SetLoginHint(login string) {
	r.Spec.LoginHint = login
}

// GetDryRun returns true if this request should not be created and is only
// a dry run to validate request capabilities.
func (r *AccessRequestV3) GetDryRun() bool {
	return r.Spec.DryRun
}

// GetMaxDuration gets the maximum time at which the access should be approved for.
func (r *AccessRequestV3) GetMaxDuration() time.Time {
	return r.Spec.MaxDuration
}

// SetMaxDuration sets the maximum time at which the access should be approved for.
func (r *AccessRequestV3) SetMaxDuration(t time.Time) {
	r.Spec.MaxDuration = t
}

// SetDryRun sets the dry run flag on the request.
func (r *AccessRequestV3) SetDryRun(dryRun bool) {
	r.Spec.DryRun = dryRun
}

// GetDryRunEnrichment gets the dry run enrichment data.
func (r *AccessRequestV3) GetDryRunEnrichment() *AccessRequestDryRunEnrichment {
	return r.Spec.DryRunEnrichment
}

// SetDryRunEnrichment sets the dry run enrichment data.
func (r *AccessRequestV3) SetDryRunEnrichment(enrichment *AccessRequestDryRunEnrichment) {
	r.Spec.DryRunEnrichment = enrichment
}

// GetRequestKind gets the kind of request.
func (r *AccessRequestV3) GetRequestKind() AccessRequestKind {
	return r.Spec.RequestKind
}

// SetRequestKind sets the kind (short/long-term) of request.
func (r *AccessRequestV3) SetRequestKind(kind AccessRequestKind) {
	r.Spec.RequestKind = kind
}

// Copy returns a copy of the access request resource.
func (r *AccessRequestV3) Copy() AccessRequest {
	return utils.CloneProtoMsg(r)
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (r *AccessRequestV3) GetLabel(key string) (value string, ok bool) {
	v, ok := r.Metadata.Labels[key]
	return v, ok
}

// GetStaticLabels returns the access request static labels.
func (r *AccessRequestV3) GetStaticLabels() map[string]string {
	return r.Metadata.Labels
}

// SetStaticLabels sets the access request static labels.
func (r *AccessRequestV3) SetStaticLabels(sl map[string]string) {
	r.Metadata.Labels = sl
}

// GetAllLabels returns the access request static labels.
func (r *AccessRequestV3) GetAllLabels() map[string]string {
	return r.Metadata.Labels
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (r *AccessRequestV3) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(r.GetAllLabels()), r.GetName(), r.GetUser())
	fieldVals = append(fieldVals, r.GetRoles()...)
	for _, resource := range r.GetRequestedResourceIDs() {
		fieldVals = append(fieldVals, resource.Name)
	}
	return MatchSearch(fieldVals, values, nil)
}

// Origin returns the origin value of the resource.
func (r *AccessRequestV3) Origin() string {
	return r.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (r *AccessRequestV3) SetOrigin(origin string) {
	r.Metadata.SetOrigin(origin)
}

// String returns a text representation of this AccessRequest
func (r *AccessRequestV3) String() string {
	return fmt.Sprintf("AccessRequest(user=%v,roles=%+v)", r.Spec.User, r.Spec.Roles)
}

func (c AccessReviewConditions) IsZero() bool {
	return reflect.ValueOf(c).IsZero()
}

func (s AccessReviewSubmission) Check() error {
	if s.RequestID == "" {
		return trace.BadParameter("missing request ID")
	}

	return s.Review.Check()
}

func (s AccessReview) Check() error {
	if s.Author == "" {
		return trace.BadParameter("missing review author")
	}

	return nil
}

// GetAccessListName returns the access list name used for the promotion.
func (s AccessReview) GetAccessListName() string {
	if s.AccessList == nil {
		return ""
	}
	return s.AccessList.Name
}

// GetAccessListTitle returns the access list title used for the promotion.
func (s AccessReview) GetAccessListTitle() string {
	if s.AccessList == nil {
		return ""
	}
	return s.AccessList.Title
}

// IsEqual t is equivalent to the provide AccessReviewThreshold.
func (t *AccessReviewThreshold) IsEqual(o *AccessReviewThreshold) bool {
	return deriveTeleportEqualAccessReviewThreshold(t, o)
}

// AccessRequestUpdate encompasses the parameters of a
// SetAccessRequestState call.
type AccessRequestUpdate struct {
	// RequestID is the ID of the request to be updated.
	RequestID string
	// State is the state that the target request
	// should resolve to.
	State RequestState
	// Reason is an optional description of *why* the
	// the request is being resolved.
	Reason string
	// Annotations supplies extra data associated with
	// the resolution; primarily for audit purposes.
	Annotations map[string][]string
	// Roles, if non-empty declares a list of roles
	// that should override the role list of the request.
	// This parameter is only accepted on approvals
	// and must be a subset of the role list originally
	// present on the request.
	Roles []string
	// AssumeStartTime sets the time the requestor can assume
	// the requested roles.
	AssumeStartTime *time.Time
}

// Check validates the request's fields
func (u *AccessRequestUpdate) Check() error {
	if u.RequestID == "" {
		return trace.BadParameter("missing request id")
	}
	if u.State.IsNone() {
		return trace.BadParameter("missing request state")
	}
	if len(u.Roles) > 0 && !u.State.IsApproved() {
		return trace.BadParameter("cannot override roles when setting state: %s", u.State)
	}
	return nil
}

// RequestReasonMode can be either "required" or "optional". Empty-string is treated as "optional".
// If a role has the request reason mode set to "required", then reason is required for all Access
// Requests requesting roles or resources allowed by this role. It applies only to users who have
// this role assigned.
type RequestReasonMode string

const (
	// RequestReasonModeRequired indicates required mode. See [[RequestReasonMode]] godoc for
	// more details.
	RequestReasonModeRequired RequestReasonMode = "required"
	// RequestReasonModeRequired indicates optional mode. See [[RequestReasonMode]] godoc for
	// more details.
	RequestReasonModeOptional RequestReasonMode = "optional"
)

var allRequestReasonModes = []RequestReasonMode{
	RequestReasonModeRequired,
	RequestReasonModeOptional,
}

// Required checks if this mode is "required". Empty mode is treated as "optional".
func (m RequestReasonMode) Required() bool {
	switch m {
	case RequestReasonModeRequired:
		return true
	default:
		return false
	}
}

// Check validates this mode value. Note that an empty value is considered invalid.
func (m RequestReasonMode) Check() error {
	for _, x := range allRequestReasonModes {
		if m == x {
			return nil
		}
	}
	return trace.BadParameter("unrecognized request reason mode %q, must be one of: %v",
		m, allRequestReasonModes)
}

// RequestStrategy is an indicator of how access requests
// should be handled for holders of a given role.
type RequestStrategy string

const (
	// RequestStrategyOptional is the default request strategy,
	// indicating that no special actions/requirements exist.
	RequestStrategyOptional RequestStrategy = "optional"

	// RequestStrategyReason indicates that client implementations
	// should automatically generate wildcard requests on login, and
	// users should be prompted for a reason.
	RequestStrategyReason RequestStrategy = "reason"

	// RequestStrategyAlways indicates that client implementations
	// should automatically generate wildcard requests on login, but
	// that reasons are not required.
	RequestStrategyAlways RequestStrategy = "always"
)

// ShouldAutoRequest checks if the request strategy
// indicates that a request should be automatically
// generated on login.
func (s RequestStrategy) ShouldAutoRequest() bool {
	switch s {
	case RequestStrategyReason, RequestStrategyAlways:
		return true
	default:
		return false
	}
}

// RequireReason checks if the request strategy
// is one that requires users to always supply
// reasons with their requests.
func (s RequestStrategy) RequireReason() bool {
	return s == RequestStrategyReason
}

// stateVariants allows iteration of the expected variants
// of RequestState.
var stateVariants = [5]RequestState{
	RequestState_NONE,
	RequestState_PENDING,
	RequestState_APPROVED,
	RequestState_DENIED,
	RequestState_PROMOTED,
}

// Parse attempts to interpret a value as a string representation
// of a RequestState.
func (s *RequestState) Parse(val string) error {
	for _, state := range stateVariants {
		if state.String() == val {
			*s = state
			return nil
		}
	}
	return trace.BadParameter("unknown request state: %q", val)
}

// IsNone request state
func (s RequestState) IsNone() bool {
	return s == RequestState_NONE
}

// IsPending request state
func (s RequestState) IsPending() bool {
	return s == RequestState_PENDING
}

// IsApproved request state
func (s RequestState) IsApproved() bool {
	return s == RequestState_APPROVED
}

// IsDenied request state
func (s RequestState) IsDenied() bool {
	return s == RequestState_DENIED
}

// IsPromoted returns true is the request in the PROMOTED state.
func (s RequestState) IsPromoted() bool {
	return s == RequestState_PROMOTED
}

// IsResolved request state
func (s RequestState) IsResolved() bool {
	return s.IsApproved() || s.IsDenied() || s.IsPromoted()
}

// key values for map encoding of request filter
const (
	keyID    = "id"
	keyUser  = "user"
	keyState = "state"
)

// IntoMap copies AccessRequestFilter values into a map
func (f *AccessRequestFilter) IntoMap() map[string]string {
	m := make(map[string]string)
	if f.ID != "" {
		m[keyID] = f.ID
	}
	if f.User != "" {
		m[keyUser] = f.User
	}
	if !f.State.IsNone() {
		m[keyState] = f.State.String()
	}
	return m
}

// FromMap copies values from a map into this AccessRequestFilter value
func (f *AccessRequestFilter) FromMap(m map[string]string) error {
	for key, val := range m {
		switch key {
		case keyID:
			f.ID = val
		case keyUser:
			f.User = val
		case keyState:
			if err := f.State.Parse(val); err != nil {
				return trace.Wrap(err)
			}
		default:
			return trace.BadParameter("unknown filter key %s", key)
		}
	}
	return nil
}

func hasReviewed(req AccessRequest, author string) bool {
	reviews := req.GetReviews()
	var reviewers []string
	for _, review := range reviews {
		reviewers = append(reviewers, review.Author)
	}
	return slices.Contains(reviewers, author)
}

// Match checks if a given access request matches this filter.
func (f *AccessRequestFilter) Match(req AccessRequest) bool {
	// only return if the request was made by the api requester
	if f.Scope == AccessRequestScope_MY_REQUESTS && req.GetUser() != f.Requester {
		return false
	}
	// a user cannot review their own requests
	if f.Scope == AccessRequestScope_NEEDS_REVIEW {
		if req.GetUser() == f.Requester {
			return false
		}
		if req.GetState() != RequestState_PENDING {
			return false
		}
		if hasReviewed(req, f.Requester) {
			return false
		}
	}
	// only match if the api requester has submit a review
	if f.Scope == AccessRequestScope_REVIEWED {
		// users cant review their own requests so we can early return
		if req.GetUser() == f.Requester {
			return false
		}
		if !hasReviewed(req, f.Requester) {
			return false
		}
	}
	if !req.MatchSearch(f.SearchKeywords) {
		return false
	}
	if f.ID != "" && req.GetName() != f.ID {
		return false
	}
	if f.User != "" && req.GetUser() != f.User {
		return false
	}
	if !f.State.IsNone() && req.GetState() != f.State {
		return false
	}
	return true
}

// AccessRequests is a list of AccessRequest resources.
type AccessRequests []AccessRequest

// ToMap returns these access requests as a map keyed by access request name.
func (a AccessRequests) ToMap() map[string]AccessRequest {
	m := make(map[string]AccessRequest)
	for _, accessRequest := range a {
		m[accessRequest.GetName()] = accessRequest
	}
	return m
}

// AsResources returns these access requests as resources with labels.
func (a AccessRequests) AsResources() (resources ResourcesWithLabels) {
	for _, accessRequest := range a {
		resources = append(resources, accessRequest)
	}
	return resources
}

// Len returns the slice length.
func (a AccessRequests) Len() int { return len(a) }

// Less compares access requests by name.
func (a AccessRequests) Less(i, j int) bool { return a[i].GetName() < a[j].GetName() }

// Swap swaps two access requests.
func (a AccessRequests) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// NewAccessRequestAllowedPromotions returns a new AccessRequestAllowedPromotions resource.
func NewAccessRequestAllowedPromotions(promotions []*AccessRequestAllowedPromotion) *AccessRequestAllowedPromotions {
	if promotions == nil {
		promotions = make([]*AccessRequestAllowedPromotion, 0)
	}

	return &AccessRequestAllowedPromotions{
		Promotions: promotions,
	}
}

// ValidateAssumeStartTime returns error if start time is in an invalid range.
func ValidateAssumeStartTime(assumeStartTime time.Time, accessExpiry time.Time, creationTime time.Time) error {
	// Guard against requesting a start time before the request creation time.
	if assumeStartTime.Before(creationTime) {
		return trace.BadParameter("assume start time has to be after %v", creationTime.Format(time.RFC3339))
	}
	// Guard against requesting a start time after access expiry.
	if assumeStartTime.After(accessExpiry) || assumeStartTime.Equal(accessExpiry) {
		return trace.BadParameter("assume start time must be prior to access expiry time at %v",
			accessExpiry.Format(time.RFC3339))
	}
	// Access expiry can be greater than constants.MaxAssumeStartDuration, but start time
	// should be on or before constants.MaxAssumeStartDuration.
	maxAssumableStartTime := creationTime.Add(constants.MaxAssumeStartDuration)
	if maxAssumableStartTime.Before(accessExpiry) && assumeStartTime.After(maxAssumableStartTime) {
		return trace.BadParameter("assume start time is too far in the future, latest time allowed is %v",
			maxAssumableStartTime.Format(time.RFC3339))
	}

	return nil
}
