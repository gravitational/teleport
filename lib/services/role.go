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
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/predicate"
)

// AdminUserRules provides access to the default set of rules assigned to
// all users.
var AdminUserRules = []Rule{
	NewRule(KindRole, RW()),
	NewRule(KindAuthConnector, RW()),
	NewRule(KindSession, RO()),
	NewRule(KindTrustedCluster, RW()),
}

// DefaultImplicitRules provides access to the default set of implicit rules
// assigned to all roles.
var DefaultImplicitRules = []Rule{
	NewRule(KindNode, RO()),
	NewRule(KindAuthServer, RO()),
	NewRule(KindReverseTunnel, RO()),
	NewRule(KindCertAuthority, RO()),
	NewRule(KindClusterAuthPreference, RO()),
	NewRule(KindClusterName, RO()),
	NewRule(KindSSHSession, RO()),
}

// DefaultCertAuthorityRules provides access the minimal set of resources
// needed for a certificate authority to function.
var DefaultCertAuthorityRules = []Rule{
	NewRule(KindSession, RO()),
	NewRule(KindNode, RO()),
	NewRule(KindAuthServer, RO()),
	NewRule(KindReverseTunnel, RO()),
	NewRule(KindCertAuthority, RO()),
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

// NewAdminRole is the default admin role for all local users if another role
// is not explicitly assigned (Enterprise only).
func NewAdminRole() Role {
	role := &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      teleport.AdminRoleName,
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    true,
				ForwardAgent:      true,
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      CopyRulesSlice(AdminUserRules),
			},
		},
	}
	role.SetLogins(Allow, modules.GetModules().DefaultAllowedLogins())
	return role
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
				Rules:      CopyRulesSlice(DefaultImplicitRules),
			},
		},
	}
}

// RoleForUser creates an admin role for a services.User.
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
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    true,
				ForwardAgent:      true,
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: map[string]string{Wildcard: Wildcard},
				Rules:      CopyRulesSlice(AdminUserRules),
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
				Rules:      CopyRulesSlice(DefaultCertAuthorityRules),
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

	// CreateRole creates a role
	CreateRole(role Role, ttl time.Duration) error

	// UpsertRole creates or updates role
	UpsertRole(role Role, ttl time.Duration) error

	// DeleteAllRoles deletes all roles
	DeleteAllRoles() error

	// GetRole returns role by name
	GetRole(name string) (Role, error)

	// DeleteRole deletes role by name
	DeleteRole(name string) error
}

// TODO: [ev] can we please define a RoleOption type (instead of using strings)
// and use RoleOption prefix for naming these? It's impossible right now to find
// all possible role options.
const (
	// ForwardAgent is SSH agent forwarding.
	ForwardAgent = "forward_agent"

	// MaxSessionTTL defines how long a SSH session can last for.
	MaxSessionTTL = "max_session_ttl"

	// PortForwarding defines if the certificate will have "permit-port-forwarding"
	// in the certificate.
	PortForwarding = "port_forwarding"

	// CertificateFormat defines the format of the user certificate to allow
	// compatibility with older versions of OpenSSH.
	CertificateFormat = "cert_format"
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
	GetRules(rct RoleConditionType) []Rule
	// SetRules sets an allow or deny rule.
	SetRules(rct RoleConditionType, rules []Rule)
}

// ApplyTraits applies the passed in traits to any variables within the role
// and returns itself.
func ApplyTraits(r Role, traits map[string][]string) Role {
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
		if !RuleSlicesEqual(r.GetRules(condition), other.GetRules(condition)) {
			return false
		}
	}

	return true
}

// ApplyTraits applies the passed in traits to any variables within the role
// and returns itself.
func (r *RoleV3) ApplyTraits(traits map[string][]string) Role {
	return ApplyTraits(r, traits)
}

