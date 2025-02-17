/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// AccessChecker interface checks access to resources based on roles, traits,
// and allowed resources
type AccessChecker interface {
	// HasRole checks if the checker includes the role
	HasRole(role string) bool

	// RoleNames returns a list of role names
	RoleNames() []string

	// Traits returns the set of user traits
	Traits() wrappers.Traits

	// Roles returns the list underlying roles this AccessChecker is based on.
	Roles() []types.Role

	// CheckAccess checks access to the specified resource.
	CheckAccess(r AccessCheckable, state AccessState, matchers ...RoleMatcher) error

	// CheckAccessToRemoteCluster checks access to remote cluster
	CheckAccessToRemoteCluster(cluster types.RemoteCluster) error

	// CheckAccessToRule checks access to a rule within a namespace.
	CheckAccessToRule(context RuleContext, namespace string, rule string, verb string) error

	// CheckLoginDuration checks if role set can login up to given duration and
	// returns a combined list of allowed logins.
	CheckLoginDuration(ttl time.Duration) ([]string, error)

	// CheckKubeGroupsAndUsers check if role can login into kubernetes
	// and returns two lists of combined allowed groups and users
	CheckKubeGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) (groups []string, users []string, err error)

	// CheckAWSRoleARNs returns a list of AWS role ARNs role is allowed to assume.
	CheckAWSRoleARNs(ttl time.Duration, overrideTTL bool) ([]string, error)

	// CheckAzureIdentities returns a list of Azure identities the user is allowed to assume.
	CheckAzureIdentities(ttl time.Duration, overrideTTL bool) ([]string, error)

	// CheckGCPServiceAccounts returns a list of GCP service accounts the user is allowed to assume.
	CheckGCPServiceAccounts(ttl time.Duration, overrideTTL bool) ([]string, error)

	// CheckAccessToSAMLIdP checks access to the SAML IdP.
	//
	//nolint:revive // Because we want this to be IdP.
	CheckAccessToSAMLIdP(readonly.AuthPreference, AccessState) error

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

	// SSHPortForwardMode returns the SSHPortForwardMode that the RoleSet allows.
	SSHPortForwardMode() SSHPortForwardMode

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

	// PermitX11Forwarding returns true if this RoleSet allows X11 Forwarding.
	PermitX11Forwarding() bool

	// CanCopyFiles returns true if the role set has enabled remote file
	// operations via SCP or SFTP. Remote file operations are disabled if
	// one or more of the roles in the set has disabled it.
	CanCopyFiles() bool

	// CertificateFormat returns the most permissive certificate format in a
	// RoleSet.
	CertificateFormat() string

	// EnhancedRecordingSet returns a set of events that will be recorded
	// for enhanced session recording.
	EnhancedRecordingSet() map[string]bool

	// CheckDatabaseNamesAndUsers returns database names and users this role
	// is allowed to use.
	CheckDatabaseNamesAndUsers(ttl time.Duration, overrideTTL bool) (names []string, users []string, err error)

	// DatabaseAutoUserMode returns whether a user should be auto-created in
	// the database.
	DatabaseAutoUserMode(types.Database) (types.CreateDatabaseUserMode, error)

	// CheckDatabaseRoles returns a list of database roles to assign, when
	// auto-user provisioning is enabled. If no user-requested roles, all
	// allowed roles are returned.
	CheckDatabaseRoles(database types.Database, userRequestedRoles []string) (roles []string, err error)

	// GetDatabasePermissions returns a set of database permissions applicable for the user.
	GetDatabasePermissions(database types.Database) (allow types.DatabasePermissions, deny types.DatabasePermissions, err error)

	// CheckImpersonate checks whether current user is allowed to impersonate
	// users and roles
	CheckImpersonate(currentUser, impersonateUser types.User, impersonateRoles []types.Role) error

	// CheckImpersonateRoles checks whether the current user is allowed to
	// perform roles-only impersonation.
	CheckImpersonateRoles(currentUser types.User, impersonateRoles []types.Role) error

	// CanImpersonateSomeone returns true if this checker has any impersonation rules
	CanImpersonateSomeone() bool

	// LockingMode returns the locking mode to apply with this checker.
	LockingMode(defaultMode constants.LockingMode) constants.LockingMode

	// ExtractConditionForIdentifier returns a restrictive filter expression
	// for list queries based on the rules' `where` conditions.
	ExtractConditionForIdentifier(ctx RuleContext, namespace, resource, verb, identifier string) (*types.WhereExpr, error)

	// CertificateExtensions returns the list of extensions for each role in the RoleSet
	CertificateExtensions() []*types.CertExtension

	// GetAllowedSearchAsRoles returns all of the allowed SearchAsRoles.
	GetAllowedSearchAsRoles(allowFilters ...SearchAsRolesOption) []string

	// GetAllowedSearchAsRolesForKubeResourceKind returns all of the allowed SearchAsRoles
	// that allowed requesting to the requested Kubernetes resource kind.
	GetAllowedSearchAsRolesForKubeResourceKind(requestedKubeResourceKind string) []string

	// GetAllowedPreviewAsRoles returns all of the allowed PreviewAsRoles.
	GetAllowedPreviewAsRoles() []string

	// MaxConnections returns the maximum number of concurrent ssh connections
	// allowed.  If MaxConnections is zero then no maximum was defined and the
	// number of concurrent connections is unconstrained.
	MaxConnections() int64

	// MaxSessions returns the maximum number of concurrent ssh sessions per
	// connection. If MaxSessions is zero then no maximum was defined and the
	// number of sessions is unconstrained.
	MaxSessions() int64

	// SessionPolicySets returns the list of SessionPolicySets for all roles.
	SessionPolicySets() []*types.SessionTrackerPolicySet

	// GetAllLogins returns all valid unix logins for the AccessChecker.
	GetAllLogins() []string

	// GetAllowedResourceIDs returns the list of allowed resources the identity for
	// the AccessChecker is allowed to access. An empty or nil list indicates that
	// there are no resource-specific restrictions.
	GetAllowedResourceIDs() []types.ResourceID

	// SessionRecordingMode returns the recording mode for a specific service.
	SessionRecordingMode(service constants.SessionRecordingService) constants.SessionRecordingMode

	// HostUsers returns host user information matching a server or nil if
	// a role disallows host user creation
	HostUsers(types.Server) (*HostUsersInfo, error)

	// HostSudoers returns host sudoers entries matching a server
	HostSudoers(types.Server) ([]string, error)

	// DesktopGroups returns the desktop groups a user is allowed to create or an access denied error if a role disallows desktop user creation
	DesktopGroups(types.WindowsDesktop) ([]string, error)

	// PinSourceIP forces the same client IP for certificate generation and SSH usage
	PinSourceIP() bool

	// GetAccessState returns the AccessState for the user given their roles, the
	// cluster auth preference, and whether MFA and the user's device were
	// verified.
	GetAccessState(authPref readonly.AuthPreference) AccessState
	// PrivateKeyPolicy returns the enforced private key policy for this role set,
	// or the provided defaultPolicy - whichever is stricter.
	PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error)

	// GetKubeResources returns the allowed and denied Kubernetes Resources configured
	// for a user.
	GetKubeResources(cluster types.KubeCluster) (allowed, denied []types.KubernetesResource)

	// EnumerateEntities works on a given role set to return a minimal description
	// of allowed set of entities (db_users, db_names, etc). It is biased towards
	// *allowed* entities; It is meant to describe what the user can do, rather than
	// cannot do. For that reason if the user isn't allowed to pick *any* entities,
	// the output will be empty.
	//
	// In cases where * is listed in set of allowed entities, it may be hard for
	// users to figure out the expected entity to use. For this reason the parameter
	// extraEntities provides an extra set of entities to be checked against
	// RoleSet. This extra set of entities may be sourced e.g. from user connection
	// history.
	EnumerateEntities(resource AccessCheckable, listFn roleEntitiesListFn, newMatcher roleMatcherFactoryFn, extraEntities ...string) EnumerationResult

	// EnumerateDatabaseUsers specializes EnumerateEntities to enumerate db_users.
	EnumerateDatabaseUsers(database types.Database, extraUsers ...string) (EnumerationResult, error)

	// EnumerateDatabaseNames specializes EnumerateEntities to enumerate db_names.
	EnumerateDatabaseNames(database types.Database, extraNames ...string) EnumerationResult

	// GetAllowedLoginsForResource returns all of the allowed logins for the passed resource.
	//
	// Supports the following resource types:
	//
	// - types.Server with GetKind() == types.KindNode
	// - types.KindWindowsDesktop
	// - types.KindApp with IsAWSConsole() == true
	GetAllowedLoginsForResource(resource AccessCheckable) ([]string, error)

	// CheckSPIFFESVID checks if the role set has access to generating the
	// requested SPIFFE ID. Returns an error if the role set does not have the
	// ability to generate the requested SVID.
	CheckSPIFFESVID(spiffeIDPath string, dnsSANs []string, ipSANs []net.IP) error
}

