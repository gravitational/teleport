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
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"
	"github.com/jonboulle/clockwork"
)

// DefaultUserRules provides access to the minimal set of rules needed for
// a user to function.
var DefaultUserRules = map[string][]string{
	KindSession:       RO(),
	KindRole:          RO(),
	KindNode:          RO(),
	KindAuthServer:    RO(),
	KindReverseTunnel: RO(),
	KindCertAuthority: RO(),
}

// DefaultCertAuthorityRules provides access the minimal set of rules needed
// for a certificate authority to function.
var DefaultCertAuthorityRules = map[string][]string{
	KindSession:       RO(),
	KindNode:          RO(),
	KindAuthServer:    RO(),
	KindReverseTunnel: RO(),
	KindCertAuthority: RO(),
}

// DefaultAdministratorRules provides access to all resources.
var DefaultAdministratorRules = map[string][]string{
	Wildcard: RW(),
}

// RoleNameForUser returns role name associated with user
func RoleNameForUser(name string) string {
	return "user:" + name
}

// RoleNameForCertAuthority returns role name associated with cert authority
func RoleNameForCertAuthority(name string) string {
	return "ca:" + name
}

// RoleForUser creates role using AllowedLogins parameter
func RoleForUser(u User) Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      RoleNameForUser(u.GetName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			Options:       map[string]string{},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      DefaultUserRules,
			},
		},
	}
}

// RoleForCertauthority creates role using AllowedLogins parameter
func RoleForCertAuthority(ca CertAuthority) Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      RoleNameForCertAuthority(ca.GetClusterName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			Options:       map[string]string{},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      DefaultCertAuthorityRules,
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
	ForwardAgent = "ForwardAgent"
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
	// Equals returns true if the roles are equal.
	Equals(other Role) bool

	// GetMaxSessionTTL gets the maximum duration for a SSH or Web session.
	GetMaxSessionTTL() Duration
	// SetMaxSessionTTL sets the maximum duration for a SSH or Web session.
	SetMaxSessionTTL(duration time.Duration)

	// GetStringOption gets an OpenSSH option.
	GetOption(string) string
	// SetOption sets an OpenSSH option.
	SetOption(string, string)

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
}

