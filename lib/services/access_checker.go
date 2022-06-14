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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
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

	// LockingMode returns the locking mode to apply with this checker.
	LockingMode(defaultMode constants.LockingMode) constants.LockingMode

	// ExtractConditionForIdentifier returns a restrictive filter expression
	// for list queries based on the rules' `where` conditions.
	ExtractConditionForIdentifier(ctx RuleContext, namespace, resource, verb, identifier string) (*types.WhereExpr, error)

	// CertificateExtensions returns the list of extensions for each role in the RoleSet
	CertificateExtensions() []*types.CertExtension

	// GetSearchAsRoles returns the list of roles which the checker should be able to
	// "assume" while searching for resources, and should be able to request with a
	// search-based access request.
	GetSearchAsRoles() []string

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

	// PinSourceIP forces the same client IP for certificate generation and SSH usage
	PinSourceIP() bool
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
	// RoleSet holds the fetched and parsed roles from Roles.
	RoleSet RoleSet
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

// NewAccessChecker returns a new AccessChecker which may be used to check
// access to resources.
// Args:
// - `info *AccessInfo` should at a minimum hold a valid RoleSet for the
//   identity for which resource access should be checked. It must also hold the
//   AllowedResourceIDs for the identity if there is any possibility that it has
//   been granted a search-based access request.
// - `localCluster string` should be the name of the local cluster in which
//   access will be checked. You cannot check for access to resources in remote
//   clusters.
func NewAccessChecker(info *AccessInfo, localCluster string) AccessChecker {
	return &accessChecker{
		info:         info,
		localCluster: localCluster,
		RoleSet:      info.RoleSet,
	}
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
	return trace.Wrap(a.RoleSet.checkAccess(r, mfa, matchers...))
}

// GetAllowedResourceIDs returns the list of allowed resources the identity for
// the AccessChecker is allowed to access. An empty or nil list indicates that
// there are no resource-specific restrictions.
func (a *accessChecker) GetAllowedResourceIDs() []types.ResourceID {
	return a.info.AllowedResourceIDs
}

// GetSearchAsRoles returns the list of roles which the AccessChecker should be
// able to "assume" while searching for resources, and should be able to request
// with a search-based access request.
func (a *accessChecker) GetSearchAsRoles() []string {
	if len(a.info.AllowedResourceIDs) > 0 {
		// cannot search with extended roles while already logged in the
		// search-based access request.
		return nil
	}
	return a.RoleSet.GetSearchAsRoles()
}

// AccessInfoFromLocalCertificate returns a new AccessInfo populated from the
// given ssh certificate. Should only be used for cluster local users as roles
// will not be mapped.
func AccessInfoFromLocalCertificate(cert *ssh.Certificate, access RoleGetter) (*AccessInfo, error) {
	traits, err := ExtractTraitsFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := ExtractRolesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleSet, err := FetchRoles(roles, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedResourceIDs, err := ExtractAllowedResourcesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessInfo{
		Roles:              roles,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
		RoleSet:            roleSet,
	}, nil
}

// AccessInfoFromRemoteCertificate returns a new AccessInfo populated from the
// given remote cluster user's ssh certificate. Remote roles will be mapped to
// local roles based on the given roleMap.
func AccessInfoFromRemoteCertificate(cert *ssh.Certificate, access RoleGetter, roleMap types.RoleMap) (*AccessInfo, error) {
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
	traits[teleport.TraitLogins] = cert.ValidPrincipals

	unmappedRoles, err := ExtractRolesFromCert(cert)
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

	roleSet, err := FetchRoles(roles, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedResourceIDs, err := ExtractAllowedResourcesFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessInfo{
		Roles:              roles,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
		RoleSet:            roleSet,
	}, nil
}

type RoleAndUserGetter interface {
	RoleGetter
	UserGetter
}

// AccessInfoFromLocalIdentity returns a new AccessInfo populated from the given
// tlsca.Identity. Should only be used for cluster local users as roles will not
// be mapped.
func AccessInfoFromLocalIdentity(identity tlsca.Identity, access RoleAndUserGetter) (*AccessInfo, error) {
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

		log.Warnf("Failed to find roles or traits in x509 identity for %v. Fetching	"+
			"from backend. If the identity provider allows username changes, this can "+
			"potentially allow an attacker to change the role of the existing user.",
			identity.Username)
		roles = u.GetRoles()
		traits = u.GetTraits()
	}

	roleSet, err := FetchRoles(roles, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessInfo{
		Roles:              roles,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
		RoleSet:            roleSet,
	}, nil
}

// AccessInfoFromRemoteIdentity returns a new AccessInfo populated from the
// given remote cluster user's tlsca.Identity. Remote roles will be mapped to
// local roles based on the given roleMap.
func AccessInfoFromRemoteIdentity(identity tlsca.Identity, access RoleGetter, roleMap types.RoleMap) (*AccessInfo, error) {
	// Set internal traits for the remote user. This allows Teleport to work by
	// passing exact logins, Kubernetes users/groups and database users/names
	// to the remote cluster.
	traits := map[string][]string{
		teleport.TraitLogins:     identity.Principals,
		teleport.TraitKubeGroups: identity.KubernetesGroups,
		teleport.TraitKubeUsers:  identity.KubernetesUsers,
		teleport.TraitDBNames:    identity.DatabaseNames,
		teleport.TraitDBUsers:    identity.DatabaseUsers,
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

	roleSet, err := FetchRoles(roles, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedResourceIDs := identity.AllowedResourceIDs

	return &AccessInfo{
		Roles:              roles,
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
		RoleSet:            roleSet,
	}, nil
}

// AccessInfoFromUser return a new AccessInfo populated from the roles and
// traits held be the given user. This should only be used in cases where the
// user does not have any active access requests (initial web login, initial
// tbot certs, tests).
func AccessInfoFromUser(user types.User, access RoleGetter) (*AccessInfo, error) {
	roles := user.GetRoles()
	traits := user.GetTraits()
	roleSet, err := FetchRoles(roles, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AccessInfo{
		Roles:   roles,
		Traits:  traits,
		RoleSet: roleSet,
	}, nil
}