// AccessInfo hold information about an identity necessary to check whether that
// identity has access to cluster resources. This info can come from a user or
// host SSH certificate, TLS certificate, or user information stored in the
// backend.
type AccessInfo struct {
	// Roles is the list of cluster local roles for the identity.
	Roles []string
	// Traits is the set of traits for the identity.
	Traits wrappers.Traits
	// AllowedResourceIDs is the list of resource IDs the identity is allowed to
	// access. A nil or empty list indicates that no resource-specific
	// access restrictions should be applied. Used for search-based access
	// requests.
	AllowedResourceIDs []types.ResourceID
	// Username is the Teleport username.
	Username string
}

// accessChecker implements the AccessChecker interface.
type accessChecker struct {
	info         *AccessInfo
	localCluster string

	// RoleSet is embedded to use the existing implementation for most
	// AccessChecker methods. Methods which require AllowedResourceIDs (relevant
	// to search-based access requests) will be implemented by
	// accessChecker.
	RoleSet
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
	return &accessChecker{
		info:         info,
		localCluster: localCluster,
		RoleSet:      roleSet,
	}, nil
}

// NewAccessCheckerWithRoleSet is similar to NewAccessChecker, but accepts the
// full RoleSet rather than a RoleGetter.
func NewAccessCheckerWithRoleSet(info *AccessInfo, localCluster string, roleSet RoleSet) AccessChecker {
	return &accessChecker{
		info:         info,
		localCluster: localCluster,
		RoleSet:      roleSet,
	}
}

// CurrentUserRoleGetter limits the interface of auth.ClientI to methods needed
// by NewAccessCheckerForRemoteCluster.
type CurrentUserRoleGetter interface {
	// GetCurrentUserRoles returns the remote cluster roles for the current
	// user, traits have not been applied.
	GetCurrentUserRoles(context.Context) ([]types.Role, error)
	// GetCurrentUser returns the remote cluster's view of the current user.
	GetCurrentUser(context.Context) (types.User, error)
}

