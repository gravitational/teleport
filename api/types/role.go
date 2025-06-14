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
	"net"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
)

type OnSessionLeaveAction string

const (
	// OnSessionLeaveTerminate is a moderated sessions policy constant that terminates
	// a session once the require policy is no longer fulfilled.
	OnSessionLeaveTerminate OnSessionLeaveAction = "terminate"

	// OnSessionLeaveTerminate is a moderated sessions policy constant that pauses
	// a session once the require policies is no longer fulfilled. It is resumed
	// once the requirements are fulfilled again.
	OnSessionLeavePause OnSessionLeaveAction = "pause"
)

// Match checks if the given role matches this filter.
func (f *RoleFilter) Match(role *RoleV6) bool {
	if f.SkipSystemRoles && IsSystemResource(role) {
		return false
	}

	if len(f.SearchKeywords) != 0 {
		if !role.MatchSearch(f.SearchKeywords) {
			return false
		}
	}

	return true
}

// Role contains a set of permissions or settings
type Role interface {
	// Resource provides common resource methods.
	ResourceWithLabels

	// SetMetadata sets role metadata
	SetMetadata(meta Metadata)

	// GetOptions gets role options.
	GetOptions() RoleOptions
	// SetOptions sets role options
	SetOptions(opt RoleOptions)

	// GetCreateDatabaseUserMode gets the create database user mode option.
	GetCreateDatabaseUserMode() CreateDatabaseUserMode

	// GetLogins gets *nix system logins for allow or deny condition.
	GetLogins(RoleConditionType) []string
	// SetLogins sets *nix system logins for allow or deny condition.
	SetLogins(RoleConditionType, []string)

	// GetNamespaces gets a list of namespaces this role is allowed or denied access to.
	GetNamespaces(RoleConditionType) []string
	// SetNamespaces sets a list of namespaces this role is allowed or denied access to.
	SetNamespaces(RoleConditionType, []string)

	// GetRoleConditions gets the RoleConditions for the RoleConditionType.
	GetRoleConditions(rct RoleConditionType) RoleConditions

	// GetRequestReasonMode gets the RequestReasonMode for the RoleConditionType.
	GetRequestReasonMode(RoleConditionType) RequestReasonMode

	// GetLabelMatchers gets the LabelMatchers that match labels of resources of
	// type [kind] this role is allowed or denied access to.
	GetLabelMatchers(rct RoleConditionType, kind string) (LabelMatchers, error)
	// SetLabelMatchers sets the LabelMatchers that match labels of resources of
	// type [kind] this role is allowed or denied access to.
	SetLabelMatchers(rct RoleConditionType, kind string, labelMatchers LabelMatchers) error

	// GetNodeLabels gets the map of node labels this role is allowed or denied access to.
	GetNodeLabels(RoleConditionType) Labels
	// SetNodeLabels sets the map of node labels this role is allowed or denied access to.
	SetNodeLabels(RoleConditionType, Labels)

	// GetWorkloadIdentityLabels gets the map of node labels this role is
	// allowed or denied access to.
	GetWorkloadIdentityLabels(RoleConditionType) Labels
	// SetWorkloadIdentityLabels sets the map of WorkloadIdentity labels this
	// role is allowed or denied access to.
	SetWorkloadIdentityLabels(RoleConditionType, Labels)

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

	// GetKubeResources returns the Kubernetes Resources this role grants
	// access to.
	GetKubeResources(rct RoleConditionType) []KubernetesResource
	// SetKubeResources configures the Kubernetes Resources for the RoleConditionType.
	SetKubeResources(rct RoleConditionType, pods []KubernetesResource)

	// GetRequestKubernetesResources returns the request Kubernetes resources.
	GetRequestKubernetesResources(rct RoleConditionType) []RequestKubernetesResource
	// SetRequestKubernetesResources sets the request kubernetes resources.
	SetRequestKubernetesResources(rct RoleConditionType, resources []RequestKubernetesResource)

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

	// GetDatabaseRoles gets a list of database roles for auto-provisioned users.
	GetDatabaseRoles(RoleConditionType) []string
	// SetDatabaseRoles sets a list of database roles for auto-provisioned users.
	SetDatabaseRoles(RoleConditionType, []string)

	// GetDatabasePermissions gets database permissions for auto-provisioned users.
	GetDatabasePermissions(rct RoleConditionType) DatabasePermissions
	// SetDatabasePermissions sets database permissions for auto-provisioned users.
	SetDatabasePermissions(RoleConditionType, DatabasePermissions)

	// GetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
	GetImpersonateConditions(rct RoleConditionType) ImpersonateConditions
	// SetImpersonateConditions sets conditions this role is allowed or denied to impersonate.
	SetImpersonateConditions(rct RoleConditionType, cond ImpersonateConditions)

	// GetAWSRoleARNs returns a list of AWS role ARNs this role is allowed to assume.
	GetAWSRoleARNs(RoleConditionType) []string
	// SetAWSRoleARNs sets a list of AWS role ARNs this role is allowed to assume.
	SetAWSRoleARNs(RoleConditionType, []string)

	// GetAzureIdentities returns a list of Azure identities this role is allowed to assume.
	GetAzureIdentities(RoleConditionType) []string
	// SetAzureIdentities sets a list of Azure identities this role is allowed to assume.
	SetAzureIdentities(RoleConditionType, []string)

	// GetGCPServiceAccounts returns a list of GCP service accounts this role is allowed to assume.
	GetGCPServiceAccounts(RoleConditionType) []string
	// SetGCPServiceAccounts sets a list of GCP service accounts this role is allowed to assume.
	SetGCPServiceAccounts(RoleConditionType, []string)

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

	// GetSearchAsRoles returns the list of extra roles which should apply to a
	// user while they are searching for resources as part of a Resource Access
	// Request, and defines the underlying roles which will be requested as part
	// of any Resource Access Request.
	GetSearchAsRoles(RoleConditionType) []string
	// SetSearchAsRoles sets the list of extra roles which should apply to a
	// user while they are searching for resources as part of a Resource Access
	// Request, and defines the underlying roles which will be requested as part
	// of any Resource Access Request.
	SetSearchAsRoles(RoleConditionType, []string)

	// GetPreviewAsRoles returns the list of extra roles which should apply to a
	// reviewer while they are viewing a Resource Access Request for the
	// purposes of viewing details such as the hostname and labels of requested
	// resources.
	GetPreviewAsRoles(RoleConditionType) []string
	// SetPreviewAsRoles sets the list of extra roles which should apply to a
	// reviewer while they are viewing a Resource Access Request for the
	// purposes of viewing details such as the hostname and labels of requested
	// resources.
	SetPreviewAsRoles(RoleConditionType, []string)

	// GetHostGroups gets the list of groups this role is put in when users are provisioned
	GetHostGroups(RoleConditionType) []string
	// SetHostGroups sets the list of groups this role is put in when users are provisioned
	SetHostGroups(RoleConditionType, []string)

	// GetDesktopGroups gets the list of groups this role is put in when desktop users are provisioned
	GetDesktopGroups(RoleConditionType) []string
	// SetDesktopGroups sets the list of groups this role is put in when desktop users are provisioned
	SetDesktopGroups(RoleConditionType, []string)

	// GetHostSudoers gets the list of sudoers entries for the role
	GetHostSudoers(RoleConditionType) []string
	// SetHostSudoers sets the list of sudoers entries for the role
	SetHostSudoers(RoleConditionType, []string)

	// GetPrivateKeyPolicy returns the private key policy enforced for this role.
	GetPrivateKeyPolicy() keys.PrivateKeyPolicy

	// GetDatabaseServiceLabels gets the map of db service labels this role is allowed or denied access to.
	GetDatabaseServiceLabels(RoleConditionType) Labels
	// SetDatabaseServiceLabels sets the map of db service labels this role is allowed or denied access to.
	SetDatabaseServiceLabels(RoleConditionType, Labels)

	// GetGroupLabels gets the map of group labels this role is allowed or denied access to.
	GetGroupLabels(RoleConditionType) Labels
	// SetGroupLabels sets the map of group labels this role is allowed or denied access to.
	SetGroupLabels(RoleConditionType, Labels)

	// GetSPIFFEConditions returns the allow or deny SPIFFERoleCondition.
	GetSPIFFEConditions(rct RoleConditionType) []*SPIFFERoleCondition
	// SetSPIFFEConditions sets the allow or deny SPIFFERoleCondition.
	SetSPIFFEConditions(rct RoleConditionType, cond []*SPIFFERoleCondition)

	// GetGitHubPermissions returns the allow or deny GitHub-related permissions.
	GetGitHubPermissions(RoleConditionType) []GitHubPermission
	// SetGitHubPermissions sets the allow or deny GitHub-related permissions.
	SetGitHubPermissions(RoleConditionType, []GitHubPermission)

	// GetIdentityCenterAccountAssignments fetches the allow or deny Account
	// Assignments for the role
	GetIdentityCenterAccountAssignments(RoleConditionType) []IdentityCenterAccountAssignment
	// GetIdentityCenterAccountAssignments sets the allow or deny Account
	// Assignments for the role
	SetIdentityCenterAccountAssignments(RoleConditionType, []IdentityCenterAccountAssignment)

	// GetMCPPermissions returns the allow or deny MCP permissions.
	GetMCPPermissions(RoleConditionType) *MCPPermissions
	// SetMCPPermissions sets the allow or deny MCP permissions.
	SetMCPPermissions(RoleConditionType, *MCPPermissions)

	// Clone creats a copy of the role.
	Clone() Role
}

