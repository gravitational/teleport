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

package teleport

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// Role identifies the role of an SSH connection. Unlike "user roles"
// introduced as part of RBAC in Teleport 1.4+ these are built-in roles used
// for different Teleport components when connecting to each other.
type Role string
type Roles []Role

const (
	// RoleAuth is for teleport auth server (authority, authentication and authorization)
	RoleAuth Role = "Auth"
	// RoleWeb is for web access users
	RoleWeb Role = "Web"
	// RoleNode is a role for SSH node in the cluster
	RoleNode Role = "Node"
	// RoleProxy is a role for SSH proxy in the cluster
	RoleProxy Role = "Proxy"
	// RoleAdmin is admin role
	RoleAdmin Role = "Admin"
	// RoleProvisionToken is a role for nodes authenticated using provisioning tokens
	RoleProvisionToken Role = "ProvisionToken"
	// RoleTrustedCluster is a role needed for tokens used to add trusted clusters.
	RoleTrustedCluster Role = "Trusted_cluster"
	// RoleSignup is for first time signing up users
	RoleSignup Role = "Signup"
	// RoleNop is used for actions that already using external authz mechanisms
	// e.g. tokens or passwords
	RoleNop Role = "Nop"
)

// this constant exists for backwards compatibility reasons, needed to upgrade to 2.3
const LegacyClusterTokenType Role = "Trustedcluster"

// ParseRoles takes a comma-separated list of roles and returns a slice
// of roles, or an error if parsing failed
func ParseRoles(str string) (roles Roles, err error) {
	for _, s := range strings.Split(str, ",") {
		r := Role(strings.Title(strings.ToLower(strings.TrimSpace(s))))
		if err = r.Check(); err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, r)
	}
	return roles, nil
}

// Includes returns 'true' if a given list of roles includes a given role
func (roles Roles) Include(role Role) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// Equals compares two sets of roles
func (roles Roles) Equals(other Roles) bool {
	if len(roles) != len(other) {
		return false
	}
	for _, r := range roles {
		if !other.Include(r) {
			return false
		}
	}
	return true
}

// Check returns an error if the role set is incorrect (contains unknown roles)
func (roles Roles) Check() (err error) {
	for _, role := range roles {
		if err = role.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (roles Roles) String() string {
	s := make([]string, 0)
	for _, r := range roles {
		s = append(s, string(r))
	}
	return strings.Join(s, ",")
}

// Set sets the value of the role from string, used to integrate with CLI tools
func (r *Role) Set(v string) error {
	val := Role(strings.Title(v))
	if err := val.Check(); err != nil {
		return trace.Wrap(err)
	}
	*r = val
	return nil
}

// String returns debug-friendly representation of this role
func (r *Role) String() string {
	return fmt.Sprintf("%v", strings.ToUpper(string(*r)))
}

// Check checks if this a a valid role value, returns nil
// if it's ok, false otherwise
func (r *Role) Check() error {
	switch *r {
	case RoleAuth, RoleWeb, RoleNode,
		RoleAdmin, RoleProvisionToken,
		RoleTrustedCluster, LegacyClusterTokenType,
		RoleSignup, RoleProxy, RoleNop:
		return nil
	}
	return trace.BadParameter("role %v is not registered", *r)
}

// ContextUser is a user set in the context of the request
const ContextUser = "teleport-user"

// LocalUsername is a local username
type LocalUser struct {
	// Username is local username
	Username string
}

// BuiltinRole is monitoring
type BuiltinRole struct {
	// Role is the builtin role this username is associated with
	Role Role
}

// RemoteUser defines encoded remote user
type RemoteUser struct {
	// Username is a name of the remote user
	Username string `json:"username"`
	// ClusterName is a name of the remote cluster
	// of the user
	ClusterName string `json:"cluster_name"`
	// RemoteRoles is optional list of remote roles
	RemoteRoles []string `json:"remote_roles"`
}