// NewAccessCheckerForRemoteCluster returns an AccessChecker that can check
// user's access to resources that may be located in remote/leaf Teleport
// clusters.
func NewAccessCheckerForRemoteCluster(ctx context.Context, localAccessInfo *AccessInfo, clusterName string, access CurrentUserRoleGetter) (AccessChecker, error) {
	// Fetch the remote cluster's view of the current user's roles.
	remoteRoles, err := access.GetCurrentUserRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the remote cluster's view of the current user's traits.
	// These can technically be different than the local user's traits, see
	// AccessInfoFromRemote(Certificate|Identity).
	remoteUser, err := access.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	remoteAccessInfo := &AccessInfo{
		Username: remoteUser.GetName(),
		Traits:   remoteUser.GetTraits(),
		// Will fill this in with the names of the remote/mapped roles we got
		// from GetCurrentUserRoles.
		Roles: make([]string, 0, len(remoteRoles)),
		// AllowedResourceIDs are always the same across clusters.
		AllowedResourceIDs: localAccessInfo.AllowedResourceIDs,
	}

	for i := range remoteRoles {
		remoteRoles[i], err = ApplyTraits(remoteRoles[i], remoteAccessInfo.Traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		remoteAccessInfo.Roles = append(remoteAccessInfo.Roles, remoteRoles[i].GetName())
	}
	roleSet := NewRoleSet(remoteRoles...)

	return &accessChecker{
		info: remoteAccessInfo,
		// localCluster is a bit of a misnomer here, but it means the local
		// cluster of the resources to which access will be checked, which in
		// this case may be a remote cluster. localCluster is used for access
		// checks involving Resource Access Requests, the cluster name is
		// included in the unique ID of the resource, the accessChecker can only
		// check access to resources in that cluster.
		localCluster: clusterName,
		RoleSet:      roleSet,
	}, nil
}

func (a *accessChecker) checkAllowedResources(r AccessCheckable) error {
	if len(a.info.AllowedResourceIDs) == 0 {
		// certificate does not contain a list of specifically allowed
		// resources, only role-based access control is used
		return nil
	}

	// Note: logging in this function only happens in trace mode. This is because
	// adding logging to this function (which is called on every resource returned
	// by the backend) can slow down this function by 50x for large clusters!
	ctx := context.Background()
	isLoggingEnabled := rbacLogger.Enabled(ctx, logutils.TraceLevel)

	for _, resourceID := range a.info.AllowedResourceIDs {
		if resourceID.ClusterName == a.localCluster &&
			// If the allowed resource has `Kind=types.KindKubePod` or any other
			// Kubernetes supported kinds - types.KubernetesResourcesKinds-, we allow the user to
			// access the Kubernetes cluster that it belongs to.
			// At this point, we do not verify that the accessed resource matches the
			// allowed resources, but that verification happens in the caller function.
			(resourceID.Kind == r.GetKind() || (slices.Contains(types.KubernetesResourcesKinds, resourceID.Kind) && r.GetKind() == types.KindKubernetesCluster)) &&
			resourceID.Name == r.GetName() {
			// Allowed to access this resource by resource ID, move on to role checks.

			if isLoggingEnabled {
				rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Matched allowed resource ID",
					slog.String("resource_id", types.ResourceIDToString(resourceID)),
				)
			}

			return nil
		}
	}

	if isLoggingEnabled {
		allowedResources, err := types.ResourceIDsToString(a.info.AllowedResourceIDs)
		if err != nil {
			return trace.Wrap(err)
		}

		slog.LogAttrs(ctx, logutils.TraceLevel, "Access to resource denied, not in allowed resource IDs",
			slog.String("resource_kind", r.GetKind()),
			slog.String("resource_name", r.GetName()),
			slog.Any("allowed_resources", allowedResources),
		)

		return trace.AccessDenied("access to %v denied, %q not in allowed resource IDs %s",
			r.GetKind(), r.GetName(), allowedResources)
	}

	return trace.AccessDenied("access to %v denied, not in allowed resource IDs", r.GetKind())
}

// CheckAccess checks if the identity for this AccessChecker has access to the
// given resource.
func (a *accessChecker) CheckAccess(r AccessCheckable, state AccessState, matchers ...RoleMatcher) error {
	if err := a.checkAllowedResources(r); err != nil {
		return trace.Wrap(err)
	}

	switch rr := r.(type) {
	case types.Resource153Unwrapper:
		switch urr := rr.Unwrap().(type) {
		case IdentityCenterAccount:
			matchers = append(matchers, NewIdentityCenterAccountMatcher(urr))

		case IdentityCenterAccountAssignment:
			matchers = append(matchers, NewIdentityCenterAccountAssignmentMatcher(urr))
		}
	}

	return trace.Wrap(a.RoleSet.checkAccess(r, a.info.Traits, state, matchers...))
}

// GetKubeResources returns the allowed and denied Kubernetes Resources configured
// for a user.
func (a *accessChecker) GetKubeResources(cluster types.KubeCluster) (allowed, denied []types.KubernetesResource) {
	if len(a.info.AllowedResourceIDs) == 0 {
		return a.RoleSet.GetKubeResources(cluster, a.info.Traits)
	}
	var err error
	rolesAllowed, rolesDenied := a.RoleSet.GetKubeResources(cluster, a.info.Traits)
	// Allways append the denied resources from the roles. This is because
	// the denied resources from the roles take precedence over the allowed
	// resources from the certificate.
	denied = rolesDenied
	for _, r := range a.info.AllowedResourceIDs {
		if r.Name != cluster.GetName() || r.ClusterName != a.localCluster {
			continue
		}
		switch {
		case slices.Contains(types.KubernetesResourcesKinds, r.Kind):
			namespace := ""
			name := ""
			if slices.Contains(types.KubernetesClusterWideResourceKinds, r.Kind) {
				// Cluster wide resources do not have a namespace.
				name = r.SubResourceName
			} else {
				splitted := strings.SplitN(r.SubResourceName, "/", 3)
				// This condition should never happen since SubResourceName is validated
				// but it's better to validate it.
				if len(splitted) != 2 {
					continue
				}
				namespace = splitted[0]
				name = splitted[1]
			}

			r := types.KubernetesResource{
				Kind:      r.Kind,
				Namespace: namespace,
				Name:      name,
			}
			// matchKubernetesResource checks if the Kubernetes Resource matches the tuple
			// (kind, namespace, kame) from the allowed/denied list and does not match the resource
			// verbs. Verbs are not checked here because they are not included in the
			// ResourceID but we collect them and set them in the returned KubernetesResource
			// so that they can be matched when the resource is accessed.
			if r.Verbs, err = matchKubernetesResource(r, rolesAllowed, rolesDenied); err == nil {
				allowed = append(allowed, r)
			}
		case r.Kind == types.KindKubernetesCluster:
			// When a user has access to a Kubernetes cluster through Resource Access request,
			// he has access to all resources in that cluster that he has access to through his roles.
			// In that case, we append the allowed and denied resources from the roles.
			return rolesAllowed, rolesDenied
		}
	}
	return
}