// DefaultRoleVersion for NewRole() and test helpers.
// When incrementing the role version, make sure to update the
// role version in the asset file used by the UI.
// See: web/packages/teleport/src/Roles/templates/role.yaml
const DefaultRoleVersion = V8

// NewRole constructs new standard V8 role.
// This creates a V8 role with V4+ RBAC semantics.
func NewRole(name string, spec RoleSpecV6) (Role, error) {
	role, err := NewRoleWithVersion(name, DefaultRoleVersion, spec)
	return role, trace.Wrap(err)
}

// NewRoleWithVersion constructs new standard role with the version specified.
func NewRoleWithVersion(name string, version string, spec RoleSpecV6) (Role, error) {
	role := RoleV6{
		Version: version,
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
func (r *RoleV6) GetVersion() string {
	return r.Version
}

// GetKind returns resource kind
func (r *RoleV6) GetKind() string {
	return r.Kind
}

// GetSubKind returns resource sub kind
func (r *RoleV6) GetSubKind() string {
	return r.SubKind
}

// SetSubKind sets resource subkind
func (r *RoleV6) SetSubKind(s string) {
	r.SubKind = s
}

// GetRevision returns the revision
func (r *RoleV6) GetRevision() string {
	return r.Metadata.GetRevision()
}

// SetRevision sets the revision
func (r *RoleV6) SetRevision(rev string) {
	r.Metadata.SetRevision(rev)
}

// SetExpiry sets expiry time for the object.
func (r *RoleV6) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns the expiry time for the object.
func (r *RoleV6) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetName sets the role name and is a shortcut for SetMetadata().Name.
func (r *RoleV6) SetName(s string) {
	r.Metadata.Name = s
}

// GetName gets the role name and is a shortcut for GetMetadata().Name.
func (r *RoleV6) GetName() string {
	return r.Metadata.Name
}

// GetMetadata returns role metadata.
func (r *RoleV6) GetMetadata() Metadata {
	return r.Metadata
}

// SetMetadata sets role metadata
func (r *RoleV6) SetMetadata(meta Metadata) {
	r.Metadata = meta
}

// GetOptions gets role options.
func (r *RoleV6) GetOptions() RoleOptions {
	return r.Spec.Options
}

// SetOptions sets role options.
func (r *RoleV6) SetOptions(options RoleOptions) {
	r.Spec.Options = options
}

// GetCreateDatabaseUserMode gets the create database user mode option.
func (r *RoleV6) GetCreateDatabaseUserMode() CreateDatabaseUserMode {
	if r.Spec.Options.CreateDatabaseUserMode != CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED {
		return r.Spec.Options.CreateDatabaseUserMode
	}
	// To keep backwards compatibility, look at the create database user option.
	if r.Spec.Options.CreateDatabaseUser != nil && r.Spec.Options.CreateDatabaseUser.Value {
		return CreateDatabaseUserMode_DB_USER_MODE_KEEP
	}
	return CreateDatabaseUserMode_DB_USER_MODE_OFF
}

// GetLogins gets system logins for allow or deny condition.
func (r *RoleV6) GetLogins(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.Logins
	}
	return r.Spec.Deny.Logins
}

// SetLogins sets system logins for allow or deny condition.
func (r *RoleV6) SetLogins(rct RoleConditionType, logins []string) {
	lcopy := utils.CopyStrings(logins)

	if rct == Allow {
		r.Spec.Allow.Logins = lcopy
	} else {
		r.Spec.Deny.Logins = lcopy
	}
}

// GetKubeGroups returns kubernetes groups
func (r *RoleV6) GetKubeGroups(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.KubeGroups
	}
	return r.Spec.Deny.KubeGroups
}

// SetKubeGroups sets kubernetes groups for allow or deny condition.
func (r *RoleV6) SetKubeGroups(rct RoleConditionType, groups []string) {
	lcopy := utils.CopyStrings(groups)

	if rct == Allow {
		r.Spec.Allow.KubeGroups = lcopy
	} else {
		r.Spec.Deny.KubeGroups = lcopy
	}
}

// GetKubeResources returns the Kubernetes Resources this role grants
// access to.
func (r *RoleV6) GetKubeResources(rct RoleConditionType) []KubernetesResource {
	if rct == Allow {
		return r.convertAllowKubernetesResourcesBetweenRoleVersions(r.Spec.Allow.KubernetesResources)
	}
	return r.convertKubernetesResourcesBetweenRoleVersions(r.Spec.Deny.KubernetesResources)
}

// convertKubernetesResourcesBetweenRoleVersions converts Kubernetes resources between role versions.
// This is required to keep compatibility between role versions to avoid breaking changes
// when using an older role version.
//
// For roles v8, it returns the list as it is.
//
// For roles <=v7, it maps the legacy teleport Kinds to k8s plurals and sets the APIGroup to wildcard.
//
// Must be in sync with RoleV6.convertRequestKubernetesResourcesBetweenRoleVersions.
func (r *RoleV6) convertKubernetesResourcesBetweenRoleVersions(resources []KubernetesResource) []KubernetesResource {
	switch r.Version {
	case V8:
		return resources
	default:
		v7resources := slices.Clone(resources)
		var extraResources []KubernetesResource
		for i, r := range v7resources {
			// "namespace" kind used to mean "namespaces" and all resources in the namespace.
			// It is now represented by 'namespaces' for the resource itself and wildcard for
			// all resources in the namespace.
			if r.Kind == KindKubeNamespace {
				r.Kind = Wildcard
				if r.Name == Wildcard {
					r.Namespace = "^.+$"
				} else {
					r.Namespace = r.Name
				}
				r.Name = Wildcard
				r.APIGroup = Wildcard
				v7resources[i] = r
				extraResources = append(extraResources, KubernetesResource{
					Kind:  "namespaces",
					Name:  r.Namespace,
					Verbs: r.Verbs,
				})
				continue
			}
			// The namespace field was ignored in v7 for global resources.
			if r.Namespace != "" && slices.Contains(KubernetesClusterWideResourceKinds, r.Kind) {
				r.Namespace = ""
			}
			if k, ok := KubernetesResourcesKindsPlurals[r.Kind]; ok { // Can be empty if the kind is a wildcard.
				r.APIGroup = KubernetesResourcesV7KindGroups[r.Kind]
				r.Kind = k
			} else {
				r.APIGroup = Wildcard
			}
			v7resources[i] = r
			if r.Kind == Wildcard { // If we have a wildcard, inject the clusterwide resources.
				for _, elem := range KubernetesClusterWideResourceKinds {
					if elem == KindKubeNamespace { // Namespace is handled separately.
						continue
					}
					extraResources = append(extraResources, KubernetesResource{
						Kind:     KubernetesResourcesKindsPlurals[elem],
						Name:     r.Name,
						Verbs:    r.Verbs,
						APIGroup: Wildcard,
					})
				}
			}
		}
		return append(v7resources, extraResources...)
	}
}

// convertAllowKubeResourcesBetweenRoleVersions converts Kubernetes resources between role versions.
// This is required to keep compatibility between role versions to avoid breaking changes
// when using an older role version.
//
// For roles v8, it returns the list as it is.
//
// For roles v7, if we have a Wildcard kind, add the v7 cluster-wide resources to maintain
// the existing behavior as in Teleport <=v17, those resources ignored the namespace value
// of the rbac entry. Earlier roles didn't support wildcard so it is not a concern.
//
// For roles v7, if we have a "namespace" kind, map it to a wildcard + namespaces kind.
//
// For roles <=v7, it sets the APIGroup to wildcard for all resources and maps the legacy
// teleport Kinds to k8s plurals.
//
// For older roles <v7, if the kind is pod and name and namespace are wildcards,
// then return a wildcard resource since RoleV6 and below do not restrict access
// to other resources. This is a simple optimization to reduce the number of resources.
//
// Finally, if the older role version is not a wildcard, then it returns the pod resources as is
// and append the other supported resources - KubernetesResourcesKinds - for Role v8.
func (r *RoleV6) convertAllowKubernetesResourcesBetweenRoleVersions(resources []KubernetesResource) []KubernetesResource {
	switch r.Version {
	case V7, V8:
		// V7 and v8 uses the same logic for allow and deny.
		return r.convertKubernetesResourcesBetweenRoleVersions(resources)
	// Teleport does not support role versions < v3.
	case V6, V5, V4, V3:
		switch {
		// If role does not have kube labels, return empty list since it won't match
		// any kubernetes cluster.
		case !r.HasLabelMatchers(Allow, KindKubernetesCluster):
			return nil
			// If role is not V7 and resources is wildcard, return wildcard for kind as well.
			// This is an optimization to avoid appending multiple resources.
			// This check ignores the Kind field because `validateKubeResources` ensures
			// that for older roles, the Kind field can only be pod.
		case len(resources) == 1 && resources[0].Name == Wildcard && resources[0].Namespace == Wildcard:
			return []KubernetesResource{{Kind: Wildcard, Name: Wildcard, Namespace: Wildcard, Verbs: []string{Wildcard}, APIGroup: Wildcard}}
		default:
			v6resources := slices.Clone(resources)
			for i, r := range v6resources {
				if k, ok := KubernetesResourcesKindsPlurals[r.Kind]; ok {
					r.APIGroup = KubernetesResourcesV7KindGroups[r.Kind]
					r.Kind = k
				} else {
					r.APIGroup = Wildcard
				}
				v6resources[i] = r
			}

			for _, resource := range KubernetesResourcesKinds { // Iterate over the list to have deterministic order.
				group := KubernetesResourcesV7KindGroups[resource]
				resource = KubernetesResourcesKindsPlurals[resource]
				// Ignore Pod resources for older roles because Pods were already supported
				// so we don't need to keep backwards compatibility for them.
				// Also ignore Namespace resources because it grants access to all resources
				// in the namespace.
				if resource == "pods" || resource == "namespaces" {
					continue
				}
				v6resources = append(v6resources, KubernetesResource{Kind: resource, Name: Wildcard, Namespace: Wildcard, Verbs: []string{Wildcard}, APIGroup: group})
			}
			return v6resources
		}
	default:
		return nil
	}
}

