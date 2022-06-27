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

const (
	// OnSessionLeaveTerminate is a moderated sessions policy constant that terminates
	// a session once the require policy is no longer fulfilled.
	OnSessionLeaveTerminate = "terminate"

	// OnSessionLeaveTerminate is a moderated sessions policy constant that pauses
	// a session once the require policies is no longer fulfilled. It is resumed
	// once the requirements are fulfilled again.
	OnSessionLeavePause = "pause"
)

// Role contains a set of permissions or settings
type Role interface {
	// Resource provides common resource methods.
	Resource

	// SetMetadata sets role metadata
	SetMetadata(meta Metadata)

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
	// SetNamespaces sets a list of namespaces this role is allowed or denied access to.
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
	// SetDatabaseNames sets a list of database names this role is allowed or denied access to.
	SetDatabaseNames(RoleConditionType, []string)

	// GetDatabaseUsers gets a list of database users this role is allowed or denied access to.
	GetDatabaseUsers(RoleConditionType) []string
	// SetDatabaseUsers sets a list of database users this role is allowed or denied access to.
	SetDatabaseUsers(RoleConditionType, []string)

	// GetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
	GetImpersonateConditions(rct RoleConditionType) ImpersonateConditions
	// SetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
	SetImpersonateConditions(rct RoleConditionType, cond ImpersonateConditions)

	// GetAWSRoleARNs returns a list of AWS role ARNs this role is allowed to assume.
	GetAWSRoleARNs(RoleConditionType) []string
	// SetAWSRoleARNs returns a list of AWS role ARNs this role is allowed to assume.
	SetAWSRoleARNs(RoleConditionType, []string)

	// GetWindowsDesktopLabels gets the Windows desktop labels this role
	// is allowed or denied access to.
	GetWindowsDesktopLabels(RoleConditionType) Labels
	// SetWindowsDesktopLabels sets the Windows desktop labels this role
	// is allowed or denied access to.
	SetWindowsDesktopLabels(RoleConditionType, Labels)
	// GetWindowsLogins gets Windows desktop logins for allow or deny condition.
	GetWindowsLogins(RoleConditionType) []string
	// SetWindowsLogins sets Windows desktop logins for allow or deny condition.
	SetWindowsLogins(RoleConditionType, []string)

	// GetSessionRequirePolicies returns the RBAC required policies for a session.
	GetSessionRequirePolicies() []*SessionRequirePolicy
	// SetSessionRequirePolicies sets the RBAC required policies for a session.
	SetSessionRequirePolicies([]*SessionRequirePolicy)
	// GetSessionJoinPolicies returns the RBAC join policies for a session.
	GetSessionJoinPolicies() []*SessionJoinPolicy
	// SetSessionJoinPolicies sets the RBAC join policies for a session.
	SetSessionJoinPolicies([]*SessionJoinPolicy)
	// GetSessionPolicySet returns the RBAC policy set for a role.
	GetSessionPolicySet() SessionTrackerPolicySet

	// GetSearchAsRoles returns the list of roles which the user should be able
	// to "assume" while searching for resources, and should be able to request
	// with a search-based access request.
	GetSearchAsRoles() []string
	// SetSearchAsRoles sets the list of roles which the user should be able
	// to "assume" while searching for resources, and should be able to request
	// with a search-based access request.
	SetSearchAsRoles([]string)

	// GetHostGroups gets the list of groups this role is put in when users are provisioned
	GetHostGroups(RoleConditionType) []string
	// SetHostGroups sets the list of groups this role is put in when users are provisioned
	SetHostGroups(RoleConditionType, []string)

	// GetHostSudoers gets the list of sudoers entries for the role
	GetHostSudoers(RoleConditionType) []string
	// SetHostSudoers sets the list of sudoers entries for the role
	SetHostSudoers(RoleConditionType, []string)
}