// matchKubernetesResource checks if the Kubernetes Resource does not match any
// entry from the deny list and matches at least one entry from the allowed list.
func matchKubernetesResource(resource types.KubernetesResource, allowed, denied []types.KubernetesResource) ([]string, error) {
	// utils.KubeResourceMatchesRegex checks if the resource.Kind is strictly equal
	// to each entry and validates if the Name and Namespace fields matches the
	// regex allowed by each entry.
	result, _, err := utils.KubeResourceMatchesRegexWithVerbsCollector(resource, denied)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if result {
		return nil, trace.AccessDenied("access to %s %q denied", resource.Kind, resource.ClusterResource())
	}

	result, verbs, err := utils.KubeResourceMatchesRegexWithVerbsCollector(resource, allowed)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if !result {
		return nil, trace.AccessDenied("access to %s %q denied", resource.Kind, resource.ClusterResource())
	}
	return verbs, nil
}

// GetAllowedResourceIDs returns the list of allowed resources the identity for
// the AccessChecker is allowed to access. An empty or nil list indicates that
// there are no resource-specific restrictions.
func (a *accessChecker) GetAllowedResourceIDs() []types.ResourceID {
	return a.info.AllowedResourceIDs
}

// Traits returns the set of user traits
func (a *accessChecker) Traits() wrappers.Traits {
	return a.info.Traits
}

// DatabaseAutoUserMode returns whether a user should be auto-created in
// the database.
func (a *accessChecker) DatabaseAutoUserMode(database types.Database) (types.CreateDatabaseUserMode, error) {
	result, err := a.checkDatabaseRoles(database)
	return result.createDatabaseUserMode(), trace.Wrap(err)
}

// CheckDatabaseRoles returns whether a user should be auto-created in the
// database and a list of database roles to assign.
func (a *accessChecker) CheckDatabaseRoles(database types.Database, userRequestedRoles []string) ([]string, error) {
	result, err := a.checkDatabaseRoles(database)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case !result.createDatabaseUserMode().IsEnabled():
		return []string{}, nil

	// If user requested a list of roles, make sure all requested roles are
	// allowed.
	case len(userRequestedRoles) > 0:
		for _, requestedRole := range userRequestedRoles {
			if !slices.Contains(result.allowedRoles(), requestedRole) {
				return nil, trace.AccessDenied("access to database role %q denied", requestedRole)
			}
		}
		return userRequestedRoles, nil

	// If user does not provide any roles, use all allowed roles from roleset.
	default:
		return result.allowedRoles(), nil
	}
}

type checkDatabaseRolesResult struct {
	allowedRoleSet RoleSet
	deniedRoleSet  RoleSet
}

func (result *checkDatabaseRolesResult) createDatabaseUserMode() types.CreateDatabaseUserMode {
	if result == nil {
		return types.CreateDatabaseUserMode_DB_USER_MODE_UNSPECIFIED
	}
	return result.allowedRoleSet.GetCreateDatabaseUserMode()
}

func (result *checkDatabaseRolesResult) allowedRoles() []string {
	if result == nil {
		return nil
	}

	rolesMap := make(map[string]struct{})
	for _, role := range result.allowedRoleSet {
		for _, dbRole := range role.GetDatabaseRoles(types.Allow) {
			rolesMap[dbRole] = struct{}{}
		}
	}
	for _, role := range result.deniedRoleSet {
		for _, dbRole := range role.GetDatabaseRoles(types.Deny) {
			delete(rolesMap, dbRole)
		}
	}
	return utils.StringsSliceFromSet(rolesMap)
}