// SetKubeResources configures the Kubernetes Resources for the RoleConditionType.
func (r *RoleV6) SetKubeResources(rct RoleConditionType, pods []KubernetesResource) {
	if rct == Allow {
		r.Spec.Allow.KubernetesResources = pods
	} else {
		r.Spec.Deny.KubernetesResources = pods
	}
}

// GetRequestKubernetesResources returns the upgraded request kubernetes resources.
func (r *RoleV6) GetRequestKubernetesResources(rct RoleConditionType) []RequestKubernetesResource {
	if rct == Allow {
		if r.Spec.Allow.Request == nil {
			return nil
		}
		return r.convertRequestKubernetesResourcesBetweenRoleVersions(r.Spec.Allow.Request.KubernetesResources)
	}
	if r.Spec.Deny.Request == nil {
		return nil
	}
	return r.convertRequestKubernetesResourcesBetweenRoleVersions(r.Spec.Deny.Request.KubernetesResources)
}

// SetRequestKubernetesResources sets the request kubernetes resources.
func (r *RoleV6) SetRequestKubernetesResources(rct RoleConditionType, resources []RequestKubernetesResource) {
	roleConditions := &r.Spec.Allow
	if rct == Deny {
		roleConditions = &r.Spec.Deny
	}
	if roleConditions.Request == nil {
		roleConditions.Request = &AccessRequestConditions{}
	}
	roleConditions.Request.KubernetesResources = resources
}

// GetKubeUsers returns kubernetes users
func (r *RoleV6) GetKubeUsers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.KubeUsers
	}
	return r.Spec.Deny.KubeUsers
}

// SetKubeUsers sets kubernetes user for allow or deny condition.
func (r *RoleV6) SetKubeUsers(rct RoleConditionType, users []string) {
	lcopy := utils.CopyStrings(users)

	if rct == Allow {
		r.Spec.Allow.KubeUsers = lcopy
	} else {
		r.Spec.Deny.KubeUsers = lcopy
	}
}

// GetAccessRequestConditions gets conditions for access requests.
func (r *RoleV6) GetAccessRequestConditions(rct RoleConditionType) AccessRequestConditions {
	cond := r.Spec.Deny.Request
	if rct == Allow {
		cond = r.Spec.Allow.Request
	}
	if cond == nil {
		return AccessRequestConditions{}
	}
	return *cond
}

// convertRequestKubernetesResourcesBetweenRoleVersions converts Access Request Kubernetes resources between role versions.
//
// This is required to keep compatibility between role versions to avoid breaking changes
// when using an older role version.
//
// For roles v8, it returns the list as it is.
//
// For roles <=v7, it maps the legacy teleport Kinds to k8s plurals and sets the APIGroup to wildcard.
//
// Must be in sync with RoleV6.convertDenyKubernetesResourcesBetweenRoleVersions.
func (r *RoleV6) convertRequestKubernetesResourcesBetweenRoleVersions(resources []RequestKubernetesResource) []RequestKubernetesResource {
	if len(resources) == 0 {
		return nil
	}
	switch r.Version {
	case V8:
		return resources
	default:
		v7resources := slices.Clone(resources)
		for i, r := range v7resources {
			if k, ok := KubernetesResourcesKindsPlurals[r.Kind]; ok { // Can be empty if the kind is a wildcard.
				r.APIGroup = KubernetesResourcesV7KindGroups[r.Kind]
				r.Kind = k
			} else if r.Kind == KindKubeNamespace {
				r.Kind = "namespaces"
			} else {
				r.APIGroup = Wildcard
			}
			v7resources[i] = r
		}
		return v7resources
	}
}

// SetAccessRequestConditions sets allow/deny conditions for access requests.
func (r *RoleV6) SetAccessRequestConditions(rct RoleConditionType, cond AccessRequestConditions) {
	if rct == Allow {
		r.Spec.Allow.Request = &cond
	} else {
		r.Spec.Deny.Request = &cond
	}
}

// GetAccessReviewConditions gets conditions for access reviews.
func (r *RoleV6) GetAccessReviewConditions(rct RoleConditionType) AccessReviewConditions {
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
func (r *RoleV6) SetAccessReviewConditions(rct RoleConditionType, cond AccessReviewConditions) {
	if rct == Allow {
		r.Spec.Allow.ReviewRequests = &cond
	} else {
		r.Spec.Deny.ReviewRequests = &cond
	}
}

// GetNamespaces gets a list of namespaces this role is allowed or denied access to.
func (r *RoleV6) GetNamespaces(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.Namespaces
	}
	return r.Spec.Deny.Namespaces
}

// SetNamespaces sets a list of namespaces this role is allowed or denied access to.
func (r *RoleV6) SetNamespaces(rct RoleConditionType, namespaces []string) {
	ncopy := utils.CopyStrings(namespaces)

	if rct == Allow {
		r.Spec.Allow.Namespaces = ncopy
	} else {
		r.Spec.Deny.Namespaces = ncopy
	}
}

// GetNodeLabels gets the map of node labels this role is allowed or denied access to.
func (r *RoleV6) GetNodeLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.NodeLabels
	}
	return r.Spec.Deny.NodeLabels
}

// SetNodeLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV6) SetNodeLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.NodeLabels = labels.Clone()
	} else {
		r.Spec.Deny.NodeLabels = labels.Clone()
	}
}

// GetWorkloadIdentityLabels gets the map of WorkloadIdentity labels for
// allow or deny.
func (r *RoleV6) GetWorkloadIdentityLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.WorkloadIdentityLabels
	}
	return r.Spec.Deny.WorkloadIdentityLabels
}

// SetWorkloadIdentityLabels sets the map of WorkloadIdentity labels this role
// is allowed or denied access to.
func (r *RoleV6) SetWorkloadIdentityLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.WorkloadIdentityLabels = labels.Clone()
	} else {
		r.Spec.Deny.WorkloadIdentityLabels = labels.Clone()
	}
}

// GetAppLabels gets the map of app labels this role is allowed or denied access to.
func (r *RoleV6) GetAppLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.AppLabels
	}
	return r.Spec.Deny.AppLabels
}

// SetAppLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV6) SetAppLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.AppLabels = labels.Clone()
	} else {
		r.Spec.Deny.AppLabels = labels.Clone()
	}
}

// GetClusterLabels gets the map of cluster labels this role is allowed or denied access to.
func (r *RoleV6) GetClusterLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.ClusterLabels
	}
	return r.Spec.Deny.ClusterLabels
}

// SetClusterLabels sets the map of cluster labels this role is allowed or denied access to.
func (r *RoleV6) SetClusterLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.ClusterLabels = labels.Clone()
	} else {
		r.Spec.Deny.ClusterLabels = labels.Clone()
	}
}

// GetKubernetesLabels gets the map of app labels this role is allowed or denied access to.
func (r *RoleV6) GetKubernetesLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.KubernetesLabels
	}
	return r.Spec.Deny.KubernetesLabels
}

// SetKubernetesLabels sets the map of node labels this role is allowed or denied access to.
func (r *RoleV6) SetKubernetesLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.KubernetesLabels = labels.Clone()
	} else {
		r.Spec.Deny.KubernetesLabels = labels.Clone()
	}
}

// GetDatabaseServiceLabels gets the map of db service labels this role is allowed or denied access to.
func (r *RoleV6) GetDatabaseServiceLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.DatabaseServiceLabels
	}
	return r.Spec.Deny.DatabaseServiceLabels
}

// SetDatabaseServiceLabels sets the map of db service labels this role is allowed or denied access to.
func (r *RoleV6) SetDatabaseServiceLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.DatabaseServiceLabels = labels.Clone()
	} else {
		r.Spec.Deny.DatabaseServiceLabels = labels.Clone()
	}
}

// GetDatabaseLabels gets the map of db labels this role is allowed or denied access to.
func (r *RoleV6) GetDatabaseLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.DatabaseLabels
	}
	return r.Spec.Deny.DatabaseLabels
}

// SetDatabaseLabels sets the map of db labels this role is allowed or denied access to.
func (r *RoleV6) SetDatabaseLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.DatabaseLabels = labels.Clone()
	} else {
		r.Spec.Deny.DatabaseLabels = labels.Clone()
	}
}

// GetDatabaseNames gets a list of database names this role is allowed or denied access to.
func (r *RoleV6) GetDatabaseNames(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DatabaseNames
	}
	return r.Spec.Deny.DatabaseNames
}

// SetDatabaseNames sets a list of database names this role is allowed or denied access to.
func (r *RoleV6) SetDatabaseNames(rct RoleConditionType, values []string) {
	if rct == Allow {
		r.Spec.Allow.DatabaseNames = values
	} else {
		r.Spec.Deny.DatabaseNames = values
	}
}

// GetDatabaseUsers gets a list of database users this role is allowed or denied access to.
func (r *RoleV6) GetDatabaseUsers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DatabaseUsers
	}
	return r.Spec.Deny.DatabaseUsers
}