// Equals returns true if roles are equal.
func (r *RoleV3) Equals(other Role) bool {
	return true
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

// SetMaxSessionTTL sets the maximum duration for a SSH or Web session.
func (r *RoleV3) SetMaxSessionTTL(duration time.Duration) {
	r.Spec.MaxSessionTTL.Duration = duration
}

// GetMaxSessionTTL gets the maximum duration for a SSH or Web session.
func (r *RoleV3) GetMaxSessionTTL() Duration {
	return r.Spec.MaxSessionTTL
}

// GetOption gets an OpenSSH option.
func (r *RoleV3) GetOption(optionName string) string {
	return r.Spec.Options[optionName]
}

// SetOption sets an OpenSSH option.
func (r *RoleV3) SetOption(optionName string, optionValue string) {
	if r.Spec.Options == nil {
		r.Spec.Options = make(map[string]string)
	}
	r.Spec.Options[optionName] = optionValue
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
	if rct == Allow {
		r.Spec.Allow.Logins = logins
	} else {
		r.Spec.Deny.Logins = logins
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
	if rct == Allow {
		r.Spec.Allow.Namespaces = namespaces
	} else {
		r.Spec.Deny.Namespaces = namespaces
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
	if rct == Allow {
		r.Spec.Allow.NodeLabels = labels
	} else {
		r.Spec.Deny.NodeLabels = labels
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
	// make a copy of rules. we don't want someone to accidentally update the
	// map they passed us and modify the role.
	mcopy := make(map[string][]string)
	for resource, verbs := range rrs {
		verbscopy := make([]string, len(verbs))
		copy(verbscopy, verbs)
		mcopy[resource] = verbscopy
	}

	if rct == Allow {
		r.Spec.Allow.Rules = mcopy
	} else {
		r.Spec.Deny.Rules = mcopy
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
	if r.Spec.MaxSessionTTL.Duration == 0 {
		r.Spec.MaxSessionTTL.Duration = defaults.MaxCertDuration
	}
	if r.Spec.MaxSessionTTL.Duration < defaults.MinCertDuration {
		return trace.BadParameter("maximum session TTL can not be less than")
	}
	if r.Spec.Allow.Namespaces == nil {
		r.Spec.Allow.Namespaces = []string{defaults.Namespace}
	}
	if r.Spec.Allow.NodeLabels == nil {
		r.Spec.Allow.NodeLabels = map[string]string{Wildcard: Wildcard}
	}
	if r.Spec.Allow.Rules == nil {
		r.Spec.Allow.Rules = DefaultUserRules
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
	return fmt.Sprintf("Role(Name=%v,MaxSessionTTL=%v,Options=%v,Allow=%v,Deny=%v)",
		r.GetName(), r.GetMaxSessionTTL(), r.Spec.Options, r.Spec.Allow, r.Spec.Deny)
}

// RoleSpecV3 is role specification for RoleV3.
type RoleSpecV3 struct {
	// MaxSessionTTL is a maximum duration for a SSH or Web session.
	MaxSessionTTL Duration `json:"max_session_ttl" yaml:"max_session_ttl"`
	// Options is for OpenSSH options like agent forwarding.
	Options map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
	// Allow is the set of conditions evaluated to grant access.
	Allow RoleConditions `json:"allow,omitempty" yaml:"allow,omitempty"`
	// Deny is the set of conditions evaluated to deny access. Deny takes priority over allow.
	Deny RoleConditions `json:"deny,omitempty" yaml:"deny,omitempty"`
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
	// Rules is a list of resources and their access levels.
	Rules RoleRules `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// Equals returns true if the role conditions are equal and false if they are not.
func (r *RoleConditions) Equals(o RoleConditions) bool {
	if utils.StringSlicesEqual(r.Logins, o.Logins) == false {
		return false
	}
	if utils.StringSlicesEqual(r.Namespaces, o.Namespaces) == false {
		return false
	}
	if utils.StringMapsEqual(r.NodeLabels, o.NodeLabels) == false {
		return false
	}
	if utils.StringMapSlicesEqual(r.Rules, o.Rules) == false {
		return false
	}

	return true
}

type RoleRules map[string][]string

type rules struct {
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

func (rrs *RoleRules) MarshalJSON() ([]byte, error) {
	var r []rules
	for resource, verbs := range *rrs {
		r = append(r, rules{Resources: []string{resource}, Verbs: verbs})
	}

	return json.Marshal(r)
}

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

// Equals returns true if roles are equal.
func (r *RoleV2) Equals(other Role) bool {
	// if the other role has deny rules, then it can't be equal because v2 only
	// had allow rules.
	if len(other.GetRules(Deny)) != 0 {
		return false
	}

	if r.GetMaxSessionTTL() != other.GetMaxSessionTTL() {
		return false
	}

	forwardAgent, err := strconv.ParseBool(other.GetOption(ForwardAgent))
	if err != nil {
		return false
	}
	if r.CanForwardAgent() && forwardAgent {
		return false
	}

	if !utils.StringSlicesEqual(r.GetLogins(), other.GetLogins(Allow)) {
		return false
	}
	if !utils.StringSlicesEqual(r.GetNamespaces(), other.GetNamespaces(Allow)) {
		return false
	}
	if !utils.StringMapsEqual(r.GetNodeLabels(), other.GetNodeLabels(Allow)) {
		return false
	}

	for resourceName, resourceActions := range r.GetResources() {
		for _, resourceAction := range resourceActions {
			if MatchRule(other.GetRules(Allow), resourceName, resourceAction) == false {
				return false
			}
		}
	}

	return true
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
			MaxSessionTTL: r.GetMaxSessionTTL(),
			Allow: RoleConditions{
				Logins:     r.GetLogins(),
				Namespaces: r.GetNamespaces(),
				NodeLabels: r.GetNodeLabels(),
				Rules:      r.GetResources(),
			},
		},
	}

	// translate old v2 agent forwarding to a v3 option
	if r.CanForwardAgent() {
		role.Spec.Options[ForwardAgent] = "true"
	}

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
	// CheckAccessToResource check access to a resource.
	CheckAccessToResource(resourceNamespace, resourceName, accessType string) error
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

// RW returns read write action list
func RW() []string {
	return []string{ActionRead, ActionWrite}
}

// RO returns read only action list
func RO() []string {
	return []string{ActionRead}
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

// FetchRoles fetches roles by their names and returns role set
func FetchRoles(roleNames []string, access RoleGetter) (RoleSet, error) {
	var roles RoleSet
	for _, roleName := range roleNames {
		role, err := access.GetRole(roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// NewRoleSet returns new RoleSet based on the roles
func NewRoleSet(roles ...Role) RoleSet {
	return roles
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
		if ttl > role.GetMaxSessionTTL().Duration {
			ttl = role.GetMaxSessionTTL().Duration
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
		if ttl <= role.GetMaxSessionTTL().Duration && role.GetMaxSessionTTL().Duration != 0 {
			matchedTTL = true
		}
		for _, login := range role.GetLogins(Allow) {
			logins[login] = true
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
			errorMessage := fmt.Sprintf("Role %v denied access to node %v: deny rule matched; match(namespace=%v, label=%v, login=%v)",
				role.GetName(), s.GetHostname(), matchNamespace, matchLabels, matchLogin)
			log.Warnf("[RBAC] " + errorMessage)
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
			log.Debugf("[RBAC] Role %v granted access to node %v", role.GetName(), s.GetHostname())
			return nil
		}

		errorMessage := fmt.Sprintf("Role %v denied access: allow rules did not match; match(namespace=%v, label=%v, login=%v)",
			role.GetName(), matchNamespace, matchLabels, matchLogin)
		errs = append(errs, trace.AccessDenied(errorMessage))
	}

	return trace.AccessDenied("access to node %v is denied to role(s): %v", s.GetHostname(), errs)
}

// CanForwardAgents returns true if role set allows forwarding agents.
func (set RoleSet) CanForwardAgents() bool {
	for _, role := range set {
		forwardAgent, err := strconv.ParseBool(role.GetOption(ForwardAgent))
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
	// deny check: check if role we have permission to login, can't forward connections
	// for a login we don't have access to.
	for _, role := range set {
		if utils.SliceContainsStr(role.GetLogins(Deny), login) {
			return trace.AccessDenied("%v can not forward agent for %v: denied access to login", set, login)
		}
	}

	// allow check: check if we have permission to login and forward agent.
	for _, role := range set {
		for _, l := range role.GetLogins(Allow) {
			forwardAgent, err := strconv.ParseBool(role.GetOption(ForwardAgent))
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

// CheckAccessToResource checks if role set has access to this resource action
func (set RoleSet) CheckAccessToResource(namespace string, resource string, verb string) error {
	var errs []error

	// check deny: a single match on a deny rule prohibits access
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Deny), ProcessNamespace(namespace))
		matchRule := MatchRule(role.GetRules(Deny), resource, verb)
		if matchNamespace && matchRule {
			errorMessage := fmt.Sprintf("Role %v denied %v access to %v in namespace %v: deny rule matched", role, verb, resource, namespace)
			log.Warnf("[RBAC] " + errorMessage)
			return trace.AccessDenied(errorMessage)
		}
	}

	// check allow: if rule matches, grant access to resource
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Allow), ProcessNamespace(namespace))
		matchRule := MatchRule(role.GetRules(Allow), resource, verb)
		if matchNamespace && matchRule {
			return nil
		}

		errorMessage := fmt.Sprintf("Role %v denied %v access to %v in namespace %v: no matching allow rule", role, verb, resource, namespace)
		errs = append(errs, trace.AccessDenied(errorMessage))
	}

	return trace.AccessDenied("%v access to %v denied to role(s): %v", verb, resource, set)
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

		return role.V3(), nil
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

// MarshalRole marshalls role into JSON
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

func indexInSlice(slice []string, value string) (int, bool) {
	for i := range slice {
		if slice[i] == value {
			return i, true
		}
	}
	return 0, false
}