func (a *accessChecker) checkDatabaseRoles(database types.Database) (*checkDatabaseRolesResult, error) {
	// First, collect roles from this roleset that have create database user mode set.
	var autoCreateRoles RoleSet
	for _, role := range a.RoleSet {
		if role.GetCreateDatabaseUserMode().IsEnabled() {
			autoCreateRoles = append(autoCreateRoles, role)
		}
	}
	// If there are no "auto-create user" roles, nothing to do.
	if len(autoCreateRoles) == 0 {
		return nil, nil
	}
	// Otherwise, iterate over auto-create roles matching the database user
	// is connecting to and compile a list of roles database user should be
	// assigned.
	var allowedRoleSet RoleSet
	for _, role := range autoCreateRoles {
		match, _, err := checkRoleLabelsMatch(types.Allow, role, a.info.Traits, database, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !match {
			continue
		}
		allowedRoleSet = append(allowedRoleSet, role)

	}
	var deniedRoleSet RoleSet
	for _, role := range autoCreateRoles {
		match, _, err := checkRoleLabelsMatch(types.Deny, role, a.info.Traits, database, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !match {
			continue
		}
		deniedRoleSet = append(deniedRoleSet, role)

	}
	// The collected role list can be empty and that should be ok, we want to
	// leave the behavior of what happens when a user is created with default
	// "no roles" configuration up to the target database.
	result := checkDatabaseRolesResult{
		allowedRoleSet: allowedRoleSet,
		deniedRoleSet:  deniedRoleSet,
	}
	return &result, nil
}

// GetDatabasePermissions returns a set of database permissions applicable for the user in the context of particular database.
func (a *accessChecker) GetDatabasePermissions(database types.Database) (allow types.DatabasePermissions, deny types.DatabasePermissions, err error) {
	result, err := a.checkDatabaseRoles(database)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if !result.createDatabaseUserMode().IsEnabled() {
		return nil, nil, nil
	}

	for _, role := range result.allowedRoleSet {
		allow = append(allow, role.GetDatabasePermissions(types.Allow)...)
	}
	for _, role := range result.deniedRoleSet {
		deny = append(deny, role.GetDatabasePermissions(types.Deny)...)
	}
	return allow, deny, nil
}

// EnumerateDatabaseUsers specializes EnumerateEntities to enumerate db_users.
func (a *accessChecker) EnumerateDatabaseUsers(database types.Database, extraUsers ...string) (EnumerationResult, error) {
	// When auto-user provisioning is enabled, only Teleport username is allowed.
	if database.SupportsAutoUsers() && database.GetAdminUser().Name != "" {
		result := NewEnumerationResult()
		autoUser, err := a.DatabaseAutoUserMode(database)
		if err != nil {
			return result, trace.Wrap(err)
		} else if autoUser.IsEnabled() {
			result.allowedDeniedMap[a.info.Username] = true
			return result, nil
		}
	}

	listFn := func(role types.Role, condition types.RoleConditionType) []string {
		return role.GetDatabaseUsers(condition)
	}
	newMatcher := func(user string) RoleMatcher {
		return NewDatabaseUserMatcher(database, user)
	}
	return a.EnumerateEntities(database, listFn, newMatcher, extraUsers...), nil
}

// EnumerateDatabaseNames specializes EnumerateEntities to enumerate db_names.
func (a *accessChecker) EnumerateDatabaseNames(database types.Database, extraNames ...string) EnumerationResult {
	listFn := func(role types.Role, condition types.RoleConditionType) []string {
		return role.GetDatabaseNames(condition)
	}
	newMatcher := func(dbName string) RoleMatcher {
		return &DatabaseNameMatcher{Name: dbName}
	}
	return a.EnumerateEntities(database, listFn, newMatcher, extraNames...)
}

// roleEntitiesListFn is used for listing a role's allowed/denied entities.
type roleEntitiesListFn func(types.Role, types.RoleConditionType) []string

// roleMatcherFactoryFn is used for making a role matcher for a given entity.
type roleMatcherFactoryFn func(entity string) RoleMatcher

// EnumerateEntities works on a given role set to return a minimal description
// of allowed set of entities (db_users, db_names, etc). It is biased towards
// *allowed* entities; It is meant to describe what the user can do, rather than
// cannot do. For that reason if the user isn't allowed to pick *any* entities,
// the output will be empty.
//
// In cases where * is listed in set of allowed entities, it may be hard for
// users to figure out the expected entity to use. For this reason the parameter
// extraEntities provides an extra set of entities to be checked against
// RoleSet. This extra set of entities may be sourced e.g. from user connection
// history.
func (a *accessChecker) EnumerateEntities(resource AccessCheckable, listFn roleEntitiesListFn, newMatcher roleMatcherFactoryFn, extraEntities ...string) EnumerationResult {
	result := NewEnumerationResult()

	// gather entities for checking from the roles, check wildcards.
	var entities []string
	for _, role := range a.RoleSet {
		wildcardAllowed := false
		wildcardDenied := false

		for _, e := range listFn(role, types.Allow) {
			if e == types.Wildcard {
				wildcardAllowed = true
			} else {
				entities = append(entities, e)
			}
		}

		for _, e := range listFn(role, types.Deny) {
			if e == types.Wildcard {
				wildcardDenied = true
			} else {
				entities = append(entities, e)
			}
		}

		result.wildcardDenied = result.wildcardDenied || wildcardDenied

		if err := NewRoleSet(role).checkAccess(resource, a.info.Traits, AccessState{MFAVerified: true}); err == nil {
			result.wildcardAllowed = result.wildcardAllowed || wildcardAllowed
		}

	}

	entities = apiutils.Deduplicate(append(entities, extraEntities...))

	// check each individual role spec entity against the resource.
	for _, e := range entities {
		err := a.CheckAccess(resource, AccessState{MFAVerified: true}, newMatcher(e))
		result.allowedDeniedMap[e] = err == nil
	}

	return result
}

// GetAllowedLoginsForResource returns all of the allowed logins for the passed resource.
//
// Supports the following resource types:
//
// - types.Server with GetKind() == types.KindNode
// - types.KindWindowsDesktop
// - types.KindApp with IsAWSConsole() == true
func (a *accessChecker) GetAllowedLoginsForResource(resource AccessCheckable) ([]string, error) {
	// Create a map indexed by all logins in the RoleSet,
	// mapped to false if any role has it in its deny section,
	// true otherwise.
	mapped := make(map[string]bool)

	resourceAsApp, resourceIsApp := resource.(interface{ IsAWSConsole() bool })

	for _, role := range a.RoleSet {
		var loginGetter func(types.RoleConditionType) []string

		switch resource.GetKind() {
		case types.KindNode:
			loginGetter = role.GetLogins
		case types.KindWindowsDesktop:
			loginGetter = role.GetWindowsLogins
		case types.KindApp:
			if !resourceIsApp {
				return nil, trace.BadParameter("received unsupported resource type for Application kind: %T", resource)
			}
			// For Apps, only AWS currently supports listing the possible logins.
			if !resourceAsApp.IsAWSConsole() {
				return nil, nil
			}

			loginGetter = role.GetAWSRoleARNs
		default:
			return nil, trace.BadParameter("received unsupported resource kind: %s", resource.GetKind())
		}

		for _, login := range loginGetter(types.Allow) {
			// Only set to true if not already set, the login is denied if any
			// role denies it.
			if _, alreadySet := mapped[login]; !alreadySet {
				mapped[login] = true
			}
		}
		for _, login := range loginGetter(types.Deny) {
			mapped[login] = false
		}
	}

	// Create a list of only the logins not denied by a role in the set.
	var notDenied []string
	for login, isNotDenied := range mapped {
		if isNotDenied {
			notDenied = append(notDenied, login)
		}
	}

	var newLoginMatcher func(login string) RoleMatcher
	switch resource.GetKind() {
	case types.KindNode:
		newLoginMatcher = NewLoginMatcher
	case types.KindWindowsDesktop:
		newLoginMatcher = NewWindowsLoginMatcher
	case types.KindApp:
		if !resourceIsApp || !resourceAsApp.IsAWSConsole() {
			return nil, trace.BadParameter("received unsupported resource type for Application: %T", resource)
		}

		newLoginMatcher = NewAppAWSLoginMatcher
	default:
		return nil, trace.BadParameter("received unsupported resource kind: %s", resource.GetKind())
	}

	// Filter the not-denied logins for those allowed to be used with the given resource.
	var allowed []string
	for _, login := range notDenied {
		err := a.CheckAccess(resource, AccessState{MFAVerified: true}, newLoginMatcher(login))
		if err == nil {
			allowed = append(allowed, login)
		}
	}

	return allowed, nil
}

// CheckAccessToRemoteCluster checks if a role has access to remote cluster. Deny rules are
// checked first then allow rules. Access to a cluster is determined by
// namespaces, labels, and logins.
func (a *accessChecker) CheckAccessToRemoteCluster(rc types.RemoteCluster) error {
	if len(a.RoleSet) == 0 {
		return trace.AccessDenied("access to cluster denied")
	}

	// Note: logging in this function only happens in trace mode, this is because
	// adding logging to this function (which is called on every server returned
	// by GetRemoteClusters) can slow down this function by 50x for large clusters!
	ctx := context.Background()
	isLoggingEnabled := rbacLogger.Enabled(ctx, logutils.TraceLevel)

	rcLabels := rc.GetMetadata().Labels

	// For backwards compatibility, if there is no role in the set with label
	// matchers and the cluster has no labels, assume that the role set has
	// access to the cluster.
	usesLabels := false
	for _, role := range a.RoleSet {
		unset, err := labelMatchersUnset(role, types.KindRemoteCluster)
		if err != nil {
			return trace.Wrap(err)
		}
		if !unset {
			usesLabels = true
			break
		}
	}

	if !usesLabels && len(rcLabels) == 0 {
		rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Grant access to cluster - no role uses cluster labels and the cluster is not labeled",
			slog.String("cluster_name", rc.GetName()),
			slog.Any("roles", a.RoleNames()),
		)
		return nil
	}

	// Check deny rules first: a single matching label from
	// the deny role set prohibits access.
	var errs []error
	for _, role := range a.RoleSet {
		matchLabels, labelsMessage, err := checkRoleLabelsMatch(types.Deny, role, a.info.Traits, rc, isLoggingEnabled)
		if err != nil {
			return trace.Wrap(err)
		}
		if matchLabels {
			// This condition avoids formatting calls on large scale.
			rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Access to cluster denied, deny rule matched",
				slog.String("cluster", rc.GetName()),
				slog.String("role", role.GetName()),
				slog.String("label_message", labelsMessage),
			)
			return trace.AccessDenied("access to cluster denied")
		}
	}

	// Check allow rules: label has to match in any role in the role set to be granted access.
	for _, role := range a.RoleSet {
		matchLabels, labelsMessage, err := checkRoleLabelsMatch(types.Allow, role, a.info.Traits, rc, isLoggingEnabled)
		if err != nil {
			return trace.Wrap(err)
		}
		labelMatchers, err := role.GetLabelMatchers(types.Allow, types.KindRemoteCluster)
		if err != nil {
			return trace.Wrap(err)
		}
		rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Check access to role",
			slog.String("role", role.GetName()),
			slog.String("cluster", rc.GetName()),
			slog.Any("cluster_labels", rcLabels),
			slog.Any("match_labels", matchLabels),
			slog.String("labels_message", labelsMessage),
			slog.Any("error", err),
			slog.Any("allow", labelMatchers),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		if matchLabels {
			return nil
		}
		if isLoggingEnabled {
			deniedError := trace.AccessDenied("role=%v, match(%s)",
				role.GetName(), labelsMessage)
			errs = append(errs, deniedError)
		}
	}

	rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Access to cluster denied, no allow rule matched",
		slog.String("cluster", rc.GetName()),
		slog.Any("error", errs),
	)
	return trace.AccessDenied("access to cluster denied")
}

