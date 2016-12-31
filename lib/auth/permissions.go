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
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// NewAccessChecker returns new access checker that's using roles and users
func NewAccessChecker(access services.Access, identity services.Identity) (NewChecker, error) {
	if access == nil {
		return nil, trace.BadParameter("missing parameter access")
	}
	if identity == nil {
		return nil, trace.BadParameter("missing parameter identity")
	}
	return (&AccessCheckers{Access: access, Identity: identity}).GetChecker, nil
}

// NewChecker is a function that returns new access checker based on username
type NewChecker func(username string) (services.AccessChecker, error)

// AccessCheckers creates new checkers using Access services
type AccessCheckers struct {
	Access   services.Access
	Identity services.Identity
}

// GetChecker returns access checker based on the username
func (a *AccessCheckers) GetChecker(username string) (services.AccessChecker, error) {
	checker, err := GetCheckerForSystemUsers(username)
	if err == nil {
		return checker, nil
	}
	user, err := a.Identity.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var roles services.RoleSet
	for _, roleName := range user.GetRoles() {
		role, err := a.Access.GetRole(roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// GetCheckerForSystemUsers returns checkers for embedded system users
// hardcoded in the system
func GetCheckerForSystemUsers(username string) (services.AccessChecker, error) {
	switch username {
	case teleport.RoleAuth.User():
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{services.Wildcard},
				Resources: map[string][]string{
					services.KindAuthServer: services.RW()},
			})
	case teleport.RoleProvisionToken.User():
		return services.FromSpec(username, services.RoleSpecV2{})
	case teleport.RoleNode.User():
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
	case teleport.RoleProxy.User():
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
	case teleport.RoleWeb.User():
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
	case teleport.RoleSignup.User():
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{services.Wildcard},
				Resources: map[string][]string{
					services.KindAuthServer: services.RO(),
				},
			})
	case teleport.RoleAdmin.User():
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
	case teleport.RoleNop.User():
		return services.FromSpec(
			username,
			services.RoleSpecV2{
				Namespaces: []string{},
				Resources:  map[string][]string{},
			})
	}

	return nil, trace.NotFound("%v is not reconginzed", username)
}
