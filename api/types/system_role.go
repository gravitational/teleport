/*
Copyright 2021 Gravitational, Inc.

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

// SystemRole identifies the role of an SSH connection. Unlike "user roles"
// introduced as part of RBAC in Teleport 1.4+ these are built-in roles used
// for different Teleport components when connecting to each other.
type SystemRole string

// SystemRoles is a TeleportRole list
type SystemRoles []SystemRole

const (
	// RoleAuth is for teleport auth server (authority, authentication and authorization)
	RoleAuth SystemRole = "Auth"
	// RoleNode is a role for SSH node in the cluster
	RoleNode SystemRole = "Node"
	// RoleProxy is a role for SSH proxy in the cluster
	RoleProxy SystemRole = "Proxy"
	// RoleAdmin is admin role
	RoleAdmin SystemRole = "Admin"
	// RoleProvisionToken is a role for nodes authenticated using provisioning tokens
	RoleProvisionToken SystemRole = "ProvisionToken"
	// RoleTrustedCluster is a role needed for tokens used to add trusted clusters.
	RoleTrustedCluster SystemRole = "Trusted_cluster"
	// RoleSignup is for first time signing up users
	RoleSignup SystemRole = "Signup"
	// RoleNop is used for actions that are already using external authz mechanisms
	// e.g. tokens or passwords
	RoleNop SystemRole = "Nop"
	// RoleRemoteProxy is a role for remote SSH proxy in the cluster
	RoleRemoteProxy SystemRole = "RemoteProxy"
	// RoleKube is a role for a kubernetes service.
	RoleKube SystemRole = "Kube"
	// RoleApp is a role for a app proxy in the cluster.
	RoleApp SystemRole = "App"
	// RoleDatabase is a role for a database proxy in the cluster.
	RoleDatabase SystemRole = "Db"
)

// LegacyClusterTokenType exists for backwards compatibility reasons, needed to upgrade to 2.3
const LegacyClusterTokenType SystemRole = "Trustedcluster"

// NewTeleportRoles return a list of teleport roles from slice of strings
func NewTeleportRoles(in []string) (SystemRoles, error) {
	var roles SystemRoles
	for _, val := range in {
		roles = append(roles, SystemRole(val))
	}
	return roles, roles.Check()
}

// ParseTeleportRoles takes a comma-separated list of roles and returns a slice
// of teleport roles, or an error if parsing failed
func ParseTeleportRoles(str string) (SystemRoles, error) {
	var roles SystemRoles
	for _, s := range strings.Split(str, ",") {
		r := SystemRole(strings.Title(strings.ToLower(strings.TrimSpace(s))))
		roles = append(roles, r)
	}
	return roles, roles.Check()
}

// Include returns 'true' if a given list of teleport roles includes a given role
func (roles SystemRoles) Include(role SystemRole) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// StringSlice returns teleport roles as string slice
func (roles SystemRoles) StringSlice() []string {
	s := make([]string, 0)
	for _, r := range roles {
		s = append(s, r.String())
	}
	return s
}

// asSet returns teleport roles as set (map).
func (roles SystemRoles) asSet() map[SystemRole]struct{} {
	s := make(map[SystemRole]struct{}, len(roles))
	for _, r := range roles {
		s[r] = struct{}{}
	}
	return s
}

// Equals compares two sets of teleport roles
func (roles SystemRoles) Equals(other SystemRoles) bool {
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

// Check returns an error if the teleport role set is incorrect (contains unknown roles)
func (roles SystemRoles) Check() error {
	seen := make(map[SystemRole]struct{})
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

// String returns comma separated string with teleport roles
func (roles SystemRoles) String() string {
	return strings.Join(roles.StringSlice(), ",")
}

// Set sets the value of the teleport role from string, used to integrate with CLI tools
func (r *SystemRole) Set(v string) error {
	val := SystemRole(strings.Title(v))
	if err := val.Check(); err != nil {
		return trace.Wrap(err)
	}
	*r = val
	return nil
}

// String returns debug-friendly representation of this teleport role.
func (r *SystemRole) String() string {
	switch *r {
	case RoleSignup:
		return "Password"
	case RoleTrustedCluster, LegacyClusterTokenType:
		return "trusted_cluster"
	default:
		return fmt.Sprintf("%v", string(*r))
	}
}

// Check checks if this a a valid teleport role value, returns nil
// if it's ok, false otherwise
func (r *SystemRole) Check() error {
	switch *r {
	case RoleAuth, RoleNode, RoleApp, RoleDatabase,
		RoleAdmin, RoleProvisionToken,
		RoleTrustedCluster, LegacyClusterTokenType,
		RoleSignup, RoleProxy, RoleNop, RoleKube:
		return nil
	}
	return trace.BadParameter("role %v is not registered", *r)
}
