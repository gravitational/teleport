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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate/builder"
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
	GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

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
	if r, ok := c.Identity.(BuiltinRole); ok && r.Role == types.RoleNode {
		lockTargets = append(lockTargets,
			types.LockTarget{Node: r.GetServerID()},
			types.LockTarget{Node: r.Identity.Username})
	}
	return lockTargets
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
	return authContext, nil
}

func (a *authorizer) fromUser(ctx context.Context, userI interface{}) (*Context, error) {
	switch user := userI.(type) {
	case LocalUser:
		return a.authorizeLocalUser(user)
	case RemoteUser:
		return a.authorizeRemoteUser(user)
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
	return contextForLocalUser(u, a.accessPoint)
}

// authorizeRemoteUser returns checker based on cert authority roles
func (a *authorizer) authorizeRemoteUser(u RemoteUser) (*Context, error) {
	ca, err := a.accessPoint.GetCertAuthority(types.CertAuthID{
		Type:       types.UserCA,
		DomainName: u.ClusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleNames, err := services.MapRoles(ca.CombinedMapping(), u.RemoteRoles)
	if err != nil {
		return nil, trace.AccessDenied("failed to map roles for remote user %q from cluster %q with remote roles %v", u.Username, u.ClusterName, u.RemoteRoles)
	}
	if len(roleNames) == 0 {
		return nil, trace.AccessDenied("no roles mapped for remote user %q from cluster %q with remote roles %v", u.Username, u.ClusterName, u.RemoteRoles)
	}
	// Set internal traits for the remote user. This allows Teleport to work by
	// passing exact logins, Kubernetes users/groups and database users/names
	// to the remote cluster.
	traits := map[string][]string{
		teleport.TraitLogins:     u.Principals,
		teleport.TraitKubeGroups: u.KubernetesGroups,
		teleport.TraitKubeUsers:  u.KubernetesUsers,
		teleport.TraitDBNames:    u.DatabaseNames,
		teleport.TraitDBUsers:    u.DatabaseUsers,
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
	for k, v := range u.Identity.Traits {
		if _, ok := traits[k]; !ok {
			traits[k] = v
		}
	}
	log.Debugf("Mapped roles %v of remote user %q to local roles %v and traits %v.",
		u.RemoteRoles, u.Username, roleNames, traits)
	checker, err := services.FetchRoles(roleNames, a.accessPoint, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// The user is prefixed with "remote-" and suffixed with cluster name with
	// the hope that it does not match a real local user.
	user, err := types.NewUser(fmt.Sprintf("remote-%v-%v", u.Username, u.ClusterName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetTraits(traits)

	// Set the list of roles this user has in the remote cluster.
	user.SetRoles(roleNames)

	// Adjust expiry based on locally mapped roles.
	ttl := time.Until(u.Identity.Expires)
	ttl = checker.AdjustSessionTTL(ttl)

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
		Username:         user.GetName(),
		Groups:           user.GetRoles(),
		Traits:           wrappers.Traits(traits),
		Principals:       principals,
		KubernetesGroups: kubeGroups,
		KubernetesUsers:  kubeUsers,
		TeleportCluster:  a.clusterName,
		Expires:          time.Now().Add(ttl),

		// These fields are for routing and restrictions, safe to re-use from
		// unmapped identity.
		Usage:             u.Identity.Usage,
		RouteToCluster:    u.Identity.RouteToCluster,
		KubernetesCluster: u.Identity.KubernetesCluster,
		RouteToApp:        u.Identity.RouteToApp,
		RouteToDatabase:   u.Identity.RouteToDatabase,
		MFAVerified:       u.Identity.MFAVerified,
		ClientIP:          u.Identity.ClientIP,
	}

	return &Context{
		User:             user,
		Checker:          RemoteUserRoleSet{checker},
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
	roles, err := services.FromSpec(
		string(types.RoleRemoteProxy),
		types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces: []string{types.Wildcard},
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
	user.SetRoles([]string{string(types.RoleRemoteProxy)})
	return &Context{
		User:             user,
		Checker:          RemoteBuiltinRoleSet{roles},
		Identity:         r,
		UnmappedIdentity: r,
	}, nil
}

// GetCheckerForBuiltinRole returns checkers for embedded builtin role
func GetCheckerForBuiltinRole(clusterName string, recConfig types.SessionRecordingConfig, role types.SystemRole) (services.RoleSet, error) {
	switch role {
	case types.RoleAuth:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindAuthServer, services.RW()),
					},
				},
			})
	case types.RoleProvisionToken:
		return services.FromSpec(role.String(), types.RoleSpecV4{})
	case types.RoleNode:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindNode, services.RW()),
						types.NewRule(types.KindSSHSession, services.RW()),
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
					},
				},
			})
	case types.RoleApp:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
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
						types.NewRule(types.KindApp, services.RW()),
						types.NewRule(types.KindWebSession, services.RO()),
						types.NewRule(types.KindWebToken, services.RO()),
						types.NewRule(types.KindJWT, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
					},
				},
			})
	case types.RoleDatabase:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
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
						types.NewRule(types.KindDatabase, services.RW()),
						types.NewRule(types.KindSemaphore, services.RW()),
						types.NewRule(types.KindLock, services.RO()),
					},
				},
			})
	case types.RoleProxy:
		// if in recording mode, return a different set of permissions than regular
		// mode. recording proxy needs to be able to generate host certificates.
		if services.IsRecordAtProxy(recConfig.GetMode()) {
			return services.FromSpec(
				role.String(),
				types.RoleSpecV4{
					Allow: types.RoleConditions{
						Namespaces:    []string{types.Wildcard},
						ClusterLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
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
							types.NewRule(types.KindHostCert, services.RW()),
							types.NewRule(types.KindRemoteCluster, services.RO()),
							types.NewRule(types.KindSemaphore, services.RW()),
							types.NewRule(types.KindAppServer, services.RO()),
							types.NewRule(types.KindWebSession, services.RW()),
							types.NewRule(types.KindWebToken, services.RW()),
							types.NewRule(types.KindKubeService, services.RW()),
							types.NewRule(types.KindDatabaseServer, services.RO()),
							types.NewRule(types.KindLock, services.RO()),
							types.NewRule(types.KindWindowsDesktopService, services.RO()),
							types.NewRule(types.KindWindowsDesktop, services.RO()),
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
				})
		}
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces:    []string{types.Wildcard},
					ClusterLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
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
						types.NewRule(types.KindDatabaseServer, services.RO()),
						types.NewRule(types.KindLock, services.RO()),
						types.NewRule(types.KindWindowsDesktopService, services.RO()),
						types.NewRule(types.KindWindowsDesktop, services.RO()),
						// this rule allows local proxy to update the remote cluster's host certificate authorities
						// during certificates renewal
						{
							Resources: []string{types.KindCertAuthority},
							Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbUpdate},
							// allow administrative access to the certificate authority names
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
			})
	case types.RoleSignup:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindAuthServer, services.RO()),
						types.NewRule(types.KindClusterAuthPreference, services.RO()),
					},
				},
			})
	case types.RoleAdmin:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Options: types.RoleOptions{
					MaxSessionTTL: types.MaxDuration(),
				},
				Allow: types.RoleConditions{
					Namespaces:           []string{types.Wildcard},
					Logins:               []string{},
					NodeLabels:           types.Labels{types.Wildcard: []string{types.Wildcard}},
					ClusterLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
					WindowsDesktopLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Rules: []types.Rule{
						types.NewRule(types.Wildcard, services.RW()),
					},
				},
			})
	case types.RoleNop:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces: []string{},
					Rules:      []types.Rule{},
				},
			})
	case types.RoleKube:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
				Allow: types.RoleConditions{
					Namespaces: []string{types.Wildcard},
					Rules: []types.Rule{
						types.NewRule(types.KindKubeService, services.RW()),
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
					},
				},
			})
	case types.RoleWindowsDesktop:
		return services.FromSpec(
			role.String(),
			types.RoleSpecV4{
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
	}

	return nil, trace.NotFound("%q is not recognized", role.String())
}

