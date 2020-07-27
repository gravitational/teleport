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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate/builder"
)

// NewAdminContext returns new admin auth context
func NewAdminContext() (*AuthContext, error) {
	authContext, err := contextForBuiltinRole("", nil, teleport.RoleAdmin, fmt.Sprintf("%v", teleport.RoleAdmin))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return authContext, nil
}

// NewAuthorizer returns new authorizer using backends
func NewAuthorizer(access services.Access, identity services.UserGetter, trust services.Trust) (Authorizer, error) {
	if access == nil {
		return nil, trace.BadParameter("missing parameter access")
	}
	if identity == nil {
		return nil, trace.BadParameter("missing parameter identity")
	}
	if trust == nil {
		return nil, trace.BadParameter("missing parameter trust")
	}
	return &authorizer{access: access, identity: identity, trust: trust}, nil
}

// Authorizer authorizes identity and returns auth context
type Authorizer interface {
	// Authorize authorizes user based on identity supplied via context
	Authorize(ctx context.Context) (*AuthContext, error)
}

// authorizer creates new local authorizer
type authorizer struct {
	access   services.Access
	identity services.UserGetter
	trust    services.Trust
}

// AuthContext is authorization context
type AuthContext struct {
	// User is the user name
	User services.User
	// Checker is access checker
	Checker services.AccessChecker
	// Identity is x509 derived identity
	Identity tlsca.Identity
}

// Authorize authorizes user based on identity supplied via context
func (a *authorizer) Authorize(ctx context.Context) (*AuthContext, error) {
	if ctx == nil {
		return nil, trace.AccessDenied("missing authentication context")
	}
	userI := ctx.Value(ContextUser)
	userWithIdentity, ok := userI.(IdentityGetter)
	if !ok {
		return nil, trace.AccessDenied("unsupported context type %T", userI)
	}
	identity := userWithIdentity.GetIdentity()
	authContext, err := a.fromUser(userI)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authContext.Identity = identity
	return authContext, nil
}

func (a *authorizer) fromUser(userI interface{}) (*AuthContext, error) {
	switch user := userI.(type) {
	case LocalUser:
		return a.authorizeLocalUser(user)
	case RemoteUser:
		return a.authorizeRemoteUser(user)
	case BuiltinRole:
		return a.authorizeBuiltinRole(user)
	case RemoteBuiltinRole:
		return a.authorizeRemoteBuiltinRole(user)
	default:
		return nil, trace.AccessDenied("unsupported context type %T", userI)
	}
}

// authorizeLocalUser returns authz context based on the username
func (a *authorizer) authorizeLocalUser(u LocalUser) (*AuthContext, error) {
	return contextForLocalUser(u, a.identity, a.access)
}

