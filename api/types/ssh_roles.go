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
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// SSHRole identifies the role of an SSH connection. Unlike "user roles"
// introduced as part of RBAC in Teleport 1.4+ these are built-in roles used
// for different Teleport components when connecting to each other.
type SSHRole string
type SSHRoles []SSHRole

const (
	// RoleAuth is for teleport auth server (authority, authentication and authorization)
	RoleAuth SSHRole = "Auth"
	// RoleWeb is for web access users
	RoleWeb SSHRole = "Web"
	// RoleNode is a role for SSH node in the cluster
	RoleNode SSHRole = "Node"
	// RoleProxy is a role for SSH proxy in the cluster
	RoleProxy SSHRole = "Proxy"
	// RoleAdmin is admin role
	RoleAdmin SSHRole = "Admin"
	// RoleProvisionToken is a role for nodes authenticated using provisioning tokens
	RoleProvisionToken SSHRole = "ProvisionToken"
	// RoleTrustedCluster is a role needed for tokens used to add trusted clusters.
	RoleTrustedCluster SSHRole = "Trusted_cluster"
	// RoleSignup is for first time signing up users
	RoleSignup SSHRole = "Signup"
	// RoleNop is used for actions that already using external authz mechanisms
	// e.g. tokens or passwords
	RoleNop SSHRole = "Nop"
	// RoleRemoteProxy is a role for remote SSH proxy in the cluster
	RoleRemoteProxy SSHRole = "RemoteProxy"
	// RoleKube is a role for a kubernetes service.
	RoleKube SSHRole = "Kube"
	// RoleApp is a role for a app proxy in the cluster.
	RoleApp SSHRole = "App"
)

// this constant exists for backwards compatibility reasons, needed to upgrade to 2.3
const LegacyClusterTokenType SSHRole = "Trustedcluster"

// NewRoles return a list of roles from slice of strings
func NewRoles(in []string) (SSHRoles, error) {
	var roles SSHRoles
	for _, val := range in {
		roles = append(roles, SSHRole(val))
	}
	return roles, roles.Check()
}

// ParseRoles takes a comma-separated list of roles and returns a slice
// of roles, or an error if parsing failed
func ParseRoles(str string) (SSHRoles, error) {
	var roles SSHRoles
	for _, s := range strings.Split(str, ",") {
		r := SSHRole(strings.Title(strings.ToLower(strings.TrimSpace(s))))
		roles = append(roles, r)
	}
	return roles, roles.Check()
}

// Includes returns 'true' if a given list of roles includes a given role
func (roles SSHRoles) Include(role SSHRole) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// Slice returns roles as string slice
func (roles SSHRoles) StringSlice() []string {
	s := make([]string, 0)
	for _, r := range roles {
		s = append(s, r.String())
	}
	return s
}

// asSet returns roles as set (map).
func (roles SSHRoles) asSet() map[SSHRole]struct{} {
	s := make(map[SSHRole]struct{}, len(roles))
	for _, r := range roles {
		s[r] = struct{}{}
	}
	return s
}

// Equals compares two sets of roles
func (roles SSHRoles) Equals(other SSHRoles) bool {
	rs, os := roles.asSet(), other.asSet()
	if len(rs) != len(os) {
		return false
	}
	for r := range rs {
		if _, ok := os[r]; !ok {
			return false
		}
	}
	return true
}

// Check returns an error if the role set is incorrect (contains unknown roles)
func (roles SSHRoles) Check() error {
	seen := make(map[SSHRole]struct{})
	for _, role := range roles {
		if err := role.Check(); err != nil {
			return trace.Wrap(err)
		}
		if _, ok := seen[role]; ok {
			return trace.BadParameter("duplicate role %q", role)
		}
		seen[role] = struct{}{}
	}
	return nil
}

// String returns comma separated string with roles
func (roles SSHRoles) String() string {
	return strings.Join(roles.StringSlice(), ",")
}

// Set sets the value of the role from string, used to integrate with CLI tools
func (r *SSHRole) Set(v string) error {
	val := SSHRole(strings.Title(v))
	if err := val.Check(); err != nil {
		return trace.Wrap(err)
	}
	*r = val
	return nil
}

// String returns debug-friendly representation of this role.
func (r *SSHRole) String() string {
	switch string(*r) {
	case string(RoleSignup):
		return "Password"
	case string(RoleTrustedCluster), string(LegacyClusterTokenType):
		return "trusted_cluster"
	default:
		return fmt.Sprintf("%v", string(*r))
	}
}

// Check checks if this a a valid role value, returns nil
// if it's ok, false otherwise
func (r *SSHRole) Check() error {
	switch *r {
	case RoleAuth, RoleWeb, RoleNode, RoleApp,
		RoleAdmin, RoleProvisionToken,
		RoleTrustedCluster, LegacyClusterTokenType,
		RoleSignup, RoleProxy, RoleNop, RoleKube:
		return nil
	}
	return trace.BadParameter("role %v is not registered", *r)
}
