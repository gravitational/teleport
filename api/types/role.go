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

package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
)

// Role contains a set of permissions or settings
type Role interface {
	// Resource provides common resource methods.
	Resource
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
	// Equals returns true if the roles are equal. Roles are equal if options and
	// conditions match.
	Equals(other Role) bool

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

	// GetAppLabels gets the map of app labels this role is allowed or denied access to.
	GetAppLabels(RoleConditionType) Labels
	// SetAppLabels sets the map of app labels this role is allowed or denied access to.
	SetAppLabels(RoleConditionType, Labels)

	// GetClusterLabels gets the map of cluster labels this role is allowed or denied access to.
	GetClusterLabels(RoleConditionType) Labels
	// SetClusterLabels sets the map of cluster labels this role is allowed or denied access to.
	SetClusterLabels(RoleConditionType, Labels)

	// GetKubernetesLabels gets the map of kubernetes labels this role is
	// allowed or denied access to.
	GetKubernetesLabels(RoleConditionType) Labels
	// SetKubernetesLabels sets the map of kubernetes labels this role is
	// allowed or denied access to.
	SetKubernetesLabels(RoleConditionType, Labels)

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

	// GetAccessReviewConditions gets allow/deny conditions for access review.
	GetAccessReviewConditions(RoleConditionType) AccessReviewConditions
	// SetAccessReviewConditions sets allow/deny conditions for access review.
	SetAccessReviewConditions(RoleConditionType, AccessReviewConditions)

	// GetDatabaseLabels gets the map of db labels this role is allowed or denied access to.
	GetDatabaseLabels(RoleConditionType) Labels
	// SetDatabaseLabels sets the map of db labels this role is allowed or denied access to.
	SetDatabaseLabels(RoleConditionType, Labels)

	// GetDatabaseNames gets a list of database names this role is allowed or denied access to.
	GetDatabaseNames(RoleConditionType) []string
	// SetDatabasenames sets a list of database names this role is allowed or denied access to.
	SetDatabaseNames(RoleConditionType, []string)

	// GetDatabaseUsers gets a list of database users this role is allowed or denied access to.
	GetDatabaseUsers(RoleConditionType) []string
	// SetDatabaseUsers sets a list of database users this role is allowed or denied access to.
	SetDatabaseUsers(RoleConditionType, []string)

	// GetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
	GetImpersonateConditions(rct RoleConditionType) ImpersonateConditions
	// SetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
	SetImpersonateConditions(rct RoleConditionType, cond ImpersonateConditions)
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

// RoleConditionType specifies if it's an allow rule (true) or deny rule (false).
type RoleConditionType bool

const (
	// Allow is the set of conditions that allow access.
	Allow RoleConditionType = true
	// Deny is the set of conditions that prevent access.
	Deny RoleConditionType = false
)

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
		if !r.GetAppLabels(condition).Equals(other.GetAppLabels(condition)) {
			return false
		}
		if !r.GetDatabaseLabels(condition).Equals(other.GetDatabaseLabels(condition)) {
			return false
		}
		if !utils.StringSlicesEqual(r.GetDatabaseNames(condition), other.GetDatabaseNames(condition)) {
			return false
		}
		if !utils.StringSlicesEqual(r.GetDatabaseUsers(condition), other.GetDatabaseUsers(condition)) {
			return false
		}
		if !RuleSlicesEqual(r.GetRules(condition), other.GetRules(condition)) {
			return false
		}
		if !r.GetClusterLabels(condition).Equals(other.GetClusterLabels(condition)) {
			return false
		}
		if !r.GetKubernetesLabels(condition).Equals(other.GetKubernetesLabels(condition)) {
			return false
		}
		if !r.GetImpersonateConditions(condition).Equals(other.GetImpersonateConditions(condition)) {
			return false
		}
	}

	return true
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

// SetExpiry sets expiry time for the object.
func (r *RoleV3) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns the expiry time for the object.
func (r *RoleV3) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (r *RoleV3) SetTTL(clock Clock, ttl time.Duration) {
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

// GetAccessReviewConditions gets conditions for access reviews.
func (r *RoleV3) GetAccessReviewConditions(rct RoleConditionType) AccessReviewConditions {
	cond := r.Spec.Deny.ReviewRequests
	if rct == Allow {
		cond = r.Spec.Allow.ReviewRequests
	}
	if cond == nil {
		return AccessReviewConditions{}
	}
	return *cond
}

// SetAccessReviewConditions sets allow/deny conditions for access reviews.
func (r *RoleV3) SetAccessReviewConditions(rct RoleConditionType, cond AccessReviewConditions) {
	if rct == Allow {
		r.Spec.Allow.ReviewRequests = &cond
	} else {
		r.Spec.Deny.ReviewRequests = &cond
	}
}

// GetNamespaces gets a list of namespaces this role is allowed or denied access to.
func (r *RoleV3) GetNamespaces(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.Namespaces
	}
	return r.Spec.Deny.Namespaces
}

