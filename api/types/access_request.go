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
	"sort"
	"time"

	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
)

// AccessRequest is a request for temporarily granted roles
type AccessRequest interface {
	Resource
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
	// GetAccessExpiry gets the upper limit for which this request
	// may be considered active.
	GetAccessExpiry() time.Time
	// SetAccessExpiry sets the upper limit for which this request
	// may be considered active.
	SetAccessExpiry(time.Time)
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
	// GetSuggestedReviewers gets the suggested reviewer list.
	GetSuggestedReviewers() []string
	// SetSuggestedReviewers sets the suggested reviewer list.
	SetSuggestedReviewers([]string)
	// CheckAndSetDefaults validates the access request and
	// supplies default values where appropriate.
	CheckAndSetDefaults() error
	// Equals checks equality between access request values.
	Equals(AccessRequest) bool
}

// NewAccessRequest assembled an AccessRequest resource.
func NewAccessRequest(name string, user string, roles ...string) (AccessRequest, error) {
	req := AccessRequestV3{
		Kind:    KindAccessRequest,
		Version: V3,
		Metadata: Metadata{
			Name: name,
		},
		Spec: AccessRequestSpecV3{
			User:  user,
			Roles: roles,
			State: RequestState_PENDING,
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
	r.Spec.Created = t
}

// GetAccessExpiry gets AccessExpiry
func (r *AccessRequestV3) GetAccessExpiry() time.Time {
	return r.Spec.Expires
}

// SetAccessExpiry sets AccessExpiry
func (r *AccessRequestV3) SetAccessExpiry(expiry time.Time) {
	r.Spec.Expires = expiry
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
	r.Spec.Reviews = revs
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

// CheckAndSetDefaults validates set values and sets default values
func (r *AccessRequestV3) CheckAndSetDefaults() error {
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.GetState().IsNone() {
		if err := r.SetState(RequestState_PENDING); err != nil {
			return trace.Wrap(err)
		}
	}

	// dedupe and sort roles to simplify comparing role lists
	r.Spec.Roles = utils.Deduplicate(r.Spec.Roles)
	sort.Strings(r.Spec.Roles)

	if err := r.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Check validates AccessRequest values
func (r *AccessRequestV3) Check() error {
	if r.Kind == "" {
		return trace.BadParameter("access request kind not set")
	}
	if r.Version == "" {
		return trace.BadParameter("access request version not set")
	}
	if r.GetName() == "" {
		return trace.BadParameter("access request id not set")
	}
	if r.GetUser() == "" {
		return trace.BadParameter("access request user name not set")
	}
	if len(r.GetRoles()) < 1 {
		return trace.BadParameter("access request does not specify any roles")
	}
	if r.GetState().IsPending() {
		if r.GetResolveReason() != "" {
			return trace.BadParameter("pending requests cannot include resolve reason")
		}
		if len(r.GetResolveAnnotations()) != 0 {
			return trace.BadParameter("pending requests cannot include resolve annotations")
		}
	}
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
	return r.Metadata.Expiry()
}

// SetExpiry sets Expiry
func (r *AccessRequestV3) SetExpiry(expiry time.Time) {
	r.Metadata.SetExpiry(expiry)
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (r *AccessRequestV3) SetTTL(clock Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// GetMetadata gets Metadata
func (r *AccessRequestV3) GetMetadata() Metadata {
	return r.Metadata
}

// GetResourceID gets ResourceID
func (r *AccessRequestV3) GetResourceID() int64 {
	return r.Metadata.GetID()
}

// SetResourceID sets ResourceID
func (r *AccessRequestV3) SetResourceID(id int64) {
	r.Metadata.SetID(id)
}

// String returns a text representation of this AccessRequest
func (r *AccessRequestV3) String() string {
	return fmt.Sprintf("AccessRequest(user=%v,roles=%+v)", r.Spec.User, r.Spec.Roles)
}

// Equals compares two AccessRequests
func (r *AccessRequestV3) Equals(other AccessRequest) bool {
	o, ok := other.(*AccessRequestV3)
	if !ok {
		return false
	}
	if r.GetName() != o.GetName() {
		return false
	}
	return r.Spec.Equals(&o.Spec)
}

func (t AccessReviewThreshold) Equals(other AccessReviewThreshold) bool {
	return reflect.DeepEqual(t, other)
}

func (c AccessReviewConditions) IsZero() bool {
	return reflect.ValueOf(c).IsZero()
}

func (c AccessRequestConditions) IsZero() bool {
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
var stateVariants = [4]RequestState{
	RequestState_NONE,
	RequestState_PENDING,
	RequestState_APPROVED,
	RequestState_DENIED,
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

// IsResolved request state
func (s RequestState) IsResolved() bool {
	return s.IsApproved() || s.IsDenied()
}

// Equals compares two AccessRequestSpecs
func (s *AccessRequestSpecV3) Equals(other *AccessRequestSpecV3) bool {
	if s.User != other.User {
		return false
	}
	if len(s.Roles) != len(other.Roles) {
		return false
	}
	for i, role := range s.Roles {
		if role != other.Roles[i] {
			return false
		}
	}
	if s.Created != other.Created {
		return false
	}
	if s.Expires != other.Expires {
		return false
	}
	return s.State == other.State
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

// Match checks if a given access request matches this filter.
func (f *AccessRequestFilter) Match(req AccessRequest) bool {
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

// Equals compares two AccessRequestFilters
func (f *AccessRequestFilter) Equals(o AccessRequestFilter) bool {
	return f.ID == o.ID && f.User == o.User && f.State == o.State
}
