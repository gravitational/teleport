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
	// RoleWindowsDesktop is a role for a Windows desktop service.
	RoleWindowsDesktop SystemRole = "WindowsDesktop"
	// RoleBot is a role for a bot.
	RoleBot SystemRole = "Bot"
	// RoleInstance is a role implicitly held by teleport servers (i.e. any teleport
	// auth token which grants a server role such as proxy/node/etc also implicitly
	// grants the instance role, and any valid cert that proves that the caller holds
	// a server role also implies that the caller holds the instance role). This role
	// doesn't grant meaningful privileges on its own, but is a useful placeholder in
	// contexts such as multi-role certs where there is no particular system role that
	// is "primary".
	RoleInstance SystemRole = "Instance"
	// RoleDiscovery is a role for discovery nodes in the cluster
	RoleDiscovery SystemRole = "Discovery"
	// RoleOkta is a role for Okta nodes in the cluster
	RoleOkta SystemRole = "Okta"
	// RoleMDM is the role for MDM services in the cluster.
	// An MDM service, like Jamf Service, has the powers to manage the cluster's
	// device inventory.
	// Device Trust requires Teleport Enteprise.
	RoleMDM SystemRole = "MDM"

	// RoleAccessGraphPlugin is a role for Access Graph plugins to access
	// Teleport's internal API and access graph.
	RoleAccessGraphPlugin SystemRole = "AccessGraphPlugin"
)

// roleMappings maps a set of allowed lowercase system role names
// to the proper system role
var roleMappings = map[string]SystemRole{
	"auth":              RoleAuth,
	"node":              RoleNode,
	"proxy":             RoleProxy,
	"admin":             RoleAdmin,
	"provisiontoken":    RoleProvisionToken,
	"trusted_cluster":   RoleTrustedCluster,
	"trustedcluster":    RoleTrustedCluster,
	"signup":            RoleSignup,
	"nop":               RoleNop,
	"remoteproxy":       RoleRemoteProxy,
	"remote_proxy":      RoleRemoteProxy,
	"kube":              RoleKube,
	"app":               RoleApp,
	"db":                RoleDatabase,
	"windowsdesktop":    RoleWindowsDesktop,
	"windows_desktop":   RoleWindowsDesktop,
	"bot":               RoleBot,
	"instance":          RoleInstance,
	"discovery":         RoleDiscovery,
	"okta":              RoleOkta,
	"mdm":               RoleMDM,
	"accessgraphplugin": RoleAccessGraphPlugin,
}

func normalizedSystemRole(s string) SystemRole {
	if role, ok := roleMappings[strings.ToLower(strings.TrimSpace(s))]; ok {
		return role
	}
	return SystemRole(s)
}

func normalizedSystemRoles(s []string) []SystemRole {
	roles := make([]SystemRole, 0, len(s))
	for _, role := range s {
		roles = append(roles, normalizedSystemRole(role))
	}
	return roles
}

// localServiceMappings is the subset of role mappings which happen to be true
// teleport services (e.g. db, kube, etc), excluding those which represent remote
// services (i.e. remoteproxy).
var localServiceMappings = map[SystemRole]struct{}{
	RoleAuth:              {},
	RoleNode:              {},
	RoleProxy:             {},
	RoleKube:              {},
	RoleApp:               {},
	RoleDatabase:          {},
	RoleWindowsDesktop:    {},
	RoleDiscovery:         {},
	RoleOkta:              {},
	RoleMDM:               {},
	RoleAccessGraphPlugin: {},
}

// controlPlaneMapping is the subset of local services which are definitively control plane
// elements.
var controlPlaneMapping = map[SystemRole]struct{}{
	RoleAuth:  {},
	RoleProxy: {},
}

// LocalServiceMappings returns the subset of role mappings which happen
// to be true Teleport services (e.g. db, kube, proxy, etc), excluding
// those which represent remote service (i.e. remoteproxy).
func LocalServiceMappings() SystemRoles {
	var sr SystemRoles
	for k := range localServiceMappings {
		sr = append(sr, k)
	}
	return sr
}

// NewTeleportRoles return a list of teleport roles from slice of strings
func NewTeleportRoles(in []string) (SystemRoles, error) {
	roles := SystemRoles(normalizedSystemRoles(in))
	return roles, roles.Check()
}

// ParseTeleportRoles takes a comma-separated list of roles and returns a slice
// of teleport roles, or an error if parsing failed
func ParseTeleportRoles(str string) (SystemRoles, error) {
	var roles SystemRoles
	for _, s := range strings.Split(str, ",") {
		if r := normalizedSystemRole(s); r.Check() == nil {
			roles = append(roles, r)
			continue
		}
		return nil, trace.BadParameter("invalid role %q", s)
	}
	if len(roles) == 0 {
		return nil, trace.BadParameter("no valid roles in $%q", str)
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

// IncludeAny returns 'true' if a given list of teleport roles includes any of
// the given candidate roles.
func (roles SystemRoles) IncludeAny(candidates ...SystemRole) bool {
	for _, r := range candidates {
		if roles.Include(r) {
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
	if len(v) > 0 {
		v = strings.ToUpper(v[:1]) + v[1:]
	}
	val := SystemRole(v)
	if err := val.Check(); err != nil {
		return trace.Wrap(err)
	}
	*r = val
	return nil
}

// String returns the system role string representation. Returned values must
// match (case-insensitive) the role mappings; otherwise, the validation check
// will fail.
func (r *SystemRole) String() string {
	switch *r {
	case RoleTrustedCluster:
		return "trusted_cluster"
	default:
		return string(*r)
	}
}

// Check checks if this a a valid teleport role value, returns nil
// if it's ok, false otherwise
// Check checks if this a a valid teleport role value, returns nil
// if it's ok, false otherwise
func (r *SystemRole) Check() error {
	sr, ok := roleMappings[strings.ToLower(string(*r))]
	if ok && string(*r) == string(sr) {
		return nil
	}

	return trace.BadParameter("role %v is not registered", *r)
}

// IsLocalService checks if the given system role is a teleport service (e.g. auth),
// as opposed to some non-service role (e.g. admin). Excludes remote services such
// as remoteproxy.
func (r *SystemRole) IsLocalService() bool {
	_, ok := localServiceMappings[*r]
	return ok
}

// IsControlPlane checks if the given system role is a control plane element (i.e. auth/proxy).
func (r *SystemRole) IsControlPlane() bool {
	_, ok := controlPlaneMapping[*r]
	return ok
}