// SetDatabaseUsers sets a list of database users this role is allowed or denied access to.
func (r *RoleV6) SetDatabaseUsers(rct RoleConditionType, values []string) {
	if rct == Allow {
		r.Spec.Allow.DatabaseUsers = values
	} else {
		r.Spec.Deny.DatabaseUsers = values
	}
}

// GetDatabaseRoles gets a list of database roles for auto-provisioned users.
func (r *RoleV6) GetDatabaseRoles(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DatabaseRoles
	}
	return r.Spec.Deny.DatabaseRoles
}

// SetDatabaseRoles sets a list of database roles for auto-provisioned users.
func (r *RoleV6) SetDatabaseRoles(rct RoleConditionType, values []string) {
	if rct == Allow {
		r.Spec.Allow.DatabaseRoles = values
	} else {
		r.Spec.Deny.DatabaseRoles = values
	}
}

// GetDatabasePermissions gets a list of database permissions for auto-provisioned users.
func (r *RoleV6) GetDatabasePermissions(rct RoleConditionType) DatabasePermissions {
	if rct == Allow {
		return r.Spec.Allow.DatabasePermissions
	}
	return r.Spec.Deny.DatabasePermissions
}

// SetDatabasePermissions sets a list of database permissions for auto-provisioned users.
func (r *RoleV6) SetDatabasePermissions(rct RoleConditionType, values DatabasePermissions) {
	if rct == Allow {
		r.Spec.Allow.DatabasePermissions = values
	} else {
		r.Spec.Deny.DatabasePermissions = values
	}
}

// GetImpersonateConditions returns conditions this role is allowed or denied to impersonate.
func (r *RoleV6) GetImpersonateConditions(rct RoleConditionType) ImpersonateConditions {
	cond := r.Spec.Deny.Impersonate
	if rct == Allow {
		cond = r.Spec.Allow.Impersonate
	}
	if cond == nil {
		return ImpersonateConditions{}
	}
	return *cond
}

// SetImpersonateConditions sets conditions this role is allowed or denied to impersonate.
func (r *RoleV6) SetImpersonateConditions(rct RoleConditionType, cond ImpersonateConditions) {
	if rct == Allow {
		r.Spec.Allow.Impersonate = &cond
	} else {
		r.Spec.Deny.Impersonate = &cond
	}
}

// GetAWSRoleARNs returns a list of AWS role ARNs this role is allowed to impersonate.
func (r *RoleV6) GetAWSRoleARNs(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.AWSRoleARNs
	}
	return r.Spec.Deny.AWSRoleARNs
}

// SetAWSRoleARNs sets a list of AWS role ARNs this role is allowed to impersonate.
func (r *RoleV6) SetAWSRoleARNs(rct RoleConditionType, arns []string) {
	if rct == Allow {
		r.Spec.Allow.AWSRoleARNs = arns
	} else {
		r.Spec.Deny.AWSRoleARNs = arns
	}
}

// GetAzureIdentities returns a list of Azure identities this role is allowed to assume.
func (r *RoleV6) GetAzureIdentities(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.AzureIdentities
	}
	return r.Spec.Deny.AzureIdentities
}

// SetAzureIdentities sets a list of Azure identities this role is allowed to assume.
func (r *RoleV6) SetAzureIdentities(rct RoleConditionType, identities []string) {
	if rct == Allow {
		r.Spec.Allow.AzureIdentities = identities
	} else {
		r.Spec.Deny.AzureIdentities = identities
	}
}

// GetGCPServiceAccounts returns a list of GCP service accounts this role is allowed to assume.
func (r *RoleV6) GetGCPServiceAccounts(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.GCPServiceAccounts
	}
	return r.Spec.Deny.GCPServiceAccounts
}

// SetGCPServiceAccounts sets a list of GCP service accounts this role is allowed to assume.
func (r *RoleV6) SetGCPServiceAccounts(rct RoleConditionType, accounts []string) {
	if rct == Allow {
		r.Spec.Allow.GCPServiceAccounts = accounts
	} else {
		r.Spec.Deny.GCPServiceAccounts = accounts
	}
}

// GetWindowsDesktopLabels gets the desktop labels this role is allowed or denied access to.
func (r *RoleV6) GetWindowsDesktopLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.WindowsDesktopLabels
	}
	return r.Spec.Deny.WindowsDesktopLabels
}

// SetWindowsDesktopLabels sets the desktop labels this role is allowed or denied access to.
func (r *RoleV6) SetWindowsDesktopLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.WindowsDesktopLabels = labels.Clone()
	} else {
		r.Spec.Deny.WindowsDesktopLabels = labels.Clone()
	}
}

// GetWindowsLogins gets Windows desktop logins for the role's allow or deny condition.
func (r *RoleV6) GetWindowsLogins(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.WindowsDesktopLogins
	}
	return r.Spec.Deny.WindowsDesktopLogins
}

// SetWindowsLogins sets Windows desktop logins for the role's allow or deny condition.
func (r *RoleV6) SetWindowsLogins(rct RoleConditionType, logins []string) {
	lcopy := utils.CopyStrings(logins)

	if rct == Allow {
		r.Spec.Allow.WindowsDesktopLogins = lcopy
	} else {
		r.Spec.Deny.WindowsDesktopLogins = lcopy
	}
}

// GetRules gets all allow or deny rules.
func (r *RoleV6) GetRules(rct RoleConditionType) []Rule {
	if rct == Allow {
		return r.Spec.Allow.Rules
	}
	return r.Spec.Deny.Rules
}

// SetRules sets an allow or deny rule.
func (r *RoleV6) SetRules(rct RoleConditionType, in []Rule) {
	rcopy := CopyRulesSlice(in)

	if rct == Allow {
		r.Spec.Allow.Rules = rcopy
	} else {
		r.Spec.Deny.Rules = rcopy
	}
}

// GetHostGroups gets all groups for provisioned user
func (r *RoleV6) GetHostGroups(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.HostGroups
	}
	return r.Spec.Deny.HostGroups
}

// SetHostGroups sets all groups for provisioned user
func (r *RoleV6) SetHostGroups(rct RoleConditionType, groups []string) {
	ncopy := utils.CopyStrings(groups)
	if rct == Allow {
		r.Spec.Allow.HostGroups = ncopy
	} else {
		r.Spec.Deny.HostGroups = ncopy
	}
}

// GetDesktopGroups gets all groups for provisioned user
func (r *RoleV6) GetDesktopGroups(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.DesktopGroups
	}
	return r.Spec.Deny.DesktopGroups
}

// SetDesktopGroups sets all groups for provisioned user
func (r *RoleV6) SetDesktopGroups(rct RoleConditionType, groups []string) {
	ncopy := utils.CopyStrings(groups)
	if rct == Allow {
		r.Spec.Allow.DesktopGroups = ncopy
	} else {
		r.Spec.Deny.DesktopGroups = ncopy
	}
}

// GetHostSudoers gets the list of sudoers entries for the role
func (r *RoleV6) GetHostSudoers(rct RoleConditionType) []string {
	if rct == Allow {
		return r.Spec.Allow.HostSudoers
	}
	return r.Spec.Deny.HostSudoers
}

// GetHostSudoers sets the list of sudoers entries for the role
func (r *RoleV6) SetHostSudoers(rct RoleConditionType, sudoers []string) {
	ncopy := utils.CopyStrings(sudoers)
	if rct == Allow {
		r.Spec.Allow.HostSudoers = ncopy
	} else {
		r.Spec.Deny.HostSudoers = ncopy
	}
}

// GetSPIFFEConditions returns the allow or deny SPIFFERoleCondition.
func (r *RoleV6) GetSPIFFEConditions(rct RoleConditionType) []*SPIFFERoleCondition {
	if rct == Allow {
		return r.Spec.Allow.SPIFFE
	}
	return r.Spec.Deny.SPIFFE
}

// SetSPIFFEConditions sets the allow or deny SPIFFERoleCondition.
func (r *RoleV6) SetSPIFFEConditions(rct RoleConditionType, cond []*SPIFFERoleCondition) {
	if rct == Allow {
		r.Spec.Allow.SPIFFE = cond
	} else {
		r.Spec.Deny.SPIFFE = cond
	}
}

// GetGitHubPermissions returns the allow or deny GitHubPermission.
func (r *RoleV6) GetGitHubPermissions(rct RoleConditionType) []GitHubPermission {
	if rct == Allow {
		return r.Spec.Allow.GitHubPermissions
	}
	return r.Spec.Deny.GitHubPermissions
}

// SetGitHubPermissions sets the allow or deny GitHubPermission.
func (r *RoleV6) SetGitHubPermissions(rct RoleConditionType, perms []GitHubPermission) {
	if rct == Allow {
		r.Spec.Allow.GitHubPermissions = perms
	} else {
		r.Spec.Deny.GitHubPermissions = perms
	}
}

// GetPrivateKeyPolicy returns the private key policy enforced for this role.
func (r *RoleV6) GetPrivateKeyPolicy() keys.PrivateKeyPolicy {
	switch r.Spec.Options.RequireMFAType {
	case RequireMFAType_SESSION_AND_HARDWARE_KEY:
		return keys.PrivateKeyPolicyHardwareKey
	case RequireMFAType_HARDWARE_KEY_TOUCH:
		return keys.PrivateKeyPolicyHardwareKeyTouch
	case RequireMFAType_HARDWARE_KEY_PIN:
		return keys.PrivateKeyPolicyHardwareKeyPIN
	case RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN:
		return keys.PrivateKeyPolicyHardwareKeyTouchAndPIN
	default:
		return keys.PrivateKeyPolicyNone
	}
}