// NewRole constructs new standard V5 role.
// This creates a V5 role with V4+ RBAC semantics.
func NewRole(name string, spec RoleSpecV5) (Role, error) {
	role := RoleV5{
		Version: V5,
		Metadata: Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := role.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &role, nil
}

// NewRoleV3 constructs new standard V3 role.
// This is mostly a legacy function and will create a role with V3 RBAC semantics.
func NewRoleV3(name string, spec RoleSpecV5) (Role, error) {
	role := RoleV5{
		Version: V3,
		Metadata: Metadata{
			Name: name,
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

// GetVersion returns resource version
func (r *RoleV5) GetVersion() string {
	return r.Version
}

// GetKind returns resource kind
func (r *RoleV5) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource sub kind
func (r *RoleV5) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *RoleV5) SetSubKind(s string) {
	r.SubKind = s
}

// GetResourceID returns resource ID
func (r *RoleV5) GetResourceID() int64 {
	return r.Metadata.ID
}

// SetResourceID sets resource ID
func (r *RoleV5) SetResourceID(id int64) {
	r.Metadata.ID = id
}

// SetExpiry sets expiry time for the object.
func (r *RoleV5) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns the expiry time for the object.
func (r *RoleV5) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetName sets the role name and is a shortcut for SetMetadata().Name.
func (r *RoleV5) SetName(s string) {
	r.Metadata.Name = s
}

// GetName gets the role name and is a shortcut for GetMetadata().Name.
func (r *RoleV5) GetName() string {
	return r.Metadata.Name
}

// GetMetadata returns role metadata.
func (r *RoleV5) GetMetadata() Metadata {
	return r.Metadata
}

// SetMetadata sets role metadata
func (r *RoleV5) SetMetadata(meta Metadata) {
	r.Metadata = meta
}

// GetOptions gets role options.
func (r *RoleV5) GetOptions() RoleOptions {
	return r.Spec.Options
}

// SetOptions sets role options.
func (r *RoleV5) SetOptions(options RoleOptions) {
	r.Spec.Options = options
}

// GetLogins gets system logins for allow or deny condition.
func (r *RoleV5) GetLogins(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.Logins
	}
	return r.Spec.Deny.Logins
}

// SetLogins sets system logins for allow or deny condition.
func (r *RoleV5) SetLogins(rct RoleConditionType, logins []string) {
	lcopy := utils.CopyStrings(logins)

	if rct == Allow {
		r.Spec.Allow.Logins = lcopy
	} else {
		r.Spec.Deny.Logins = lcopy
	}
}

// GetKubeGroups returns kubernetes groups
func (r *RoleV5) GetKubeGroups(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.KubeGroups
	}
	return r.Spec.Deny.KubeGroups
}

// SetKubeGroups sets kubernetes groups for allow or deny condition.
func (r *RoleV5) SetKubeGroups(rct RoleConditionType, groups []string) {
	lcopy := utils.CopyStrings(groups)

	if rct == Allow {
		r.Spec.Allow.KubeGroups = lcopy
	} else {
		r.Spec.Deny.KubeGroups = lcopy
	}
}

// GetKubeUsers returns kubernetes users
func (r *RoleV5) GetKubeUsers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.KubeUsers
	}
	return r.Spec.Deny.KubeUsers
}

// SetKubeUsers sets kubernetes user for allow or deny condition.
func (r *RoleV5) SetKubeUsers(rct RoleConditionType, users []string) {
	lcopy := utils.CopyStrings(users)

	if rct == Allow {
		r.Spec.Allow.KubeUsers = lcopy
	} else {
		r.Spec.Deny.KubeUsers = lcopy
	}
}

