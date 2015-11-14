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
	permissions map[string](map[string]int)
}

func NewStandardPermissions() PermissionChecker {
	sp := standardPermissions{}
	sp.permissions = make(map[string](map[string]int))

	sp.permissions[RoleUser] = map[string]int{
		ActionSignIn:           1,
		ActionGenerateUserCert: 1,
	}

	sp.permissions[RoleProvisionToken] = map[string]int{
		ActionRegisterUsingToken:    1,
		ActionRegisterNewAuthServer: 1,
	}

	sp.permissions[RoleNode] = map[string]int{
		ActionUpsertServer:   1,
		ActionGetUserCAPub:   1,
		ActionGetRemoteCerts: 1,
		ActionGetUserKeys:    1,
		ActionGetServers:     1,
		ActionGetHostCAPub:   1,
		ActionUpsertParty:    1,
		ActionLogEntry:       1,
		ActionGetChunkWriter: 1,
	}

	sp.permissions[RoleWeb] = map[string]int{
		ActionGetWebSession:    1,
		ActionDeleteWebSession: 1,
	}

	return &sp
}

func (sp *standardPermissions) HasPermission(role, action string) error {
	if role == RoleAdmin {
		return nil
	}
	if permissions, ok := sp.permissions[role]; ok {
		if permissions[action] == 1 {
			return nil
		} else {
			return trace.Errorf("role '%v' doesn't have permission for action '%v'",
				role, action)
		}
	}
	return trace.Errorf("role '%v' is not allowed",
		role)
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

var StandardRoles = []string{
	RoleAuth,
	RoleUser,
	RoleWeb,
	RoleNode,
	RoleAdmin,
	RoleProvisionToken,
}

const (
	RoleAuth           = "Auth"
	RoleUser           = "User"
	RoleWeb            = "Web"
	RoleNode           = "Node"
	RoleAdmin          = "Admin"
	RoleProvisionToken = "ProvisionToken"

	ActionGetSessions           = "GetSession"
	ActionGetSession            = "GetSession"
	ActionDeleteSession         = "DeleteSession"
	ActionUpsertSession         = "UpsertSession"
	ActionUpsertParty           = "UpsertParty"
	ActionUpsertRemoteCert      = "UpsertRemoteCert"
	ActionGetRemoteCerts        = "UpsertRemoteCert"
	ActionDeleteRemoteCert      = "GetRemoteCerts"
	ActionGenerateToken         = "GenerateToken"
	ActionRegisterUsingToken    = "RegisterUsingToken"
	ActionRegisterNewAuthServer = "RegisterNewAuthServer"
	ActionLog                   = "Log"
	ActionLogEntry              = "LogEntry"
	ActionGetEvents             = "GetEvents"
	ActionGetChunkWriter        = "GetChunkWriter"
	ActionGetChunkReader        = "GetChunkReader"
	ActionUpsertServer          = "UpsertServer"
	ActionGetServers            = "GetServers"
	ActionUpsertWebTun          = "UpsertWebTun"
	ActionGetWebTuns            = "GetWebTuns"
	ActionGetWebTun             = "GetWebTun"
	ActionDeleteWebTun          = "DeleteWebTun"
	ActionUpsertPassword        = "UpsertPassword"
	ActionCheckPassword         = "CheckPassword"
	ActionSignIn                = "SignIn"
	ActionGetWebSession         = "GetWebSession"
	ActionGetWebSessionsKeys    = "GetWebSessionsKeys"
	ActionDeleteWebSession      = "DeleteWebSession"
	ActionGetUsers              = "GetUsers"
	ActionDeleteUser            = "DeleteUser"
	ActionUpsertUserKey         = "UpsertUserKey"
	ActionGetUserKeys           = "GetUserKeys"
	ActionDeleteUserKey         = "DeleteUserKey"
	ActionGetHostCAPub          = "GetHostCAPub"
	ActionGetUserCAPub          = "GetUserCAPub"
	ActionGenerateKeyPair       = "GenerateKeyPair"
	ActionGenerateHostCert      = "GenerateHostCert"
	ActionGenerateUserCert      = "GenerateUserCert"
	ActionResetHostCA           = "ResetHostCA"
	ActionResetUserCA           = "ResetUserCA"
	ActionGenerateSealKey       = "GenerateSealKey"
	ActionGetSealKeys           = "GetSeakKeys"
	ActionGetSealKey            = "GetSealKey"
	ActionDeleteSealKey         = "DeleteSealKey"
	ActionAddSealKey            = "AddSealKey"
)