// SetRawObject sets raw object as it was stored in the database
// used for migrations and should not be modifed
func (r *RoleV3) SetRawObject(raw interface{}) {
	r.rawObject = raw
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
func (r *RoleV3) GetRules(rct RoleConditionType) []Rule {
	if rct == Allow {
		return r.Spec.Allow.Rules
	}
	return r.Spec.Deny.Rules
}

// SetRules sets an allow or deny rule.
func (r *RoleV3) SetRules(rct RoleConditionType, in []Rule) {
	rcopy := CopyRulesSlice(in)

	if rct == Allow {
		r.Spec.Allow.Rules = rcopy
	} else {
		r.Spec.Deny.Rules = rcopy
	}
}

// Check checks validity of all parameters and sets defaults
func (r *RoleV3) CheckAndSetDefaults() error {
	err := r.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	// make sure we have defaults for all fields
	if r.Spec.Options == nil {
		r.Spec.Options = map[string]interface{}{
			CertificateFormat: teleport.CertificateFormatStandard,
			MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
			PortForwarding:    true,
		}
	}
	if r.Spec.Allow.Namespaces == nil {
		r.Spec.Allow.Namespaces = []string{defaults.Namespace}
	}
	if r.Spec.Allow.NodeLabels == nil {
		r.Spec.Allow.NodeLabels = map[string]string{Wildcard: Wildcard}
	}
	if r.Spec.Deny.Namespaces == nil {
		r.Spec.Deny.Namespaces = []string{defaults.Namespace}
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
	for i := range r.Spec.Allow.Rules {
		err := r.Spec.Allow.Rules[i].CheckAndSetDefaults()
		if err != nil {
			return trace.BadParameter("failed to process 'allow' rule %v: %v", i, err)
		}
	}
	for i := range r.Spec.Deny.Rules {
		err := r.Spec.Deny.Rules[i].CheckAndSetDefaults()
		if err != nil {
			return trace.BadParameter("failed to process 'deny' rule %v: %v", i, err)
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
	Options RoleOptions `json:"options,omitempty"`
	// Allow is the set of conditions evaluated to grant access.
	Allow RoleConditions `json:"allow,omitempty"`
	// Deny is the set of conditions evaluated to deny access. Deny takes priority over allow.
	Deny RoleConditions `json:"deny,omitempty"`
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
	Logins []string `json:"logins,omitempty"`
	// Namespaces is a list of namespaces (used to partition a cluster). The
	// field should be called "namespaces" when it returns in Teleport 2.4.
	Namespaces []string `json:"-"`
	// NodeLabels is a map of node labels (used to dynamically grant access to nodes).
	NodeLabels map[string]string `json:"node_labels,omitempty"`

	// Rules is a list of rules and their access levels. Rules are a high level
	// construct used for access control.
	Rules []Rule `json:"rules,omitempty"`
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
	if len(r.Rules) != len(o.Rules) {
		return false
	}
	for i := range r.Rules {
		if !r.Rules[i].Equals(o.Rules[i]) {
			return false
		}
	}
	return true
}

// NewRule creates a rule based on a resource name and a list of verbs
func NewRule(resource string, verbs []string) Rule {
	return Rule{
		Resources: []string{resource},
		Verbs:     verbs,
	}
}

// Rule represents allow or deny rule that is executed to check
// if user or service have access to resource
type Rule struct {
	// Resources is a list of resources
	Resources []string `json:"resources"`
	// Verbs is a list of verbs
	Verbs []string `json:"verbs"`
	// Where specifies optional advanced matcher
	Where string `json:"where,omitempty"`
	// Actions specifies optional actions taken when this rule matches
	Actions []string `json:"actions,omitempty"`
}

// CheckAndSetDefaults checks and sets defaults for this rule
func (r *Rule) CheckAndSetDefaults() error {
	if len(r.Resources) == 0 {
		return trace.BadParameter("missing resources to match")
	}
	if len(r.Verbs) == 0 {
		return trace.BadParameter("missing verbs")
	}
	if len(r.Where) != 0 {
		parser, err := GetWhereParserFn()(&Context{})
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = parser.Parse(r.Where)
		if err != nil {
			return trace.BadParameter("could not parse 'where' rule: %q, error: %v", r.Where, err)
		}
	}
	if len(r.Actions) != 0 {
		parser, err := GetActionsParserFn()(&Context{})
		if err != nil {
			return trace.Wrap(err)
		}
		for i, action := range r.Actions {
			_, err = parser.Parse(action)
			if err != nil {
				return trace.BadParameter("could not parse action %v %q, error: %v", i, action, err)
			}
		}
	}
	return nil
}

// score is a sorting score of the rule, the more the score, the more
// specific the rule is
func (r *Rule) score() int {
	score := 0
	// wilcard rules are less specific
	if utils.SliceContainsStr(r.Resources, Wildcard) {
		score -= 4
	} else if len(r.Resources) == 1 {
		// rules that match specific resource are more specific than
		// fields that match several resources
		score += 2
	}
	// rules that have wilcard verbs are less specific
	if utils.SliceContainsStr(r.Verbs, Wildcard) {
		score -= 2
	}
	// rules that supply 'where' or 'actions' are more specific
	// having 'where' or 'actions' is more important than
	// whether the rules are wildcard or not, so here we have +8 vs
	// -4 and -2 score penalty for wildcards in resources and verbs
	if len(r.Where) > 0 {
		score += 8
	}
	// rules featuring actions are more specific
	if len(r.Actions) > 0 {
		score += 8
	}
	return score
}

// IsMoreSpecificThan returns true if the rule is more specific than the other.
//
// * nRule matching wildcard resource is less specific
// than same rule matching specific resource.
// * Rule that has wildcard verbs is less specific
// than the same rules matching specific verb.
// * Rule that has where section is more specific
// than the same rule without where section.
// * Rule that has actions list is more specific than
// rule without actions list.
func (r *Rule) IsMoreSpecificThan(o Rule) bool {
	return r.score() > o.score()
}

// MatchesWhere returns true if Where rule matches
// Empty Where block always matches
func (r *Rule) MatchesWhere(parser predicate.Parser) (bool, error) {
	if r.Where == "" {
		return true, nil
	}
	ifn, err := parser.Parse(r.Where)
	if err != nil {
		return false, trace.Wrap(err)
	}
	fn, ok := ifn.(predicate.BoolPredicate)
	if !ok {
		return false, trace.BadParameter("unsupported type: %T", ifn)
	}
	return fn(), nil
}

// ProcessActions processes actions specified for this rule
func (r *Rule) ProcessActions(parser predicate.Parser) error {
	for _, action := range r.Actions {
		ifn, err := parser.Parse(action)
		if err != nil {
			return trace.Wrap(err)
		}
		fn, ok := ifn.(predicate.BoolPredicate)
		if !ok {
			return trace.BadParameter("unsupported type: %T", ifn)
		}
		fn()
	}
	return nil
}

// HasVerb returns true if the rule has verb,
// this method also matches wildcard
func (r *Rule) HasVerb(verb string) bool {
	for _, v := range r.Verbs {
		// readnosecrets can be satisfied by having readnosecrets or read
		if verb == VerbReadNoSecrets {
			if v == VerbReadNoSecrets || v == VerbRead {
				return true
			}
			continue
		}
		if v == verb {
			return true
		}
	}
	return false
}

// Equals returns true if the rule equals to another
func (r *Rule) Equals(other Rule) bool {
	if !utils.StringSlicesEqual(r.Resources, other.Resources) {
		return false
	}
	if !utils.StringSlicesEqual(r.Verbs, other.Verbs) {
		return false
	}
	if !utils.StringSlicesEqual(r.Actions, other.Actions) {
		return false
	}
	if r.Where != other.Where {
		return false
	}
	return true
}

// RuleSet maps resource to a set of rules defined for it
type RuleSet map[string][]Rule

// MatchRule tests if the resource name and verb are in a given list of rules.
// More specific rules will be matched first. See Rule.IsMoreSpecificThan
// for exact specs on whether the rule is more or less specific.
//
// Specifying order solves the problem on having multiple rules, e.g. one wildcard
// rule can override more specific rules with 'where' sections that can have
// 'actions' lists with side effects that will not be triggered otherwise.
//
func (set RuleSet) Match(whereParser predicate.Parser, actionsParser predicate.Parser, resource string, verb string) (bool, error) {
	// empty set matches nothing
	if len(set) == 0 {
		return false, nil
	}

	// check for matching resource by name
	// the most specific rule should win
	rules := set[resource]
	for _, rule := range rules {
		match, err := rule.MatchesWhere(whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if match && (rule.HasVerb(Wildcard) || rule.HasVerb(verb)) {
			if err := rule.ProcessActions(actionsParser); err != nil {
				return true, trace.Wrap(err)
			}
			return true, nil
		}
	}

	// check for wildcard resource matcher
	for _, rule := range set[Wildcard] {
		match, err := rule.MatchesWhere(whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if match && (rule.HasVerb(Wildcard) || rule.HasVerb(verb)) {
			if err := rule.ProcessActions(actionsParser); err != nil {
				return true, trace.Wrap(err)
			}
			return true, nil
		}
	}

	return false, nil
}

// Slice returns slice from a set
func (set RuleSet) Slice() []Rule {
	var out []Rule
	for _, rules := range set {
		out = append(out, rules...)
	}
	return out
}

// MakeRuleSet converts slice of rules to the set of rules
func MakeRuleSet(rules []Rule) RuleSet {
	set := make(RuleSet)
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			rules, ok := set[resource]
			if !ok {
				set[resource] = []Rule{rule}
			} else {
				rules = append(rules, rule)
				set[resource] = rules
			}
		}
	}
	for resource := range set {
		rules := set[resource]
		// sort rules by most specific rule, the rule that has actions
		// is more specific than the one that has no actions
		sort.Slice(rules, func(i, j int) bool {
			return rules[i].IsMoreSpecificThan(rules[j])
		})
		set[resource] = rules
	}
	return set
}

// CopyRulesSlice copies input slice of Rules and returns the copy
func CopyRulesSlice(in []Rule) []Rule {
	out := make([]Rule, len(in))
	copy(out, in)
	return out
}

// RuleSlicesEqual returns true if two rule slices are equal
func RuleSlicesEqual(a, b []Rule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equals(b[i]) {
			return false
		}
	}
	return true
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

// Expires returns object expiry setting
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
			KindSSHSession:    RO(),
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
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     r.GetMaxSessionTTL(),
				PortForwarding:    true,
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
	rules := []Rule{}
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
			verbs = []string{VerbCreate, VerbRead, VerbUpdate, VerbDelete}
		}

		rules = append(rules, NewRule(resource, verbs))
	}
	role.Spec.Allow.Rules = rules

	err := role.CheckAndSetDefaults()
	if err != nil {
		// as V2 to V3 migration should not throw any errors, we can ignore this error
		log.Warnf("[RBAC] Errors while converting %v from V2 to V3: %v ", r.String(), err)
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
	// Namespaces is a list of namespaces, guarding access to resources
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	// Resources limits access to resources
	Resources map[string][]string `json:"resources,omitempty" yaml:"resources,omitempty"`
	// ForwardAgent permits SSH agent forwarding if requested by the client
	ForwardAgent bool `json:"forward_agent" yaml:"forward_agent"`
}

// AccessChecker interface implements access checks for given role or role set
type AccessChecker interface {
	// HasRole checks if the checker includes the role
	HasRole(role string) bool

	// RoleNames returns a list of role names
	RoleNames() []string

	// CheckAccessToServer checks access to server.
	CheckAccessToServer(login string, server Server) error

	// CheckAccessToRule checks access to a rule within a namespace.
	CheckAccessToRule(context RuleContext, namespace string, rule string, verb string) error

	// CheckLoginDuration checks if role set can login up to given duration and
	// returns a combined list of allowed logins.
	CheckLoginDuration(ttl time.Duration) ([]string, error)

	// AdjustSessionTTL will reduce the requested ttl to lowest max allowed TTL
	// for this role set, otherwise it returns ttl unchanged
	AdjustSessionTTL(ttl time.Duration) time.Duration

	// CheckAgentForward checks if the role can request agent forward for this
	// user.
	CheckAgentForward(login string) error

	// CanForwardAgents returns true if this role set offers capability to forward
	// agents.
	CanForwardAgents() bool

	// CanPortForward returns true if this RoleSet can forward ports.
	CanPortForward() bool

	// CertificateFormat returns the most permissive certificate format in a
	// RoleSet.
	CertificateFormat() string
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
	return []string{VerbList, VerbCreate, VerbRead, VerbUpdate, VerbDelete}
}

// RO is a shortcut that returns read only verbs that provide access to secrets.
func RO() []string {
	return []string{VerbList, VerbRead}
}

// ReadNoSecrets is a shortcut that returns read only verbs that do not
// provide access to secrets.
func ReadNoSecrets() []string {
	return []string{VerbList, VerbReadNoSecrets}
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
	// unauthenticated Nop role should not have any privileges
	// by default, otherwise it is too permissive
	if len(roles) == 1 && roles[0].GetName() == string(teleport.RoleNop) {
		return roles
	}
	return append(roles, NewImplicitRole())
}

// RoleSet is a set of roles that implements access control functionality
type RoleSet []Role

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

// RoleNames returns a slice with role names
func (set RoleSet) RoleNames() []string {
	out := make([]string, len(set))
	for i, r := range set {
		out[i] = r.GetName()
	}
	return out
}

// HasRole checks if the role set has the role
func (set RoleSet) HasRole(role string) bool {
	for _, r := range set {
		if r.GetName() == role {
			return true
		}
	}
	return false
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
		if matchNamespace && (matchLabels || matchLogin) {
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

// CanPortForward returns true if a role in the RoleSet allows port forwarding.
func (set RoleSet) CanPortForward() bool {
	for _, role := range set {
		portForwarding, err := role.GetOptions().GetBoolean(PortForwarding)
		if err != nil {
			return false
		}
		if portForwarding == true {
			return true
		}
	}
	return false
}

// CertificateFormat returns the most permissive certificate format in a
// RoleSet.
func (set RoleSet) CertificateFormat() string {
	var formats []string

	for _, role := range set {
		// get the certificate format for each individual role. if a role does not
		// have a certificate format (like implicit roles) skip over it
		certificateFormat, err := role.GetOptions().GetString(CertificateFormat)
		if err != nil {
			continue
		}

		formats = append(formats, certificateFormat)
	}

	// if no formats were found, return standard
	if len(formats) == 0 {
		return teleport.CertificateFormatStandard
	}

	// sort the slice so the most permissive is the first element
	sort.Slice(formats, func(i, j int) bool {
		return certificatePriority(formats[i]) < certificatePriority(formats[j])
	})

	return formats[0]
}

// certificatePriority returns the priority of the certificate format. The
// most permissive has lowest value.
func certificatePriority(s string) int {
	switch s {
	case teleport.CertificateFormatOldSSH:
		return 0
	case teleport.CertificateFormatStandard:
		return 1
	default:
		return 2
	}
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

func (set RoleSet) CheckAccessToRule(ctx RuleContext, namespace string, resource string, verb string) error {
	whereParser, err := GetWhereParserFn()(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	actionsParser, err := GetActionsParserFn()(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// check deny: a single match on a deny rule prohibits access
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Deny), ProcessNamespace(namespace))
		if matchNamespace {
			matched, err := MakeRuleSet(role.GetRules(Deny)).Match(whereParser, actionsParser, resource, verb)
			if err != nil {
				return trace.Wrap(err)
			}
			if matched {
				log.Infof("[RBAC] %s access to %s [namespace %s] denied for role %q: deny rule matched", verb, resource, namespace, role.GetName())
				return trace.AccessDenied("access denied to perform action '%s' on %s", verb, resource)
			}
		}
	}

	// check allow: if rule matches, grant access to resource
	for _, role := range set {
		matchNamespace := MatchNamespace(role.GetNamespaces(Allow), ProcessNamespace(namespace))
		if matchNamespace {
			match, err := MakeRuleSet(role.GetRules(Allow)).Match(whereParser, actionsParser, resource, verb)
			if err != nil {
				return trace.Wrap(err)
			}
			if match {
				return nil
			}
		}
	}

	log.Infof("[RBAC] %s access to %s [namespace %s] denied for %v: no allow rule matched", verb, resource, namespace, set)
	return trace.AccessDenied("access denied to perform action %q on %q", verb, resource)
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
            },
            "where": {
               "type": "string"
            },
            "actions": {
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

		if err := role.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		roleV3 := role.V3()
		roleV3.rawObject = role

		return roleV3, nil
	case V3:
		var role RoleV3
		if err := utils.UnmarshalWithSchema(GetRoleSchema(V3, ""), &role, data); err != nil {
			return nil, trace.BadParameter(err.Error())
		}

		if err := role.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

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
