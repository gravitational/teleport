/*
Copyright 2022 Gravitational, Inc.

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
	"strconv"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/predicate"
	"github.com/gravitational/teleport/lib/tlsca"
)

// AccessChecker interface checks access to resources based on roles, traits,
// and allowed resources
type AccessChecker interface {
	// HasRole checks if the checker includes the role
	HasRole(role string) bool

	// RoleNames returns a list of role names
	RoleNames() []string

	// Roles returns the list underlying roles this AccessChecker is based on.
	Roles() []types.Role

	// AccessPolicyNames returns the list of underlying policy names this AccessChecker is based on.
	AccessPolicyNames() []string

	// AccessPolicies returns the list of underlying policies this AccessChecker is based on.
	AccessPolicies() []types.AccessPolicy

	// Traits returns the list of underlying traits this AccessChecker is based on.
	Traits() map[string][]string

	// CheckAccess checks access to the specified resource.
	CheckAccess(r AccessCheckable, mfa AccessMFAParams, matchers ...RoleMatcher) error

	// CheckAccessToRemoteCluster checks access to remote cluster
	CheckAccessToRemoteCluster(cluster types.RemoteCluster) error

	// CheckAccessToRule checks access to a rule within a namespace.
	CheckAccessToRule(context RuleContext, namespace string, rule string, verb string, silent bool) error

	// CheckLoginDuration checks if role set can login up to given duration and
	// returns a combined list of allowed logins.
	CheckLoginDuration(ttl time.Duration) ([]string, error)

	// CheckKubeGroupsAndUsers check if role can login into kubernetes
	// and returns two lists of combined allowed groups and users
	CheckKubeGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) (groups []string, users []string, err error)

	// CheckAWSRoleARNs returns a list of AWS role ARNs role is allowed to assume.
	CheckAWSRoleARNs(ttl time.Duration, overrideTTL bool) ([]string, error)

	// DesktopClipboard returns true if the role set has enabled shared
	// clipboard for desktop sessions. Clipboard sharing is disabled if
	// one or more of the roles in the set has disabled it.
	DesktopClipboard() bool
	// RecordDesktopSession returns true if a role in the role set has enabled
	// desktop session recoring.
	RecordDesktopSession() bool
	// DesktopDirectorySharing returns true if the role set has directory sharing
	// enabled. This setting is enabled if one or more of the roles in the set has
	// enabled it.
	DesktopDirectorySharing() bool

	// MaybeCanReviewRequests attempts to guess if this RoleSet belongs
	// to a user who should be submitting access reviews. Because not all rolesets
	// are derived from statically assigned roles, this may return false positives.
	MaybeCanReviewRequests() bool

	// CertificateFormat returns the most permissive certificate format in a
	// RoleSet.
	CertificateFormat() string

	// EnhancedRecordingSet returns a set of events that will be recorded
	// for enhanced session recording.
	EnhancedRecordingSet() map[string]bool

	// CheckDatabaseNamesAndUsers returns database names and users this role
	// is allowed to use.
	CheckDatabaseNamesAndUsers(ttl time.Duration, overrideTTL bool) (names []string, users []string, err error)

	// CheckImpersonate checks whether current user is allowed to impersonate
	// users and roles
	CheckImpersonate(currentUser, impersonateUser types.User, impersonateRoles []types.Role) error

	// CheckImpersonateRoles checks whether the current user is allowed to
	// perform roles-only impersonation.
	CheckImpersonateRoles(currentUser types.User, impersonateRoles []types.Role) error

	// CanImpersonateSomeone returns true if this checker has any impersonation rules
	CanImpersonateSomeone() bool

	// ExtractConditionForIdentifier returns a restrictive filter expression
	// for list queries based on the rules' `where` conditions.
	ExtractConditionForIdentifier(ctx RuleContext, namespace, resource, verb, identifier string) (*types.WhereExpr, error)

	// CertificateExtensions returns the list of extensions for each role in the RoleSet
	CertificateExtensions() []*types.CertExtension

	// GetAllowedSearchAsRoles returns all of the allowed SearchAsRoles.
	GetAllowedSearchAsRoles() []string

	// GetAllowedPreviewAsRoles returns all of the allowed PreviewAsRoles.
	GetAllowedPreviewAsRoles() []string

	// SessionPolicySets returns the list of SessionPolicySets for all roles.
	SessionPolicySets() []*types.SessionTrackerPolicySet

	// GetAllLogins returns all valid unix logins for the AccessChecker.
	GetAllLogins() []string

	// GetAllowedResourceIDs returns the list of allowed resources the identity for
	// the AccessChecker is allowed to access. An empty or nil list indicates that
	// there are no resource-specific restrictions.
	GetAllowedResourceIDs() []types.ResourceID

	// HostUsers returns host user information matching a server or nil if
	// a role disallows host user creation
	HostUsers(types.Server) (*HostUsersInfo, error)

	// PrivateKeyPolicy returns the enforced private key policy for this role set,
	// or the provided defaultPolicy - whichever is stricter.
	PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) keys.PrivateKeyPolicy

	// GuessIfAccessIsPossible guesses if access is possible for an entire category
	// of resources.
	// It responds the question: "is it possible that there is a resource of this
	// kind that the current user can access?".
	// GuessIfAccessIsPossible is used, mainly, for UI decisions ("should the tab
	// for resource X appear"?). Most callers should use CheckAccessToRule instead.
	GuessIfAccessIsPossible(ctx RuleContext, namespace string, resource string, verb string, silent bool) error

	// CheckAccessToNode checks login access to a given node.
	CheckLoginAccessToNode(r types.Server, login string, mfa AccessMFAParams) error

	// CheckSessionJoinAccess checks if the identity has access to join the given session.
	CheckSessionJoinAccess(session types.SessionTracker, sessionOwner *predicate.User, mode types.SessionParticipantMode) error

	OptionSessionTTL() types.SessionTTL

	OptionLockingMode() types.LockingMode

	OptionSessionMFA() types.SessionMFA

	OptionSSHSessionRecordingMode() types.SSHSessionRecordingMode

	OptionSSHAllowAgentForwarding() types.SSHAllowAgentForwarding

	OptionSSHAllowPortForwarding() types.SSHAllowPortForwarding

	OptionSSHAllowX11Forwarding() types.SSHAllowX11Forwarding

	OptionsSSHAllowFileCopying() types.SSHAllowFileCopying

	OptionsSSHAllowExpiredCert() types.SSHAllowExpiredCert

	OptionsSSHPinSourceIP() types.SSHPinSourceIP

	OptionSSHMaxConnections() types.SSHMaxConnections

	OptionSSHMaxSessionsPerConnection() types.SSHMaxSessionsPerConnection

	OptionSSHClientIdleTimeout() types.SSHClientIdleTimeout
}

// AccessInfo hold information about an identity necessary to check whether that
// identity has access to cluster resources. This info can come from a user or
// host SSH certificate, TLS certificate, or user information stored in the
// backend.
type AccessInfo struct {
	// Name is the username of the identity.
	Name string
	// Roles is the list of cluster local roles for the identity.
	Roles []string
	// AccessPolicies is the list of cluster local access policies for the identity.
	AccessPolicies []string
	// Traits is the set of traits for the identity.
	Traits wrappers.Traits
	// AllowedResourceIDs is the list of resource IDs the identity is allowed to
	// access. A nil or empty list indicates that no resource-specific
	// access restrictions should be applied. Used for search-based access
	// requests.
	AllowedResourceIDs []types.ResourceID
}

// accessChecker implements the AccessChecker interface.
type accessChecker struct {
	info         *AccessInfo
	localCluster string

	// RoleSet is embedded to use the existing implementation for most
	// AccessChecker methods. Methods which require AllowedResourceIDs (relevant
	// to search-based access requests) will be implemented by
	// accessChecker.
	roleSet RoleSet

	// PredicateAccessChecker is embedded to allow access checking via access policy resources.
	predicateChecker *predicate.PredicateAccessChecker
}

// NewAccessChecker returns a new AccessChecker which can be used to check
// access to resources.
// Args:
//   - `info *AccessInfo` should hold the roles, traits, and allowed resource IDs
//     for the identity.
//   - `localCluster string` should be the name of the local cluster in which
//     access will be checked. You cannot check for access to resources in remote
//     clusters.
//   - `access RoleGetter` should be a RoleGetter which will be used to fetch the
//     full RoleSet
func NewAccessChecker(info *AccessInfo, localCluster string, access RoleGetter) (AccessChecker, error) {
	roleSet, err := FetchRoles(info.Roles, access, info.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policies, err := FetchAccessPoliciesList(info.AccessPolicies, access)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &accessChecker{
		info:             info,
		localCluster:     localCluster,
		roleSet:          roleSet,
		predicateChecker: predicate.NewPredicateAccessChecker(policies),
	}, nil
}

// NewAccessCheckerWithRoleSet is similar to NewAccessChecker, but accepts the
// full RoleSet rather than a RoleGetter.
func NewAccessCheckerWithRoleSet(info *AccessInfo, localCluster string, roleSet RoleSet) AccessChecker {
	return &accessChecker{
		info:         info,
		localCluster: localCluster,
		roleSet:      roleSet,
	}
}

// blendAccessDecision combines two access decisions into one using the following rule logic:
// 1. If either decision is AccessDenied, the result is denial.
// 2. If both decisions are AccessUndecided, the result is denial.
// 3. If one decision is AccessAllowed and the other is AccessUndecided, the result is allow.
// 4. If both decisions are AccessAllowed, the result is allow.
func blendAccessDecision(a, b predicate.AccessDecision) error {
	// Allow access if at least one checks pass AND no one responds with an explicity deny. Access if granted if one allows and the other is undecided.
	if a != predicate.AccessDenied && b != predicate.AccessDenied && (b == predicate.AccessAllowed || a == predicate.AccessAllowed) {
		return nil
	}

	return trace.AccessDenied("access denied")
}

// CanImpersonateSomeone returns true if this checker has any impersonation rules
func (a *accessChecker) CanImpersonateSomeone() bool {
	return a.roleSet.CanImpersonateSomeone()
}

// CertificateExtensions returns the list of extensions for each role in the RoleSet
func (a *accessChecker) CertificateExtensions() []*types.CertExtension {
	return a.roleSet.CertificateExtensions()
}

// CertificateFormat returns the most permissive certificate format in a
// RoleSet.
func (a *accessChecker) CertificateFormat() string {
	return a.roleSet.CertificateFormat()
}

// CheckAWSRoleARNs returns a list of AWS role ARNs this role set is allowed to assume.
func (a *accessChecker) CheckAWSRoleARNs(ttl time.Duration, overrideTTL bool) ([]string, error) {
	return a.roleSet.CheckAWSRoleARNs(ttl, overrideTTL)
}

// CheckAccessToRemoteCluster checks if a role has access to remote cluster. Deny rules are
// checked first then allow rules. Access to a cluster is determined by
// namespaces, labels, and logins.
func (a *accessChecker) CheckAccessToRemoteCluster(rc types.RemoteCluster) error {
	return a.roleSet.CheckAccessToRemoteCluster(rc)
}

// CheckDatabaseNamesAndUsers checks if the role has any allowed database
// names or users.
func (a *accessChecker) CheckDatabaseNamesAndUsers(ttl time.Duration, overrideTTL bool) ([]string, []string, error) {
	return a.roleSet.CheckDatabaseNamesAndUsers(ttl, overrideTTL)
}

// CheckImpersonate returns nil if this role set can impersonate
// a user and their roles, returns AccessDenied otherwise
// CheckImpersonate checks whether current user is allowed to impersonate
// users and roles
func (a *accessChecker) CheckImpersonate(currentUser, impersonateUser types.User, impersonateRoles []types.Role) error {
	return a.roleSet.CheckImpersonate(currentUser, impersonateUser, impersonateRoles)
}

// CheckImpersonateRoles validates that the current user can perform role-only impersonation
// of the given roles. Role-only impersonation requires an allow rule with
// roles but no users (and no user-less deny rules). All requested roles must
// be allowed for the check to succeed.
func (a *accessChecker) CheckImpersonateRoles(currentUser types.User, impersonateRoles []types.Role) error {
	return a.roleSet.CheckImpersonateRoles(currentUser, impersonateRoles)
}

// CheckKubeGroupsAndUsers check if role can login into kubernetes
// and returns two lists of allowed groups and users
func (a *accessChecker) CheckKubeGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) ([]string, []string, error) {
	return a.roleSet.CheckKubeGroupsAndUsers(ttl, overrideTTL, matchers...)
}

// CheckLoginDuration checks if role set can login up to given duration and
// returns a combined list of allowed logins.
func (a *accessChecker) CheckLoginDuration(ttl time.Duration) ([]string, error) {
	return a.roleSet.CheckLoginDuration(ttl)
}

// DesktopClipboard returns true if the role set has enabled shared
// clipboard for desktop sessions. Clipboard sharing is disabled if
// one or more of the roles in the set has disabled it.
func (a *accessChecker) DesktopClipboard() bool {
	return a.roleSet.DesktopClipboard()
}

// DesktopDirectorySharing returns true if the role set has directory sharing
// enabled. This setting is enabled if one or more of the roles in the set has
// enabled it.
func (a *accessChecker) DesktopDirectorySharing() bool {
	return a.roleSet.DesktopDirectorySharing()
}

// EnhancedRecordingSet returns a set of events that will be recorded
// for enhanced session recording.
func (a *accessChecker) EnhancedRecordingSet() map[string]bool {
	return a.roleSet.EnhancedRecordingSet()
}

// ExtractConditionForIdentifier returns a restrictive filter expression
// for list queries based on the rules' `where` conditions.
func (a *accessChecker) ExtractConditionForIdentifier(ctx RuleContext, namespace, resource, verb, identifier string) (*types.WhereExpr, error)

// GetAllLogins returns all valid unix logins for the RoleSet.
func (a *accessChecker) GetAllLogins() []string {
	return a.roleSet.GetAllLogins()
}

// GetAllowedPreviewAsRoles returns all PreviewAsRoles for this RoleSet.
func (a *accessChecker) GetAllowedPreviewAsRoles() []string {
	return a.roleSet.GetAllowedPreviewAsRoles()
}

// GetSearchAsRoles returns all SearchAsRoles for this RoleSet.
func (a *accessChecker) GetAllowedSearchAsRoles() []string {
	return a.roleSet.GetAllowedSearchAsRoles()
}

// HasRole checks if the checker includes the role
func (a *accessChecker) HasRole(role string) bool {
	return a.roleSet.HasRole(role)
}

// HostUsers returns host user information matching a server or nil if
// a role disallows host user creation
func (a *accessChecker) HostUsers(s types.Server) (*HostUsersInfo, error) {
	return a.roleSet.HostUsers(s)
}

// MaybeCanReviewRequests attempts to guess if this RoleSet belongs
// to a user who should be submitting access reviews.  Because not all rolesets
// are derived from statically assigned roles, this may return false positives.
func (a *accessChecker) MaybeCanReviewRequests() bool {
	return a.roleSet.MaybeCanReviewRequests()
}

// PrivateKeyPolicy returns the enforced private key policy for this role set.
func (a *accessChecker) PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) keys.PrivateKeyPolicy {
	return a.roleSet.PrivateKeyPolicy(defaultPolicy)
}

// RecordDesktopSession returns true if the role set has enabled desktop
// session recording. Recording is considered enabled if at least one
// role in the set has enabled it.
func (a *accessChecker) RecordDesktopSession() bool {
	return a.roleSet.RecordDesktopSession()
}

// RoleNames returns a list of role names
func (a *accessChecker) RoleNames() []string {
	return a.roleSet.RoleNames()
}

// Roles returns the list underlying roles this AccessChecker is based on.
func (a *accessChecker) Roles() []types.Role {
	return a.roleSet.Roles()
}

// SessionPolicySets returns the list of SessionPolicySets for all roles.
func (a *accessChecker) SessionPolicySets() []*types.SessionTrackerPolicySet {
	return a.roleSet.SessionPolicySets()
}

// AccessPolicyNames returns the list of underlying policy names this AccessChecker is based on.
func (a *accessChecker) AccessPolicyNames() []string {
	return a.info.AccessPolicies
}

// AccessPolicies returns the list of underlying policies this AccessChecker is based on.
func (a *accessChecker) AccessPolicies() []types.AccessPolicy {
	return a.predicateChecker.Policies
}

// Traits returns the list of underlying traits this AccessChecker is based on.
func (a *accessChecker) Traits() map[string][]string {
	return a.info.Traits
}

func (a *accessChecker) checkAllowedResources(r AccessCheckable) error {
	if len(a.info.AllowedResourceIDs) == 0 {
		// certificate does not contain a list of specifically allowed
		// resources, only role-based access control is used
		return nil
	}

	// Note: logging in this function only happens in debug mode. This is because
	// adding logging to this function (which is called on every resource returned
	// by the backend) can slow down this function by 50x for large clusters!
	isDebugEnabled, debugf := rbacDebugLogger()

	for _, resourceID := range a.info.AllowedResourceIDs {
		if resourceID.ClusterName == a.localCluster &&
			resourceID.Kind == r.GetKind() &&
			resourceID.Name == r.GetName() {
			// Allowed to access this resource by resource ID, move on to role checks.
			if isDebugEnabled {
				debugf("Matched allowed resource ID %q", types.ResourceIDToString(resourceID))
			}
			return nil
		}
	}

	if isDebugEnabled {
		allowedResources, err := types.ResourceIDsToString(a.info.AllowedResourceIDs)
		if err != nil {
			return trace.Wrap(err)
		}
		err = trace.AccessDenied("access to %v denied, %q not in allowed resource IDs %s",
			r.GetKind(), r.GetName(), allowedResources)
		debugf("Access denied: %v", err)
		return err
	}
	return trace.AccessDenied("access to %v denied, not in allowed resource IDs", r.GetKind())
}

// CheckAccess checks if the identity for this AccessChecker has access to the
// given resource.
func (a *accessChecker) CheckAccess(r AccessCheckable, mfa AccessMFAParams, matchers ...RoleMatcher) error {
	if err := a.checkAllowedResources(r); err != nil {
		return trace.Wrap(err)
	}

	decision, err := a.roleSet.checkAccess(r, mfa, matchers...)
	if err != nil {
		return trace.Wrap(err)
	}

	return blendAccessDecision(decision, predicate.AccessUndecided)
}

// CheckAccessToRule checks if the identity has access in the given
// namespace to the specified resource and verb.
// silent controls whether the access violations are logged.
func (a *accessChecker) CheckAccessToRule(ctx RuleContext, namespace string, resource string, verb string, silent bool) error {
	hasStandardAccess, err := a.roleSet.checkAccessToRule(ctx, namespace, resource, verb, silent)
	if !trace.IsAccessDenied(err) {
		return trace.Wrap(err)
	}

	r, err := ctx.GetResource()
	if err != nil {
		return trace.Wrap(err)
	}

	hasPredicateAccess, err := a.predicateChecker.CheckAccessToResource(&predicate.Resource{
		Kind:    r.GetKind(),
		SubKind: r.GetSubKind(),
		Version: r.GetVersion(),
		Name:    r.GetName(),
		Id:      strconv.FormatInt(r.GetResourceID(), 10),
		Verb:    verb,
	}, &predicate.User{
		Name:     a.info.Name,
		Policies: a.info.AccessPolicies,
		Traits:   a.info.Traits,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return blendAccessDecision(hasPredicateAccess, hasStandardAccess)
}

// GuessIfAccessIsPossible guesses if access is possible for an entire category
// of resources.
// It responds the question: "is it possible that there is a resource of this
// kind that the current user can access?".
// GuessIfAccessIsPossible is used, mainly, for UI decisions ("should the tab
// for resource X appear"?). Most callers should use CheckAccessToRule instead.
func (a *accessChecker) GuessIfAccessIsPossible(ctx RuleContext, namespace string, resource string, verb string, silent bool) error {
	hasStandardAccess, err := a.roleSet.guessIfAccessIsPossible(ctx, namespace, resource, verb, silent)
	if !trace.IsAccessDenied(err) {
		return trace.Wrap(err)
	}

	return blendAccessDecision(hasStandardAccess, predicate.AccessUndecided)
}

// CheckSessionJoinAccess checks if the identity has access to join the given session.
func (a *accessChecker) CheckSessionJoinAccess(session types.SessionTracker, sessionOwner *predicate.User, mode types.SessionParticipantMode) error {
	var participants []string
	for _, p := range session.GetParticipants() {
		participants = append(participants, p.User)
	}

	evaluator := NewSessionAccessEvaluator(session.GetHostPolicySets(), session.GetSessionKind(), session.GetHostUser())
	standardDecision := evaluator.CanJoin(SessionAccessContext{
		Username: a.info.Name,
		Roles:    a.roleSet,
		Mode:     mode,
	})

	predicateDecision, err := a.predicateChecker.CheckSessionJoinAccess(&predicate.Session{
		Owner:        sessionOwner,
		Participants: participants,
	}, &predicate.JoinSession{
		Mode: string(mode),
	}, &predicate.User{
		Name:     a.info.Name,
		Policies: a.info.AccessPolicies,
		Traits:   a.info.Traits,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return blendAccessDecision(standardDecision, predicateDecision)
}

// CheckLoginAccessToNode checks login access to a given node.
func (a *accessChecker) CheckLoginAccessToNode(r types.Server, login string, mfa AccessMFAParams) error {
	if err := a.checkAllowedResources(r); err != nil {
		return trace.Wrap(err)
	}

	hasStandardAccess, err := a.roleSet.checkAccess(r, mfa)
	if !trace.IsAccessDenied(err) {
		return trace.Wrap(err)
	}

	hasPredicateAccess, err := a.predicateChecker.CheckLoginAccessToNode(&predicate.Node{
		Hostname: r.GetHostname(),
		Address:  r.GetAddr(),
		Labels:   r.GetAllLabels(),
	}, &predicate.AccessNode{Login: login}, &predicate.User{
		Name:     a.info.Name,
		Policies: a.info.AccessPolicies,
		Traits:   a.info.Traits,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return blendAccessDecision(hasPredicateAccess, hasStandardAccess)
}

// GetAllowedResourceIDs returns the list of allowed resources the identity for
// the AccessChecker is allowed to access. An empty or nil list indicates that
// there are no resource-specific restrictions.
func (a *accessChecker) GetAllowedResourceIDs() []types.ResourceID {
	return a.info.AllowedResourceIDs
}

func (a *accessChecker) OptionSessionTTL() types.SessionTTL {
	return *RoleOption[*types.SessionTTL](a)
}

func (a *accessChecker) OptionLockingMode() types.LockingMode {
	return *RoleOption[*types.LockingMode](a)
}

func (a *accessChecker) OptionSessionMFA() types.SessionMFA {
	return *RoleOption[*types.SessionMFA](a)
}

func (a *accessChecker) OptionSSHSessionRecordingMode() types.SSHSessionRecordingMode {
	return *RoleOption[*types.SSHSessionRecordingMode](a)
}

func (a *accessChecker) OptionSSHAllowAgentForwarding() types.SSHAllowAgentForwarding {
	return *RoleOption[*types.SSHAllowAgentForwarding](a)
}

func (a *accessChecker) OptionSSHAllowPortForwarding() types.SSHAllowPortForwarding {
	return *RoleOption[*types.SSHAllowPortForwarding](a)
}

func (a *accessChecker) OptionSSHAllowX11Forwarding() types.SSHAllowX11Forwarding {
	return *RoleOption[*types.SSHAllowX11Forwarding](a)
}

func (a *accessChecker) OptionsSSHAllowFileCopying() types.SSHAllowFileCopying {
	return *RoleOption[*types.SSHAllowFileCopying](a)
}

func (a *accessChecker) OptionsSSHAllowExpiredCert() types.SSHAllowExpiredCert {
	return *RoleOption[*types.SSHAllowExpiredCert](a)
}

func (a *accessChecker) OptionsSSHPinSourceIP() types.SSHPinSourceIP {
	return *RoleOption[*types.SSHPinSourceIP](a)
}

func (a *accessChecker) OptionSSHMaxConnections() types.SSHMaxConnections {
	return *RoleOption[*types.SSHMaxConnections](a)
}

func (a *accessChecker) OptionSSHMaxSessionsPerConnection() types.SSHMaxSessionsPerConnection {
	return *RoleOption[*types.SSHMaxSessionsPerConnection](a)
}

func (a *accessChecker) OptionSSHClientIdleTimeout() types.SSHClientIdleTimeout {
	return *RoleOption[*types.SSHClientIdleTimeout](a)
}

// RoleOption retrieves and attempts to deserialize it into the provided type.
func RoleOption[T types.FromRawOption[T]](checker AccessChecker) T {
	var instances []T

	for _, role := range checker.Roles() {
		if opt, err := types.RoleOption[T](role); err != nil {
			instances = append(instances, opt)
		}
	}

	for _, policy := range checker.AccessPolicies() {
		if opt, err := types.AccessPolicyOption[T](policy); err != nil {
			instances = append(instances, opt)
		}
	}

	return types.CombineOptions(instances...)
}

// AccessInfoFromLocalCertificate returns a new AccessInfo populated from the
// given ssh certificate. Should only be used for cluster local users as roles
// will not be mapped.
func AccessInfoFromLocalCertificate(cert *ssh.Certificate) (*AccessInfo, error) {
	traits, err := ExtractTraitsFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := ExtractRolesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessPolicies, err := ExtractAccessPoliciesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedResourceIDs, err := ExtractAllowedResourcesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessInfo{
		// TODO(joel): is this correct?
		Name:               cert.ValidPrincipals[0],
		Roles:              roles,
		AccessPolicies:     accessPolicies,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
	}, nil
}

// AccessInfoFromRemoteCertificate returns a new AccessInfo populated from the
// given remote cluster user's ssh certificate. Remote roles will be mapped to
// local roles based on the given roleMap.
func AccessInfoFromRemoteCertificate(cert *ssh.Certificate, roleMap types.RoleMap) (*AccessInfo, error) {
	// Old-style SSH certificates don't have traits in metadata.
	traits, err := ExtractTraitsFromCert(cert)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.AccessDenied("failed to parse certificate traits: %v", err)
	}
	if traits == nil {
		traits = make(map[string][]string)
	}
	// Prior to Teleport 6.2 the only trait passed to the remote cluster
	// was the "logins" trait set to the SSH certificate principals.
	//
	// Keep backwards-compatible behavior and set it in addition to the
	// traits extracted from the certificate.
	traits[constants.TraitLogins] = cert.ValidPrincipals

	unmappedRoles, err := ExtractRolesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	unmappedAccessPolicies, err := ExtractAccessPoliciesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := MapRoles(roleMap, unmappedRoles)
	if err != nil {
		return nil, trace.AccessDenied("failed to map roles for user with remote roles %v: %v", unmappedRoles, err)
	}
	if len(roles) == 0 {
		return nil, trace.AccessDenied("no roles mapped for user with remote roles %v", unmappedRoles)
	}
	log.Debugf("Mapped remote roles %v to local roles %v and traits %v.",
		unmappedRoles, roles, traits)

	allowedResourceIDs, err := ExtractAllowedResourcesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessInfo{
		Name:  cert.ValidPrincipals[0],
		Roles: roles,
		// TODO(joel): this will be resolved later after speccing out policy mapping further
		AccessPolicies:     unmappedAccessPolicies,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
	}, nil
}

// AccessInfoFromLocalIdentity returns a new AccessInfo populated from the given
// tlsca.Identity. Should only be used for cluster local users as roles will not
// be mapped.
func AccessInfoFromLocalIdentity(identity tlsca.Identity, access UserGetter) (*AccessInfo, error) {
	roles := identity.Groups
	traits := identity.Traits
	allowedResourceIDs := identity.AllowedResourceIDs

	// Legacy certs are not encoded with roles or traits,
	// so we fallback to the traits and roles in the backend.
	// empty traits are a valid use case in standard certs,
	// so we only check for whether roles are empty.
	if len(identity.Groups) == 0 {
		u, err := access.GetUser(identity.Username, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		log.Warnf("Failed to find roles in x509 identity for %v. Fetching "+
			"from backend. If the identity provider allows username changes, this can "+
			"potentially allow an attacker to change the role of the existing user.",
			identity.Username)
		roles = u.GetRoles()
		traits = u.GetTraits()
	}

	return &AccessInfo{
		Name:               identity.Username,
		Roles:              roles,
		AccessPolicies:     identity.AccessPolicies,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
	}, nil
}

// AccessInfoFromRemoteIdentity returns a new AccessInfo populated from the
// given remote cluster user's tlsca.Identity. Remote roles will be mapped to
// local roles based on the given roleMap.
func AccessInfoFromRemoteIdentity(identity tlsca.Identity, roleMap types.RoleMap) (*AccessInfo, error) {
	// Set internal traits for the remote user. This allows Teleport to work by
	// passing exact logins, Kubernetes users/groups and database users/names
	// to the remote cluster.
	traits := map[string][]string{
		constants.TraitLogins:     identity.Principals,
		constants.TraitKubeGroups: identity.KubernetesGroups,
		constants.TraitKubeUsers:  identity.KubernetesUsers,
		constants.TraitDBNames:    identity.DatabaseNames,
		constants.TraitDBUsers:    identity.DatabaseUsers,
	}
	// Prior to Teleport 6.2 no user traits were passed to remote clusters
	// except for the internal ones specified above.
	//
	// To preserve backwards compatible behavior, when applying traits from user
	// identity, make sure to filter out those already present in the map above.
	//
	// This ensures that if e.g. there's a "logins" trait in the root user's
	// identity, it won't overwrite the internal "logins" trait set above
	// causing behavior change.
	for k, v := range identity.Traits {
		if _, ok := traits[k]; !ok {
			traits[k] = v
		}
	}

	unmappedRoles := identity.Groups
	roles, err := MapRoles(roleMap, unmappedRoles)
	if err != nil {
		return nil, trace.AccessDenied("failed to map roles for remote user %q from cluster %q with remote roles %v: %v", identity.Username, identity.TeleportCluster, unmappedRoles, err)
	}
	if len(roles) == 0 {
		return nil, trace.AccessDenied("no roles mapped for remote user %q from cluster %q with remote roles %v", identity.Username, identity.TeleportCluster, unmappedRoles)
	}
	log.Debugf("Mapped roles %v of remote user %q to local roles %v and traits %v.",
		unmappedRoles, identity.Username, roles, traits)

	allowedResourceIDs := identity.AllowedResourceIDs

	return &AccessInfo{
		Name:  identity.Username,
		Roles: roles,
		// TODO(joel): this will be resolved later after speccing out policy mapping further
		AccessPolicies:     identity.AccessPolicies,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
	}, nil
}

// AccessInfoFromUser return a new AccessInfo populated from the roles and
// traits held be the given user. This should only be used in cases where the
// user does not have any active access requests (initial web login, initial
// tbot certs, tests).
func AccessInfoFromUser(user types.User) *AccessInfo {
	roles := user.GetRoles()
	accessPolicies := user.GetAccessPolicies()
	traits := user.GetTraits()
	return &AccessInfo{
		Name:           user.GetName(),
		Roles:          roles,
		AccessPolicies: accessPolicies,
		Traits:         traits,
	}
}
