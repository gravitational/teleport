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
	"github.com/gravitational/trace"
)

// PermissionChecker interface verifies that clients have permissions
// to execute any action of the auth server
type PermissionChecker interface {
	// HasPermission checks if the given role has a permission to execute
	// the action
	HasPermission(role teleport.Role, action string) error
}

// NewStandardPermissions returns permission checker with hardcoded roles
// that are built in when auth server starts in standard mode
func NewStandardPermissions() PermissionChecker {
	sp := standardPermissions{}
	sp.permissions = make(map[teleport.Role](map[string]bool))

	sp.permissions[teleport.RoleAuth] = map[string]bool{
		ActionUpsertAuthServer: true,
		ActionGetAuthServers:   true,
	}

	sp.permissions[teleport.RoleUser] = map[string]bool{
		ActionGetAuthServers:     true,
		ActionSignIn:             true,
		ActionExtendWebSession:   true,
		ActionGenerateUserCert:   true,
		ActionGetCertAuthorities: true,
		ActionGetServers:         true,
		ActionGetSession:         true,
		ActionGetSessions:        true,
	}

	sp.permissions[teleport.RoleProvisionToken] = map[string]bool{
		ActionRegisterUsingToken:    true,
		ActionRegisterNewAuthServer: true,
	}

	sp.permissions[teleport.RoleNode] = map[string]bool{
		ActionUpsertServer:       true,
		ActionGetServers:         true,
		ActionGetProxies:         true,
		ActionGetAuthServers:     true,
		ActionGetCertAuthorities: true,
		ActionGetUsers:           true,
		ActionGetUser:            true,
		ActionGetLocalDomain:     true,
		ActionGetUserKeys:        true,
		ActionUpsertParty:        true,
		ActionUpsertSession:      true,
		ActionGetSession:         true,
		ActionGetSessions:        true,
	}

	sp.permissions[teleport.RoleProxy] = map[string]bool{
		ActionGetUser:                         true,
		ActionGetOIDCConnectorsWithoutSecrets: true,
		ActionGetReverseTunnels:               true,
		ActionGetServers:                      true,
		ActionUpsertProxy:                     true,
		ActionGetProxies:                      true,
		ActionGetAuthServers:                  true,
		ActionGetCertAuthorities:              true,
		ActionGetUsers:                        true,
		ActionGetLocalDomain:                  true,
		ActionGetUserKeys:                     true,
		ActionGetSession:                      true,
		ActionGetSessions:                     true,
		ActionCreateOIDCAuthRequest:           true,
		ActionValidateOIDCAuthCallback:        true,
	}

	sp.permissions[teleport.RoleWeb] = map[string]bool{
		ActionGetUser:          true,
		ActionGetAuthServers:   true,
		ActionUpsertSession:    true,
		ActionExtendWebSession: true,
		ActionGetWebSession:    true,
		ActionDeleteWebSession: true,
		ActionGetSession:       true,
		ActionGetSessions:      true,
	}

	sp.permissions[teleport.RoleSignup] = map[string]bool{
		ActionGetSignupTokenData:  true,
		ActionCreateUserWithToken: true,
		ActionGetAuthServers:      true,
	}

	return &sp
}

type standardPermissions struct {
	permissions map[teleport.Role](map[string]bool)
}

func (sp *standardPermissions) HasPermission(role teleport.Role, action string) error {
	if role == teleport.RoleAdmin {
		return nil
	}
	if permissions, ok := sp.permissions[role]; ok {
		if permissions[action] {
			return nil
		}
		return trace.AccessDenied(
			"role '%v' doesn't have permission for action '%v'",
			role, action)
	}
	return trace.AccessDenied("role '%v' is not allowed", role)
}

type allowAllPermissions struct {
}

func NewAllowAllPermissions() PermissionChecker {
	aap := allowAllPermissions{}
	return &aap
}

func (aap *allowAllPermissions) HasPermission(role teleport.Role, action string) error {
	return nil
}