// SetNamespaces sets a list of namespaces this role is allowed or denied access to.
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

// GetAppLabels gets the map of app labels this role is allowed or denied access to.
func (r *RoleV3) GetAppLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.AppLabels
	}
	return r.Spec.Deny.AppLabels
}

// SetAppLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV3) SetAppLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.AppLabels = labels.Clone()
	} else {
		r.Spec.Deny.AppLabels = labels.Clone()
	}
}

// GetClusterLabels gets the map of cluster labels this role is allowed or denied access to.
func (r *RoleV3) GetClusterLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.ClusterLabels
	}
	return r.Spec.Deny.ClusterLabels
}

// SetClusterLabels sets the map of cluster labels this role is allowed or denied access to.
func (r *RoleV3) SetClusterLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.ClusterLabels = labels.Clone()
	} else {
		r.Spec.Deny.ClusterLabels = labels.Clone()
	}
}

// GetKubernetesLabels gets the map of app labels this role is allowed or denied access to.
func (r *RoleV3) GetKubernetesLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.KubernetesLabels
	}
	return r.Spec.Deny.KubernetesLabels
}

// SetKubernetesLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV3) SetKubernetesLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.KubernetesLabels = labels.Clone()
	} else {
		r.Spec.Deny.KubernetesLabels = labels.Clone()
	}
}

// GetDatabaseLabels gets the map of db labels this role is allowed or denied access to.
func (r *RoleV3) GetDatabaseLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.DatabaseLabels
	}
	return r.Spec.Deny.DatabaseLabels
}

// SetDatabaseLabels sets the map of db labels this role is allowed or denied access to.
func (r *RoleV3) SetDatabaseLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.DatabaseLabels = labels.Clone()
	} else {
		r.Spec.Deny.DatabaseLabels = labels.Clone()
	}
}

// GetDatabaseNames gets a list of database names this role is allowed or denied access to.
func (r *RoleV3) GetDatabaseNames(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DatabaseNames
	}
	return r.Spec.Deny.DatabaseNames
}

// SetDatabaseNames sets a list of database names this role is allowed or denied access to.
func (r *RoleV3) SetDatabaseNames(rct RoleConditionType, values []string) {
	if rct == Allow {
		r.Spec.Allow.DatabaseNames = values
	} else {
		r.Spec.Deny.DatabaseNames = values
	}
}

// GetDatabaseUsers gets a list of database users this role is allowed or denied access to.
func (r *RoleV3) GetDatabaseUsers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DatabaseUsers
	}
	return r.Spec.Deny.DatabaseUsers
}

// SetDatabaseUsers sets a list of database users this role is allowed or denied access to.
func (r *RoleV3) SetDatabaseUsers(rct RoleConditionType, values []string) {
	if rct == Allow {
		r.Spec.Allow.DatabaseUsers = values
	} else {
		r.Spec.Deny.DatabaseUsers = values
	}
}

// GetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
func (r *RoleV3) GetImpersonateConditions(rct RoleConditionType) ImpersonateConditions {
	cond := r.Spec.Deny.Impersonate
	if rct == Allow {
		cond = r.Spec.Allow.Impersonate
	}
	if cond == nil {
		return ImpersonateConditions{}
	}
	return *cond
}

// SetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
func (r *RoleV3) SetImpersonateConditions(rct RoleConditionType, cond ImpersonateConditions) {
	if rct == Allow {
		r.Spec.Allow.Impersonate = &cond
	} else {
		r.Spec.Deny.Impersonate = &cond
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

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (r *RoleV3) CheckAndSetDefaults() error {
	err := r.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	// Make sure all fields have defaults.
	if r.Spec.Options.CertificateFormat == "" {
		r.Spec.Options.CertificateFormat = constants.CertificateFormatStandard
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
		if len(r.Spec.Allow.Logins) == 0 {
			// no logins implies no node access
			r.Spec.Allow.NodeLabels = Labels{}
		} else {
			r.Spec.Allow.NodeLabels = Labels{Wildcard: []string{Wildcard}}
		}
	}

	if r.Spec.Allow.AppLabels == nil {
		r.Spec.Allow.AppLabels = Labels{Wildcard: []string{Wildcard}}
	}

	if r.Spec.Allow.KubernetesLabels == nil {
		r.Spec.Allow.KubernetesLabels = Labels{Wildcard: []string{Wildcard}}
	}

	if r.Spec.Allow.DatabaseLabels == nil {
		r.Spec.Allow.DatabaseLabels = Labels{Wildcard: []string{Wildcard}}
	}

	if r.Spec.Deny.Namespaces == nil {
		r.Spec.Deny.Namespaces = []string{defaults.Namespace}
	}

	// Validate that enhanced recording options are all valid.
	for _, opt := range r.Spec.Options.BPF {
		if opt == constants.EnhancedRecordingCommand ||
			opt == constants.EnhancedRecordingDisk ||
			opt == constants.EnhancedRecordingNetwork {
			continue
		}
		return trace.BadParameter("found invalid option in session_recording: %v", opt)
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
	for key, val := range r.Spec.Allow.AppLabels {
		if key == Wildcard && !(len(val) == 1 && val[0] == Wildcard) {
			return trace.BadParameter("selector *:<val> is not supported")
		}
	}
	for key, val := range r.Spec.Allow.KubernetesLabels {
		if key == Wildcard && !(len(val) == 1 && val[0] == Wildcard) {
			return trace.BadParameter("selector *:<val> is not supported")
		}
	}
	for key, val := range r.Spec.Allow.DatabaseLabels {
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
	if r.Spec.Allow.Impersonate != nil {
		if err := r.Spec.Allow.Impersonate.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	if r.Spec.Deny.Impersonate != nil {
		if r.Spec.Deny.Impersonate.Where != "" {
			return trace.BadParameter("'where' is not supported in deny.impersonate conditions")
		}
		if err := r.Spec.Deny.Impersonate.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
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
	if !r.AppLabels.Equals(o.AppLabels) {
		return false
	}
	if !r.KubernetesLabels.Equals(o.KubernetesLabels) {
		return false
	}
	if !r.DatabaseLabels.Equals(o.DatabaseLabels) {
		return false
	}
	if !utils.StringSlicesEqual(r.DatabaseNames, o.DatabaseNames) {
		return false
	}
	if !utils.StringSlicesEqual(r.DatabaseUsers, o.DatabaseUsers) {
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

// IsEmpty returns true if conditions are unspecified
func (i ImpersonateConditions) IsEmpty() bool {
	return len(i.Users) == 0 || len(i.Roles) == 0
}

// Equals returns true if the impersonate conditions (logins, roles
// and rules) are equal and false if they are not.
func (i ImpersonateConditions) Equals(o ImpersonateConditions) bool {
	if !utils.StringSlicesEqual(i.Users, o.Users) {
		return false
	}
	if !utils.StringSlicesEqual(i.Roles, o.Roles) {
		return false
	}
	if i.Where != o.Where {
		return false
	}
	return true
}

// CheckAndSetDefaults checks and sets default values
func (i ImpersonateConditions) CheckAndSetDefaults() error {
	if len(i.Users) != 0 && len(i.Roles) == 0 {
		return trace.BadParameter("please set both impersonate.users and impersonate.roles")
	}
	if len(i.Users) == 0 && len(i.Roles) != 0 {
		return trace.BadParameter("please set both impersonate.users and impersonate.roles")
	}
	return nil
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

// NewBool returns Bool struct based on bool value
func NewBool(b bool) Bool {
	return Bool(b)
}

// NewBoolP returns Bool pointer
func NewBoolP(b bool) *Bool {
	val := NewBool(b)
	return &val
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

// UnmarshalYAML unmarshals bool value from yaml
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

// MarshalYAML marshals BoolOption into yaml value
func (b *BoolOption) MarshalYAML() (interface{}, error) {
	return b.Value, nil
}

// UnmarshalYAML unmarshals BoolOption to YAML
func (b *BoolOption) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val Bool
	if err := val.UnmarshalYAML(unmarshal); err != nil {
		return err
	}
	b.Value = val.Value()
	return nil
}

// ProcessNamespace sets default namespace in case if namespace is empty
func ProcessNamespace(namespace string) string {
	if namespace == "" {
		return defaults.Namespace
	}
	return namespace
}