// DesktopGroups returns the desktop groups a user is allowed to create or an access denied error if a role disallows desktop user creation
func (a *accessChecker) DesktopGroups(s types.WindowsDesktop) ([]string, error) {
	groups := make(map[string]struct{})
	for _, role := range a.RoleSet {
		result, _, err := checkRoleLabelsMatch(types.Allow, role, a.info.Traits, s, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// skip nodes that dont have matching labels
		if !result {
			continue
		}
		createDesktopUser := role.GetOptions().CreateDesktopUser
		// if any of the matching roles do not enable create host
		// user, the user should not be allowed on
		if createDesktopUser == nil || !createDesktopUser.Value {
			return nil, trace.AccessDenied("user is not allowed to create host users")
		}
		for _, group := range role.GetDesktopGroups(types.Allow) {
			groups[group] = struct{}{}
		}
	}
	for _, role := range a.RoleSet {
		result, _, err := checkRoleLabelsMatch(types.Deny, role, a.info.Traits, s, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !result {
			continue
		}
		for _, group := range role.GetDesktopGroups(types.Deny) {
			delete(groups, group)
		}
	}

	return utils.StringsSliceFromSet(groups), nil
}

// HostUserMode determines how host users should be created.
type HostUserMode int

const (
	// HostUserModeUndefined is the default mode, for when the mode couldn't be
	// determined from a types.CreateHostUserMode.
	HostUserModeUndefined HostUserMode = iota
	// HostUserModeKeep creates a home directory and persists after a session ends.
	HostUserModeKeep
	// HostUserModeDrop does not create a home directory, and it is removed after
	// a session ends.
	HostUserModeDrop
	// HostUserModeStatic creates a home directory and exists independently of a
	// session.
	HostUserModeStatic
)

func convertHostUserMode(mode types.CreateHostUserMode) HostUserMode {
	switch mode {
	case types.CreateHostUserMode_HOST_USER_MODE_KEEP:
		return HostUserModeKeep
	case types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP:
		return HostUserModeDrop
	default:
		return HostUserModeUndefined
	}
}

// HostUsersInfo keeps information about groups and sudoers entries
// for a particular host user
type HostUsersInfo struct {
	// Groups is the list of groups to include host users in
	Groups []string
	// Mode determines if a host user should be deleted after a session
	// ends or not.
	Mode HostUserMode
	// UID is the UID that the host user will be created with
	UID string
	// GID is the GID that the host user will be created with
	GID string
	// Shell is the default login shell for a host user
	Shell string
	// TakeOwnership determines whether or not an existing user should be
	// taken over by teleport. This currently only applies to 'static' mode
	// users, 'keep' mode users still need to assign 'teleport-keep' in the
	// Groups slice in order to take ownership.
	TakeOwnership bool
}

// HostUsers returns host user information matching a server or nil if
// a role disallows host user creation
func (a *accessChecker) HostUsers(s types.Server) (*HostUsersInfo, error) {
	groups := make(map[string]struct{})
	shellToRoles := make(map[string][]string)
	var shell string
	var mode types.CreateHostUserMode

	for _, role := range a.RoleSet {
		result, _, err := checkRoleLabelsMatch(types.Allow, role, a.info.Traits, s, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// skip nodes that dont have matching labels
		if !result {
			continue
		}

		createHostUserMode := role.GetOptions().CreateHostUserMode
		//nolint:staticcheck // this field is preserved for existing deployments, but shouldn't be used going forward
		createHostUser := role.GetOptions().CreateHostUser
		if createHostUserMode == types.CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED {
			createHostUserMode = types.CreateHostUserMode_HOST_USER_MODE_OFF
			if createHostUser != nil && createHostUser.Value {
				createHostUserMode = types.CreateHostUserMode_HOST_USER_MODE_KEEP
			}
		}

		// if any of the matching roles do not enable create host
		// user, the user should not be allowed on
		if createHostUserMode == types.CreateHostUserMode_HOST_USER_MODE_OFF {
			return nil, trace.AccessDenied("role %q prevents creating host users", role.GetName())
		}

		if mode == types.CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED {
			mode = createHostUserMode
		}
		// prefer to use HostUserModeKeep over InsecureDrop if mode has already been set.
		if mode == types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP &&
			createHostUserMode == types.CreateHostUserMode_HOST_USER_MODE_KEEP {
			mode = types.CreateHostUserMode_HOST_USER_MODE_KEEP
		}

		hostUserShell := role.GetOptions().CreateHostUserDefaultShell
		shell = cmp.Or(shell, hostUserShell)
		if hostUserShell != "" {
			shellToRoles[hostUserShell] = append(shellToRoles[hostUserShell], role.GetName())
		}

		for _, group := range role.GetHostGroups(types.Allow) {
			groups[group] = struct{}{}
		}
	}

	if len(shellToRoles) > 1 {
		b := &strings.Builder{}
		for shell, roles := range shellToRoles {
			fmt.Fprintf(b, "%s=%v ", shell, roles)
		}

		slog.WarnContext(context.Background(), "Host user shell resolution is ambiguous due to conflicting roles, consider unifying roles around a single shell",
			"selected_shell", shell,
			"shell_assignments", b,
		)
	}

	for _, role := range a.RoleSet {
		result, _, err := checkRoleLabelsMatch(types.Deny, role, a.info.Traits, s, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !result {
			continue
		}
		for _, group := range role.GetHostGroups(types.Deny) {
			delete(groups, group)
		}
	}

	traits := a.Traits()
	var gid string
	gidL := traits[constants.TraitHostUserGID]
	if len(gidL) >= 1 {
		gid = gidL[0]
	}
	var uid string
	uidL := traits[constants.TraitHostUserUID]
	if len(uidL) >= 1 {
		uid = uidL[0]
	}

	return &HostUsersInfo{
		Groups: utils.StringsSliceFromSet(groups),
		Mode:   convertHostUserMode(mode),
		UID:    uid,
		GID:    gid,
		Shell:  shell,
	}, nil
}

// HostSudoers returns host sudoers entries matching a server
func (a *accessChecker) HostSudoers(s types.Server) ([]string, error) {
	var sudoers []string

	roleSet := slices.Clone(a.RoleSet)
	slices.SortFunc(roleSet, func(a types.Role, b types.Role) int {
		return strings.Compare(a.GetName(), b.GetName())
	})

	seenSudoers := make(map[string]struct{})
	for _, role := range roleSet {
		result, _, err := checkRoleLabelsMatch(types.Allow, role, a.info.Traits, s, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// skip nodes that dont have matching labels
		if !result {
			continue
		}

		for _, sudoer := range role.GetHostSudoers(types.Allow) {
			if _, ok := seenSudoers[sudoer]; ok {
				continue
			}
			seenSudoers[sudoer] = struct{}{}
			sudoers = append(sudoers, sudoer)
		}
	}

	var finalSudoers []string
	for _, role := range roleSet {
		result, _, err := checkRoleLabelsMatch(types.Deny, role, a.info.Traits, s, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !result {
			continue
		}

	outer:
		for _, sudoer := range sudoers {
			for _, deniedSudoer := range role.GetHostSudoers(types.Deny) {
				if deniedSudoer == "*" {
					finalSudoers = nil
					break outer
				}
				if sudoer != deniedSudoer {
					finalSudoers = append(finalSudoers, sudoer)
				}
			}
		}
		sudoers = finalSudoers
	}

	return sudoers, nil
}

// AccessInfoFromLocalSSHIdentity returns a new AccessInfo populated from the
// given sshca.Identity. Should only be used for cluster local users as roles
// will not be mapped.
func AccessInfoFromLocalSSHIdentity(ident *sshca.Identity) *AccessInfo {
	return &AccessInfo{
		Username:           ident.Username,
		Roles:              ident.Roles,
		Traits:             ident.Traits,
		AllowedResourceIDs: ident.AllowedResourceIDs,
	}
}

// AccessInfoFromRemoteSSHIdentity returns a new AccessInfo populated from the
// given remote cluster user's ssh identity. Remote roles will be mapped to
// local roles based on the given roleMap.
func AccessInfoFromRemoteSSHIdentity(unmappedIdentity *sshca.Identity, roleMap types.RoleMap) (*AccessInfo, error) {
	// make a shallow copy of traits to avoid modifying the original
	traits := make(map[string][]string, len(unmappedIdentity.Traits)+1)
	for k, v := range unmappedIdentity.Traits {
		traits[k] = v
	}

	// Prior to Teleport 6.2 the only trait passed to the remote cluster
	// was the "logins" trait set to the SSH certificate principals.
	//
	// Keep backwards-compatible behavior and set it in addition to the
	// traits extracted from the certificate.
	traits[constants.TraitLogins] = unmappedIdentity.Principals

	roles, err := MapRoles(roleMap, unmappedIdentity.Roles)
	if err != nil {
		return nil, trace.AccessDenied("failed to map roles for user with remote roles %v: %v", unmappedIdentity.Roles, err)
	}
	if len(roles) == 0 {
		return nil, trace.AccessDenied("no roles mapped for user with remote roles %v", unmappedIdentity.Roles)
	}
	slog.DebugContext(context.Background(), "Mapped remote roles to local roles and traits",
		"remote_roles", unmappedIdentity.Roles,
		"local_roles", roles,
		"traits", traits,
	)

	return &AccessInfo{
		Username:           unmappedIdentity.Username,
		Roles:              roles,
		Traits:             traits,
		AllowedResourceIDs: unmappedIdentity.AllowedResourceIDs,
	}, nil
}

// AccessInfoFromLocalTLSIdentity returns a new AccessInfo populated from the given
// tlsca.Identity. Should only be used for cluster local users as roles will not
// be mapped.
func AccessInfoFromLocalTLSIdentity(identity tlsca.Identity, access UserGetter) (*AccessInfo, error) {
	roles := identity.Groups
	traits := identity.Traits

	// Legacy certs are not encoded with roles or traits,
	// so we fallback to the traits and roles in the backend.
	// empty traits are a valid use case in standard certs,
	// so we only check for whether roles are empty.
	if len(identity.Groups) == 0 {
		u, err := access.GetUser(context.TODO(), identity.Username, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		const msg = "Failed to find roles in x509 identity. Fetching " +
			"from backend. If the identity provider allows username changes, this can " +
			"potentially allow an attacker to change the role of the existing user."
		slog.WarnContext(context.Background(), msg, "username", identity.Username)
		roles = u.GetRoles()
		traits = u.GetTraits()
	}

	return &AccessInfo{
		Username:           identity.Username,
		Roles:              roles,
		Traits:             traits,
		AllowedResourceIDs: identity.AllowedResourceIDs,
	}, nil
}

// AccessInfoFromRemoteTLSIdentity returns a new AccessInfo populated from the
// given remote cluster user's tlsca.Identity. Remote roles will be mapped to
// local roles based on the given roleMap.
func AccessInfoFromRemoteTLSIdentity(identity tlsca.Identity, roleMap types.RoleMap) (*AccessInfo, error) {
	// Set internal traits for the remote user. This allows Teleport to work by
	// passing exact logins, Kubernetes users/groups, database users/names, and
	// AWS Role ARNs to the remote cluster.
	traits := map[string][]string{
		constants.TraitLogins:      identity.Principals,
		constants.TraitKubeGroups:  identity.KubernetesGroups,
		constants.TraitKubeUsers:   identity.KubernetesUsers,
		constants.TraitDBNames:     identity.DatabaseNames,
		constants.TraitDBUsers:     identity.DatabaseUsers,
		constants.TraitAWSRoleARNs: identity.AWSRoleARNs,
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
	slog.DebugContext(context.Background(), "Mapped roles of remote user to local roles and traits",
		"remote_roles", unmappedRoles,
		"user", identity.Username,
		"local_roles", roles,
		"traits", traits,
	)

	return &AccessInfo{
		Username:           identity.Username,
		Roles:              roles,
		Traits:             traits,
		AllowedResourceIDs: identity.AllowedResourceIDs,
	}, nil
}

// UserState is a representation of a user's current state.
type UserState interface {
	// GetName returns the username associated with the user state.
	GetName() string

	// GetRoles returns the roles associated with the user's current state.
	GetRoles() []string

	// GetTraits returns the traits associated with the user's current sate.
	GetTraits() map[string][]string

	// GetUserType returns the user type for the user login state.
	GetUserType() types.UserType

	// IsBot returns true if the user belongs to a bot.
	IsBot() bool

	// GetGithubIdentities returns a list of connected GitHub identities
	GetGithubIdentities() []types.ExternalIdentity
}

// AccessInfoFromUser return a new AccessInfo populated from the roles and
// traits held be the given user. This should only be used in cases where the
// user does not have any active access requests (initial web login, initial
// tbot certs, tests).
// TODO(mdwn): Remove this once enterprise has been moved away from this function.
func AccessInfoFromUser(user types.User) *AccessInfo {
	return AccessInfoFromUserState(user)
}

// AccessInfoFromUserState return a new AccessInfo populated from the roles and
// traits held be the given user state. This should only be used in cases where the
// user does not have any active access requests (initial web login, initial
// tbot certs, tests).
func AccessInfoFromUserState(user UserState) *AccessInfo {
	roles := user.GetRoles()
	traits := user.GetTraits()
	return &AccessInfo{
		Username: user.GetName(),
		Roles:    roles,
		Traits:   traits,
	}
}