// GetAccessRequestConditions gets conditions for access requests.
func (r *RoleV5) GetAccessRequestConditions(rct RoleConditionType) AccessRequestConditions {
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
func (r *RoleV5) SetAccessRequestConditions(rct RoleConditionType, cond AccessRequestConditions) {
	if rct == Allow {
		r.Spec.Allow.Request = &cond
	} else {
		r.Spec.Deny.Request = &cond
	}
}

// GetAccessReviewConditions gets conditions for access reviews.
func (r *RoleV5) GetAccessReviewConditions(rct RoleConditionType) AccessReviewConditions {
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
func (r *RoleV5) SetAccessReviewConditions(rct RoleConditionType, cond AccessReviewConditions) {
	if rct == Allow {
		r.Spec.Allow.ReviewRequests = &cond
	} else {
		r.Spec.Deny.ReviewRequests = &cond
	}
}

// GetNamespaces gets a list of namespaces this role is allowed or denied access to.
func (r *RoleV5) GetNamespaces(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.Namespaces
	}
	return r.Spec.Deny.Namespaces
}

// SetNamespaces sets a list of namespaces this role is allowed or denied access to.
func (r *RoleV5) SetNamespaces(rct RoleConditionType, namespaces []string) {
	ncopy := utils.CopyStrings(namespaces)

	if rct == Allow {
		r.Spec.Allow.Namespaces = ncopy
	} else {
		r.Spec.Deny.Namespaces = ncopy
	}
}

// GetNodeLabels gets the map of node labels this role is allowed or denied access to.
func (r *RoleV5) GetNodeLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.NodeLabels
	}
	return r.Spec.Deny.NodeLabels
}

// SetNodeLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV5) SetNodeLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.NodeLabels = labels.Clone()
	} else {
		r.Spec.Deny.NodeLabels = labels.Clone()
	}
}

// GetAppLabels gets the map of app labels this role is allowed or denied access to.
func (r *RoleV5) GetAppLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.AppLabels
	}
	return r.Spec.Deny.AppLabels
}

// SetAppLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV5) SetAppLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.AppLabels = labels.Clone()
	} else {
		r.Spec.Deny.AppLabels = labels.Clone()
	}
}

// GetClusterLabels gets the map of cluster labels this role is allowed or denied access to.
func (r *RoleV5) GetClusterLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.ClusterLabels
	}
	return r.Spec.Deny.ClusterLabels
}

// SetClusterLabels sets the map of cluster labels this role is allowed or denied access to.
func (r *RoleV5) SetClusterLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.ClusterLabels = labels.Clone()
	} else {
		r.Spec.Deny.ClusterLabels = labels.Clone()
	}
}

// GetKubernetesLabels gets the map of app labels this role is allowed or denied access to.
func (r *RoleV5) GetKubernetesLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.KubernetesLabels
	}
	return r.Spec.Deny.KubernetesLabels
}

// SetKubernetesLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV5) SetKubernetesLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.KubernetesLabels = labels.Clone()
	} else {
		r.Spec.Deny.KubernetesLabels = labels.Clone()
	}
}

// GetDatabaseLabels gets the map of db labels this role is allowed or denied access to.
func (r *RoleV5) GetDatabaseLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.DatabaseLabels
	}
	return r.Spec.Deny.DatabaseLabels
}

// SetDatabaseLabels sets the map of db labels this role is allowed or denied access to.
func (r *RoleV5) SetDatabaseLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.DatabaseLabels = labels.Clone()
	} else {
		r.Spec.Deny.DatabaseLabels = labels.Clone()
	}
}

// GetDatabaseNames gets a list of database names this role is allowed or denied access to.
func (r *RoleV5) GetDatabaseNames(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DatabaseNames
	}
	return r.Spec.Deny.DatabaseNames
}

// SetDatabaseNames sets a list of database names this role is allowed or denied access to.
func (r *RoleV5) SetDatabaseNames(rct RoleConditionType, values []string) {
	if rct == Allow {
		r.Spec.Allow.DatabaseNames = values
	} else {
		r.Spec.Deny.DatabaseNames = values
	}
}

// GetDatabaseUsers gets a list of database users this role is allowed or denied access to.
func (r *RoleV5) GetDatabaseUsers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DatabaseUsers
	}
	return r.Spec.Deny.DatabaseUsers
}