func contextForBuiltinRole(r BuiltinRole, recConfig types.SessionRecordingConfig) (*Context, error) {
	checker, err := GetCheckerForBuiltinRole(r.ClusterName, recConfig, r.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(r.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles([]string{string(r.Role)})
	return &Context{
		User:             user,
		Checker:          BuiltinRoleSet{checker},
		Identity:         r,
		UnmappedIdentity: r,
	}, nil
}

func contextForLocalUser(u LocalUser, accessPoint AuthorizerAccessPoint) (*Context, error) {
	// User has to be fetched to check if it's a blocked username
	user, err := accessPoint.GetUser(u.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles, traits, err := services.ExtractFromIdentity(accessPoint, u.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(roles, accessPoint, traits)
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
	user.SetRoles(roles)
	user.SetTraits(traits)

	return &Context{
		User:             user,
		Checker:          LocalUserRoleSet{checker},
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
	// Role is the builtin role this username is associated with
	Role types.SystemRole

	// Username is for authentication tracking purposes
	Username string

	// ClusterName is the name of the local cluster
	ClusterName string

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// IsServer returns true if the role is one of the builtin server roles.
func (r BuiltinRole) IsServer() bool {
	return r.Role == types.RoleProxy ||
		r.Role == types.RoleNode ||
		r.Role == types.RoleAuth ||
		r.Role == types.RoleApp ||
		r.Role == types.RoleKube ||
		r.Role == types.RoleDatabase ||
		r.Role == types.RoleWindowsDesktop
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

// BuiltinRoleSet wraps a services.RoleSet. The type is used to determine if
// the role is builtin or not.
type BuiltinRoleSet struct {
	services.RoleSet
}

// RemoteBuiltinRoleSet wraps a services.RoleSet. The type is used to determine if
// the role is a remote builtin or not.
type RemoteBuiltinRoleSet struct {
	services.RoleSet
}

// LocalUserRoleSet wraps a services.RoleSet. This type is used to determine
// if the role is a local user or not.
type LocalUserRoleSet struct {
	services.RoleSet
}

// RemoteUserRoleSet wraps a services.RoleSet. This type is used to determine
// if the role is a remote user or not.
type RemoteUserRoleSet struct {
	services.RoleSet
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
