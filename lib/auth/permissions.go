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
	"github.com/gravitational/trace"
)

// PermissionChecker interface verifies that clients have permissions
// to execute any action of the auth server
type PermissionChecker interface {
	// HasPermission checks if the given role has a permission to execute
	// the action
	HasPermission(role, action string) error
}

// NewStandardPermissions returns permission checker with hardcoded roles
// that are built in
func NewStandardPermissions() PermissionChecker {
	sp := standardPermissions{}
	sp.permissions = make(map[string](map[string]bool))

	sp.permissions[RoleUser] = map[string]bool{
		ActionSignIn:           true,
		ActionGenerateUserCert: true,
	}

	sp.permissions[RoleProvisionToken] = map[string]bool{
		ActionRegisterUsingToken:    true,
		ActionRegisterNewAuthServer: true,
	}

	sp.permissions[RoleNode] = map[string]bool{
		ActionUpsertServer:       true,
		ActionGetCertAuthorities: true,
		ActionGetLocalDomain:     true,
		ActionGetUserKeys:        true,
		ActionGetServers:         true,
		ActionUpsertParty:        true,
		ActionLogEntry:           true,
		ActionGetChunkWriter:     true,
	}

	sp.permissions[RoleWeb] = map[string]bool{
		ActionGetWebSession:    true,
		ActionDeleteWebSession: true,
	}

	sp.permissions[RoleSignup] = map[string]bool{
		ActionGetSignupTokenData:  true,
		ActionCreateUserWithToken: true,
	}

	return &sp
}

type standardPermissions struct {
	permissions map[string](map[string]bool)
}

func (sp *standardPermissions) HasPermission(role, action string) error {
	if role == RoleAdmin {
		return nil
	}
	if permissions, ok := sp.permissions[role]; ok {
		if permissions[action] {
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
	RoleSignup,
}

const (
	PermissionRole = "role"

	RoleAuth           = "Auth"
	RoleUser           = "User"
	RoleWeb            = "Web"
	RoleNode           = "Node"
	RoleAdmin          = "Admin"
	RoleProvisionToken = "ProvisionToken"
	RoleSignup         = "Signup"

	ActionGetSessions                   = "GetSession"
	ActionGetSession                    = "GetSession"
	ActionDeleteSession                 = "DeleteSession"
	ActionUpsertSession                 = "UpsertSession"
	ActionUpsertParty                   = "UpsertParty"
	ActionUpsertCertAuthority           = "UpsertCertAuthority"
	ActionGetCertAuthorities            = "GetCertAuthorities"
	ActionGetLocalDomain                = "GetLocalDomain"
	ActionDeleteCertAuthority           = "DeleteCertAuthority"
	ActionGenerateToken                 = "GenerateToken"
	ActionRegisterUsingToken            = "RegisterUsingToken"
	ActionRegisterNewAuthServer         = "RegisterNewAuthServer"
	ActionLog                           = "Log"
	ActionLogEntry                      = "LogEntry"
	ActionGetEvents                     = "GetEvents"
	ActionGetChunkWriter                = "GetChunkWriter"
	ActionGetChunkReader                = "GetChunkReader"
	ActionUpsertServer                  = "UpsertServer"
	ActionGetServers                    = "GetServers"
	ActionUpsertWebTun                  = "UpsertWebTun"
	ActionGetWebTuns                    = "GetWebTuns"
	ActionGetWebTun                     = "GetWebTun"
	ActionDeleteWebTun                  = "DeleteWebTun"
	ActionUpsertPassword                = "UpsertPassword"
	ActionCheckPassword                 = "CheckPassword"
	ActionSignIn                        = "SignIn"
	ActionGetWebSession                 = "GetWebSession"
	ActionGetWebSessionsKeys            = "GetWebSessionsKeys"
	ActionDeleteWebSession              = "DeleteWebSession"
	ActionGetUsers                      = "GetUsers"
	ActionDeleteUser                    = "DeleteUser"
	ActionUpsertUserKey                 = "UpsertUserKey"
	ActionGetUserKeys                   = "GetUserKeys"
	ActionDeleteUserKey                 = "DeleteUserKey"
	ActionGenerateKeyPair               = "GenerateKeyPair"
	ActionGenerateHostCert              = "GenerateHostCert"
	ActionGenerateUserCert              = "GenerateUserCert"
	ActionResetHostCertificateAuthority = "ResetHostCertificateAuthority"
	ActionResetUserCertificateAuthority = "ResetUserCertificateAuthority"
	ActionGenerateSealKey               = "GenerateSealKey"
	ActionGetSealKeys                   = "GetSeakKeys"
	ActionGetSealKey                    = "GetSealKey"
	ActionDeleteSealKey                 = "DeleteSealKey"
	ActionAddSealKey                    = "AddSealKey"
	ActionCreateSignupToken             = "CreateSignupToken"
	ActionGetSignupTokenData            = "GetSignupTokenData"
	ActionCreateUserWithToken           = "CreateUserWithToken"
	ActionUpsertUser                    = "UpsertUser"
)
