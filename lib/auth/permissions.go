/*
Copyright 2015-2018 Gravitational, Inc.

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

package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// NewAdminContext returns new admin auth context
func NewAdminContext() (*Context, error) {
	return NewBuiltinRoleContext(types.RoleAdmin)
}

// NewBuiltinRoleContext create auth context for the provided builtin role.
func NewBuiltinRoleContext(role types.SystemRole) (*Context, error) {
	authContext, err := contextForBuiltinRole(BuiltinRole{Role: role, Username: fmt.Sprintf("%v", role)}, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return authContext, nil
}

// NewAuthorizer returns new authorizer using backends
func NewAuthorizer(clusterName string, accessPoint AuthorizerAccessPoint, lockWatcher *services.LockWatcher) (Authorizer, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing parameter clusterName")
	}
	if accessPoint == nil {
		return nil, trace.BadParameter("missing parameter accessPoint")
	}
	return &authorizer{
		clusterName: clusterName,
		accessPoint: accessPoint,
		lockWatcher: lockWatcher,
	}, nil
}

// Authorizer authorizes identity and returns auth context
type Authorizer interface {
	// Authorize authorizes user based on identity supplied via context
	Authorize(ctx context.Context) (*Context, error)
}

// AuthorizerAccessPoint is the access point contract required by an Authorizer
type AuthorizerAccessPoint interface {
	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetUser returns a services.User for this cluster.
	GetUser(name string, withSecrets bool) (types.User, error)

	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

	// GetClusterAuditConfig returns cluster audit configuration.
	GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)
}

// authorizer creates new local authorizer
type authorizer struct {
	clusterName string
	accessPoint AuthorizerAccessPoint
	lockWatcher *services.LockWatcher
}

// Context is authorization context
type Context struct {
	// User is the username
	User types.User
	// Checker is access checker
	Checker services.AccessChecker
	// Identity holds the caller identity:
	// 1. If caller is a user
	//   a. local user identity
	//   b. remote user identity remapped to local identity based on trusted
	//      cluster role mapping.
	// 2. If caller is a teleport instance, Identity holds their identity as-is
	//    (because there's no role mapping for non-human roles)
	Identity IdentityGetter
	// UnmappedIdentity holds the original caller identity. If this is a remote
	// user, UnmappedIdentity holds the data before role mapping. Otherwise,
	// it's identical to Identity.
	UnmappedIdentity IdentityGetter
}

// LockTargets returns a list of LockTargets inferred from the context's
// Identity and UnmappedIdentity.
func (c *Context) LockTargets() []types.LockTarget {
	lockTargets := services.LockTargetsFromTLSIdentity(c.Identity.GetIdentity())
Loop:
	for _, unmappedTarget := range services.LockTargetsFromTLSIdentity(c.UnmappedIdentity.GetIdentity()) {
		// Append a lock target from UnmappedIdentity only if it is not already
		// known from Identity.
		for _, knownTarget := range lockTargets {
			if unmappedTarget.Equals(knownTarget) {
				continue Loop
			}
		}
		lockTargets = append(lockTargets, unmappedTarget)
	}

	if r, ok := c.Identity.(BuiltinRole); ok {
		switch r.Role {
		// Node role is a special case because it was previously suported as a
		// lock target that only locked the `ssh_service`. If the same Teleport server
		// had multiple roles, Node lock would only lock the `ssh_service` while
		// other roles would be able to authenticate into Teleport without a problem.
		// To remove the ambiguity, we now lock the entire Teleport server for
		// all roles using the LockTarget.ServerID field and `Node` field is
		// deprecated.
		// In order to support legacy behavior, we need fill in both `ServerID`
		// and `Node` fields if the role is `Node` so that the previous behavior
		// is preserved.
		// This is a legacy behavior that we need to support for backwards compatibility.
		case types.RoleNode:
			lockTargets = append(lockTargets,
				types.LockTarget{Node: r.GetServerID(), ServerID: r.GetServerID()},
				types.LockTarget{Node: r.Identity.Username, ServerID: r.Identity.Username},
			)
		default:
			lockTargets = append(lockTargets,
				types.LockTarget{ServerID: r.GetServerID()},
				types.LockTarget{ServerID: r.Identity.Username},
			)
		}
	}
	return lockTargets
}

// UseExtraRoles extends the roles of the Checker on the current Context with
// the given extra roles.
func (c *Context) UseExtraRoles(access services.RoleGetter, clusterName string, roles []string) error {
	var newRoleNames []string
	newRoleNames = append(newRoleNames, c.Checker.RoleNames()...)
	newRoleNames = append(newRoleNames, roles...)
	newRoleNames = utils.Deduplicate(newRoleNames)

	// set new roles on the context user and create a new access checker
	c.User.SetRoles(newRoleNames)
	accessInfo := services.AccessInfoFromUser(c.User)
	checker, err := services.NewAccessChecker(accessInfo, clusterName, access)
	if err != nil {
		return trace.Wrap(err)
	}
	c.Checker = checker
	return nil
}

// MFAParams returns MFA params for the given auth context and auth preference MFA requirement.
func (c *Context) MFAParams(authPrefMFARequirement types.RequireMFAType) services.AccessMFAParams {
	params := c.Checker.MFAParams(authPrefMFARequirement)

	// Builtin services (like proxy_service and kube_service) are not gated
	// on MFA and only need to pass normal RBAC action checks.
	_, isService := c.Identity.(BuiltinRole)
	params.Verified = isService || c.Identity.GetIdentity().MFAVerified != ""
	return params
}

// Authorize authorizes user based on identity supplied via context
func (a *authorizer) Authorize(ctx context.Context) (*Context, error) {
	if ctx == nil {
		return nil, trace.AccessDenied("missing authentication context")
	}
	userI := ctx.Value(ContextUser)
	authContext, err := a.fromUser(ctx, userI)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Enforce applicable locks.
	authPref, err := a.accessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if lockErr := a.lockWatcher.CheckLockInForce(
		authContext.Checker.LockingMode(authPref.GetLockingMode()),
		authContext.LockTargets()...); lockErr != nil {
		return nil, trace.Wrap(lockErr)
	}

	// Enforce required private key policy if set.
	if err := a.enforcePrivateKeyPolicy(ctx, authContext, authPref); err != nil {
		return nil, trace.Wrap(err)
	}

	return authContext, nil
}

func (a *authorizer) enforcePrivateKeyPolicy(ctx context.Context, authContext *Context, authPref types.AuthPreference) error {
	switch authContext.Identity.(type) {
	case BuiltinRole, RemoteBuiltinRole:
		// built in roles do not need to pass private key policies
		return nil
	}

	// Check that the required private key policy, defined by roles and auth pref,
	// is met by this Identity's tls certificate.
	identityPolicy := authContext.Identity.GetIdentity().PrivateKeyPolicy
	requiredPolicy := authContext.Checker.PrivateKeyPolicy(authPref.GetPrivateKeyPolicy())
	if err := requiredPolicy.VerifyPolicy(identityPolicy); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *authorizer) fromUser(ctx context.Context, userI interface{}) (*Context, error) {
	switch user := userI.(type) {
	case LocalUser:
		return a.authorizeLocalUser(user)
	case RemoteUser:
		return a.authorizeRemoteUser(ctx, user)
	case BuiltinRole:
		return a.authorizeBuiltinRole(ctx, user)
	case RemoteBuiltinRole:
		return a.authorizeRemoteBuiltinRole(user)
	default:
		return nil, trace.AccessDenied("unsupported context type %T", userI)
	}
}

// authorizeLocalUser returns authz context based on the username
func (a *authorizer) authorizeLocalUser(u LocalUser) (*Context, error) {
	return contextForLocalUser(u, a.accessPoint, a.clusterName)
}

// authorizeRemoteUser returns checker based on cert authority roles
func (a *authorizer) authorizeRemoteUser(ctx context.Context, u RemoteUser) (*Context, error) {
	ca, err := a.accessPoint.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: u.ClusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessInfo, err := services.AccessInfoFromRemoteIdentity(u.Identity, ca.CombinedMapping())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, a.clusterName, a.accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The user is prefixed with "remote-" and suffixed with cluster name with
	// the hope that it does not match a real local user.
	user, err := types.NewUser(fmt.Sprintf("remote-%v-%v", u.Username, u.ClusterName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetTraits(accessInfo.Traits)
	user.SetRoles(accessInfo.Roles)

	// Adjust expiry based on locally mapped roles.
	ttl := time.Until(u.Identity.Expires)
	ttl = checker.AdjustSessionTTL(ttl)
	var previousIdentityExpires time.Time
	if u.Identity.MFAVerified != "" {
		prevIdentityTTL := time.Until(u.Identity.PreviousIdentityExpires)
		prevIdentityTTL = checker.AdjustSessionTTL(prevIdentityTTL)
		previousIdentityExpires = time.Now().Add(prevIdentityTTL)
	}

	kubeUsers, kubeGroups, err := checker.CheckKubeGroupsAndUsers(ttl, false)
	// IsNotFound means that the user has no k8s users or groups, which is fine
	// in many cases. The downstream k8s handler will ensure that users/groups
	// are set if this is a k8s request.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	principals, err := checker.CheckLoginDuration(ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Convert u.Identity into the mapped local identity.
	//
	// This prevents downstream users from accidentally using the unmapped
	// identity information and confusing who's accessing a resource.
	identity := tlsca.Identity{
		Username:                user.GetName(),
		Groups:                  user.GetRoles(),
		Traits:                  accessInfo.Traits,
		Principals:              principals,
		KubernetesGroups:        kubeGroups,
		KubernetesUsers:         kubeUsers,
		TeleportCluster:         a.clusterName,
		Expires:                 time.Now().Add(ttl),
		PreviousIdentityExpires: previousIdentityExpires,

		// These fields are for routing and restrictions, safe to re-use from
		// unmapped identity.
		Usage:             u.Identity.Usage,
		RouteToCluster:    u.Identity.RouteToCluster,
		KubernetesCluster: u.Identity.KubernetesCluster,
		RouteToApp:        u.Identity.RouteToApp,
		RouteToDatabase:   u.Identity.RouteToDatabase,
		MFAVerified:       u.Identity.MFAVerified,
		ClientIP:          u.Identity.ClientIP,
		PrivateKeyPolicy:  u.Identity.PrivateKeyPolicy,
	}

	return &Context{
		User:             user,
		Checker:          checker,
		Identity:         WrapIdentity(identity),
		UnmappedIdentity: u,
	}, nil
}

// authorizeBuiltinRole authorizes builtin role
func (a *authorizer) authorizeBuiltinRole(ctx context.Context, r BuiltinRole) (*Context, error) {
	recConfig, err := a.accessPoint.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return contextForBuiltinRole(r, recConfig)
}

func (a *authorizer) authorizeRemoteBuiltinRole(r RemoteBuiltinRole) (*Context, error) {
	if r.Role != types.RoleProxy {
		return nil, trace.AccessDenied("access denied for remote %v connecting to cluster", r.Role)
	}
	roleSet, err := services.RoleSetFromSpec(
		string(types.RoleRemoteProxy),
		types.RoleSpecV5{
			Allow: types.RoleConditions{
				Namespaces:       []string{types.Wildcard},
				NodeLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Rules: []types.Rule{
					types.NewRule(types.KindNode, services.RO()),
					types.NewRule(types.KindProxy, services.RO()),
					types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
					types.NewRule(types.KindNamespace, services.RO()),
					types.NewRule(types.KindUser, services.RO()),
					types.NewRule(types.KindRole, services.RO()),
					types.NewRule(types.KindAuthServer, services.RO()),
					types.NewRule(types.KindReverseTunnel, services.RO()),
					types.NewRule(types.KindTunnelConnection, services.RO()),
					types.NewRule(types.KindClusterName, services.RO()),
					types.NewRule(types.KindClusterAuditConfig, services.RO()),
					types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
					types.NewRule(types.KindSessionRecordingConfig, services.RO()),
					types.NewRule(types.KindClusterAuthPreference, services.RO()),
					types.NewRule(types.KindKubeService, services.RO()),
					types.NewRule(types.KindKubeServer, services.RO()),
					types.NewRule(types.KindInstaller, services.RO()),
					types.NewRule(types.KindDatabaseService, services.RO()),
					// this rule allows remote proxy to update the cluster's certificate authorities
					// during certificates renewal
					{
						Resources: []string{types.KindCertAuthority},
						// It is important that remote proxy can only rotate
						// existing certificate authority, and not create or update new ones
						Verbs: []string{types.VerbRead, types.VerbRotate},
						// allow administrative access to the certificate authority names
						// matching the cluster name only
						Where: builder.Equals(services.ResourceNameExpr, builder.String(r.ClusterName)).String(),
					},
				},
			},
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(r.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles := []string{string(types.RoleRemoteProxy)}
	user.SetRoles(roles)
	checker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
		Roles:              roles,
		Traits:             nil,
		AllowedResourceIDs: nil,
	}, a.clusterName, roleSet)
	return &Context{
		User:             user,
		Checker:          checker,
		Identity:         r,
		UnmappedIdentity: r,
	}, nil
}

func roleSpecForProxyWithRecordAtProxy(clusterName string) types.RoleSpecV5 {
	base := roleSpecForProxy(clusterName)
	base.Allow.Rules = append(base.Allow.Rules, types.NewRule(types.KindHostCert, services.RW()))
	return base
}

func roleSpecForProxy(clusterName string) types.RoleSpecV5 {
	return types.RoleSpecV5{
		Allow: types.RoleConditions{
			Namespaces:            []string{types.Wildcard},
			ClusterLabels:         types.Labels{types.Wildcard: []string{types.Wildcard}},
			NodeLabels:            types.Labels{types.Wildcard: []string{types.Wildcard}},
			AppLabels:             types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubernetesLabels:      types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindProxy, services.RW()),
				types.NewRule(types.KindOIDCRequest, services.RW()),
				types.NewRule(types.KindSSHSession, services.RW()),
				types.NewRule(types.KindSession, services.RO()),
				types.NewRule(types.KindEvent, services.RW()),
				types.NewRule(types.KindSAMLRequest, services.RW()),
				types.NewRule(types.KindOIDC, services.ReadNoSecrets()),
				types.NewRule(types.KindSAML, services.ReadNoSecrets()),
				types.NewRule(types.KindGithub, services.ReadNoSecrets()),
				types.NewRule(types.KindGithubRequest, services.RW()),
				types.NewRule(types.KindNamespace, services.RO()),
				types.NewRule(types.KindNode, services.RO()),
				types.NewRule(types.KindAuthServer, services.RO()),
				types.NewRule(types.KindReverseTunnel, services.RO()),
				types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
				types.NewRule(types.KindUser, services.RO()),
				types.NewRule(types.KindRole, services.RO()),
				types.NewRule(types.KindClusterAuthPreference, services.RO()),
				types.NewRule(types.KindClusterName, services.RO()),
				types.NewRule(types.KindClusterAuditConfig, services.RO()),
				types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
				types.NewRule(types.KindSessionRecordingConfig, services.RO()),
				types.NewRule(types.KindStaticTokens, services.RO()),
				types.NewRule(types.KindTunnelConnection, services.RW()),
				types.NewRule(types.KindRemoteCluster, services.RO()),
				types.NewRule(types.KindSemaphore, services.RW()),
				types.NewRule(types.KindAppServer, services.RO()),
				types.NewRule(types.KindWebSession, services.RW()),
				types.NewRule(types.KindWebToken, services.RW()),
				types.NewRule(types.KindKubeService, services.RW()),
				types.NewRule(types.KindKubeServer, services.RW()),
				types.NewRule(types.KindDatabaseServer, services.RO()),
				types.NewRule(types.KindLock, services.RO()),
				types.NewRule(types.KindToken, []string{types.VerbRead, types.VerbDelete}),
				types.NewRule(types.KindWindowsDesktopService, services.RO()),
				types.NewRule(types.KindDatabaseCertificate, []string{types.VerbCreate}),
				types.NewRule(types.KindWindowsDesktop, services.RO()),
				types.NewRule(types.KindInstaller, services.RO()),
				types.NewRule(types.KindConnectionDiagnostic, services.RW()),
				types.NewRule(types.KindDatabaseService, services.RO()),
				// this rule allows local proxy to update the remote cluster's host certificate authorities
				// during certificates renewal
				{
					Resources: []string{types.KindCertAuthority},
					Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbUpdate},
					// allow administrative access to the host certificate authorities
					// matching any cluster name except local
					Where: builder.And(
						builder.Equals(services.CertAuthorityTypeExpr, builder.String(string(types.HostCA))),
						builder.Not(
							builder.Equals(
								services.ResourceNameExpr,
								builder.String(clusterName),
							),
						),
					).String(),
				},
			},
		},
	}
}

// RoleSetForBuiltinRole returns RoleSet for embedded builtin role
func RoleSetForBuiltinRoles(clusterName string, recConfig types.SessionRecordingConfig, roles ...types.SystemRole) (services.RoleSet, error) {
	var definitions []types.Role
	for _, role := range roles {
		rd, err := definitionForBuiltinRole(clusterName, recConfig, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		definitions = append(definitions, rd)
	}
	return services.NewRoleSet(definitions...), nil
}

// definitionForBuiltinRole constructs the appropriate role definition for a given builtin role.
func definitionForBuiltinRole(clusterName string, recConfig types.SessionRecordingConfig, role types.SystemRole) (types.Role, error) {
	switch role {
	case types.RoleAuth:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindAuthServer, services.RW()),
					},
				},
			})
	case types.RoleProvisionToken:
		return services.RoleFromSpec(role.String(), types.RoleSpecV5{})
	case types.RoleNode:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindNode, services.RW()),
						types.NewRule(types.KindSSHSession, services.RW()),
						types.NewRule(types.KindSession, services.RO()),
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindReverseTunnel, services.RW()),
						types.NewRule(types.KindTunnelConnection, services.RO()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindSemaphore, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindNetworkRestrictions, services.RO()),
						types.NewRule(types.KindConnectionDiagnostic, services.RW()),
					},
				},
			})
	case types.RoleApp:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					AppLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindReverseTunnel, services.RW()),
						types.NewRule(types.KindTunnelConnection, services.RO()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindAppServer, services.RW()),
						types.NewRule(types.KindApp, services.RO()),
						types.NewRule(types.KindWebSession, services.RO()),
						types.NewRule(types.KindWebToken, services.RO()),
						types.NewRule(types.KindJWT, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
					},
				},
			})
	case types.RoleDatabase:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces:     []string{types.Wildcard},
					DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindProxy, services.RO()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindReverseTunnel, services.RW()),
						types.NewRule(types.KindTunnelConnection, services.RO()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindDatabaseServer, services.RW()),
						types.NewRule(types.KindDatabaseService, services.RW()),
						types.NewRule(types.KindDatabase, services.RO()),
						types.NewRule(types.KindSemaphore, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindConnectionDiagnostic, services.RW()),
					},
				},
			})
	case types.RoleProxy:
		// if in recording mode, return a different set of permissions than regular
		// mode. recording proxy needs to be able to generate host certificates.
		if services.IsRecordAtProxy(recConfig.GetMode()) {
			return services.RoleFromSpec(
				role.String(),
				roleSpecForProxyWithRecordAtProxy(clusterName),
			)
		}
		return services.RoleFromSpec(
			role.String(),
			roleSpecForProxy(clusterName),
		)
	case types.RoleSignup:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
					},
				},
			})
	case types.RoleAdmin:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Options: types.RoleOptions{
					MaxSessionTTL: types.MaxDuration(),
				},
				Allow: types.RoleConditions{
					Namespaces:            []string{types.Wildcard},
					Logins:                []string{},
					NodeLabels:            types.Labels{types.Wildcard: []string{types.Wildcard}},
					AppLabels:             types.Labels{types.Wildcard: []string{types.Wildcard}},
					KubernetesLabels:      types.Labels{types.Wildcard: []string{types.Wildcard}},
					DatabaseLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
					DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					ClusterLabels:         types.Labels{types.Wildcard: []string{types.Wildcard}},
					WindowsDesktopLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.Wildcard, services.RW()),
					},
				},
			})
	case types.RoleNop:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces: []string{},
					Rules:      []types.Rule{},
				},
			})
	case types.RoleKube:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces:       []string{types.Wildcard},
					KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindKubeService, services.RW()),
						types.NewRule(types.KindKubeServer, services.RW()),
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindKubernetesCluster, services.RO()),
						types.NewRule(types.KindSemaphore, services.RW()),
					},
				},
			})
	case types.RoleWindowsDesktop:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces:           []string{types.Wildcard},
					WindowsDesktopLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindClusterAuditConfig, services.RO()),
						types.NewRule(types.KindClusterNetworkingConfig, services.RO()),
						types.NewRule(types.KindSessionRecordingConfig, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
						types.NewRule(types.KindUser, services.RO()),
						types.NewRule(types.KindRole, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindWindowsDesktopService, services.RW()),
						types.NewRule(types.KindWindowsDesktop, services.RW()),
					},
				},
			})
	case types.RoleDiscovery:
		return services.RoleFromSpec(
			role.String(),
			types.RoleSpecV5{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindEvent, services.RW()),
						types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
						types.NewRule(types.KindClusterName, services.RO()),
						types.NewRule(types.KindNamespace, services.RO()),
						types.NewRule(types.KindNode, services.RO()),
						types.NewRule(types.KindKubernetesCluster, services.RW()),
					},
					// wildcard any cluster available.
					KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				},
			})
	}

	return nil, trace.NotFound("builtin role %q is not recognized", role.String())
}

func contextForBuiltinRole(r BuiltinRole, recConfig types.SessionRecordingConfig) (*Context, error) {
	var systemRoles []types.SystemRole
	if r.Role == types.RoleInstance {
		// instance certs encode multiple system roles in a separate field
		systemRoles = r.AdditionalSystemRoles
		if len(systemRoles) == 0 {
			// note: previous parsing skipped unknown roles for this field, so its possible that some
			// system roles were defined, but they were all unknown to us.
			return nil, trace.BadParameter("cannot create instance context, no additional system roles recognized")
		}
	} else {
		// all other certs encode a single system role
		systemRoles = []types.SystemRole{r.Role}
	}
	roleSet, err := RoleSetForBuiltinRoles(r.ClusterName, recConfig, systemRoles...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(r.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var roles []string
	for _, r := range systemRoles {
		roles = append(roles, string(r))
	}
	user.SetRoles(roles)
	checker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
		Roles:              roles,
		Traits:             nil,
		AllowedResourceIDs: nil,
	}, r.ClusterName, roleSet)
	return &Context{
		User:             user,
		Checker:          checker,
		Identity:         r,
		UnmappedIdentity: r,
	}, nil
}

func contextForLocalUser(u LocalUser, accessPoint AuthorizerAccessPoint, clusterName string) (*Context, error) {
	// User has to be fetched to check if it's a blocked username
	user, err := accessPoint.GetUser(u.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessInfo, err := services.AccessInfoFromLocalIdentity(u.Identity, accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName, accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Override roles and traits from the local user based on the identity roles
	// and traits, this is done to prevent potential conflict. Imagine a scenario
	// when SSO user has left the company, but local user entry remained with old
	// privileged roles. New user with the same name has been onboarded and would
	// have derived the roles from the stale user entry. This code prevents
	// that by extracting up to date identity traits and roles from the user's
	// certificate metadata.
	user.SetRoles(accessInfo.Roles)
	user.SetTraits(accessInfo.Traits)

	return &Context{
		User:             user,
		Checker:          accessChecker,
		Identity:         u,
		UnmappedIdentity: u,
	}, nil
}

type contextKey string

const (
	// ContextUser is a user set in the context of the request
	ContextUser contextKey = "teleport-user"
	// ContextClientAddr is a client address set in the context of the request
	ContextClientAddr contextKey = "client-addr"
)

// WithDelegator alias for backwards compatibility
var WithDelegator = utils.WithDelegator

// ClientUsername returns the username of a remote HTTP client making the call.
// If ctx didn't pass through auth middleware or did not come from an HTTP
// request, teleport.UserSystem is returned.
func ClientUsername(ctx context.Context) string {
	userI := ctx.Value(ContextUser)
	userWithIdentity, ok := userI.(IdentityGetter)
	if !ok {
		return teleport.UserSystem
	}
	identity := userWithIdentity.GetIdentity()
	if identity.Username == "" {
		return teleport.UserSystem
	}
	return identity.Username
}

// GetClientUsername returns the username of a remote HTTP client making the call.
// If ctx didn't pass through auth middleware or did not come from an HTTP
// request, returns an error.
func GetClientUsername(ctx context.Context) (string, error) {
	userI := ctx.Value(ContextUser)
	userWithIdentity, ok := userI.(IdentityGetter)
	if !ok {
		return "", trace.AccessDenied("missing identity")
	}
	identity := userWithIdentity.GetIdentity()
	if identity.Username == "" {
		return "", trace.AccessDenied("missing identity username")
	}
	return identity.Username, nil
}

// ClientImpersonator returns the impersonator username of a remote client
// making the call. If not present, returns an empty string
func ClientImpersonator(ctx context.Context) string {
	userI := ctx.Value(ContextUser)
	userWithIdentity, ok := userI.(IdentityGetter)
	if !ok {
		return ""
	}
	identity := userWithIdentity.GetIdentity()
	return identity.Impersonator
}

// ClientUserMetadata returns a UserMetadata suitable for events caused by a
// remote client making a call. If ctx didn't pass through auth middleware or
// did not come from an HTTP request, metadata for teleport.UserSystem is
// returned.
func ClientUserMetadata(ctx context.Context) apievents.UserMetadata {
	userI := ctx.Value(ContextUser)
	userWithIdentity, ok := userI.(IdentityGetter)
	if !ok {
		return apievents.UserMetadata{
			User: teleport.UserSystem,
		}
	}
	meta := userWithIdentity.GetIdentity().GetUserMetadata()
	if meta.User == "" {
		meta.User = teleport.UserSystem
	}
	return meta
}

// ClientUserMetadataWithUser returns a UserMetadata suitable for events caused
// by a remote client making a call, with the specified username overriding the one
// from the remote client.
func ClientUserMetadataWithUser(ctx context.Context, user string) apievents.UserMetadata {
	userI := ctx.Value(ContextUser)
	userWithIdentity, ok := userI.(IdentityGetter)
	if !ok {
		return apievents.UserMetadata{
			User: user,
		}
	}
	meta := userWithIdentity.GetIdentity().GetUserMetadata()
	meta.User = user
	return meta
}

// LocalUser is a local user
type LocalUser struct {
	// Username is local username
	Username string
	// Identity is x509-derived identity used to build this user
	Identity tlsca.Identity
}

// GetIdentity returns client identity
func (l LocalUser) GetIdentity() tlsca.Identity {
	return l.Identity
}

// IdentityGetter returns the unmapped client identity.
//
// Unmapped means that if the client is a remote cluster user, the returned
// tlsca.Identity contains data from the remote cluster before role mapping is
// applied.
type IdentityGetter interface {
	// GetIdentity  returns x509-derived identity of the user
	GetIdentity() tlsca.Identity
}

// WrapIdentity wraps identity to return identity getter function
type WrapIdentity tlsca.Identity

// GetIdentity returns identity
func (i WrapIdentity) GetIdentity() tlsca.Identity {
	return tlsca.Identity(i)
}

// BuiltinRole is the role of the Teleport service.
type BuiltinRole struct {
	// Role is the primary builtin role this username is associated with
	Role types.SystemRole

	// AdditionalSystemRoles is a collection of additional system roles held by
	// this identity (only currently used by identities with RoleInstance as their
	// primary role).
	AdditionalSystemRoles types.SystemRoles

	// Username is for authentication tracking purposes
	Username string

	// ClusterName is the name of the local cluster
	ClusterName string

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// IsServer returns true if the primary role is either RoleInstance, or one of
// the local service roles (e.g. proxy).
func (r BuiltinRole) IsServer() bool {
	return r.Role == types.RoleInstance || r.Role.IsLocalService()
}

// GetServerID extracts the identity from the full name. The username
// extracted from the node's identity (x.509 certificate) is expected to
// consist of "<server-id>.<cluster-name>" so strip the cluster name suffix
// to get the server id.
//
// Note that as of right now Teleport expects server id to be a UUID4 but
// older Gravity clusters used to override it with strings like
// "192_168_1_1.<cluster-name>" so this code can't rely on it being
// UUID4 to account for clusters upgraded from older versions.
func (r BuiltinRole) GetServerID() string {
	return strings.TrimSuffix(r.Identity.Username, "."+r.ClusterName)
}

// GetIdentity returns client identity
func (r BuiltinRole) GetIdentity() tlsca.Identity {
	return r.Identity
}

// RemoteBuiltinRole is the role of the remote (service connecting via trusted cluster link)
// Teleport service.
type RemoteBuiltinRole struct {
	// Role is the builtin role of the user
	Role types.SystemRole

	// Username is for authentication tracking purposes
	Username string

	// ClusterName is the name of the remote cluster.
	ClusterName string

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// GetIdentity returns client identity
func (r RemoteBuiltinRole) GetIdentity() tlsca.Identity {
	return r.Identity
}

// RemoteUser defines encoded remote user.
type RemoteUser struct {
	// Username is a name of the remote user
	Username string `json:"username"`

	// ClusterName is the name of the remote cluster
	// of the user.
	ClusterName string `json:"cluster_name"`

	// RemoteRoles is optional list of remote roles
	RemoteRoles []string `json:"remote_roles"`

	// Principals is a list of Unix logins.
	Principals []string `json:"principals"`

	// KubernetesGroups is a list of Kubernetes groups
	KubernetesGroups []string `json:"kubernetes_groups"`

	// KubernetesUsers is a list of Kubernetes users
	KubernetesUsers []string `json:"kubernetes_users"`

	// DatabaseNames is a list of database names a user can connect to.
	DatabaseNames []string `json:"database_names"`

	// DatabaseUsers is a list of database users a user can connect as.
	DatabaseUsers []string `json:"database_users"`

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// GetIdentity returns client identity
func (r RemoteUser) GetIdentity() tlsca.Identity {
	return r.Identity
}