// authorizeRemoteUser returns checker based on cert authority roles
func (a *authorizer) authorizeRemoteUser(u RemoteUser) (*AuthContext, error) {
	ca, err := a.trust.GetCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: u.ClusterName}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleNames, err := ca.CombinedMapping().Map(u.RemoteRoles)
	if err != nil {
		return nil, trace.AccessDenied("failed to map roles for remote user %q from cluster %q", u.Username, u.ClusterName)
	}
	if len(roleNames) == 0 {
		return nil, trace.AccessDenied("no roles mapped for remote user %q from cluster %q", u.Username, u.ClusterName)
	}
	// Set "logins" trait and "kubernetes_groups" for the remote user. This allows Teleport to work by
	// passing exact logins, kubernetes groups and users to the remote cluster. Note that claims (OIDC/SAML)
	// are not passed, but rather the exact logins, this is done to prevent
	// leaking too much of identity to the remote cluster, and instead of focus
	// on main cluster's interpretation of this identity
	traits := map[string][]string{
		teleport.TraitLogins:     u.Principals,
		teleport.TraitKubeGroups: u.KubernetesGroups,
		teleport.TraitKubeUsers:  u.KubernetesUsers,
	}
	log.Debugf("Mapped roles %v of remote user %q to local roles %v and traits %v.",
		u.RemoteRoles, u.Username, roleNames, traits)
	checker, err := services.FetchRoles(roleNames, a.access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// The user is prefixed with "remote-" and suffixed with cluster name with
	// the hope that it does not match a real local user.
	user, err := services.NewUser(fmt.Sprintf("remote-%v-%v", u.Username, u.ClusterName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetTraits(traits)

	// Set the list of roles this user has in the remote cluster.
	user.SetRoles(roleNames)

	return &AuthContext{
		User:    user,
		Checker: RemoteUserRoleSet{checker},
	}, nil
}

// authorizeBuiltinRole authorizes builtin role
func (a *authorizer) authorizeBuiltinRole(r BuiltinRole) (*AuthContext, error) {
	config, err := r.GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return contextForBuiltinRole(r.ClusterName, config, r.Role, r.Username)
}

func (a *authorizer) authorizeRemoteBuiltinRole(r RemoteBuiltinRole) (*AuthContext, error) {
	if r.Role != teleport.RoleProxy {
		return nil, trace.AccessDenied("access denied for remote %v connecting to cluster", r.Role)
	}
	roles, err := services.FromSpec(
		string(teleport.RoleRemoteProxy),
		services.RoleSpecV3{
			Allow: services.RoleConditions{
				Namespaces: []string{services.Wildcard},
				Rules: []services.Rule{
					services.NewRule(services.KindNode, services.RO()),
					services.NewRule(services.KindProxy, services.RO()),
					services.NewRule(services.KindCertAuthority, services.ReadNoSecrets()),
					services.NewRule(services.KindNamespace, services.RO()),
					services.NewRule(services.KindUser, services.RO()),
					services.NewRule(services.KindRole, services.RO()),
					services.NewRule(services.KindAuthServer, services.RO()),
					services.NewRule(services.KindReverseTunnel, services.RO()),
					services.NewRule(services.KindTunnelConnection, services.RO()),
					services.NewRule(services.KindClusterConfig, services.RO()),
					// this rule allows remote proxy to update the cluster's certificate authorities
					// during certificates renewal
					{
						Resources: []string{services.KindCertAuthority},
						// It is important that remote proxy can only rotate
						// existing certificate authority, and not create or update new ones
						Verbs: []string{services.VerbRead, services.VerbRotate},
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
	user, err := services.NewUser(r.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles([]string{string(teleport.RoleRemoteProxy)})
	return &AuthContext{
		User:    user,
		Checker: RemoteBuiltinRoleSet{roles},
	}, nil
}

// GetCheckerForBuiltinRole returns checkers for embedded builtin role
func GetCheckerForBuiltinRole(clusterName string, clusterConfig services.ClusterConfig, role teleport.Role) (services.RoleSet, error) {
	switch role {
	case teleport.RoleAuth:
		return services.FromSpec(
			role.String(),
			services.RoleSpecV3{
				Allow: services.RoleConditions{
					Namespaces: []string{services.Wildcard},
					Rules: []services.Rule{
						services.NewRule(services.KindAuthServer, services.RW()),
					},
				},
			})
	case teleport.RoleProvisionToken:
		return services.FromSpec(role.String(), services.RoleSpecV3{})
	case teleport.RoleNode:
		return services.FromSpec(
			role.String(),
			services.RoleSpecV3{
				Allow: services.RoleConditions{
					Namespaces: []string{services.Wildcard},
					Rules: []services.Rule{
						services.NewRule(services.KindNode, services.RW()),
						services.NewRule(services.KindSSHSession, services.RW()),
						services.NewRule(services.KindEvent, services.RW()),
						services.NewRule(services.KindProxy, services.RO()),
						services.NewRule(services.KindCertAuthority, services.ReadNoSecrets()),
						services.NewRule(services.KindUser, services.RO()),
						services.NewRule(services.KindNamespace, services.RO()),
						services.NewRule(services.KindRole, services.RO()),
						services.NewRule(services.KindAuthServer, services.RO()),
						services.NewRule(services.KindReverseTunnel, services.RW()),
						services.NewRule(services.KindTunnelConnection, services.RO()),
						services.NewRule(services.KindClusterConfig, services.RO()),
					},
				},
			})
	case teleport.RoleProxy:
		// if in recording mode, return a different set of permissions than regular
		// mode. recording proxy needs to be able to generate host certificates.
		if clusterConfig.GetSessionRecording() == services.RecordAtProxy {
			return services.FromSpec(
				role.String(),
				services.RoleSpecV3{
					Allow: services.RoleConditions{
						Namespaces: []string{services.Wildcard},
						Rules: []services.Rule{
							services.NewRule(services.KindProxy, services.RW()),
							services.NewRule(services.KindOIDCRequest, services.RW()),
							services.NewRule(services.KindSSHSession, services.RW()),
							services.NewRule(services.KindSession, services.RO()),
							services.NewRule(services.KindEvent, services.RW()),
							services.NewRule(services.KindSAMLRequest, services.RW()),
							services.NewRule(services.KindOIDC, services.ReadNoSecrets()),
							services.NewRule(services.KindSAML, services.ReadNoSecrets()),
							services.NewRule(services.KindGithub, services.ReadNoSecrets()),
							services.NewRule(services.KindGithubRequest, services.RW()),
							services.NewRule(services.KindNamespace, services.RO()),
							services.NewRule(services.KindNode, services.RO()),
							services.NewRule(services.KindAuthServer, services.RO()),
							services.NewRule(services.KindReverseTunnel, services.RO()),
							services.NewRule(services.KindCertAuthority, services.ReadNoSecrets()),
							services.NewRule(services.KindUser, services.RO()),
							services.NewRule(services.KindRole, services.RO()),
							services.NewRule(services.KindClusterAuthPreference, services.RO()),
							services.NewRule(services.KindClusterConfig, services.RO()),
							services.NewRule(services.KindClusterName, services.RO()),
							services.NewRule(services.KindStaticTokens, services.RO()),
							services.NewRule(services.KindTunnelConnection, services.RW()),
							services.NewRule(services.KindHostCert, services.RW()),
							services.NewRule(services.KindRemoteCluster, services.RO()),
							// this rule allows local proxy to update the remote cluster's host certificate authorities
							// during certificates renewal
							{
								Resources: []string{services.KindCertAuthority},
								Verbs:     []string{services.VerbCreate, services.VerbRead, services.VerbUpdate},
								// allow administrative access to the host certificate authorities
								// matching any cluster name except local
								Where: builder.And(
									builder.Equals(services.CertAuthorityTypeExpr, builder.String(string(services.HostCA))),
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
			services.RoleSpecV3{
				Allow: services.RoleConditions{
					Namespaces: []string{services.Wildcard},
					Rules: []services.Rule{
						services.NewRule(services.KindProxy, services.RW()),
						services.NewRule(services.KindOIDCRequest, services.RW()),
						services.NewRule(services.KindSSHSession, services.RW()),
						services.NewRule(services.KindSession, services.RO()),
						services.NewRule(services.KindEvent, services.RW()),
						services.NewRule(services.KindSAMLRequest, services.RW()),
						services.NewRule(services.KindOIDC, services.ReadNoSecrets()),
						services.NewRule(services.KindSAML, services.ReadNoSecrets()),
						services.NewRule(services.KindGithub, services.ReadNoSecrets()),
						services.NewRule(services.KindGithubRequest, services.RW()),
						services.NewRule(services.KindNamespace, services.RO()),
						services.NewRule(services.KindNode, services.RO()),
						services.NewRule(services.KindAuthServer, services.RO()),
						services.NewRule(services.KindReverseTunnel, services.RO()),
						services.NewRule(services.KindCertAuthority, services.ReadNoSecrets()),
						services.NewRule(services.KindUser, services.RO()),
						services.NewRule(services.KindRole, services.RO()),
						services.NewRule(services.KindClusterAuthPreference, services.RO()),
						services.NewRule(services.KindClusterConfig, services.RO()),
						services.NewRule(services.KindClusterName, services.RO()),
						services.NewRule(services.KindStaticTokens, services.RO()),
						services.NewRule(services.KindTunnelConnection, services.RW()),
						services.NewRule(services.KindRemoteCluster, services.RO()),
						// this rule allows local proxy to update the remote cluster's host certificate authorities
						// during certificates renewal
						{
							Resources: []string{services.KindCertAuthority},
							Verbs:     []string{services.VerbCreate, services.VerbRead, services.VerbUpdate},
							// allow administrative access to the certificate authority names
							// matching any cluster name except local
							Where: builder.And(
								builder.Equals(services.CertAuthorityTypeExpr, builder.String(string(services.HostCA))),
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
	case teleport.RoleWeb:
		return services.FromSpec(
			role.String(),
			services.RoleSpecV3{
				Allow: services.RoleConditions{
					Namespaces: []string{services.Wildcard},
					Rules: []services.Rule{
						services.NewRule(services.KindWebSession, services.RW()),
						services.NewRule(services.KindSSHSession, services.RW()),
						services.NewRule(services.KindAuthServer, services.RO()),
						services.NewRule(services.KindUser, services.RO()),
						services.NewRule(services.KindRole, services.RO()),
						services.NewRule(services.KindNamespace, services.RO()),
						services.NewRule(services.KindTrustedCluster, services.RO()),
					},
				},
			})
	case teleport.RoleSignup:
		return services.FromSpec(
			role.String(),
			services.RoleSpecV3{
				Allow: services.RoleConditions{
					Namespaces: []string{services.Wildcard},
					Rules: []services.Rule{
						services.NewRule(services.KindAuthServer, services.RO()),
						services.NewRule(services.KindClusterAuthPreference, services.RO()),
					},
				},
			})
	case teleport.RoleAdmin:
		return services.FromSpec(
			role.String(),
			services.RoleSpecV3{
				Options: services.RoleOptions{
					MaxSessionTTL: services.MaxDuration(),
				},
				Allow: services.RoleConditions{
					Namespaces: []string{services.Wildcard},
					Logins:     []string{},
					NodeLabels: services.Labels{services.Wildcard: []string{services.Wildcard}},
					Rules: []services.Rule{
						services.NewRule(services.Wildcard, services.RW()),
					},
				},
			})
	case teleport.RoleNop:
		return services.FromSpec(
			role.String(),
			services.RoleSpecV3{
				Allow: services.RoleConditions{
					Namespaces: []string{},
					Rules:      []services.Rule{},
				},
			})
	}

	return nil, trace.NotFound("%v is not reconginzed", role.String())
}

func contextForBuiltinRole(clusterName string, clusterConfig services.ClusterConfig, r teleport.Role, username string) (*AuthContext, error) {
	checker, err := GetCheckerForBuiltinRole(clusterName, clusterConfig, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles([]string{string(r)})
	return &AuthContext{
		User:    user,
		Checker: BuiltinRoleSet{checker},
	}, nil
}

func contextForLocalUser(u LocalUser, identity services.UserGetter, access services.Access) (*AuthContext, error) {
	// User has to be fetched to check if it's a blocked username
	user, err := identity.GetUser(u.Username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles, traits, err := services.ExtractFromIdentity(identity, &u.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(roles, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Override roles and traits from the local user based on the identity roles
	// and traits, this is done to prevent potential conflict. Imagine a scenairo
	// when SSO user has left the company, but local user entry remained with old
	// privileged roles. New user with the same name has been onboarded and would
	// have derived the roles from the stale user entry. This code prevents
	// that by extracting up to date identity traits and roles from the user's
	// certificate metadata.
	user.SetRoles(roles)
	user.SetTraits(traits)

	return &AuthContext{
		User:    user,
		Checker: LocalUserRoleSet{checker},
	}, nil
}

type contextUserKey string

// ContextUser is a user set in the context of the request
const ContextUser contextUserKey = "teleport-user"

// clientUsername returns the username of a remote HTTP client making the call.
// If ctx didn't pass through auth middleware or did not come from an HTTP
// request, teleport.UserSystem is returned.
func clientUsername(ctx context.Context) string {
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

// IdentityGetter returns client identity
type IdentityGetter interface {
	// GetIdentity  returns x509-derived identity of the user
	GetIdentity() tlsca.Identity
}

// BuiltinRole is the role of the Teleport service.
type BuiltinRole struct {
	// GetClusterConfig fetches cluster configuration.
	GetClusterConfig GetClusterConfigFunc

	// Role is the builtin role this username is associated with
	Role teleport.Role

	// Username is for authentication tracking purposes
	Username string

	// ClusterName is the name of the local cluster
	ClusterName string

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
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
	Role teleport.Role

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

	// Identity is source x509 used to build this role
	Identity tlsca.Identity
}

// GetIdentity returns client identity
func (r RemoteUser) GetIdentity() tlsca.Identity {
	return r.Identity
}

// GetClusterConfigFunc returns a cached services.ClusterConfig.
type GetClusterConfigFunc func(opts ...services.MarshalOption) (services.ClusterConfig, error)