// SetDatabaseUsers sets a list of database users this role is allowed or denied access to.
func (r *RoleV5) SetDatabaseUsers(rct RoleConditionType, values []string) {
	if rct == Allow {
		r.Spec.Allow.DatabaseUsers = values
	} else {
		r.Spec.Deny.DatabaseUsers = values
	}
}

// GetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
func (r *RoleV5) GetImpersonateConditions(rct RoleConditionType) ImpersonateConditions {
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
func (r *RoleV5) SetImpersonateConditions(rct RoleConditionType, cond ImpersonateConditions) {
	if rct == Allow {
		r.Spec.Allow.Impersonate = &cond
	} else {
		r.Spec.Deny.Impersonate = &cond
	}
}

// GetAWSRoleARNs returns a list of AWS role ARNs this role is allowed to impersonate.
func (r *RoleV5) GetAWSRoleARNs(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.AWSRoleARNs
	}
	return r.Spec.Deny.AWSRoleARNs
}

// SetAWSRoleARNs sets a list of AWS role ARNs this role is allowed to impersonate.
func (r *RoleV5) SetAWSRoleARNs(rct RoleConditionType, arns []string) {
	if rct == Allow {
		r.Spec.Allow.AWSRoleARNs = arns
	} else {
		r.Spec.Deny.AWSRoleARNs = arns
	}
}

// GetWindowsDesktopLabels gets the desktop labels this role is allowed or denied access to.
func (r *RoleV5) GetWindowsDesktopLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.WindowsDesktopLabels
	}
	return r.Spec.Deny.WindowsDesktopLabels
}

// SetWindowsDesktopLabels sets the desktop labels this role is allowed or denied access to.
func (r *RoleV5) SetWindowsDesktopLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.WindowsDesktopLabels = labels.Clone()
	} else {
		r.Spec.Deny.WindowsDesktopLabels = labels.Clone()
	}
}

// GetWindowsLogins gets Windows desktop logins for the role's allow or deny condition.
func (r *RoleV5) GetWindowsLogins(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.WindowsDesktopLogins
	}
	return r.Spec.Deny.WindowsDesktopLogins
}

// SetWindowsLogins sets Windows desktop logins for the role's allow or deny condition.
func (r *RoleV5) SetWindowsLogins(rct RoleConditionType, logins []string) {
	lcopy := utils.CopyStrings(logins)

	if rct == Allow {
		r.Spec.Allow.WindowsDesktopLogins = lcopy
	} else {
		r.Spec.Deny.WindowsDesktopLogins = lcopy
	}
}

// GetRules gets all allow or deny rules.
func (r *RoleV5) GetRules(rct RoleConditionType) []Rule {
	if rct == Allow {
		return r.Spec.Allow.Rules
	}
	return r.Spec.Deny.Rules
}

// SetRules sets an allow or deny rule.
func (r *RoleV5) SetRules(rct RoleConditionType, in []Rule) {
	rcopy := CopyRulesSlice(in)

	if rct == Allow {
		r.Spec.Allow.Rules = rcopy
	} else {
		r.Spec.Deny.Rules = rcopy
	}
}

// GetGroups gets all groups for provisioned user
func (r *RoleV5) GetHostGroups(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.HostGroups
	}
	return r.Spec.Deny.HostGroups

}

// SetHostGroups sets all groups for provisioned user
func (r *RoleV5) SetHostGroups(rct RoleConditionType, groups []string) {
	ncopy := utils.CopyStrings(groups)
	if rct == Allow {
		r.Spec.Allow.HostGroups = ncopy
	} else {
		r.Spec.Deny.HostGroups = ncopy
	}
}

// GetHostSudoers gets the list of sudoers entries for the role
func (r *RoleV5) GetHostSudoers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.HostSudoers
	}
	return r.Spec.Deny.HostSudoers

}

