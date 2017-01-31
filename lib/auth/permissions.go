/*
Copyright 2015 Gravitational, Inc.

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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// NewAccessChecker returns new access checker that's using roles and users
func NewAccessChecker(access services.Access, identity services.Identity, trust services.Trust) (NewChecker, error) {
	if access == nil {
		return nil, trace.BadParameter("missing parameter access")
	}
	if identity == nil {
		return nil, trace.BadParameter("missing parameter identity")
	}
	return &AccessCheckers{Access: access, Identity: identity, Trust: trust}, nil
}

// Authorizer authorizes identity and returns auth context
type Authorizer interface {
	// Authorize authorizes user based on identity supplied via context
	Authorize(ctx context.Context) (*AuthContext, error)
}

// AccessCheckers creates new checkers using Access services
type AccessCheckers struct {
	Access   services.Access
	Identity services.Identity
	Trust    services.Trust
}

// AuthzContext is authorization context
type AuthContext struct {
	// Username is the user name
	Username string
	// Checker is access checker
	Checker services.AccessChekcer
}

// Authorize authorizes user based on identity supplied via context
func (a *AccessCheckers) Authorize(ctx context.Context) (*AuthContext, error) {
	if ctx == nil {
		return nil, trace.AccessDenied("missing authentication context")
	}
	userI := ctx.GetValue(teleport.ContextUser)
	switch user := userI.(type) {
	case teleport.LocalUser:
		return a.authorizeForLocalUser(user)
	case teleport.RemoteUser:
		return a.authorizeRemoteUser(user)
	case teleport.BuiltinRole:
		return a.authorizeBuiltinRole(user)
	default:
		return nil, trace.AccessDenied("unsupported context type")
	}
}

// authorizeLocalUser returns authz context based on the username
func (a *AccessCheckers) authorizeLocalUser(u teleport.LocalUser) (*AuthContext, error) {
	user, err := a.Identity.GetUser(u.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(user.GetRoles(), a.Access)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthContext{Username: username, Checker: checker}, nil
}

// authorizeRemoteUser returns checker based on cert authority roles
func (a *AccessCheckers) authorizeRemoteUser(u teleport.RemoteUser) (*AuthContext, error) {
	ca, err := a.Trust.GetCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: u.ClusterName}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.FetchRoles(ca.GetRoles(), a.Access)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthContext{
		// this is done on purpose to make sure user does not match some real local user
		Username: fmt.Sprintf("remote user %v from %v", u.Username, u.ClusterName),
		Checker:  checker,
	}, nil
}

// authorizeBuiltinRole authorizes builtin role
func (a *AccessCheckers) authorizeRemoteUser(r teleport.BuiltinRole) (*AuthContext, error) {
	checker, err := GetCheckerForBuiltinRole(r.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthContext{
		// this is done on purpose to make sure user does not match some real local user
		Username: fmt.Sprintf("user from builtin role %v", r.Role),
		Checker:  checker,
	}, nil
}

// GetCheckerForBuiltinRole returns checkers for embedded builtin role
func GetCheckerForBuiltinRole(role teleport.Role) (*AuthContext, error) {
	switch role {
	case teleport.RoleAuth:
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{services.Wildcard},
				Resources: map[string][]string{
					services.KindAuthServer: services.RW()},
			})
	case teleport.RoleProvisionToken:
		return services.FromSpec(username, services.RoleSpecV2{})
	case teleport.RoleNode:
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{services.Wildcard},
				Resources: map[string][]string{
					services.KindNode:          services.RW(),
					services.KindSession:       services.RW(),
					services.KindEvent:         services.RW(),
					services.KindProxy:         services.RO(),
					services.KindCertAuthority: services.RO(),
					services.KindUser:          services.RO(),
					services.KindNamespace:     services.RO(),
					services.KindRole:          services.RO(),
					services.KindAuthServer:    services.RO(),
				},
			})
	case teleport.RoleProxy:
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{services.Wildcard},
				Resources: map[string][]string{
					services.KindProxy:         services.RW(),
					services.KindOIDCRequest:   services.RW(),
					services.KindOIDC:          services.RO(),
					services.KindNamespace:     services.RO(),
					services.KindEvent:         services.RW(),
					services.KindSession:       services.RW(),
					services.KindNode:          services.RO(),
					services.KindAuthServer:    services.RO(),
					services.KindReverseTunnel: services.RO(),
					services.KindCertAuthority: services.RO(),
					services.KindUser:          services.RO(),
					services.KindRole:          services.RO(),
				},
			})
	case teleport.RoleWeb:
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{services.Wildcard},
				Resources: map[string][]string{
					services.KindWebSession: services.RW(),
					services.KindSession:    services.RW(),
					services.KindAuthServer: services.RO(),
					services.KindUser:       services.RO(),
					services.KindRole:       services.RO(),
					services.KindNamespace:  services.RO(),
				},
			})
	case teleport.RoleSignup:
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{services.Wildcard},
				Resources: map[string][]string{
					services.KindAuthServer: services.RO(),
				},
			})
	case teleport.RoleAdmin:
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				MaxSessionTTL: services.MaxDuration(),
				Logins:        []string{},
				Namespaces:    []string{services.Wildcard},
				NodeLabels:    map[string]string{services.Wildcard: services.Wildcard},
				Resources: map[string][]string{
					services.Wildcard: services.RW(),
				},
			})
	case teleport.RoleNop:
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{},
				Resources:  map[string][]string{},
			})
	}

	return nil, trace.NotFound("%v is not reconginzed", username)
}
