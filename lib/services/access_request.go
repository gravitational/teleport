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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"

	"github.com/gravitational/trace"

	"github.com/pborman/uuid"
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

// DynamicAccess is a service which manages dynamic RBAC.
type DynamicAccess interface {
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
	if req.RequestableRoles {
		v, err := NewRequestValidator(clt, req.User)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		caps.RequestableRoles, err = v.GetRequestableRoles()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &caps, nil
}

// DynamicAccessExt is an extended dynamic access interface
// used to implement some auth server internals.
type DynamicAccessExt interface {
	DynamicAccess
	// UpsertAccessRequest creates or updates an access request.
	UpsertAccessRequest(ctx context.Context, req AccessRequest) error
	// DeleteAllAccessRequests deletes all existent access requests.
	DeleteAllAccessRequests(ctx context.Context) error
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
func GetTraitMappings(c AccessRequestConditions) TraitMappingSet {
	tm := make([]TraitMapping, 0, len(c.ClaimsToRoles))
	for _, mapping := range c.ClaimsToRoles {
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
	GetRoles() ([]Role, error)
}

// appendRoleMatchers constructs all role matchers for a given
// AccessRequestConditions instance and appends them to the
// supplied matcher slice.
func appendRoleMatchers(matchers []parse.Matcher, conditions AccessRequestConditions, traits map[string][]string) ([]parse.Matcher, error) {
	// build matchers for the role list
	for _, r := range conditions.Roles {
		m, err := parse.NewMatcher(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		matchers = append(matchers, m)
	}

	// build matchers for all role mappings
	ms, err := TraitsToRoleMatchers(GetTraitMappings(conditions), traits)
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
		expandRoles, annotate bool
	}
	Roles struct {
		Allow, Deny []parse.Matcher
	}
	Annotations struct {
		Allow, Deny map[string][]string
	}
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
	if m.opts.annotate {
		// validation process for incoming access requests requires
		// generating system annotations to be attached to the request
		// before it is inserted into the backend.
		m.Annotations.Allow = make(map[string][]string)
		m.Annotations.Deny = make(map[string][]string)
	}

	// load all statically assigned roles for the user and
	// use them to build our validation state.
	for _, roleName := range m.user.GetRoles() {
		role, err := m.getter.GetRole(roleName)
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

		if !m.opts.expandRoles {
			// teleport always validates new incoming pending access requests
			// with ExpandRoles(true). after that, it should be impossible to
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

	if m.opts.annotate {
		// incoming requests must have system annotations attached
		// before being inserted into the backend. this is how the
		// RBAC system propagates sideband information to plugins.
		req.SetSystemAnnotations(m.SystemAnnotations())
	}
	return nil
}

// GetRequestableRoles gets the list of all existent roles which the user is
// able to request.  This operation is expensive since it loads all existent
// roles in order to determine the role list.  Prefer calling CanRequestRole
// when checking againt a known role list.
func (m *RequestValidator) GetRequestableRoles() ([]string, error) {
	allRoles, err := m.getter.GetRoles()
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

	m.Roles.Deny, err = appendRoleMatchers(m.Roles.Deny, deny, m.user.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	m.Roles.Allow, err = appendRoleMatchers(m.Roles.Allow, allow, m.user.GetTraits())
	if err != nil {
		return trace.Wrap(err)
	}

	if m.opts.annotate {
		// validation process for incoming access requests requires
		// generating system annotations to be attached to the request
		// before it is inserted into the backend.
		insertAnnotations(m.Annotations.Deny, deny, m.user.GetTraits())
		insertAnnotations(m.Annotations.Allow, allow, m.user.GetTraits())
	}
	return nil
}

// CanRequestRole checks if a given role can be requested.
func (m *RequestValidator) CanRequestRole(name string) bool {
	for _, deny := range m.Roles.Deny {
		if deny.Match(name) {
			return false
		}
	}
	for _, allow := range m.Roles.Allow {
		if allow.Match(name) {
			return true
		}
	}
	return false
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

// ExpandRoles activates expansion of wildcard role lists
// (`[]string{"*"}`) when true.
func ExpandRoles(expand bool) ValidateRequestOption {
	return func(v *RequestValidator) {
		v.opts.expandRoles = expand
	}
}

// ApplySystemAnnotations causes system annotations to be computed
// and attached during validation when true.
func ApplySystemAnnotations(annotate bool) ValidateRequestOption {
	return func(v *RequestValidator) {
		v.opts.annotate = annotate
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
		"system_annotations": { "type": "object" }
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
func MarshalAccessRequest(req AccessRequest, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch r := req.(type) {
	case *AccessRequestV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			cp := *r
			cp.SetResourceID(0)
			r = &cp
		}
		return utils.FastMarshal(r)
	default:
		return nil, trace.BadParameter("unrecognized access request type: %T", req)
	}
}
