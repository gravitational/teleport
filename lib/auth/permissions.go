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
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

type PermissionChecker interface {
	HasPermission(role, action string) error
}

type standardPermissions struct {
	userActions          map[string]int
	nodeActions          map[string]int
	adminActions         map[string]int
	reverseTunnelActions map[string]int
}

func NewStandardPermissions() PermissionChecker {
	sp := standardPermissions{}
	sp.userActions = map[string]int{
		ActionSignIn:           1,
		ActionGenerateUserCert: 1,
	}
	return &sp
}

func (sp *standardPermissions) HasPermission(role, action string) error {
	if role == RoleAdmin {
		return nil
	}
	if (role == RoleUser) && (sp.userActions[action] == 1) {
		return nil
	}
	if (role == RoleNode) && (sp.nodeActions[action] == 1) {
		return nil
	}
	if (role == RoleReverseTunnel) && (sp.reverseTunnelActions[action] == 1) {
		return nil
	}
	return trace.Errorf("role '%v' doesn't have permission for action '%v'",
		role, action)
}

type allowAllPermissions struct {
}

func NewAllowAllPermissions() PermissionChecker {
	aap := allowAllPermissions{}
	return &aap
}

func (aap *allowAllPermissions) HasPermission(role, action string) error {
	return nil
}

var StandartRoles = []string{
	RoleUser,
	RoleWeb,
	RoleNode,
	RoleAdmin,
	RoleProvisionToken,
	RoleReverseTunnel,
}

const (
	RoleUser           = "User"
	RoleWeb            = "Web"
	RoleNode           = "Node"
	RoleAdmin          = "Admin"
	RoleProvisionToken = "ProvisionToken"
	RoleReverseTunnel  = "ReverseTunnel"

	ActionGetSessions        = "GetSession"
	ActionGetSession         = "GetSession"
	ActionDeleteSession      = "DeleteSession"
	ActionUpsertSession      = "UpsertSession"
	ActionUpsertParty        = "UpsertParty"
	ActionUpsertRemoteCert   = "UpsertRemoteCert"
	ActionGetRemoteCerts     = "UpsertRemoteCert"
	ActionDeleteRemoteCert   = "GetRemoteCerts"
	ActionGenerateToken      = "GenerateToken"
	ActionLog                = "Log"
	ActionLogEntry           = "LogEntry"
	ActionGetEvents          = "GetEvents"
	ActionGetChunkWriter     = "GetChunkWriter"
	ActionGetChunkReader     = "GetChunkReader"
	ActionUpsertServer       = "UpsertServer"
	ActionGetServers         = "GetServers"
	ActionUpsertWebTun       = "UpsertWebTun"
	ActionGetWebTuns         = "GetWebTuns"
	ActionGetWebTun          = "GetWebTun"
	ActionDeleteWebTun       = "DeleteWebTun"
	ActionUpsertPassword     = "UpsertPassword"
	ActionCheckPassword      = "CheckPassword"
	ActionSignIn             = "SignIn"
	ActionGetWebSession      = "GetWebSession"
	ActionGetWebSessionsKeys = "GetWebSessionsKeys"
	ActionDeleteWebSession   = "DeleteWebSession"
	ActionGetUsers           = "GetUsers"
	ActionDeleteUser         = "DeleteUser"
	ActionUpsertUserKey      = "UpsertUserKey"
	ActionGetUserKeys        = "GetUserKeys"
	ActionDeleteUserKey      = "DeleteUserKey"
	ActionGetHostCAPub       = "GetHostCAPub"
	ActionGetUserCAPub       = "GetUserCAPub"
	ActionGenerateKeyPair    = "GenerateKeyPair"
	ActionGenerateHostCert   = "GenerateHostCert"
	ActionGenerateUserCert   = "GenerateUserCert"
	ActionResetHostCA        = "ResetHostCA"
	ActionResetUserCA        = "ResetUserCA"
	ActionGenerateSealKey    = "GenerateSealKey"
	ActionGetSealKeys        = "GetSeakKeys"
	ActionGetSealKey         = "GetSealKey"
	ActionDeleteSealKey      = "DeleteSealKey"
	ActionAddSealKey         = "AddSealKey"
)
