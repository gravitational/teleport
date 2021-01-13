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

package teleport

import "github.com/gravitational/teleport/api/types"

// The following types, functions, and constants have been moved to /api/types/roles.go,
// and are now imported here for backwards compatibility. DELETE IN 7.0.0

// Role identifies the role of an SSH connection. Unlike "user roles"
// introduced as part of RBAC in Teleport 1.4+ these are built-in roles used
// for different Teleport components when connecting to each other.
type Role = types.TeleportRole
type Roles = types.TeleportRoles

var (
	RoleAuth           = types.RoleAuth
	RoleWeb            = types.RoleWeb
	RoleNode           = types.RoleNode
	RoleProxy          = types.RoleProxy
	RoleAdmin          = types.RoleAdmin
	RoleProvisionToken = types.RoleProvisionToken
	RoleTrustedCluster = types.RoleTrustedCluster
	RoleSignup         = types.RoleSignup
	RoleNop            = types.RoleNop
	RoleRemoteProxy    = types.RoleRemoteProxy
	RoleKube           = types.RoleKube
	RoleApp            = types.RoleApp

	LegacyClusterTokenType = types.LegacyClusterTokenType
	NewRoles               = types.NewRoles
	ParseRoles             = types.ParseRoles
)
