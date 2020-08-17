/*
Copyright 2016-2019 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
	"github.com/gravitational/teleport/lib/wrappers"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"

	"github.com/gogo/protobuf/proto"
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
	NewRule(KindEvent, RO()),
}

// DefaultImplicitRules provides access to the default set of implicit rules
// assigned to all roles.
var DefaultImplicitRules = []Rule{
	NewRule(KindNode, RO()),
	NewRule(KindAuthServer, RO()),
	NewRule(KindReverseTunnel, RO()),
	NewRule(KindCertAuthority, ReadNoSecrets()),
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
	NewRule(KindCertAuthority, ReadNoSecrets()),
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
// is not explicitly assigned (this role applies to all users in OSS version).
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
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: Labels{Wildcard: []string{Wildcard}},
				Rules:      CopyRulesSlice(AdminUserRules),
			},
		},
	}
	role.SetLogins(Allow, modules.GetModules().DefaultAllowedLogins())
	role.SetKubeUsers(Allow, modules.GetModules().DefaultKubeUsers())
	role.SetKubeGroups(Allow, modules.GetModules().DefaultKubeGroups())
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
				MaxSessionTTL: MaxDuration(),
				// PortForwarding has to be set to false in the default-implicit-role
				// otherwise all roles will be allowed to forward ports (since we default
				// to true in the check).
				PortForwarding: NewBoolOption(false),
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
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				NodeLabels: Labels{Wildcard: []string{Wildcard}},
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
				NodeLabels: Labels{Wildcard: []string{Wildcard}},
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
	CreateRole(role Role) error

	// UpsertRole creates or updates role
	UpsertRole(ctx context.Context, role Role) error

	// DeleteAllRoles deletes all roles
	DeleteAllRoles() error

	// GetRole returns role by name
	GetRole(name string) (Role, error)

	// DeleteRole deletes role by name
	DeleteRole(ctx context.Context, name string) error
}

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
	GetNodeLabels(RoleConditionType) Labels
	// SetNodeLabels sets the map of node labels this role is allowed or denied access to.
	SetNodeLabels(RoleConditionType, Labels)

	// GetRules gets all allow or deny rules.
	GetRules(rct RoleConditionType) []Rule
	// SetRules sets an allow or deny rule.
	SetRules(rct RoleConditionType, rules []Rule)

	// GetKubeGroups returns kubernetes groups
	GetKubeGroups(RoleConditionType) []string
	// SetKubeGroups sets kubernetes groups for allow or deny condition.
	SetKubeGroups(RoleConditionType, []string)

	// GetKubeUsers returns kubernetes users to impersonate
	GetKubeUsers(RoleConditionType) []string
	// SetKubeUsers sets kubernetes users to impersonate for allow or deny condition.
	SetKubeUsers(RoleConditionType, []string)

	// GetAccessRequestConditions gets allow/deny conditions for access requests.
	GetAccessRequestConditions(RoleConditionType) AccessRequestConditions
	// SetAccessRequestConditions sets allow/deny conditions for access requests.
	SetAccessRequestConditions(RoleConditionType, AccessRequestConditions)
}

// ApplyTraits applies the passed in traits to any variables within the role
// and returns itself.
func ApplyTraits(r Role, traits map[string][]string) Role {
	for _, condition := range []RoleConditionType{Allow, Deny} {
		inLogins := r.GetLogins(condition)

		var outLogins []string
		for _, login := range inLogins {
			variableValues, err := applyValueTraits(login, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping login %v: %v.", login, err)
				}
				continue
			}

			// Filter out logins that come from variables that are not valid Unix logins.
			for _, variableValue := range variableValues {
				if !cstrings.IsValidUnixUser(variableValue) {
					log.Debugf("Skipping login %v, not a valid Unix login.", variableValue)
					continue
				}

				// A valid variable was found in the traits, append it to the list of logins.
				outLogins = append(outLogins, variableValue)
			}
		}

		r.SetLogins(condition, utils.Deduplicate(outLogins))

		// apply templates to kubernetes groups
		inKubeGroups := r.GetKubeGroups(condition)
		var outKubeGroups []string
		for _, group := range inKubeGroups {
			variableValues, err := applyValueTraits(group, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping kube group %v: %v.", group, err)
				}
				continue
			}
			outKubeGroups = append(outKubeGroups, variableValues...)
		}
		r.SetKubeGroups(condition, utils.Deduplicate(outKubeGroups))

		// apply templates to kubernetes users
		inKubeUsers := r.GetKubeUsers(condition)
		var outKubeUsers []string
		for _, user := range inKubeUsers {
			variableValues, err := applyValueTraits(user, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping kube user %v: %v.", user, err)
				}
				continue
			}
			outKubeUsers = append(outKubeUsers, variableValues...)
		}
		r.SetKubeUsers(condition, utils.Deduplicate(outKubeUsers))

		inLabels := r.GetNodeLabels(condition)
		// to avoid unnecessary allocations
		if inLabels != nil {
			outLabels := make(Labels, len(inLabels))

			// every key will be mapped to the first value
			for key, vals := range inLabels {
				keyVars, err := applyValueTraits(key, traits)
				if err != nil {
					// empty key will not match anything
					log.Debugf("Setting empty node label pair %q -> %q: %v", key, vals, err)
					keyVars = []string{""}
				}

				var values []string
				for _, val := range vals {
					valVars, err := applyValueTraits(val, traits)
					if err != nil {
						log.Debugf("Setting empty node label value %q -> %q: %v", key, val, err)
						// empty value will not match anything
						valVars = []string{""}
					}
					values = append(values, valVars...)
				}
				outLabels[keyVars[0]] = utils.Deduplicate(values)
			}
			r.SetNodeLabels(condition, outLabels)
		}
	}

	return r
}

// applyValueTraits applies the passed in traits to the variable,
// returns BadParameter in case if referenced variable is unsupported,
// returns NotFound in case if referenced trait is missing,
// mapped list of values otherwise, the function guarantees to return
// at least one value in case if return value is nil
func applyValueTraits(val string, traits map[string][]string) ([]string, error) {
	// Extract the variable from the role variable.
	variable, err := parse.RoleVariable(val)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		return []string{val}, nil
	}

	// For internal traits, only internal.logins, internal.kubernetes_users and
	// internal.kubernetes_groups are supported at the moment.
	if variable.Namespace() == teleport.TraitInternalPrefix {
		if variable.Name() != teleport.TraitLogins && variable.Name() != teleport.TraitKubeGroups && variable.Name() != teleport.TraitKubeUsers {
			return nil, trace.BadParameter("unsupported variable %q", variable.Name())
		}
	}

	// If the variable is not found in the traits, skip it.
	interpolated, err := variable.Interpolate(traits)
	if trace.IsNotFound(err) || len(interpolated) == 0 {
		return nil, trace.NotFound("variable %q not found in traits", variable.Name())
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return interpolated, nil
}

// GetVersion returns resource version
func (r *RoleV3) GetVersion() string {
	return r.Version
}

// GetKind returns resource kind
func (r *RoleV3) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource sub kind
func (r *RoleV3) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *RoleV3) SetSubKind(s string) {
	r.SubKind = s
}

// GetResourceID returns resource ID
func (r *RoleV3) GetResourceID() int64 {
	return r.Metadata.ID
}

// SetResourceID sets resource ID
func (r *RoleV3) SetResourceID(id int64) {
	r.Metadata.ID = id
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
		if !r.GetNodeLabels(condition).Equals(other.GetNodeLabels(condition)) {
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
	r.Spec.Options = options
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

// GetKubeGroups returns kubernetes groups
func (r *RoleV3) GetKubeGroups(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.KubeGroups
	}
	return r.Spec.Deny.KubeGroups
}

// SetKubeGroups sets kubernetes groups for allow or deny condition.
func (r *RoleV3) SetKubeGroups(rct RoleConditionType, groups []string) {
	lcopy := utils.CopyStrings(groups)

	if rct == Allow {
		r.Spec.Allow.KubeGroups = lcopy
	} else {
		r.Spec.Deny.KubeGroups = lcopy
	}
}

// GetKubeUsers returns kubernetes users
func (r *RoleV3) GetKubeUsers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.KubeUsers
	}
	return r.Spec.Deny.KubeUsers
}

// SetKubeUsers sets kubernetes user for allow or deny condition.
func (r *RoleV3) SetKubeUsers(rct RoleConditionType, users []string) {
	lcopy := utils.CopyStrings(users)

	if rct == Allow {
		r.Spec.Allow.KubeUsers = lcopy
	} else {
		r.Spec.Deny.KubeUsers = lcopy
	}
}

// GetAccessRequestConditions gets conditions for access requests.
func (r *RoleV3) GetAccessRequestConditions(rct RoleConditionType) AccessRequestConditions {
	cond := r.Spec.Deny.Request
	if rct == Allow {
		cond = r.Spec.Allow.Request
	}
	if cond == nil {
		return AccessRequestConditions{}
	}
	return *cond
}

// SetAccessRequestConditions sets allow/deny conditions for access requests.
func (r *RoleV3) SetAccessRequestConditions(rct RoleConditionType, cond AccessRequestConditions) {
	if rct == Allow {
		r.Spec.Allow.Request = &cond
	} else {
		r.Spec.Deny.Request = &cond
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
func (r *RoleV3) GetNodeLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.NodeLabels
	}
	return r.Spec.Deny.NodeLabels
}

// SetNodeLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV3) SetNodeLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.NodeLabels = labels.Clone()
	} else {
		r.Spec.Deny.NodeLabels = labels.Clone()
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

	// Make sure all fields have defaults.
	if r.Spec.Options.CertificateFormat == "" {
		r.Spec.Options.CertificateFormat = teleport.CertificateFormatStandard
	}
	if r.Spec.Options.MaxSessionTTL.Value() == 0 {
		r.Spec.Options.MaxSessionTTL = NewDuration(defaults.MaxCertDuration)
	}
	if r.Spec.Options.PortForwarding == nil {
		r.Spec.Options.PortForwarding = NewBoolOption(true)
	}
	if len(r.Spec.Options.BPF) == 0 {
		r.Spec.Options.BPF = defaults.EnhancedEvents()
	}
	if r.Spec.Allow.Namespaces == nil {
		r.Spec.Allow.Namespaces = []string{defaults.Namespace}
	}
	if r.Spec.Allow.NodeLabels == nil {
		r.Spec.Allow.NodeLabels = Labels{Wildcard: []string{Wildcard}}
	}
	if r.Spec.Deny.Namespaces == nil {
		r.Spec.Deny.Namespaces = []string{defaults.Namespace}
	}

	// Validate that enhanced recording options are all valid.
	for _, opt := range r.Spec.Options.BPF {
		if opt == teleport.EnhancedRecordingCommand ||
			opt == teleport.EnhancedRecordingDisk ||
			opt == teleport.EnhancedRecordingNetwork {
			continue
		}
		return trace.BadParameter("found invalid option in session_recording: %v", opt)
	}

	// if we find {{ or }} but the syntax is invalid, the role is invalid
	for _, condition := range []RoleConditionType{Allow, Deny} {
		for _, login := range r.GetLogins(condition) {
			if strings.Contains(login, "{{") || strings.Contains(login, "}}") {
				_, err := parse.RoleVariable(login)
				if err != nil {
					return trace.BadParameter("invalid login found: %v", login)
				}
			}
		}
	}

	// check and correct the session ttl
	if r.Spec.Options.MaxSessionTTL.Value() <= 0 {
		r.Spec.Options.MaxSessionTTL = NewDuration(defaults.MaxCertDuration)
	}

	// restrict wildcards
	for _, login := range r.Spec.Allow.Logins {
		if login == Wildcard {
			return trace.BadParameter("wildcard matcher is not allowed in logins")
		}
	}
	for key, val := range r.Spec.Allow.NodeLabels {
		if key == Wildcard && !(len(val) == 1 && val[0] == Wildcard) {
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

// Equals checks if all the key/values in the RoleOptions map match.
func (o RoleOptions) Equals(other RoleOptions) bool {
	return (o.ForwardAgent.Value() == other.ForwardAgent.Value() &&
		o.MaxSessionTTL.Value() == other.MaxSessionTTL.Value() &&
		BoolDefaultTrue(o.PortForwarding) == BoolDefaultTrue(other.PortForwarding) &&
		o.CertificateFormat == other.CertificateFormat &&
		o.ClientIdleTimeout.Value() == other.ClientIdleTimeout.Value() &&
		o.DisconnectExpiredCert.Value() == other.DisconnectExpiredCert.Value() &&
		utils.StringSlicesEqual(o.BPF, other.BPF))
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
	if !r.NodeLabels.Equals(o.NodeLabels) {
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

// HasResource returns true if the rule has the specified resource.
func (r *Rule) HasResource(resource string) bool {
	for _, r := range r.Resources {
		if r == resource {
			return true
		}
	}
	return false
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
	// SubKind is a resource subkind
	SubKind string `json:"sub_kind,omitempty"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is Role metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains role specification
	Spec RoleSpecV2 `json:"spec"`
}

// GetVersion returns resource version
func (r *RoleV2) GetVersion() string {
	return r.Version
}

// GetKind returns resource kind
func (r *RoleV2) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource sub kind
func (r *RoleV2) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *RoleV2) SetSubKind(s string) {
	r.SubKind = s
}

// GetResourceID returns resource ID
func (r *RoleV2) GetResourceID() int64 {
	return r.Metadata.ID
}

// SetResourceID sets resource ID
func (r *RoleV2) SetResourceID(id int64) {
	r.Metadata.ID = id
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
	r.Spec.MaxSessionTTL = Duration(duration)
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
	if r.Spec.MaxSessionTTL == 0 {
		r.Spec.MaxSessionTTL = Duration(defaults.MaxCertDuration)
	}
	if r.Spec.MaxSessionTTL.Duration() < defaults.MinCertDuration {
		return trace.BadParameter("maximum session TTL can not be less than %v", defaults.MinCertDuration)
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
				PortForwarding:    NewBoolOption(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Logins:     r.GetLogins(),
				Namespaces: r.GetNamespaces(),
				NodeLabels: scalarLabels(r.GetNodeLabels()).labels(),
			},
		},
	}

	// translate old v2 agent forwarding to a v3 option
	if r.CanForwardAgent() {
		role.Spec.Options.ForwardAgent = NewBool(true)
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
	CheckAccessToRule(context RuleContext, namespace string, rule string, verb string, silent bool) error

	// CheckLoginDuration checks if role set can login up to given duration and
	// returns a combined list of allowed logins.
	CheckLoginDuration(ttl time.Duration) ([]string, error)

	// CheckKubeGroupsAndUsers check if role can login into kubernetes
	// and returns two lists of combined allowed groups and users
	CheckKubeGroupsAndUsers(ttl time.Duration) (groups []string, users []string, err error)

	// AdjustSessionTTL will reduce the requested ttl to lowest max allowed TTL
	// for this role set, otherwise it returns ttl unchanged
	AdjustSessionTTL(ttl time.Duration) time.Duration

	// AdjustClientIdleTimeout adjusts requested idle timeout
	// to the lowest max allowed timeout, the most restrictive
	// option will be picked
	AdjustClientIdleTimeout(ttl time.Duration) time.Duration

	// AdjustDisconnectExpiredCert adjusts the value based on the role set
	// the most restrictive option will be picked
	AdjustDisconnectExpiredCert(disconnect bool) bool

	// CheckAgentForward checks if the role can request agent forward for this
	// user.
	CheckAgentForward(login string) error

	// CanForwardAgents returns true if this role set offers capability to forward
	// agents.
	CanForwardAgents() bool

	// CanPortForward returns true if this RoleSet can forward ports.
	CanPortForward() bool

	// PermitX11Forwarding returns true if this RoleSet allows X11 Forwarding.
	PermitX11Forwarding() bool

	// CertificateFormat returns the most permissive certificate format in a
	// RoleSet.
	CertificateFormat() string

	// EnhancedRecordingSet returns a set of events that will be recorded
	// for enhanced session recording.
	EnhancedRecordingSet() map[string]bool
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

// ExtractFromCertificate will extract roles and traits from a *ssh.Certificate
// or from the backend if they do not exist in the certificate.
func ExtractFromCertificate(access UserGetter, cert *ssh.Certificate) ([]string, wrappers.Traits, error) {
	// For legacy certificates, fetch roles and traits from the services.User
	// object in the backend.
	if isFormatOld(cert) {
		u, err := access.GetUser(cert.KeyId, false)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		log.Warnf("User %v using old style SSH certificate, fetching roles and traits "+
			"from backend. If the identity provider allows username changes, this can "+
			"potentially allow an attacker to change the role of the existing user. "+
			"It's recommended to upgrade to standard SSH certificates.", cert.KeyId)
		return u.GetRoles(), u.GetTraits(), nil
	}

	// Standard certificates have the roles and traits embedded in them.
	roles, err := extractRolesFromCert(cert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	traits, err := extractTraitsFromCert(cert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return roles, traits, nil
}

// ExtractFromIdentity will extract roles and traits from the *x509.Certificate
// which Teleport passes along as a *tlsca.Identity. If roles and traits do not
// exist in the certificates, they are extracted from the backend.
func ExtractFromIdentity(access UserGetter, identity *tlsca.Identity) ([]string, wrappers.Traits, error) {
	// For legacy certificates, fetch roles and traits from the services.User
	// object in the backend.
	if missingIdentity(identity) {
		u, err := access.GetUser(identity.Username, false)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		log.Warnf("Failed to find roles or traits in x509 identity for %v. Fetching	"+
			"from backend. If the identity provider allows username changes, this can "+
			"potentially allow an attacker to change the role of the existing user.",
			identity.Username)
		return u.GetRoles(), u.GetTraits(), nil
	}

	return identity.Groups, identity.Traits, nil
}

// FetchRoles fetches roles by their names, applies the traits to role
// variables, and returns the RoleSet. Adds runtime roles like the default
// implicit role to RoleSet.
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

// isFormatOld returns true if roles and traits were not found in the
// *ssh.Certificate.
func isFormatOld(cert *ssh.Certificate) bool {
	_, hasRoles := cert.Extensions[teleport.CertExtensionTeleportRoles]
	_, hasTraits := cert.Extensions[teleport.CertExtensionTeleportTraits]

	if hasRoles || hasTraits {
		return false
	}
	return true
}

// missingIdentity returns true if the identity is missing or the identity
// has no roles or traits.
func missingIdentity(identity *tlsca.Identity) bool {
	if len(identity.Groups) == 0 || len(identity.Traits) == 0 {
		return true
	}
	return false
}

// extractRolesFromCert extracts roles from certificate metadata extensions.
func extractRolesFromCert(cert *ssh.Certificate) ([]string, error) {
	data, ok := cert.Extensions[teleport.CertExtensionTeleportRoles]
	if !ok {
		return nil, trace.NotFound("no roles found")
	}
	return UnmarshalCertRoles(data)
}

// extractTraitsFromCert extracts traits from the certificate extensions.
func extractTraitsFromCert(cert *ssh.Certificate) (wrappers.Traits, error) {
	rawTraits, ok := cert.Extensions[teleport.CertExtensionTeleportTraits]
	if !ok {
		return nil, trace.NotFound("no traits found")
	}
	var traits wrappers.Traits
	err := wrappers.UnmarshalTraits([]byte(rawTraits), &traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return traits, nil
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

// MatchNamespace returns true if given list of namespace matches
// target namespace, wildcard matches everything.
func MatchNamespace(selectors []string, namespace string) (bool, string) {
	for _, n := range selectors {
		if n == namespace || n == Wildcard {
			return true, "matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, server namespace: %v", selectors, namespace)
}

// MatchLogin returns true if attempted login matches any of the logins.
func MatchLogin(selectors []string, login string) (bool, string) {
	for _, l := range selectors {
		if l == login {
			return true, "matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, login: %v", selectors, login)
}

// MatchLabels matches selector against target. Empty selector matches
// nothing, wildcard matches everything.
func MatchLabels(selector Labels, target map[string]string) (bool, string, error) {
	// Empty selector matches nothing.
	if len(selector) == 0 {
		return false, "no match, empty selector", nil
	}

	// *: * matches everything even empty target set.
	selectorValues := selector[Wildcard]
	if len(selectorValues) == 1 && selectorValues[0] == Wildcard {
		return true, "matched", nil
	}

	// Perform full match.
	for key, selectorValues := range selector {
		targetVal, hasKey := target[key]

		if !hasKey {
			return false, fmt.Sprintf("no key match: '%v'", key), nil
		}

		if !utils.SliceContainsStr(selectorValues, Wildcard) {
			result, err := utils.SliceMatchesRegex(targetVal, selectorValues)
			if err != nil {
				return false, "", trace.Wrap(err)
			} else if !result {
				return false, fmt.Sprintf("no value match: got '%v' want: '%v'", targetVal, selectorValues), nil
			}
		}
	}

	return true, "matched", nil
}

// RoleNames returns a slice with role names. Removes runtime roles like
// the default implicit role.
func (set RoleSet) RoleNames() []string {
	out := make([]string, len(set)-1)
	for i, r := range set {
		if r.GetName() == teleport.DefaultImplicitRole {
			continue
		}
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
// for this role set, otherwise it returns ttl unchanged
func (set RoleSet) AdjustSessionTTL(ttl time.Duration) time.Duration {
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if maxSessionTTL != 0 && ttl > maxSessionTTL {
			ttl = maxSessionTTL
		}
	}
	return ttl
}

// AdjustClientIdleTimeout adjusts requested idle timeout
// to the lowest max allowed timeout, the most restrictive
// option will be picked, negative values will be assumed as 0
func (set RoleSet) AdjustClientIdleTimeout(timeout time.Duration) time.Duration {
	if timeout < 0 {
		timeout = 0
	}
	for _, role := range set {
		roleTimeout := role.GetOptions().ClientIdleTimeout
		// 0 means not set, so it can't be most restrictive, disregard it too
		if roleTimeout.Duration() <= 0 {
			continue
		}
		switch {
		// in case if timeout is 0, means that incoming value
		// does not restrict the idle timeout, pick any other value
		// set by the role
		case timeout == 0:
			timeout = roleTimeout.Duration()
		case roleTimeout.Duration() < timeout:
			timeout = roleTimeout.Duration()
		}
	}
	return timeout
}

// AdjustDisconnectExpiredCert adjusts the value based on the role set
// the most restrictive option will be picked
func (set RoleSet) AdjustDisconnectExpiredCert(disconnect bool) bool {
	for _, role := range set {
		if role.GetOptions().DisconnectExpiredCert.Value() {
			disconnect = true
		}
	}
	return disconnect
}

// CheckKubeGroupsAndUsers check if role can login into kubernetes
// and returns two lists of allowed groups and users
func (set RoleSet) CheckKubeGroupsAndUsers(ttl time.Duration) ([]string, []string, error) {
	groups := make(map[string]struct{})
	users := make(map[string]struct{})
	var matchedTTL bool
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if ttl <= maxSessionTTL && maxSessionTTL != 0 {
			matchedTTL = true
			for _, group := range role.GetKubeGroups(Allow) {
				groups[group] = struct{}{}
			}
			for _, user := range role.GetKubeUsers(Allow) {
				users[user] = struct{}{}
			}
		}
	}
	for _, role := range set {
		for _, group := range role.GetKubeGroups(Deny) {
			delete(groups, group)
		}
		for _, user := range role.GetKubeUsers(Deny) {
			delete(users, user)
		}
	}
	if !matchedTTL {
		return nil, nil, trace.AccessDenied("this user cannot request kubernetes access for %v", ttl)
	}
	if len(groups) == 0 && len(users) == 0 {
		return nil, nil, trace.NotFound("this user cannot request kubernetes access, has no assigned groups or users")
	}
	return utils.StringsSliceFromSet(groups), utils.StringsSliceFromSet(users), nil
}

// CheckLoginDuration checks if role set can login up to given duration and
// returns a combined list of allowed logins.
func (set RoleSet) CheckLoginDuration(ttl time.Duration) ([]string, error) {
	logins := make(map[string]bool)
	var matchedTTL bool
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if ttl <= maxSessionTTL && maxSessionTTL != 0 {
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
//
// Note, logging in this function only happens in debug mode, this is because
// adding logging to this function (which is called on every server returned
// by GetNodes) can slow down this function by 50x for large clusters!
func (set RoleSet) CheckAccessToServer(login string, s Server) error {
	var errs []error

	// Check deny rules first: a single matching namespace, label, or login from
	// the deny role set prohibits access.
	for _, role := range set {
		matchNamespace, namespaceMessage := MatchNamespace(role.GetNamespaces(Deny), s.GetNamespace())
		matchLabels, labelsMessage, err := MatchLabels(role.GetNodeLabels(Deny), s.GetAllLabels())
		if err != nil {
			return trace.Wrap(err)
		}
		matchLogin, loginMessage := MatchLogin(role.GetLogins(Deny), login)
		if matchNamespace && (matchLabels || matchLogin) {
			if log.GetLevel() == log.DebugLevel {
				log.WithFields(log.Fields{
					trace.Component: teleport.ComponentRBAC,
				}).Debugf("Access to node %v denied, deny rule in %v matched; match(namespace=%v, label=%v, login=%v)",
					s.GetHostname(), role.GetName(), namespaceMessage, labelsMessage, loginMessage)
			}
			return trace.AccessDenied("access to server denied")
		}
	}

	// Check allow rules: namespace, label, and login have to all match in
	// one role in the role set to be granted access.
	for _, role := range set {
		matchNamespace, namespaceMessage := MatchNamespace(role.GetNamespaces(Allow), s.GetNamespace())
		matchLabels, labelsMessage, err := MatchLabels(role.GetNodeLabels(Allow), s.GetAllLabels())
		if err != nil {
			return trace.Wrap(err)
		}
		matchLogin, loginMessage := MatchLogin(role.GetLogins(Allow), login)
		if matchNamespace && matchLabels && matchLogin {
			return nil
		}
		if log.GetLevel() == log.DebugLevel {
			deniedError := trace.AccessDenied("role=%v, match(namespace=%v, label=%v, login=%v)",
				role.GetName(), namespaceMessage, labelsMessage, loginMessage)
			errs = append(errs, deniedError)
		}
	}

	if log.GetLevel() == log.DebugLevel {
		log.WithFields(log.Fields{
			trace.Component: teleport.ComponentRBAC,
		}).Debugf("Access to node %v denied, no allow rule matched; %v", s.GetHostname(), errs)
	}
	return trace.AccessDenied("access to server denied")
}

// CanForwardAgents returns true if role set allows forwarding agents.
func (set RoleSet) CanForwardAgents() bool {
	for _, role := range set {
		if role.GetOptions().ForwardAgent.Value() {
			return true
		}
	}
	return false
}

// CanPortForward returns true if a role in the RoleSet allows port forwarding.
func (set RoleSet) CanPortForward() bool {
	for _, role := range set {
		if BoolDefaultTrue(role.GetOptions().PortForwarding) {
			return true
		}
	}
	return false
}

// PermitX11Forwarding returns true if this RoleSet allows X11 Forwarding.
func (set RoleSet) PermitX11Forwarding() bool {
	for _, role := range set {
		if role.GetOptions().PermitX11Forwarding.Value() {
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
		certificateFormat := role.GetOptions().CertificateFormat
		if certificateFormat == "" {
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

// EnhancedRecordingSet returns the set of enhanced session recording
// events to capture for thi role set.
func (set RoleSet) EnhancedRecordingSet() map[string]bool {
	m := make(map[string]bool)

	// Loop over all roles and create a set of all options.
	for _, role := range set {
		for _, opt := range role.GetOptions().BPF {
			m[opt] = true
		}
	}

	return m
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
			if role.GetOptions().ForwardAgent.Value() && l == login {
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

func (set RoleSet) CheckAccessToRule(ctx RuleContext, namespace string, resource string, verb string, silent bool) error {
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
		matchNamespace, _ := MatchNamespace(role.GetNamespaces(Deny), ProcessNamespace(namespace))
		if matchNamespace {
			matched, err := MakeRuleSet(role.GetRules(Deny)).Match(whereParser, actionsParser, resource, verb)
			if err != nil {
				return trace.Wrap(err)
			}
			if matched {
				if !silent {
					log.WithFields(log.Fields{
						trace.Component: teleport.ComponentRBAC,
					}).Infof("Access to %v %v in namespace %v denied to %v: deny rule matched.",
						verb, resource, namespace, role.GetName())
				}
				return trace.AccessDenied("access denied to perform action '%s' on %s", verb, resource)
			}
		}
	}

	// check allow: if rule matches, grant access to resource
	for _, role := range set {
		matchNamespace, _ := MatchNamespace(role.GetNamespaces(Allow), ProcessNamespace(namespace))
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

	if !silent {
		log.WithFields(log.Fields{
			trace.Component: teleport.ComponentRBAC,
		}).Infof("Access to %v %v in namespace %v denied to %v: no allow rule matched.",
			verb, resource, namespace, set)
	}
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
	return Duration(d)
}

// NewBool returns Bool struct based on bool value
func NewBool(b bool) Bool {
	return Bool(b)
}

// NewBoolOption returns Bool struct based on bool value
func NewBoolOption(b bool) *BoolOption {
	v := BoolOption{Value: b}
	return &v
}

// BoolDefaultTrue returns true if v is not set (pointer is nil)
// otherwise returns real boolean value
func BoolDefaultTrue(v *BoolOption) bool {
	if v == nil {
		return true
	}
	return v.Value
}

// Labels is a wrapper around map
// that can marshal and unmarshal itself
// from scalar and list values
type Labels map[string]utils.Strings

func (l Labels) protoType() *wrappers.LabelValues {
	v := &wrappers.LabelValues{
		Values: make(map[string]wrappers.StringValues, len(l)),
	}
	for key, vals := range l {
		stringValues := wrappers.StringValues{
			Values: make([]string, len(vals)),
		}
		copy(stringValues.Values, vals)
		v.Values[key] = stringValues
	}
	return v
}

// Marshal marshals value into protobuf representation
func (l Labels) Marshal() ([]byte, error) {
	return proto.Marshal(l.protoType())
}

// MarshalTo marshals value to the array
func (l Labels) MarshalTo(data []byte) (int, error) {
	return l.protoType().MarshalTo(data)
}

// Unmarshal unmarshals value from protobuf
func (l *Labels) Unmarshal(data []byte) error {
	protoValues := &wrappers.LabelValues{}
	err := proto.Unmarshal(data, protoValues)
	if err != nil {
		return err
	}
	if protoValues.Values == nil {
		return nil
	}
	*l = make(map[string]utils.Strings, len(protoValues.Values))
	for key := range protoValues.Values {
		(*l)[key] = protoValues.Values[key].Values
	}
	return nil
}

// Size returns protobuf size
func (l Labels) Size() int {
	return l.protoType().Size()
}

// Clone returns non-shallow copy of the labels set
func (l Labels) Clone() Labels {
	if l == nil {
		return nil
	}
	out := make(Labels, len(l))
	for key, vals := range l {
		cvals := make([]string, len(vals))
		copy(cvals, vals)
		out[key] = cvals
	}
	return out
}

// Equals returns true if two label sets are equal
func (l Labels) Equals(o Labels) bool {
	if len(l) != len(o) {
		return false
	}
	for key := range l {
		if !utils.StringSlicesEqual(l[key], o[key]) {
			return false
		}
	}
	return true
}

// scalarLabels is a key value map
// with scalar values
type scalarLabels map[string]string

func (l scalarLabels) labels() Labels {
	out := make(Labels, len(l))
	for key, val := range l {
		out[key] = []string{val}
	}
	return out
}

// Bool is a wrapper around boolean values
type Bool bool

// Value returns boolean value of the wrapper
func (b Bool) Value() bool {
	return bool(b)
}

// MarshalJSON marshals boolean value.
func (b Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Value())
}

// UnmarshalJSON unmarshals JSON from string or bool,
// in case if value is missing or not recognized, defaults to false
func (b *Bool) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var boolVal bool
	// check if it's a bool variable
	if err := json.Unmarshal(data, &boolVal); err == nil {
		*b = Bool(boolVal)
		return nil
	}
	// also support string variables
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err != nil {
		return trace.Wrap(err)
	}
	v, err := utils.ParseBool(stringVar)
	if err != nil {
		*b = false
		return nil
	}
	*b = Bool(v)
	return nil
}

// MarshalYAML marshals bool into yaml value
func (b Bool) MarshalYAML() (interface{}, error) {
	return bool(b), nil
}

func (b *Bool) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var boolVar bool
	if err := unmarshal(&boolVar); err == nil {
		*b = Bool(boolVar)
		return nil
	}
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}
	v, err := utils.ParseBool(stringVar)
	if err != nil {
		*b = Bool(v)
		return nil
	}
	*b = Bool(v)
	return nil
}

// BoolOption is a wrapper around bool
// that can take multiple values:
// * true, false and non-set (when pointer is nil)
// and can marshal itself to protobuf equivalent BoolValue
type BoolOption struct {
	// Value is a value of the option
	Value bool
}

func (b *BoolOption) protoType() *BoolValue {
	return &BoolValue{
		Value: b.Value,
	}
}

// MarshalTo marshals value to the slice
func (b BoolOption) MarshalTo(data []byte) (int, error) {
	return b.protoType().MarshalTo(data)
}

// Marshal marshals value into protobuf representation
func (b BoolOption) Marshal() ([]byte, error) {
	return proto.Marshal(b.protoType())
}

// Unmarshal unmarshals value from protobuf
func (b *BoolOption) Unmarshal(data []byte) error {
	protoValue := &BoolValue{}
	err := proto.Unmarshal(data, protoValue)
	if err != nil {
		return err
	}
	b.Value = protoValue.Value
	return nil
}

// Size returns protobuf size
func (b BoolOption) Size() int {
	return b.protoType().Size()
}

// MarshalJSON marshals boolean value.
func (b BoolOption) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Value)
}

// UnmarshalJSON unmarshals JSON from string or bool,
// in case if value is missing or not recognized, defaults to false
func (b *BoolOption) UnmarshalJSON(data []byte) error {
	var val Bool
	if err := val.UnmarshalJSON(data); err != nil {
		return err
	}
	b.Value = val.Value()
	return nil
}

// MarshalYAML marshals bool into yaml value
func (b *BoolOption) MarshalYAML() (interface{}, error) {
	return b.Value, nil
}

func (b *BoolOption) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val Bool
	if err := val.UnmarshalYAML(unmarshal); err != nil {
		return err
	}
	b.Value = val.Value()
	return nil
}

// Duration is a wrapper around duration to set up custom marshal/unmarshal
type Duration time.Duration

// Duration returns time.Duration from Duration typex
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// MarshalJSON marshals Duration to string
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%v", d.Duration()))
}

// Value returns time.Duration value of this wrapper
func (d Duration) Value() time.Duration {
	return time.Duration(d)
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
	if stringVar == teleport.DurationNever {
		*d = Duration(0)
	} else {
		out, err := time.ParseDuration(stringVar)
		if err != nil {
			return trace.BadParameter(err.Error())
		}
		*d = Duration(out)
	}
	return nil
}

// MarshalYAML marshals duration into YAML value,
// encodes it as a string in format "1m"
func (d Duration) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%v", d.Duration()), nil
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}
	if stringVar == teleport.DurationNever {
		*d = Duration(0)
	} else {
		out, err := time.ParseDuration(stringVar)
		if err != nil {
			return trace.BadParameter(err.Error())
		}
		*d = Duration(out)
	}
	return nil
}

const RoleSpecV3SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "max_session_ttl": { "type": "string" },
    "options": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "forward_agent": { "type": ["boolean", "string"] },
        "permit_x11_forwarding": { "type": ["boolean", "string"] },
        "max_session_ttl": { "type": "string" },
        "port_forwarding": { "type": ["boolean", "string"] },
        "cert_format": { "type": "string" },
        "client_idle_timeout": { "type": "string" },
        "disconnect_expired_cert": { "type": ["boolean", "string"] },
        "enhanced_recording": {
          "type": "array",
          "items": { "type": "string" }
        }
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
        "additionalProperties": false,
        "patternProperties": {
          "^[a-zA-Z/.0-9_*-]+$": { "anyOf": [{"type": "string"}, { "type": "array", "items": {"type": "string"}}]}
        }
      },
      "logins": {
        "type": "array",
        "items": { "type": "string" }
      },
      "kubernetes_groups": {
        "type": "array",
        "items": { "type": "string" }
      },
	  "request": {
	    "type": "object",
		"additionalProperties": false,
		"properties": {
		  "roles": {
		    "type": "array",
			"items": { "type": "string" }
		  }
		}
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
         "^[a-zA-Z/.0-9_-]$":  { "type": "string" }
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
func UnmarshalRole(data []byte, opts ...MarshalOption) (*RoleV3, error) {
	var h ResourceHeader
	err := json.Unmarshal(data, &h)
	if err != nil {
		h.Version = V2
	}

	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
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
		roleV3.SetResourceID(cfg.ID)
		return roleV3, nil
	case V3:
		var role RoleV3
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &role); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetRoleSchema(V3, ""), &role, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := role.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.ID != 0 {
			role.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			role.SetExpiry(cfg.Expires)
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
	UnmarshalRole(bytes []byte, opts ...MarshalOption) (Role, error)
	// MarshalRole to binary representation
	MarshalRole(u Role, opts ...MarshalOption) ([]byte, error)
}

type TeleportRoleMarshaler struct{}

// UnmarshalRole unmarshals role from JSON.
func (*TeleportRoleMarshaler) UnmarshalRole(bytes []byte, opts ...MarshalOption) (Role, error) {
	return UnmarshalRole(bytes, opts...)
}

// MarshalRole marshalls role into JSON.
func (*TeleportRoleMarshaler) MarshalRole(r Role, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch role := r.(type) {
	case *RoleV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *role
			copy.SetResourceID(0)
			role = &copy
		}
		return utils.FastMarshal(role)
	default:
		return nil, trace.BadParameter("unrecognized role version %T", r)
	}
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