var StandardRoles = []teleport.Role{
	teleport.RoleAuth,
	teleport.RoleUser,
	teleport.RoleWeb,
	teleport.RoleNode,
	teleport.RoleProxy,
	teleport.RoleAdmin,
	teleport.RoleProvisionToken,
	teleport.RoleSignup,
}

const (
	ActionGetSessions                       = "GetSessions"
	ActionGetSession                        = "GetSession"
	ActionDeleteSession                     = "DeleteSession"
	ActionUpsertSession                     = "UpsertSession"
	ActionUpsertParty                       = "UpsertParty"
	ActionUpsertCertAuthority               = "UpsertCertAuthority"
	ActionGetCertAuthorities                = "GetCertAuthorities"
	ActionGetCertAuthoritiesWithSigningKeys = "GetCertAuthoritiesWithSigningKeys"
	ActionGetLocalDomain                    = "GetLocalDomain"
	ActionDeleteCertAuthority               = "DeleteCertAuthority"
	ActionGenerateToken                     = "GenerateToken"
	ActionRegisterUsingToken                = "RegisterUsingToken"
	ActionRegisterNewAuthServer             = "RegisterNewAuthServer"
	ActionUpsertServer                      = "UpsertServer"
	ActionGetServers                        = "GetServers"
	ActionUpsertAuthServer                  = "UpsertAuthServer"
	ActionGetAuthServers                    = "GetAuthServers"
	ActionUpsertProxy                       = "UpsertProxy"
	ActionGetProxies                        = "GetProxies"
	ActionUpsertReverseTunnel               = "UpsertReverseTunnel"
	ActionGetReverseTunnels                 = "GetReverseTunnels"
	ActionDeleteReverseTunnel               = "DeleteReverseTunnel"
	ActionUpsertPassword                    = "UpsertPassword"
	ActionCheckPassword                     = "CheckPassword"
	ActionSignIn                            = "SignIn"
	ActionExtendWebSession                  = "ExtendWebSession"
	ActionCreateWebSession                  = "CreateWebSession"
	ActionGetWebSession                     = "GetWebSession"
	ActionDeleteWebSession                  = "DeleteWebSession"
	ActionGetUsers                          = "GetUsers"
	ActionGetUser                           = "GetUser"
	ActionDeleteUser                        = "DeleteUser"
	ActionUpsertUserKey                     = "UpsertUserKey"
	ActionGetUserKeys                       = "GetUserKeys"
	ActionDeleteUserKey                     = "DeleteUserKey"
	ActionGenerateKeyPair                   = "GenerateKeyPair"
	ActionGenerateHostCert                  = "GenerateHostCert"
	ActionGenerateUserCert                  = "GenerateUserCert"
	ActionResetHostCertificateAuthority     = "ResetHostCertificateAuthority"
	ActionResetUserCertificateAuthority     = "ResetUserCertificateAuthority"
	ActionGenerateSealKey                   = "GenerateSealKey"
	ActionGetSealKeys                       = "GetSeakKeys"
	ActionGetSealKey                        = "GetSealKey"
	ActionDeleteSealKey                     = "DeleteSealKey"
	ActionAddSealKey                        = "AddSealKey"
	ActionCreateSignupToken                 = "CreateSignupToken"
	ActionGetSignupTokenData                = "GetSignupTokenData"
	ActionCreateUserWithToken               = "CreateUserWithToken"
	ActionUpsertUser                        = "UpsertUser"
	ActionUpsertOIDCConnector               = "UpsertOIDCConnector"
	ActionDeleteOIDCConnector               = "DeleteOIDCConnector"
	ActionGetOIDCConnectorWithSecrets       = "GetOIDCConnectorWithSecrets"
	ActionGetOIDCConnectorWithoutSecrets    = "GetOIDCConnectorWithoutSecrets"
	ActionGetOIDCConnectorsWithSecrets      = "GetOIDCConnectorsWithSecrets"
	ActionGetOIDCConnectorsWithoutSecrets   = "GetOIDCConnectorsWithoutSecrets"
	ActionCreateOIDCAuthRequest             = "CreateOIDCAuthRequest"
	ActionValidateOIDCAuthCallback          = "ValidateOIDCAuthCallback"
)