// GetHostSudoers sets the list of sudoers entries for the role
func (r *RoleV5) SetHostSudoers(rct RoleConditionType, sudoers []string) {
	ncopy := utils.CopyStrings(sudoers)
	if rct == Allow {
		r.Spec.Allow.HostSudoers = ncopy
	} else {
		r.Spec.Deny.HostSudoers = ncopy
	}
}

// setStaticFields sets static resource header and metadata fields.
func (r *RoleV5) setStaticFields() {
	r.Kind = KindRole
	if r.Version != V3 && r.Version != V4 {
		r.Version = V5
	}
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (r *RoleV5) CheckAndSetDefaults() error {
	r.setStaticFields()
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
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
	if r.Spec.Options.RecordSession == nil {
		r.Spec.Options.RecordSession = &RecordSession{
			Desktop: NewBoolOption(true),
			Default: constants.SessionRecordingModeBestEffort,
		}
	}
	if r.Spec.Options.DesktopClipboard == nil {
		r.Spec.Options.DesktopClipboard = NewBoolOption(true)
	}
	if r.Spec.Options.DesktopDirectorySharing == nil {
		r.Spec.Options.DesktopDirectorySharing = NewBoolOption(true)
	}
	if r.Spec.Options.CreateHostUser == nil {
		r.Spec.Options.CreateHostUser = NewBoolOption(false)
	}

	switch r.Version {
	case V3:
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
	case V4, V5:
		// Labels default to nil/empty for v4+ roles
	default:
		return trace.BadParameter("unrecognized role version: %v", r.Version)
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
		return trace.BadParameter("invalid value for role option enhanced_recording: %v", opt)
	}

	// Validate locking mode.
	switch r.Spec.Options.Lock {
	case "":
		// Missing locking mode implies the cluster-wide default should be used.
	case constants.LockingModeBestEffort, constants.LockingModeStrict:
	default:
		return trace.BadParameter("invalid value for role option lock: %v", r.Spec.Options.Lock)
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
	for _, arn := range r.Spec.Allow.AWSRoleARNs {
		if arn == Wildcard {
			return trace.BadParameter("wildcard matcher is not allowed in aws_role_arns")
		}
	}
	checkWildcardSelector := func(labels Labels) error {
		for key, val := range labels {
			if key == Wildcard && !(len(val) == 1 && val[0] == Wildcard) {
				return trace.BadParameter("selector *:<val> is not supported")
			}
		}
		return nil
	}
	for _, labels := range []Labels{
		r.Spec.Allow.NodeLabels,
		r.Spec.Allow.AppLabels,
		r.Spec.Allow.KubernetesLabels,
		r.Spec.Allow.DatabaseLabels,
		r.Spec.Allow.WindowsDesktopLabels,
	} {
		if err := checkWildcardSelector(labels); err != nil {
			return trace.Wrap(err)
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
func (r *RoleV5) String() string {
	return fmt.Sprintf("Role(Name=%v,Options=%v,Allow=%+v,Deny=%+v)",
		r.GetName(), r.Spec.Options, r.Spec.Allow, r.Spec.Deny)
}

// IsEmpty returns true if conditions are unspecified
func (i ImpersonateConditions) IsEmpty() bool {
	return len(i.Users) == 0 || len(i.Roles) == 0
}

// CheckAndSetDefaults checks and sets default values
func (i ImpersonateConditions) CheckAndSetDefaults() error {
	if len(i.Users) != 0 && len(i.Roles) == 0 {
		// Role-only impersonation note: the phrasing of this error message
		// assumes the user is attempting user (rather than role)
		// impersonation, but this seems like a safe assumption when a user has
		// already been specified.
		return trace.BadParameter("please set both impersonate.users and impersonate.roles for user impersonation")
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

// HasVerb returns true if the rule has the specified verb.
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

// CopyRulesSlice copies input slice of Rules and returns the copy
func CopyRulesSlice(in []Rule) []Rule {
	out := make([]Rule, len(in))
	copy(out, in)
	return out
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

// MarshalToSizedBuffer marshals value to the slice
func (b BoolOption) MarshalToSizedBuffer(data []byte) (int, error) {
	return b.protoType().MarshalToSizedBuffer(data)
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

// ProcessNamespace returns the default namespace in case the namespace is empty.
func ProcessNamespace(namespace string) string {
	if namespace == "" {
		return defaults.Namespace
	}
	return namespace
}

// WhereExpr is a tree like structure representing a `where` (sub-)expression.
type WhereExpr struct {
	Field            string
	Literal          interface{}
	And, Or          WhereExpr2
	Not              *WhereExpr
	Equals, Contains WhereExpr2
}

// WhereExpr2 is a pair of `where` (sub-)expressions.
type WhereExpr2 struct {
	L, R *WhereExpr
}

// String returns a human readable representation of WhereExpr.
func (e WhereExpr) String() string {
	if e.Field != "" {
		return e.Field
	}
	if e.Literal != nil {
		return fmt.Sprintf("%q", e.Literal)
	}
	if e.And.L != nil && e.And.R != nil {
		return fmt.Sprintf("(%s && %s)", e.And.L, e.And.R)
	}
	if e.Or.L != nil && e.Or.R != nil {
		return fmt.Sprintf("(%s || %s)", e.Or.L, e.Or.R)
	}
	if e.Not != nil {
		return fmt.Sprintf("!%s", e.Not)
	}
	if e.Equals.L != nil && e.Equals.R != nil {
		return fmt.Sprintf("equals(%s, %s)", e.Equals.L, e.Equals.R)
	}
	if e.Contains.L != nil && e.Contains.R != nil {
		return fmt.Sprintf("contains(%s, %s)", e.Contains.L, e.Contains.R)
	}
	return ""
}

// GetSessionRequirePolicies returns the RBAC required policies for a role.
func (r *RoleV5) GetSessionRequirePolicies() []*SessionRequirePolicy {
	return r.Spec.Allow.RequireSessionJoin
}

// GetSessionPolicySet returns the RBAC policy set for a session.
func (r *RoleV5) GetSessionPolicySet() SessionTrackerPolicySet {
	return SessionTrackerPolicySet{
		Name:               r.Metadata.Name,
		Version:            r.Version,
		RequireSessionJoin: r.Spec.Allow.RequireSessionJoin,
	}
}

// SetSessionRequirePolicies sets the RBAC required policies for a role.
func (r *RoleV5) SetSessionRequirePolicies(policies []*SessionRequirePolicy) {
	r.Spec.Allow.RequireSessionJoin = policies
}

// SetSessionJoinPolicies returns the RBAC join policies for a role.
func (r *RoleV5) GetSessionJoinPolicies() []*SessionJoinPolicy {
	return r.Spec.Allow.JoinSessions
}

// SetSessionJoinPolicies sets the RBAC join policies for a role.
func (r *RoleV5) SetSessionJoinPolicies(policies []*SessionJoinPolicy) {
	r.Spec.Allow.JoinSessions = policies
}

// GetSearchAsRoles returns the list of roles which the user should be able to
// "assume" while searching for resources, and should be able to request with a
// search-based access request.
func (r *RoleV5) GetSearchAsRoles() []string {
	if r.Spec.Allow.Request == nil {
		return nil
	}
	return r.Spec.Allow.Request.SearchAsRoles
}

// SetSearchAsRoles sets the list of roles which the user should be able to
// "assume" while searching for resources, and should be able to request with a
// search-based access request.
func (r *RoleV5) SetSearchAsRoles(roles []string) {
	if r.Spec.Allow.Request == nil {
		r.Spec.Allow.Request = &AccessRequestConditions{}
	}
	r.Spec.Allow.Request.SearchAsRoles = roles
}
