/*
Copyright 2016 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"
	"github.com/jonboulle/clockwork"
)

// DefaultUserRules provides access to the default set of rules assigned to
// all users.
var DefaultUserRules = map[string][]string{
	KindRole:           RO(),
	KindOIDC:           RO(),
	KindSAML:           RO(),
	KindSession:        RO(),
	KindTrustedCluster: RW(),
}

// DefaultImplicitRules provides access to the default set of implicit rules
// assigned to all roles.
var DefaultImplicitRules = map[string][]string{
	KindNode:          RO(),
	KindAuthServer:    RO(),
	KindReverseTunnel: RO(),
	KindCertAuthority: RO(),
}

// DefaultCertAuthorityRules provides access the minimal set of resources
// needed for a certificate authority to function.
var DefaultCertAuthorityRules = map[string][]string{
	KindSession:       RO(),
	KindNode:          RO(),
	KindAuthServer:    RO(),
	KindReverseTunnel: RO(),
	KindCertAuthority: RO(),
}

// RoleNameForUser returns role name associated with a user.
func RoleNameForUser(name string) string {
	return "user:" + name
}

// RoleNameForCertAuthority returns role name associated with a certificate
// authority.
func RoleNameForCertAuthority(name string) string {
	return "ca:" + name
}

// NewDefaultRole is the default role for all local users if another role
// is not explicitly assigned (Enterprise only).
func NewDefaultRole() Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      teleport.DefaultRoleName,
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Logins:     []string{teleport.TraitInternalRoleVariable},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      utils.CopyStringMapSlices(DefaultUserRules),
			},
		},
	}
}

// NewImplicitRole is the default implicit role that gets added to all
// RoleSets.
func NewImplicitRole() Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      teleport.DefaultImplicitRole,
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Logins:     []string{teleport.TraitInternalRoleVariable},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      utils.CopyStringMapSlices(DefaultImplicitRules),
			},
		},
	}
}

// RoleForUser creates role for a services.User.
func RoleForUser(u User) Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      RoleNameForUser(u.GetName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      utils.CopyStringMapSlices(DefaultUserRules),
			},
		},
	}
}

// RoleForCertauthority creates role using services.CertAuthority.
func RoleForCertAuthority(ca CertAuthority) Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      RoleNameForCertAuthority(ca.GetClusterName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      utils.CopyStringMapSlices(DefaultCertAuthorityRules),
			},
		},
	}
}

// ConvertV1CertAuthority converts V1 cert authority for new CA and Role
func ConvertV1CertAuthority(v1 *CertAuthorityV1) (CertAuthority, Role) {
	ca := v1.V2()
	role := RoleForCertAuthority(ca)
	role.SetLogins(Allow, v1.AllowedLogins)
	ca.AddRole(role.GetName())
	return ca, role
}

// Access service manages roles and permissions
type Access interface {
	// GetRoles returns a list of roles
	GetRoles() ([]Role, error)

	// UpsertRole creates or updates role
	UpsertRole(role Role, ttl time.Duration) error

	// DeleteAllRoles deletes all roles
	DeleteAllRoles() error

	// GetRole returns role by name
	GetRole(name string) (Role, error)

	// DeleteRole deletes role by name
	DeleteRole(name string) error
}

const (
	// ForwardAgent is SSH agent forwarding.
	ForwardAgent = "forward_agent"

	// MaxSessionTTL defines how long a SSH session can last for.
	MaxSessionTTL = "max_session_ttl"
)

const (
	// Allow is the set of conditions that allow access.
	Allow RoleConditionType = true
	// Deny is the set of conditions that prevent access.
	Deny RoleConditionType = false
)

// RoleConditionType specifies if it's an allow rule (true) or deny rule (false).
type RoleConditionType bool

// Role contains a set of permissions or settings
type Role interface {
	// Resource provides common resource methods.
	Resource
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// Equals returns true if the roles are equal. Roles are equal if options and
	// conditions match.
	Equals(other Role) bool
	// ApplyTraits applies the passed in traits to any variables within the role
	// and returns itself.
	ApplyTraits(map[string][]string) Role
	// GetRawObject returns the raw object stored in the backend without any
	// conversions applied, used in migrations.
	GetRawObject() interface{}

	// GetOptions gets role options.
	GetOptions() RoleOptions
	// SetOptions sets role options
	SetOptions(opt RoleOptions)

	// GetLogins gets *nix system logins for allow or deny condition.
	GetLogins(RoleConditionType) []string
	// SetLogins sets *nix system logins for allow or deny condition.
	SetLogins(RoleConditionType, []string)

	// GetNamespaces gets a list of namespaces this role is allowed or denied access to.
	GetNamespaces(RoleConditionType) []string
	// GetNamespaces sets a list of namespaces this role is allowed or denied access to.
	SetNamespaces(RoleConditionType, []string)

	// GetNodeLabels gets the map of node labels this role is allowed or denied access to.
	GetNodeLabels(RoleConditionType) map[string]string
	// SetNodeLabels sets the map of node labels this role is allowed or denied access to.
	SetNodeLabels(RoleConditionType, map[string]string)

	// GetRules gets all allow or deny rules.
	GetRules(rct RoleConditionType) map[string][]string
	// SetRules sets an allow or deny rule.
	SetRules(rct RoleConditionType, rrs map[string][]string)
}

// RoleV3 represents role resource specification
type RoleV3 struct {
	// Kind is the type of resource.
	Kind string `json:"kind"`
	// Version is the resource version.
	Version string `json:"version"`
	// Metadata is resource metadata.
	Metadata Metadata `json:"metadata"`
	// Spec contains resource specification.
	Spec RoleSpecV3 `json:"spec"`
	// rawObject is the raw object stored in the backend without any
	// conversions applied, used in migrations.
	rawObject interface{}
}

// Equals returns true if the roles are equal. Roles are equal if options,
// namespaces, logins, labels, and conditions match.
func (r *RoleV3) Equals(other Role) bool {
	if !r.GetOptions().Equals(other.GetOptions()) {
		return false
	}

	for _, condition := range []RoleConditionType{Allow, Deny} {
		if !utils.StringSlicesEqual(r.GetLogins(condition), other.GetLogins(condition)) {
			return false
		}
		if !utils.StringSlicesEqual(r.GetNamespaces(condition), other.GetNamespaces(condition)) {
			return false
		}
		if !utils.StringMapsEqual(r.GetNodeLabels(condition), other.GetNodeLabels(condition)) {
			return false
		}

		if !utils.StringMapSlicesEqual(r.GetRules(condition), other.GetRules(condition)) {
			return false
		}
	}

	return true
}

// ApplyTraits applies the passed in traits to any variables within the role
// and returns itself.
func (r *RoleV3) ApplyTraits(traits map[string][]string) Role {
	for _, condition := range []RoleConditionType{Allow, Deny} {
		inLogins := r.GetLogins(condition)

		var outLogins []string
		for _, login := range inLogins {
			// extract the variablePrefix and variableName from the role variable
			variablePrefix, variableName, err := parse.IsRoleVariable(login)

			// if we didn't find a variable (found a normal login) then append it and
			// go on to the next login
			if trace.IsNotFound(err) {
				outLogins = append(outLogins, login)
				continue
			}

			// for internal traits, we only support internal.logins at the moment
			if variablePrefix == teleport.TraitInternalPrefix {
				if variableName != teleport.TraitLogins {
					continue
				}
			}

			// if we can't find the variable in the traits, skip it
			variableValue, ok := traits[variableName]
			if !ok {
				continue
			}

			// we found the variable in the traits, append it to the list of logins
			outLogins = append(outLogins, variableValue...)
		}

		r.SetLogins(condition, utils.Deduplicate(outLogins))
	}

	return r
}

// GetRawObject returns the raw object stored in the backend without any
// conversions applied, used in migrations.
func (r *RoleV3) GetRawObject() interface{} {
	return r.rawObject
}

// SetExpiry sets expiry time for the object.
func (r *RoleV3) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns the expiry time for the object.
func (r *RoleV3) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets TTL header using realtime clock.
func (r *RoleV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// SetName sets the role name and is a shortcut for SetMetadata().Name.
func (r *RoleV3) SetName(s string) {
	r.Metadata.Name = s
}

// GetName gets the role name and is a shortcut for GetMetadata().Name.
func (r *RoleV3) GetName() string {
	return r.Metadata.Name
}

// GetMetadata returns role metadata.
func (r *RoleV3) GetMetadata() Metadata {
	return r.Metadata
}

// GetOptions gets role options.
func (r *RoleV3) GetOptions() RoleOptions {
	return r.Spec.Options
}

// SetOptions sets role options.
func (r *RoleV3) SetOptions(options RoleOptions) {
	r.Spec.Options = utils.CopyStringMapInterface(options)
}

// GetLogins gets system logins for allow or deny condition.
func (r *RoleV3) GetLogins(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.Logins
	}
	return r.Spec.Deny.Logins
}

// SetLogins sets system logins for allow or deny condition.
func (r *RoleV3) SetLogins(rct RoleConditionType, logins []string) {
	lcopy := utils.CopyStrings(logins)

	if rct == Allow {
		r.Spec.Allow.Logins = lcopy
	} else {
		r.Spec.Deny.Logins = lcopy
	}
}

// GetNamespaces gets a list of namespaces this role is allowed or denied access to.
func (r *RoleV3) GetNamespaces(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.Namespaces
	}
	return r.Spec.Deny.Namespaces
}

// GetNamespaces sets a list of namespaces this role is allowed or denied access to.
func (r *RoleV3) SetNamespaces(rct RoleConditionType, namespaces []string) {
	ncopy := utils.CopyStrings(namespaces)

	if rct == Allow {
		r.Spec.Allow.Namespaces = ncopy
	} else {
		r.Spec.Deny.Namespaces = ncopy
	}
}

// GetNodeLabels gets the map of node labels this role is allowed or denied access to.
func (r *RoleV3) GetNodeLabels(rct RoleConditionType) map[string]string {
	if rct == Allow {
		return r.Spec.Allow.NodeLabels
	}
	return r.Spec.Deny.NodeLabels
}

// SetNodeLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV3) SetNodeLabels(rct RoleConditionType, labels map[string]string) {
	lcopy := utils.CopyStringMap(labels)

	if rct == Allow {
		r.Spec.Allow.NodeLabels = lcopy
	} else {
		r.Spec.Deny.NodeLabels = lcopy
	}
}

// GetRules gets all allow or deny rules.
func (r *RoleV3) GetRules(rct RoleConditionType) map[string][]string {
	if rct == Allow {
		return r.Spec.Allow.Rules
	}
	return r.Spec.Deny.Rules
}

// SetRules sets an allow or deny rule.
func (r *RoleV3) SetRules(rct RoleConditionType, rrs map[string][]string) {
	rcopy := utils.CopyStringMapSlices(rrs)

	if rct == Allow {
		r.Spec.Allow.Rules = rcopy
	} else {
		r.Spec.Deny.Rules = rcopy
	}
}

// Check checks validity of all parameters and sets defaults
func (r *RoleV3) CheckAndSetDefaults() error {
	// make sure we have defaults for all fields
	if r.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if r.Metadata.Namespace == "" {
		r.Metadata.Namespace = defaults.Namespace
	}
	if r.Spec.Options == nil {
		r.Spec.Options = map[string]interface{}{
			MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
		}
	}
	if r.Spec.Allow.Namespaces == nil {
		r.Spec.Allow.Namespaces = []string{Wildcard}
	}
	if r.Spec.Allow.NodeLabels == nil {
		r.Spec.Allow.NodeLabels = map[string]string{Wildcard: Wildcard}
	}
	if r.Spec.Allow.Rules == nil {
		r.Spec.Allow.Rules = utils.CopyStringMapSlices(DefaultUserRules)
	}

	// if we find {{ or }} but the syntax is invalid, the role is invalid
	for _, condition := range []RoleConditionType{Allow, Deny} {
		for _, login := range r.GetLogins(condition) {
			if strings.Contains(login, "{{") || strings.Contains(login, "}}") {
				_, _, err := parse.IsRoleVariable(login)
				if err != nil {
					return trace.BadParameter("invalid login found: %v", login)
				}
			}
		}
	}

	// check and correct the session ttl
	maxSessionTTL, err := r.Spec.Options.GetDuration(MaxSessionTTL)
	if err != nil {
		return trace.BadParameter("invalid duration: %v", err)
	}
	if maxSessionTTL.Duration == 0 {
		r.Spec.Options.Set(MaxSessionTTL, NewDuration(defaults.MaxCertDuration))
	}
	if maxSessionTTL.Duration < defaults.MinCertDuration {
		return trace.BadParameter("maximum session TTL can not be less than, minimal certificate duration")
	}

	// restrict wildcards
	for _, login := range r.Spec.Allow.Logins {
		if login == Wildcard {
			return trace.BadParameter("wildcard matcher is not allowed in logins")
		}
		if !cstrings.IsValidUnixUser(login) {
			return trace.BadParameter("%q is not a valid user name", login)
		}
	}
	for key, val := range r.Spec.Allow.NodeLabels {
		if key == Wildcard && val != Wildcard {
			return trace.BadParameter("selector *:<val> is not supported")
		}
	}

	return nil
}

// String returns the human readable representation of a role.
func (r *RoleV3) String() string {
	return fmt.Sprintf("Role(Name=%v,Options=%v,Allow=%+v,Deny=%+v)",
		r.GetName(), r.Spec.Options, r.Spec.Allow, r.Spec.Deny)
}

// RoleSpecV3 is role specification for RoleV3.
type RoleSpecV3 struct {
	// Options is for OpenSSH options like agent forwarding.
	Options RoleOptions `json:"options,omitempty" yaml:"options,omitempty"`
	// Allow is the set of conditions evaluated to grant access.
	Allow RoleConditions `json:"allow,omitempty" yaml:"allow,omitempty"`
	// Deny is the set of conditions evaluated to deny access. Deny takes priority over allow.
	Deny RoleConditions `json:"deny,omitempty" yaml:"deny,omitempty"`
}

// RoleOptions are key/value pairs that always exist for a role.
type RoleOptions map[string]interface{}

// UnmarshalJSON is used when parsing RoleV3 to convert MaxSessionTTL into the
// correct type.
func (o *RoleOptions) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	rmap := make(map[string]interface{})
	for k, v := range raw {
		switch k {
		case MaxSessionTTL:
			d, err := time.ParseDuration(v.(string))
			if err != nil {
				return err
			}
			rmap[MaxSessionTTL] = NewDuration(d)
		default:
			rmap[k] = v
		}
	}

	*o = rmap
	return nil
}

// Set an option key/value pair.
func (o RoleOptions) Set(key string, value interface{}) {
	o[key] = value
}

// Get returns the option as an interface{}, it is the responsibility of the
// caller to convert to the correct type.
func (o RoleOptions) Get(key string) (interface{}, error) {
	valueI, ok := o[key]
	if !ok {
		return nil, trace.NotFound("key %q not found in options", key)
	}

	return valueI, nil
}

// GetString returns the option as a string or returns an error.
func (o RoleOptions) GetString(key string) (string, error) {
	valueI, ok := o[key]
	if !ok {
		return "", trace.NotFound("key %q not found in options", key)
	}

	value, ok := valueI.(string)
	if !ok {
		return "", trace.BadParameter("type %T for key %q is not a string", valueI, key)
	}

	return value, nil
}

// GetBoolean returns the option as a bool or returns an error.
func (o RoleOptions) GetBoolean(key string) (bool, error) {
	valueI, ok := o[key]
	if !ok {
		return false, trace.NotFound("key %q not found in options", key)
	}

	value, ok := valueI.(bool)
	if !ok {
		return false, trace.BadParameter("type %T for key %q is not a bool", valueI, key)
	}

	return value, nil
}

// GetDuration returns the option as a services.Duration or returns an error.
func (o RoleOptions) GetDuration(key string) (Duration, error) {
	valueI, ok := o[key]
	if !ok {
		return NewDuration(defaults.MinCertDuration), trace.NotFound("key %q not found in options", key)
	}

	value, ok := valueI.(Duration)
	if !ok {
		return NewDuration(defaults.MinCertDuration), trace.BadParameter("type %T for key %q is not a Duration", valueI, key)
	}

	return value, nil
}

// Equals checks if all the key/values in the RoleOptions map match.
func (o RoleOptions) Equals(other RoleOptions) bool {
	return utils.InterfaceMapsEqual(o, other)
}

// RoleConditions is a set of conditions that must all match to be allowed or
// denied access.
type RoleConditions struct {
	// Logins is a list of *nix system logins.
	Logins []string `json:"logins,omitempty" yaml:"logins,omitempty"`
	// Namespaces is a list of namespaces (used to partition a cluster).
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	// NodeLabels is a map of node labels (used to dynamically grant access to nodes).
	NodeLabels map[string]string `json:"node_labels,omitempty" yaml:"node_labels,omitempty"`

	// Rules is a list of rules and their access levels. Rules are a high level
	// construct used for access control.
	Rules RoleRules `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// Equals returns true if the role conditions (logins, namespaces, labels,
// and rules) are equal and false if they are not.
func (r *RoleConditions) Equals(o RoleConditions) bool {
	if !utils.StringSlicesEqual(r.Logins, o.Logins) {
		return false
	}
	if !utils.StringSlicesEqual(r.Namespaces, o.Namespaces) {
		return false
	}
	if !utils.StringMapsEqual(r.NodeLabels, o.NodeLabels) {
		return false
	}
	if !utils.StringMapSlicesEqual(r.Rules, o.Rules) {
		return false
	}

	return true
}

// RoleRules is a map of resources and their verbs. Role rules can be used
// to allow or deny access to resources.
type RoleRules map[string][]string

type rules struct {
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

// MarshalJSON is used to convert between the internal representation of
// rules and the format defined in RoleSpecV3.
func (rrs *RoleRules) MarshalJSON() ([]byte, error) {
	var r []rules
	for resource, verbs := range *rrs {
		r = append(r, rules{Resources: []string{resource}, Verbs: verbs})
	}

	return json.Marshal(r)
}

// UnmarshalJSON is used to convert between the internal representation of
// rules and the format defined in RoleSpecV3.
func (rrs *RoleRules) UnmarshalJSON(data []byte) error {
	var r []rules
	err := json.Unmarshal(data, &r)
	if err != nil {
		return err
	}

	rmap := make(map[string][]string)
	for _, rule := range r {
		for _, resource := range rule.Resources {
			rmap[resource] = rule.Verbs
		}
	}

	*rrs = rmap
	return nil
}

// RoleV2 represents role resource specification
type RoleV2 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains role specification
	Spec RoleSpecV2 `json:"spec"`
}

// Equals test roles for equality. Roles are considered equal if all resources,
// logins, namespaces, labels, and options match.
func (r *RoleV2) Equals(other Role) bool {
	return r.V3().Equals(other)
}

// SetResource sets resource rule
func (r *RoleV2) SetResource(kind string, actions []string) {
	if r.Spec.Resources == nil {
		r.Spec.Resources = make(map[string][]string)
	}
	r.Spec.Resources[kind] = actions
}

// RemoveResource deletes resource entry
func (r *RoleV2) RemoveResource(kind string) {
	delete(r.Spec.Resources, kind)
}

// SetLogins sets logins for role
func (r *RoleV2) SetLogins(logins []string) {
	r.Spec.Logins = logins
}

// SetNodeLabels sets node labels for role
func (r *RoleV2) SetNodeLabels(labels map[string]string) {
	r.Spec.NodeLabels = labels
}

// SetMaxSessionTTL sets a maximum TTL for SSH or Web session
func (r *RoleV2) SetMaxSessionTTL(duration time.Duration) {
	r.Spec.MaxSessionTTL.Duration = duration
}

// SetExpiry sets expiry time for the object
func (r *RoleV2) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expires retuns object expiry setting
func (r *RoleV2) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (r *RoleV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// SetName is a shortcut for SetMetadata().Name
func (r *RoleV2) SetName(s string) {
	r.Metadata.Name = s
}

// GetName returns role name and is a shortcut for GetMetadata().Name
func (r *RoleV2) GetName() string {
	return r.Metadata.Name
}

// GetMetadata returns role metadata
func (r *RoleV2) GetMetadata() Metadata {
	return r.Metadata
}

// GetMaxSessionTTL is a maximum SSH or Web session TTL
func (r *RoleV2) GetMaxSessionTTL() Duration {
	return r.Spec.MaxSessionTTL
}

// GetLogins returns a list of linux logins allowed for this role
func (r *RoleV2) GetLogins() []string {
	return r.Spec.Logins
}

// GetNodeLabels returns a list of matchign nodes this role has access to
func (r *RoleV2) GetNodeLabels() map[string]string {
	return r.Spec.NodeLabels
}

// GetNamespaces returns a list of namespaces this role has access to
func (r *RoleV2) GetNamespaces() []string {
	return r.Spec.Namespaces
}

// SetNamespaces sets a list of namespaces this role has access to
func (r *RoleV2) SetNamespaces(namespaces []string) {
	r.Spec.Namespaces = namespaces
}

// GetResources returns access to resources
func (r *RoleV2) GetResources() map[string][]string {
	return r.Spec.Resources
}

// CanForwardAgent returns true if this role is allowed
// to request agent forwarding
func (r *RoleV2) CanForwardAgent() bool {
	return r.Spec.ForwardAgent
}

// SetForwardAgent sets forward agent property
func (r *RoleV2) SetForwardAgent(forwardAgent bool) {
	r.Spec.ForwardAgent = forwardAgent
}

// Check checks validity of all parameters and sets defaults
func (r *RoleV2) CheckAndSetDefaults() error {
	// make sure we have defaults for all fields
	if r.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if r.Metadata.Namespace == "" {
		r.Metadata.Namespace = defaults.Namespace
	}
	if r.Spec.MaxSessionTTL.Duration == 0 {
		r.Spec.MaxSessionTTL.Duration = defaults.MaxCertDuration
	}
	if r.Spec.MaxSessionTTL.Duration < defaults.MinCertDuration {
		return trace.BadParameter("maximum session TTL can not be less than")
	}
	if r.Spec.Namespaces == nil {
		r.Spec.Namespaces = []string{defaults.Namespace}
	}
	if r.Spec.NodeLabels == nil {
		r.Spec.NodeLabels = map[string]string{Wildcard: Wildcard}
	}
	if r.Spec.Resources == nil {
		r.Spec.Resources = map[string][]string{
			KindSession:       RO(),
			KindRole:          RO(),
			KindNode:          RO(),
			KindAuthServer:    RO(),
			KindReverseTunnel: RO(),
			KindCertAuthority: RO(),
		}
	}

	// restrict wildcards
	for _, login := range r.Spec.Logins {
		if login == Wildcard {
			return trace.BadParameter("wildcard matcher is not allowed in logins")
		}
		if !cstrings.IsValidUnixUser(login) {
			return trace.BadParameter("'%v' is not a valid user name", login)
		}
	}
	for key, val := range r.Spec.NodeLabels {
		if key == Wildcard && val != Wildcard {
			return trace.BadParameter("selector *:<val> is not supported")
		}
	}

	return nil
}

func (r *RoleV2) V3() *RoleV3 {
	role := &RoleV3{
		Kind:     KindRole,
		Version:  V3,
		Metadata: r.Metadata,
		Spec: RoleSpecV3{
			Options: RoleOptions{
				MaxSessionTTL: r.GetMaxSessionTTL(),
			},
			Allow: RoleConditions{
				Logins:     r.GetLogins(),
				Namespaces: r.GetNamespaces(),
				NodeLabels: r.GetNodeLabels(),
			},
		},
		rawObject: *r,
	}

	// translate old v2 agent forwarding to a v3 option
	if r.CanForwardAgent() {
		role.Spec.Options[ForwardAgent] = true
	}

	// translate old v2 resources to v3 rules
	rules := make(map[string][]string)
	for resource, actions := range r.GetResources() {
		var verbs []string

		containsRead := utils.SliceContainsStr(actions, ActionRead)
		containsWrite := utils.SliceContainsStr(actions, ActionWrite)

		if containsRead && containsWrite {
			verbs = RW()
		} else if containsRead {
			verbs = RO()
		} else if containsWrite {
			// in RoleV2 ActionWrite implied the ability to read secrets.
			verbs = []string{VerbReadSecrets, VerbCreate, VerbUpdate, VerbDelete}
		}

		rules[resource] = utils.Deduplicate(verbs)
	}
	role.Spec.Allow.Rules = rules

	return role
}

func (r *RoleV2) String() string {
	return fmt.Sprintf("Role(Name=%v,MaxSessionTTL=%v,Logins=%v,NodeLabels=%v,Namespaces=%v,Resources=%v,CanForwardAgent=%v)",
		r.GetName(), r.GetMaxSessionTTL(), r.GetLogins(), r.GetNodeLabels(), r.GetNamespaces(), r.GetResources(), r.CanForwardAgent())
}

// RoleSpecV2 is role specification for RoleV2
type RoleSpecV2 struct {
	// MaxSessionTTL is a maximum SSH or Web session TTL
	MaxSessionTTL Duration `json:"max_session_ttl" yaml:"max_session_ttl"`
	// Logins is a list of linux logins allowed for this role
	Logins []string `json:"logins,omitempty" yaml:"logins,omitempty"`
	// NodeLabels is a set of matching labels that users of this role
	// will be allowed to access
	NodeLabels map[string]string `json:"node_labels,omitempty" yaml:"node_labels,omitempty"`
	// Namespaces is a list of namespaces, guarding accesss to resources
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	// Resources limits access to resources
	Resources map[string][]string `json:"resources,omitempty" yaml:"resources,omitempty"`
	// ForwardAgent permits SSH agent forwarding if requested by the client
	ForwardAgent bool `json:"forward_agent" yaml:"forward_agent"`
}

// AccessChecker interface implements access checks for given role
type AccessChecker interface {
	// CheckAccessToServer checks access to server.
	CheckAccessToServer(login string, server Server) error

	// CheckAccessToRule checks access to a rule within a namespace.
	CheckAccessToRule(namespace string, rule string, verb string) error

	// CheckLoginDuration checks if role set can login up to given duration and
	// returns a combined list of allowed logins.
	CheckLoginDuration(ttl time.Duration) ([]string, error)

	// AdjustSessionTTL will reduce the requested ttl to lowest max allowed TTL
	// for this role set, otherwise it returns ttl unchanged
	AdjustSessionTTL(ttl time.Duration) time.Duration

	// CheckAgentForward checks if the role can request agent forward for this user
	CheckAgentForward(login string) error

	// CanForwardAgents returns true if this role set offers capability to forward agents
	CanForwardAgents() bool
}

// FromSpec returns new RoleSet created from spec
func FromSpec(name string, spec RoleSpecV3) (RoleSet, error) {
	role, err := NewRole(name, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewRoleSet(role), nil
}

// RW is a shortcut that returns all verbs.
func RW() []string {
	return []string{VerbReadSecrets, VerbConnect, VerbList, VerbCreate, VerbRead, VerbUpdate, VerbDelete}
}

// RO is a shortcut that returns read only verbs.
func RO() []string {
	return []string{VerbList, VerbRead}
}

// NewRole constructs new standard role
func NewRole(name string, spec RoleSpecV3) (Role, error) {
	role := RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
	if err := role.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &role, nil
}

// RoleGetter is an interface that defines GetRole method
type RoleGetter interface {
	// GetRole returns role by name
	GetRole(name string) (Role, error)
}

// FetchRoles fetches roles by their names, applies the traits to role
// variables, and returns the RoleSet.
func FetchRoles(roleNames []string, access RoleGetter, traits map[string][]string) (RoleSet, error) {
	var roles []Role

	for _, roleName := range roleNames {
		role, err := access.GetRole(roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role.ApplyTraits(traits))
	}

	return NewRoleSet(roles...), nil
}

// NewRoleSet returns new RoleSet based on the roles
func NewRoleSet(roles ...Role) RoleSet {
	return append(roles, NewImplicitRole())
}

// RoleSet is a set of roles that implements access control functionality
type RoleSet []Role

// MatchRule tests if the resource name and verb are in a given list of rules.
func MatchRule(rules map[string][]string, resource string, verb string) bool {
	// empty selector matches nothing
	if len(rules) == 0 {
		return false
	}

	// check for wildcard resource matcher
	for _, action := range rules[Wildcard] {
		if action == Wildcard || action == verb {
			return true
		}
	}

	// check for matching resource by name
	for _, action := range rules[resource] {
		if action == Wildcard || action == verb {
			return true
		}
	}

	return false
}

// MatchLogin returns true if attempted login matches any of the logins
func MatchLogin(logins []string, login string) bool {
	for _, l := range logins {
		if l == login {
			return true
		}
	}
	return false
}

// MatchNamespace returns true if given list of namespace matches
// target namespace, wildcard matches everything
func MatchNamespace(selector []string, namespace string) bool {
	for _, n := range selector {
		if n == namespace || n == Wildcard {
			return true
		}
	}
	return false
}

// MatchLabels matches selector against target
func MatchLabels(selector map[string]string, target map[string]string) bool {
	// empty selector matches nothing
	if len(selector) == 0 {
		return false
	}
	// *: * matches everything even empty target set
	if selector[Wildcard] == Wildcard {
		return true
	}
	for key, val := range selector {
		if targetVal, ok := target[key]; !ok || (val != targetVal && val != Wildcard) {
			return false
		}
	}
	return true
}

// AdjustSessionTTL will reduce the requested ttl to lowest max allowed TTL
// for this role set, otherwise it returns ttl unchanges
func (set RoleSet) AdjustSessionTTL(ttl time.Duration) time.Duration {
	for _, role := range set {
		maxSessionTTL, err := role.GetOptions().GetDuration(MaxSessionTTL)
		if err != nil {
			continue
		}
		if ttl > maxSessionTTL.Duration {
			ttl = maxSessionTTL.Duration
		}
	}
	return ttl
}

// CheckLoginDuration checks if role set can login up to given duration and
// returns a combined list of allowed logins.
func (set RoleSet) CheckLoginDuration(ttl time.Duration) ([]string, error) {
	logins := make(map[string]bool)

	var matchedTTL bool
	for _, role := range set {
		maxSessionTTL, err := role.GetOptions().GetDuration(MaxSessionTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if ttl <= maxSessionTTL.Duration && maxSessionTTL.Duration != 0 {
			matchedTTL = true

			for _, login := range role.GetLogins(Allow) {
				logins[login] = true
			}
		}
	}
	if !matchedTTL {
		return nil, trace.AccessDenied("this user cannot request a certificate for %v", ttl)
	}
	if len(logins) == 0 {
		return nil, trace.AccessDenied("this user cannot create SSH sessions, has no allowed logins")
	}
	out := make([]string, 0, len(logins))
	for login := range logins {
		out = append(out, login)
	}
	return out, nil
}

// CheckAccessToServer checks if a role has access to a node. Deny rules are
// checked first then allow rules. Access to a node is determined by
// namespaces, labels, and logins.
func (set RoleSet) CheckAccessToServer(login string, s Server) error {
	var errs []error

	// check deny rules first: a single matching namespace, label, or login from
	// the deny role set prohibits access.
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Deny), s.GetNamespace())
		matchLabels := MatchLabels(role.GetNodeLabels(Deny), s.GetAllLabels())
		matchLogin := MatchLogin(role.GetLogins(Deny), login)
		if matchNamespace || matchLabels || matchLogin {
			errorMessage := fmt.Sprintf("role %v denied access to node %v: deny rule matched; match(namespace=%v, label=%v, login=%v)",
				role.GetName(), s.GetHostname(), matchNamespace, matchLabels, matchLogin)
			log.Warnf("[RBAC] Denied access to server: " + errorMessage)
			return trace.AccessDenied(errorMessage)
		}
	}

	// check allow rules: namespace, label, and login have to all match in
	// one role in the role set to be granted access.
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Allow), s.GetNamespace())
		matchLabels := MatchLabels(role.GetNodeLabels(Allow), s.GetAllLabels())
		matchLogin := MatchLogin(role.GetLogins(Allow), login)
		if matchNamespace && matchLabels && matchLogin {
			return nil
		}

		errorMessage := fmt.Sprintf("role %v denied access: allow rules did not match; match(namespace=%v, label=%v, login=%v)",
			role.GetName(), matchNamespace, matchLabels, matchLogin)
		errs = append(errs, trace.AccessDenied(errorMessage))
	}

	errorMessage := fmt.Sprintf("access to node %v is denied to role(s): %v", s.GetHostname(), errs)
	log.Warnf("[RBAC] Denied access to server: " + errorMessage)
	return trace.AccessDenied(errorMessage)
}

// CanForwardAgents returns true if role set allows forwarding agents.
func (set RoleSet) CanForwardAgents() bool {
	for _, role := range set {
		forwardAgent, err := role.GetOptions().GetBoolean(ForwardAgent)
		if err != nil {
			return false
		}
		if forwardAgent == true {
			return true
		}
	}
	return false
}

// CheckAgentForward checks if the role can request to forward the SSH agent
// for this user.
func (set RoleSet) CheckAgentForward(login string) error {
	// check if we have permission to login and forward agent. we don't check
	// for deny rules because if you can't forward an agent if you can't login
	// in the first place.
	for _, role := range set {
		for _, l := range role.GetLogins(Allow) {
			forwardAgent, err := role.GetOptions().GetBoolean(ForwardAgent)
			if err != nil {
				return trace.AccessDenied("unable to parse ForwardAgent: %v", err)
			}
			if forwardAgent && l == login {
				return nil
			}
		}
	}
	return trace.AccessDenied("%v can not forward agent for %v", set, login)
}

func (set RoleSet) String() string {
	if len(set) == 0 {
		return "user without assigned roles"
	}
	roleNames := make([]string, len(set))
	for i, role := range set {
		roleNames[i] = role.GetName()
	}
	return fmt.Sprintf("roles %v", strings.Join(roleNames, ","))
}

func (set RoleSet) CheckAccessToRule(namespace string, resource string, verb string) error {
	// check deny: a single match on a deny rule prohibits access
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Deny), ProcessNamespace(namespace))
		matchRule := MatchRule(role.GetRules(Deny), resource, verb)
		if matchNamespace && matchRule {
			return trace.AccessDenied("%v access to %v in namespace %v is denied for %v: deny rule matched", verb, resource, namespace, role)
		}
	}

	// check allow: if rule matches, grant access to resource
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Allow), ProcessNamespace(namespace))
		matchRule := MatchRule(role.GetRules(Allow), resource, verb)
		if matchNamespace && matchRule {
			return nil
		}
	}

	return trace.AccessDenied("%v access to %v in namespace %v is denied for %v: no allow rule matched", verb, resource, namespace, set)
}

// ProcessNamespace sets default namespace in case if namespace is empty
func ProcessNamespace(namespace string) string {
	if namespace == "" {
		return defaults.Namespace
	}
	return namespace
}

// MaxDuration returns maximum duration that is possible
func MaxDuration() Duration {
	return NewDuration(1<<63 - 1)
}

// NewDuration returns Duration struct based on time.Duration
func NewDuration(d time.Duration) Duration {
	return Duration{Duration: d}
}

// Duration is a wrapper around duration to set up custom marshal/unmarshal
type Duration struct {
	time.Duration
}

// MarshalJSON marshals Duration to string
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%v", d.Duration))
}

// UnmarshalJSON marshals Duration to string
func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err != nil {
		return trace.Wrap(err)
	}
	out, err := time.ParseDuration(stringVar)
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	d.Duration = out
	return nil
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}
	out, err := time.ParseDuration(stringVar)
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	d.Duration = out
	return nil
}

const RoleSpecV3SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "max_session_ttl": { "type": "string" },
    "options": {
      "type": "object",
      "patternProperties": {
        "^[a-zA-Z/.0-9_]$": { "type": "string" }
      }
    },
    "allow": { "$ref": "#/definitions/role_condition" },
    "deny": { "$ref": "#/definitions/role_condition" }%v
  }
}
`

const RoleSpecV3SchemaDefinitions = `
  "definitions": {
    "role_condition": {
      "namespaces": {
        "type": "array",
        "items": { "type": "string" }
      },
      "node_labels": {
        "type": "object",
        "patternProperties": {
          "^[a-zA-Z/.0-9_]$": { "type": "string" }
        }
      },
      "logins": {
        "type": "array",
        "items": { "type": "string" }
      },
      "rules": {
        "type": "array",
        "items": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "resources": {
              "type": "array",
              "items": { "type": "string" }
            },
            "verbs": {
              "type": "array",
              "items": { "type": "string" }
            }
          }
        }
      }
    }
  }
`

const RoleSpecV2SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "max_session_ttl": {"type": "string"},
    "forward_agent": {"type": "boolean"},
    "node_labels": {
      "type": "object",
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "string" }
      }
    },
    "namespaces": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "logins": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "resources": {
      "type": "object",
      "patternProperties": {
         "^[a-zA-Z/.0-9_]$":  { "type": "array", "items": {"type": "string"} }
       }
    }%v
  }
}`

// GetRoleSchema returns role schema for the version requested with optionally
// injected schema for extensions.
func GetRoleSchema(version string, extensionSchema string) string {
	schemaDefinitions := "," + RoleSpecV3SchemaDefinitions
	if version == V2 {
		schemaDefinitions = DefaultDefinitions
	}

	schemaTemplate := RoleSpecV3SchemaTemplate
	if version == V2 {
		schemaTemplate = RoleSpecV2SchemaTemplate
	}

	schema := fmt.Sprintf(schemaTemplate, ``)
	if extensionSchema != "" {
		schema = fmt.Sprintf(schemaTemplate, ","+extensionSchema)
	}

	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, schema, schemaDefinitions)
}

// UnmarshalRole unmarshals role from JSON, sets defaults, and checks schema.
func UnmarshalRole(data []byte) (*RoleV3, error) {
	var h ResourceHeader
	err := json.Unmarshal(data, &h)
	if err != nil {
		h.Version = V2
	}

	switch h.Version {
	case V2:
		var role RoleV2
		if err := utils.UnmarshalWithSchema(GetRoleSchema(V2, ""), &role, data); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		utils.UTC(&role.Metadata.Expires)
		roleV3 := role.V3()
		roleV3.rawObject = role

		return roleV3, nil
	case V3:
		var role RoleV3
		if err := utils.UnmarshalWithSchema(GetRoleSchema(V3, ""), &role, data); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		utils.UTC(&role.Metadata.Expires)

		return &role, nil
	}

	return nil, trace.BadParameter("role version %q is not supported", h.Version)
}

var roleMarshaler RoleMarshaler = &TeleportRoleMarshaler{}

func SetRoleMarshaler(m RoleMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	roleMarshaler = m
}

func GetRoleMarshaler() RoleMarshaler {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	return roleMarshaler
}

// RoleMarshaler implements marshal/unmarshal of Role implementations
// mostly adds support for extended versions
type RoleMarshaler interface {
	// UnmarshalRole from binary representation
	UnmarshalRole(bytes []byte) (Role, error)
	// MarshalRole to binary representation
	MarshalRole(u Role, opts ...MarshalOption) ([]byte, error)
}

type TeleportRoleMarshaler struct{}

// UnmarshalRole unmarshals role from JSON.
func (*TeleportRoleMarshaler) UnmarshalRole(bytes []byte) (Role, error) {
	return UnmarshalRole(bytes)
}

// MarshalRole marshalls role into JSON.
func (*TeleportRoleMarshaler) MarshalRole(u Role, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(u)
}

// SortedRoles sorts roles by name
type SortedRoles []Role

// Len returns length of a role list
func (s SortedRoles) Len() int {
	return len(s)
}

// Less compares roles by name
func (s SortedRoles) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

// Swap swaps two roles in a list
func (s SortedRoles) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