// setStaticFields sets static resource header and metadata fields.
func (r *RoleV6) setStaticFields() {
	r.Kind = KindRole
	if r.Version != V3 && r.Version != V4 && r.Version != V5 && r.Version != V6 && r.Version != V7 {
		// When incrementing the role version, make sure to update the
		// role version in the asset file used by the UI.
		// See: web/packages/teleport/src/Roles/templates/role.yaml
		r.Version = V8
	}
}

// GetGroupLabels gets the map of group labels this role is allowed or denied access to.
func (r *RoleV6) GetGroupLabels(rct RoleConditionType) Labels {
	if rct == Allow {
		return r.Spec.Allow.GroupLabels
	}
	return r.Spec.Deny.GroupLabels
}

// SetGroupLabels sets the map of group labels this role is allowed or denied access to.
func (r *RoleV6) SetGroupLabels(rct RoleConditionType, labels Labels) {
	if rct == Allow {
		r.Spec.Allow.GroupLabels = labels.Clone()
	} else {
		r.Spec.Deny.GroupLabels = labels.Clone()
	}
}

// CheckAndSetDefaults checks validity of all fields and sets defaults
func (c *SPIFFERoleCondition) CheckAndSetDefaults() error {
	if c.Path == "" {
		return trace.BadParameter("path: should be non-empty")
	}
	isRegex := strings.HasPrefix(c.Path, "^") && strings.HasSuffix(c.Path, "$")
	if !strings.HasPrefix(c.Path, "/") && !isRegex {
		return trace.BadParameter(
			"path: should start with / or be a regex expression starting with ^ and ending with $",
		)
	}
	for i, str := range c.IPSANs {
		if _, _, err := net.ParseCIDR(str); err != nil {
			return trace.BadParameter(
				"validating ip_sans[%d]: %s", i, err.Error(),
			)
		}
	}
	return nil
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
// Must be kept in sync with
// `web/packages/teleport/src/Roles/RoleEditor/withDefaults.ts`.
func (r *RoleV6) CheckAndSetDefaults() error {
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
	if len(r.Spec.Options.BPF) == 0 {
		r.Spec.Options.BPF = defaults.EnhancedEvents()
	}
	if err := checkAndSetRoleConditionNamespaces(&r.Spec.Allow.Namespaces); err != nil {
		// Using trace.BadParameter instead of trace.Wrap for a better error message.
		return trace.BadParameter("allow: %s", err)
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
	if r.Spec.Options.CreateDesktopUser == nil {
		r.Spec.Options.CreateDesktopUser = NewBoolOption(false)
	}
	if r.Spec.Options.CreateDatabaseUser == nil {
		r.Spec.Options.CreateDatabaseUser = NewBoolOption(false)
	}
	if r.Spec.Options.SSHFileCopy == nil {
		r.Spec.Options.SSHFileCopy = NewBoolOption(true)
	}
	if r.Spec.Options.IDP == nil {
		if IsLegacySAMLRBAC(r.GetVersion()) {
			// By default, allow users to access the IdP.
			r.Spec.Options.IDP = &IdPOptions{
				SAML: &IdPSAMLOptions{
					Enabled: NewBoolOption(true),
				},
			}
		}
	}

	if _, ok := CreateHostUserMode_name[int32(r.Spec.Options.CreateHostUserMode)]; !ok {
		return trace.BadParameter("invalid host user mode %q, expected one of off, drop or keep", r.Spec.Options.CreateHostUserMode)
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

		fallthrough
	case V4, V5:
		// Labels default to nil/empty for v4+ roles
		// Allow unrestricted access to all pods.
		if len(r.Spec.Allow.KubernetesResources) == 0 && r.HasLabelMatchers(Allow, KindKubernetesCluster) {
			r.Spec.Allow.KubernetesResources = []KubernetesResource{
				{
					Kind:      KindKubePod,
					Namespace: Wildcard,
					Name:      Wildcard,
				},
			}
		}
		fallthrough
	case V6:
		setDefaultKubernetesVerbs(&r.Spec)
		if err := validateRoleSpecKubeResources(r.Version, r.Spec); err != nil {
			return trace.Wrap(err)
		}
	case V7:
		// Kubernetes resources default to {kind:*, name:*, namespace:*, verbs:[*]} for v7 roles.
		if len(r.Spec.Allow.KubernetesResources) == 0 && r.HasLabelMatchers(Allow, KindKubernetesCluster) {
			r.Spec.Allow.KubernetesResources = []KubernetesResource{
				// Full access to everything.
				{
					Kind:      Wildcard,
					Namespace: Wildcard,
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
				},
			}
		}
		if err := validateRoleSpecKubeResources(r.Version, r.Spec); err != nil {
			return trace.Wrap(err)
		}
	case V8:
		// Kubernetes resources default to {kind:*, name:*, namespace:*, api_group:*, verbs:[*]} for v8 roles.
		if len(r.Spec.Allow.KubernetesResources) == 0 && r.HasLabelMatchers(Allow, KindKubernetesCluster) {
			r.Spec.Allow.KubernetesResources = []KubernetesResource{
				// Full access to everything.
				{
					Kind:      Wildcard,
					Namespace: Wildcard,
					Name:      Wildcard,
					Verbs:     []string{Wildcard},
					APIGroup:  Wildcard,
				},
			}
		}

		if err := validateRoleSpecKubeResources(r.Version, r.Spec); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unrecognized role version: %v", r.Version)
	}

	if err := checkAndSetRoleConditionNamespaces(&r.Spec.Deny.Namespaces); err != nil {
		// Using trace.BadParameter instead of trace.Wrap for a better error message.
		return trace.BadParameter("deny: %s", err)
	}

	// Validate request.kubernetes_resources fields are all valid.
	if r.Spec.Allow.Request != nil {
		if err := validateRequestKubeResources(r.Version, r.Spec.Allow.Request.KubernetesResources); err != nil {
			return trace.Wrap(err)
		}
	}
	if r.Spec.Deny.Request != nil {
		if err := validateRequestKubeResources(r.Version, r.Spec.Deny.Request.KubernetesResources); err != nil {
			return trace.Wrap(err)
		}
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
	for _, identity := range r.Spec.Allow.AzureIdentities {
		if identity == Wildcard {
			return trace.BadParameter("wildcard matcher is not allowed in allow.azure_identities")
		}
	}
	for _, identity := range r.Spec.Allow.GCPServiceAccounts {
		if identity == Wildcard {
			return trace.BadParameter("wildcard matcher is not allowed in allow.gcp_service_accounts")
		}
	}
	for _, role := range r.Spec.Allow.DatabaseRoles {
		if role == Wildcard {
			return trace.BadParameter("wildcard is not allowed in allow.database_roles")
		}
	}
	checkWildcardSelector := func(labels Labels) error {
		for key, val := range labels {
			if key == Wildcard && (len(val) != 1 || val[0] != Wildcard) {
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
		r.Spec.Allow.GroupLabels,
		r.Spec.Allow.WorkloadIdentityLabels,
	} {
		if err := checkWildcardSelector(labels); err != nil {
			return trace.Wrap(err)
		}
	}

	for i, perm := range r.Spec.Allow.DatabasePermissions {
		if err := perm.CheckAndSetDefaults(); err != nil {
			return trace.BadParameter("failed to process 'allow' db_permission #%v: %v", i+1, err)
		}
		// Wildcards permissions are disallowed. Even though this should never pass the db-specific driver,
		// it doesn't hurt to check it here. Wildcards *are* allowed on deny side,
		// which is why this check is here and not in CheckAndSetDefaults().
		for _, permission := range perm.Permissions {
			if permission == Wildcard {
				return trace.BadParameter("individual database permissions cannot be wildcards strings")
			}
		}
	}
	for i, perm := range r.Spec.Deny.DatabasePermissions {
		if err := perm.CheckAndSetDefaults(); err != nil {
			return trace.BadParameter("failed to process 'deny' db_permission #%v: %v", i+1, err)
		}
	}
	for i := range r.Spec.Allow.SPIFFE {
		err := r.Spec.Allow.SPIFFE[i].CheckAndSetDefaults()
		if err != nil {
			return trace.Wrap(err, "validating spec.allow.spiffe[%d]", i)
		}
	}
	for i := range r.Spec.Deny.SPIFFE {
		err := r.Spec.Deny.SPIFFE[i].CheckAndSetDefaults()
		if err != nil {
			return trace.Wrap(err, "validating spec.deny.spiffe[%d]", i)
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

func checkAndSetRoleConditionNamespaces(namespaces *[]string) error {
	// If nil use the default.
	// This distinguishes between nil and empty (in accordance to legacy code).
	if *namespaces == nil {
		*namespaces = []string{defaults.Namespace}
		return nil
	}

	for i, ns := range *namespaces {
		if ns == Wildcard {
			continue // OK, wildcard is accepted.
		}
		if err := ValidateNamespaceDefault(ns); err != nil {
			// Using trace.BadParameter instead of trace.Wrap for a better error message.
			return trace.BadParameter("namespaces[%d]: %s", i, err)
		}
	}

	return nil
}

// String returns the human readable representation of a role.
func (r *RoleV6) String() string {
	options, _ := json.Marshal(r.Spec.Options)
	return fmt.Sprintf("Role(Name=%v,Options=%q,Allow=%+v,Deny=%+v)",
		r.GetName(), string(options), r.Spec.Allow, r.Spec.Deny)
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

// ToProto returns a protobuf-compatible representation of Labels.
func (l Labels) ToProto() *wrappers.LabelValues {
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
	return proto.Marshal(l.ToProto())
}

// MarshalTo marshals value to the array
func (l Labels) MarshalTo(data []byte) (int, error) {
	return l.ToProto().MarshalTo(data)
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
	return l.ToProto().Size()
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
func (r *RoleV6) GetSessionRequirePolicies() []*SessionRequirePolicy {
	return r.Spec.Allow.RequireSessionJoin
}

// GetSessionPolicySet returns the RBAC policy set for a session.
func (r *RoleV6) GetSessionPolicySet() SessionTrackerPolicySet {
	return SessionTrackerPolicySet{
		Name:               r.Metadata.Name,
		Version:            r.Version,
		RequireSessionJoin: r.Spec.Allow.RequireSessionJoin,
	}
}

// SetSessionRequirePolicies sets the RBAC required policies for a role.
func (r *RoleV6) SetSessionRequirePolicies(policies []*SessionRequirePolicy) {
	r.Spec.Allow.RequireSessionJoin = policies
}

// SetSessionJoinPolicies returns the RBAC join policies for a role.
func (r *RoleV6) GetSessionJoinPolicies() []*SessionJoinPolicy {
	return r.Spec.Allow.JoinSessions
}

// SetSessionJoinPolicies sets the RBAC join policies for a role.
func (r *RoleV6) SetSessionJoinPolicies(policies []*SessionJoinPolicy) {
	r.Spec.Allow.JoinSessions = policies
}

// GetSearchAsRoles returns the list of extra roles which should apply to a
// user while they are searching for resources as part of a Resource Access
// Request, and defines the underlying roles which will be requested as part
// of any Resource Access Request.
func (r *RoleV6) GetSearchAsRoles(rct RoleConditionType) []string {
	roleConditions := &r.Spec.Allow
	if rct == Deny {
		roleConditions = &r.Spec.Deny
	}
	if roleConditions.Request == nil {
		return nil
	}
	return roleConditions.Request.SearchAsRoles
}

// SetSearchAsRoles sets the list of extra roles which should apply to a
// user while they are searching for resources as part of a Resource Access
// Request, and defines the underlying roles which will be requested as part
// of any Resource Access Request.
func (r *RoleV6) SetSearchAsRoles(rct RoleConditionType, roles []string) {
	roleConditions := &r.Spec.Allow
	if rct == Deny {
		roleConditions = &r.Spec.Deny
	}
	if roleConditions.Request == nil {
		roleConditions.Request = &AccessRequestConditions{}
	}
	roleConditions.Request.SearchAsRoles = roles
}

// GetPreviewAsRoles returns the list of extra roles which should apply to a
// reviewer while they are viewing a Resource Access Request for the
// purposes of viewing details such as the hostname and labels of requested
// resources.
func (r *RoleV6) GetPreviewAsRoles(rct RoleConditionType) []string {
	roleConditions := r.GetRoleConditions(rct)
	if roleConditions.ReviewRequests == nil {
		return nil
	}
	return roleConditions.ReviewRequests.PreviewAsRoles
}

// GetRoleConditions returns the role conditions for the role.
func (r *RoleV6) GetRoleConditions(rct RoleConditionType) RoleConditions {
	roleConditions := r.Spec.Allow
	if rct == Deny {
		roleConditions = r.Spec.Deny
	}

	return roleConditions
}

// GetRoleConditions returns the role conditions for the role.
func (r *RoleV6) GetRequestReasonMode(rct RoleConditionType) RequestReasonMode {
	roleConditions := r.GetRoleConditions(rct)
	if roleConditions.Request == nil || roleConditions.Request.Reason == nil {
		return ""
	}
	return roleConditions.Request.Reason.Mode
}

// SetPreviewAsRoles sets the list of extra roles which should apply to a
// reviewer while they are viewing a Resource Access Request for the
// purposes of viewing details such as the hostname and labels of requested
// resources.
func (r *RoleV6) SetPreviewAsRoles(rct RoleConditionType, roles []string) {
	roleConditions := &r.Spec.Allow
	if rct == Deny {
		roleConditions = &r.Spec.Deny
	}
	if roleConditions.ReviewRequests == nil {
		roleConditions.ReviewRequests = &AccessReviewConditions{}
	}
	roleConditions.ReviewRequests.PreviewAsRoles = roles
}

// validateRoleSpecKubeResources validates the Allow/Deny Kubernetes Resources
// entries.
func validateRoleSpecKubeResources(version string, spec RoleSpecV6) error {
	if err := validateKubeResources(version, spec.Allow.KubernetesResources); err != nil {
		return trace.Wrap(err)
	}
	if err := validateKubeResources(version, spec.Deny.KubernetesResources); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// setDefaultKubernetesVerbs sets the default verbs for each KubernetesResource
// entry if none are specified. This is necessary for backwards compatibility
// with older versions of Role: V3, V4, V5, and v6.
func setDefaultKubernetesVerbs(spec *RoleSpecV6) {
	for _, kubeResources := range [][]KubernetesResource{spec.Allow.KubernetesResources, spec.Deny.KubernetesResources} {
		for i := range kubeResources {
			if len(kubeResources[i].Verbs) == 0 {
				kubeResources[i].Verbs = []string{Wildcard}
			}
		}
	}
}

// validateKubeResources validates the following rules for each kubeResources entry:
// - Kind belongs to KubernetesResourcesKinds for roles <=v7, is set and doesn't belong to that list for >=v8
// - Name is not empty
// - Namespace is not empty
// - APIGroup is empty for roles <=v7 and not empty for >=v8
func validateKubeResources(roleVersion string, kubeResources []KubernetesResource) error {
	for _, kubeResource := range kubeResources {
		for _, verb := range kubeResource.Verbs {
			if !slices.Contains(KubernetesVerbs, verb) && verb != Wildcard && !strings.Contains(verb, "{{") {
				return trace.BadParameter("KubernetesResource verb %q is invalid or unsupported; Supported: %v", verb, KubernetesVerbs)
			}
			if verb == Wildcard && len(kubeResource.Verbs) > 1 {
				return trace.BadParameter("KubernetesResource verb %q cannot be used with other verbs", verb)
			}
		}

		// Only Pod resources are supported in role version <=V6.
		// This is mandatory because we must append the other resources to the
		// kubernetes resources.
		switch roleVersion {
		// Teleport does not support role versions < v3.
		case V6, V5, V4, V3:
			if kubeResource.Kind != KindKubePod {
				return trace.BadParameter("KubernetesResource kind %q is not supported in role version %q. Upgrade the role version to %q", kubeResource.Kind, roleVersion, V8)
			}
			if len(kubeResource.Verbs) != 1 || kubeResource.Verbs[0] != Wildcard {
				return trace.BadParameter("Role version %q only supports %q verb. Upgrade the role version to %q", roleVersion, Wildcard, V8)
			}
			fallthrough
		case V7:
			if kubeResource.APIGroup != "" {
				return trace.BadParameter("API Group %q is not supported in role version %q. Upgrade the role version to %q", kubeResource.APIGroup, roleVersion, V8)
			}
			if kubeResource.Kind != Wildcard && !slices.Contains(KubernetesResourcesKinds, kubeResource.Kind) {
				return trace.BadParameter("KubernetesResource kind %q is invalid or unsupported; Supported: %v", kubeResource.Kind, append([]string{Wildcard}, KubernetesResourcesKinds...))
			}
			if kubeResource.Namespace == "" && !slices.Contains(KubernetesClusterWideResourceKinds, kubeResource.Kind) {
				return trace.BadParameter("KubernetesResource kind %q must include Namespace", kubeResource.Kind)
			}
		case V8:
			if kubeResource.Kind == "" {
				return trace.BadParameter("KubernetesResource kind %q is required in role version %q", kubeResource.Kind, roleVersion)
			}
			// If we have a kind that match a role v7 one, check the api group.
			if slices.Contains(KubernetesResourcesKinds, kubeResource.Kind) {
				// If the api group is a wildcard or match v7, then it is mostly definitely a mistake, reject the role.
				if kubeResource.APIGroup == Wildcard || kubeResource.APIGroup == KubernetesResourcesV7KindGroups[kubeResource.Kind] {
					return trace.BadParameter("KubernetesResource kind %q is invalid. Please use plural name for role version %q", kubeResource.Kind, roleVersion)
				}
			}
			// Only allow empty string for known core resources.
			if kubeResource.APIGroup == "" {
				if _, ok := KubernetesCoreResourceKinds[kubeResource.Kind]; !ok {
					return trace.BadParameter("KubernetesResource api_group is required for resource %q in role version %q", kubeResource.Kind, roleVersion)
				}
			}
			// Best effort attempt to validate if the namespace field is needed.
			if kubeResource.Namespace == "" {
				if _, ok := kubernetesNamespacedResourceKinds[groupKind{kubeResource.APIGroup, kubeResource.Kind}]; ok {
					return trace.BadParameter("KubernetesResource %q must include Namespace", kubeResource.Kind)
				}
			}
		}

		if len(kubeResource.Name) == 0 {
			return trace.BadParameter("KubernetesResource must include Name")
		}
	}
	return nil
}

// validateRequestKubeResources validates each kubeResources entry for `allow.request.kubernetes_resources` field.
// Currently the only supported field for this particular field are:
//   - Kind
//   - APIGroup
//
// Mimics types.KubernetesResource data model, but opted to create own type as we don't support other fields yet.
func validateRequestKubeResources(roleVersion string, kubeResources []RequestKubernetesResource) error {
	for _, kubeResource := range kubeResources {
		// Only Pod resources are supported in role version <=V6.
		// This is mandatory because we must append the other resources to the
		// kubernetes resources.
		switch roleVersion {
		case V8:
			if kubeResource.Kind == "" {
				return trace.BadParameter("request.kubernetes_resource kind is required in role version %q", roleVersion)
			}
			// If we have a kind that match a role v7 one, check the api group.
			if slices.Contains(KubernetesResourcesKinds, kubeResource.Kind) {
				// If the api group is a wildcard or match v7, then it is mostly definitely a mistake, reject the role.
				if kubeResource.APIGroup == Wildcard || kubeResource.APIGroup == KubernetesResourcesV7KindGroups[kubeResource.Kind] {
					return trace.BadParameter("request.kubernetes_resource kind %q is invalid. Please use plural name for role version %q", kubeResource.Kind, roleVersion)
				}
			}
			// Only allow empty string for known core resources.
			if kubeResource.APIGroup == "" {
				if _, ok := KubernetesCoreResourceKinds[kubeResource.Kind]; !ok {
					return trace.BadParameter("request.kubernetes_resource api_group is required for resource %q in role version %q", kubeResource.Kind, roleVersion)
				}
			}
		case V7:
			if kubeResource.APIGroup != "" {
				return trace.BadParameter("request.kubernetes_resource api_group is not supported in role version %q. Upgrade the role version to %q", roleVersion, V8)
			}
			if !slices.Contains(KubernetesResourcesKinds, kubeResource.Kind) && kubeResource.Kind != Wildcard {
				return trace.BadParameter("request.kubernetes_resource kind %q is invalid or unsupported in role version %q; Supported: %v",
					kubeResource.Kind, roleVersion, append([]string{Wildcard}, KubernetesResourcesKinds...))
			}
		// Teleport does not support role versions < v3.
		case V6, V5, V4, V3:
			if kubeResource.APIGroup != "" {
				return trace.BadParameter("request.kubernetes_resource api_group is not supported in role version %q. Upgrade the role version to %q", roleVersion, V8)
			}
			if kubeResource.Kind != KindKubePod {
				return trace.BadParameter("request.kubernetes_resources kind %q is not supported in role version %q. Upgrade the role version to %q",
					kubeResource.Kind, roleVersion, V8)
			}
		}
	}
	return nil
}

// ClusterResource returns the resource name in the following format
// <namespace>/<name>.
func (k *KubernetesResource) ClusterResource() string {
	return path.Join(k.Namespace, k.Name)
}

// IsEmpty will return true if the condition is empty.
func (a AccessRequestConditions) IsEmpty() bool {
	return len(a.Annotations) == 0 &&
		len(a.ClaimsToRoles) == 0 &&
		len(a.Roles) == 0 &&
		len(a.SearchAsRoles) == 0 &&
		len(a.SuggestedReviewers) == 0 &&
		len(a.Thresholds) == 0
}

// IsEmpty will return true if the condition is empty.
func (a AccessReviewConditions) IsEmpty() bool {
	return len(a.ClaimsToRoles) == 0 &&
		len(a.PreviewAsRoles) == 0 &&
		len(a.Roles) == 0 &&
		len(a.Where) == 0
}

// LabelMatchers holds the role label matchers and label expression that are
// used to match resource labels of a specific resource kind and condition
// (allow/deny).
type LabelMatchers struct {
	Labels     Labels
	Expression string
}

// Empty returns true if all elements of the LabelMatchers are empty/unset.
func (l LabelMatchers) Empty() bool {
	return len(l.Labels) == 0 && len(l.Expression) == 0
}

// GetLabelMatchers gets the LabelMatchers that match labels of resources of
// type [kind] this role is allowed or denied access to.
func (r *RoleV6) GetLabelMatchers(rct RoleConditionType, kind string) (LabelMatchers, error) {
	var cond *RoleConditions
	if rct == Allow {
		cond = &r.Spec.Allow
	} else {
		cond = &r.Spec.Deny
	}
	switch kind {
	case KindRemoteCluster:
		return LabelMatchers{cond.ClusterLabels, cond.ClusterLabelsExpression}, nil
	case KindNode:
		return LabelMatchers{cond.NodeLabels, cond.NodeLabelsExpression}, nil
	case KindKubernetesCluster:
		return LabelMatchers{cond.KubernetesLabels, cond.KubernetesLabelsExpression}, nil
	case KindApp, KindSAMLIdPServiceProvider:
		// app_labels will be applied to both app and saml_idp_service_provider resources.
		// Access to the saml_idp_service_provider can be controlled by the both
		// app_labels and verbs targeting saml_idp_service_provider resource.
		return LabelMatchers{cond.AppLabels, cond.AppLabelsExpression}, nil
	case KindDatabase:
		return LabelMatchers{cond.DatabaseLabels, cond.DatabaseLabelsExpression}, nil
	case KindDatabaseService:
		return LabelMatchers{cond.DatabaseServiceLabels, cond.DatabaseServiceLabelsExpression}, nil
	case KindWindowsDesktop:
		return LabelMatchers{cond.WindowsDesktopLabels, cond.WindowsDesktopLabelsExpression}, nil
	case KindDynamicWindowsDesktop:
		return LabelMatchers{cond.WindowsDesktopLabels, cond.WindowsDesktopLabelsExpression}, nil
	case KindWindowsDesktopService:
		return LabelMatchers{cond.WindowsDesktopLabels, cond.WindowsDesktopLabelsExpression}, nil
	case KindUserGroup:
		return LabelMatchers{cond.GroupLabels, cond.GroupLabelsExpression}, nil
	case KindGitServer:
		return r.makeGitServerLabelMatchers(cond), nil
	case KindWorkloadIdentity:
		return LabelMatchers{cond.WorkloadIdentityLabels, cond.WorkloadIdentityLabelsExpression}, nil
	}
	return LabelMatchers{}, trace.BadParameter("can't get label matchers for resource kind %q", kind)
}

// SetLabelMatchers sets the LabelMatchers that match labels of resources of
// type [kind] this role is allowed or denied access to.
func (r *RoleV6) SetLabelMatchers(rct RoleConditionType, kind string, labelMatchers LabelMatchers) error {
	var cond *RoleConditions
	if rct == Allow {
		cond = &r.Spec.Allow
	} else {
		cond = &r.Spec.Deny
	}
	switch kind {
	case KindRemoteCluster:
		cond.ClusterLabels = labelMatchers.Labels
		cond.ClusterLabelsExpression = labelMatchers.Expression
		return nil
	case KindNode:
		cond.NodeLabels = labelMatchers.Labels
		cond.NodeLabelsExpression = labelMatchers.Expression
		return nil
	case KindKubernetesCluster:
		cond.KubernetesLabels = labelMatchers.Labels
		cond.KubernetesLabelsExpression = labelMatchers.Expression
		return nil
	case KindApp, KindSAMLIdPServiceProvider:
		// app_labels will be applied to both app and saml_idp_service_provider resources.
		// Access to the saml_idp_service_provider can be controlled by the both
		// app_labels and verbs targeting saml_idp_service_provider resource.
		cond.AppLabels = labelMatchers.Labels
		cond.AppLabelsExpression = labelMatchers.Expression
		return nil
	case KindDatabase:
		cond.DatabaseLabels = labelMatchers.Labels
		cond.DatabaseLabelsExpression = labelMatchers.Expression
		return nil
	case KindDatabaseService:
		cond.DatabaseServiceLabels = labelMatchers.Labels
		cond.DatabaseServiceLabelsExpression = labelMatchers.Expression
		return nil
	case KindWindowsDesktop:
		cond.WindowsDesktopLabels = labelMatchers.Labels
		cond.WindowsDesktopLabelsExpression = labelMatchers.Expression
		return nil
	case KindWindowsDesktopService:
		cond.WindowsDesktopLabels = labelMatchers.Labels
		cond.WindowsDesktopLabelsExpression = labelMatchers.Expression
		return nil
	case KindUserGroup:
		cond.GroupLabels = labelMatchers.Labels
		cond.GroupLabelsExpression = labelMatchers.Expression
		return nil
	case KindWorkloadIdentity:
		cond.WorkloadIdentityLabels = labelMatchers.Labels
		cond.WorkloadIdentityLabelsExpression = labelMatchers.Expression
		return nil
	}
	return trace.BadParameter("can't set label matchers for resource kind %q", kind)
}

// HasLabelMatchers returns true if the role has label matchers for the
// specified resource kind and condition (allow/deny).
// If the kind is not supported, false is returned.
func (r *RoleV6) HasLabelMatchers(rct RoleConditionType, kind string) bool {
	lm, err := r.GetLabelMatchers(rct, kind)
	return err == nil && !lm.Empty()
}

// GetLabel retrieves the label with the provided key.
func (r *RoleV6) GetLabel(key string) (value string, ok bool) {
	v, ok := r.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns all resource's labels.
func (r *RoleV6) GetAllLabels() map[string]string {
	return r.Metadata.Labels
}

// GetStaticLabels returns the resource's static labels.
func (r *RoleV6) GetStaticLabels() map[string]string {
	return r.Metadata.Labels
}

// SetStaticLabels sets the resource's static labels.
func (r *RoleV6) SetStaticLabels(labels map[string]string) {
	r.Metadata.Labels = labels
}

// Origin returns the origin value of the resource.
func (r *RoleV6) Origin() string {
	return r.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (r *RoleV6) SetOrigin(origin string) {
	r.Metadata.SetOrigin(origin)
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (r *RoleV6) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(r.GetAllLabels()), r.GetName())
	return MatchSearch(fieldVals, values, nil)
}

func (r *RoleV6) makeGitServerLabelMatchers(cond *RoleConditions) LabelMatchers {
	var all []string
	for _, perm := range cond.GitHubPermissions {
		all = append(all, perm.Organizations...)
	}
	return LabelMatchers{
		Labels: Labels{
			GitHubOrgLabel: all,
		},
	}
}

// GetIdentityCenterAccountAssignments fetches the allow or deny Identity Center
// Account Assignments for the role
func (r *RoleV6) GetIdentityCenterAccountAssignments(rct RoleConditionType) []IdentityCenterAccountAssignment {
	if rct == Allow {
		return r.Spec.Allow.AccountAssignments
	}
	return r.Spec.Deny.AccountAssignments
}

// SetIdentityCenterAccountAssignments sets the allow or deny Identity Center
// Account Assignments for the role
func (r *RoleV6) SetIdentityCenterAccountAssignments(rct RoleConditionType, assignments []IdentityCenterAccountAssignment) {
	cond := &r.Spec.Deny
	if rct == Allow {
		cond = &r.Spec.Allow
	}
	cond.AccountAssignments = assignments
}

// GetMCPPermissions returns the allow or deny MCP permissions.
func (r *RoleV6) GetMCPPermissions(rct RoleConditionType) *MCPPermissions {
	if rct == Allow {
		return r.Spec.Allow.MCP
	}
	return r.Spec.Deny.MCP
}

// SetMCPPermissions sets the allow or deny MCP permissions.
func (r *RoleV6) SetMCPPermissions(rct RoleConditionType, perms *MCPPermissions) {
	if rct == Allow {
		r.Spec.Allow.MCP = perms
	} else {
		r.Spec.Deny.MCP = perms
	}
}

func (r *RoleV6) Clone() Role {
	return utils.CloneProtoMsg(r)
}

// LabelMatcherKinds is the complete list of resource kinds that support label
// matchers.
var LabelMatcherKinds = []string{
	KindRemoteCluster,
	KindNode,
	KindKubernetesCluster,
	KindApp,
	KindDatabase,
	KindDatabaseService,
	KindWindowsDesktop,
	KindWindowsDesktopService,
	KindUserGroup,
}

const (
	createHostUserModeOffString          = "off"
	createHostUserModeDropString         = "drop"
	createHostUserModeKeepString         = "keep"
	createHostUserModeInsecureDropString = "insecure-drop"
)

func (h CreateHostUserMode) encode() (string, error) {
	switch h {
	case CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED:
		return "", nil
	case CreateHostUserMode_HOST_USER_MODE_OFF:
		return createHostUserModeOffString, nil
	case CreateHostUserMode_HOST_USER_MODE_DROP:
		return createHostUserModeDropString, nil
	case CreateHostUserMode_HOST_USER_MODE_KEEP:
		return createHostUserModeKeepString, nil
	case CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP:
		return createHostUserModeInsecureDropString, nil
	}
	return "", trace.BadParameter("invalid host user mode %v", h)
}

func (h *CreateHostUserMode) decode(val any) error {
	var valS string
	switch val := val.(type) {
	case int32:
		return trace.Wrap(h.setFromEnum(val))
	case int64:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case int:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case float64:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case float32:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case string:
		valS = val
	case bool:
		if val {
			return trace.BadParameter("create_host_user_mode cannot be true, got %v", val)
		}
		valS = createHostUserModeOffString
	default:
		return trace.BadParameter("bad value type %T, expected string or int", val)
	}

	switch valS {
	case "":
		*h = CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED
	case createHostUserModeOffString:
		*h = CreateHostUserMode_HOST_USER_MODE_OFF
	case createHostUserModeKeepString:
		*h = CreateHostUserMode_HOST_USER_MODE_KEEP
	case createHostUserModeInsecureDropString, createHostUserModeDropString:
		*h = CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP
	default:
		return trace.BadParameter("invalid host user mode %v", val)
	}
	return nil
}

// setFromEnum sets the value from enum value as int32.
func (h *CreateHostUserMode) setFromEnum(val int32) error {
	// Map drop to insecure-drop
	if val == int32(CreateHostUserMode_HOST_USER_MODE_DROP) {
		val = int32(CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP)
	}
	if _, ok := CreateHostUserMode_name[val]; !ok {
		return trace.BadParameter("invalid host user mode %v", val)
	}
	*h = CreateHostUserMode(val)
	return nil
}

// UnmarshalYAML supports parsing CreateHostUserMode from string.
func (h *CreateHostUserMode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val interface{}
	err := unmarshal(&val)
	if err != nil {
		return trace.Wrap(err)
	}

	err = h.decode(val)
	return trace.Wrap(err)
}

// MarshalYAML marshals CreateHostUserMode to yaml.
func (h *CreateHostUserMode) MarshalYAML() (interface{}, error) {
	val, err := h.encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return val, nil
}

// MarshalJSON marshals CreateHostUserMode to json bytes.
func (h *CreateHostUserMode) MarshalJSON() ([]byte, error) {
	val, err := h.encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := json.Marshal(val)
	return out, trace.Wrap(err)
}

// UnmarshalJSON supports parsing CreateHostUserMode from string.
func (h *CreateHostUserMode) UnmarshalJSON(data []byte) error {
	var val interface{}
	err := json.Unmarshal(data, &val)
	if err != nil {
		return trace.Wrap(err)
	}

	err = h.decode(val)
	return trace.Wrap(err)
}

const (
	createDatabaseUserModeOffString            = "off"
	createDatabaseUserModeKeepString           = "keep"
	createDatabaseUserModeBestEffortDropString = "best_effort_drop"
)

func (h CreateDatabaseUserMode) encode() (string, error) {
	switch h {
	case CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED:
		return "", nil
	case CreateDatabaseUserMode_DB_USER_MODE_OFF:
		return createDatabaseUserModeOffString, nil
	case CreateDatabaseUserMode_DB_USER_MODE_KEEP:
		return createDatabaseUserModeKeepString, nil
	case CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP:
		return createDatabaseUserModeBestEffortDropString, nil
	}

	return "", trace.BadParameter("invalid database user mode %v", h)
}

func (h *CreateDatabaseUserMode) decode(val any) error {
	var str string
	switch val := val.(type) {
	case int32:
		return trace.Wrap(h.setFromEnum(val))
	case int64:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case int:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case float64:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case float32:
		return trace.Wrap(h.setFromEnum(int32(val)))
	case string:
		str = val
	case bool:
		if val {
			return trace.BadParameter("create_database_user_mode cannot be true, got %v", val)
		}
		str = createHostUserModeOffString
	default:
		return trace.BadParameter("bad value type %T, expected string", val)
	}

	switch str {
	case "":
		*h = CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED
	case createDatabaseUserModeOffString:
		*h = CreateDatabaseUserMode_DB_USER_MODE_OFF
	case createDatabaseUserModeKeepString:
		*h = CreateDatabaseUserMode_DB_USER_MODE_KEEP
	case createDatabaseUserModeBestEffortDropString:
		*h = CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP
	default:
		return trace.BadParameter("invalid database user mode %v", val)
	}

	return nil
}

// setFromEnum sets the value from enum value as int32.
func (h *CreateDatabaseUserMode) setFromEnum(val int32) error {
	if _, ok := CreateDatabaseUserMode_name[val]; !ok {
		return trace.BadParameter("invalid database user creation mode %v", val)
	}
	*h = CreateDatabaseUserMode(val)
	return nil
}

// UnmarshalYAML supports parsing CreateDatabaseUserMode from string.
func (h *CreateDatabaseUserMode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val interface{}
	err := unmarshal(&val)
	if err != nil {
		return trace.Wrap(err)
	}

	err = h.decode(val)
	return trace.Wrap(err)
}

// MarshalYAML marshals CreateDatabaseUserMode to yaml.
func (h *CreateDatabaseUserMode) MarshalYAML() (interface{}, error) {
	val, err := h.encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return val, nil
}

// MarshalJSON marshals CreateDatabaseUserMode to json bytes.
func (h *CreateDatabaseUserMode) MarshalJSON() ([]byte, error) {
	val, err := h.encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := json.Marshal(val)
	return out, trace.Wrap(err)
}

// UnmarshalJSON supports parsing CreateDatabaseUserMode from string.
func (h *CreateDatabaseUserMode) UnmarshalJSON(data []byte) error {
	var val interface{}
	err := json.Unmarshal(data, &val)
	if err != nil {
		return trace.Wrap(err)
	}

	err = h.decode(val)
	return trace.Wrap(err)
}

// IsEnabled returns true if database automatic user provisioning is enabled.
func (m CreateDatabaseUserMode) IsEnabled() bool {
	return m != CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED && m != CreateDatabaseUserMode_DB_USER_MODE_OFF
}

// GetAccount fetches the Account ID from a Role Condition Account Assignment
func (a IdentityCenterAccountAssignment) GetAccount() string {
	return a.Account
}

// IsLegacySAMLRBAC matches a role version
// v7 and below, considered as the legacy SAML IdP RBAC.
func IsLegacySAMLRBAC(roleVersion string) bool {
	return slices.Contains([]string{V7, V6, V5, V4, V3, V2, V1}, roleVersion)
}
