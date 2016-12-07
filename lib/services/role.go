package services

import (
	"time"
)

const (
	DefaultAPIGroup = "gravitational.io/teleport"

	ActionGet    = "get"
	ActionDelete = "delete"
	ActionUpsert = "upsert"
	ActionLogin  = "login"
)

type Permission interface {
	Namespace() string
	Payload() []byte
}

type PermissionResource struct {
	APIGroup string              `json:"api_group"`
	Resource *ResourcePermission `json:"resource"`
	SSH      *SSHPermission      `json:"ssh"`
}

/*
{resource: {kind: "session", actions: {'get': true, 'upsert': false}, namespace: "default"}}
*/
type ResourcePermission struct {
	// Kind is a resource kind, e.g. Session, or CertAuthority
	Kind string
	// Actions is a set of actions allowed on this resource, e.g. 'upsert' or 'create'
	Actions map[string]bool
	// Namespace is a resource namespace
}

/*
{ssh: {login: "bob", nodes: {'*':'*'}, channels: {session: true, direct-tcpip: false, data-transfer: false}}, max_ttl: 30d}
*/
type SSHPermission struct {
	// Login is a Unix login user
	Login string `json:"login"`
	// NodesSelector is a selector to match nodes
	NodesSelector map[string]string `json:"nodes"`
	// Channels limits types of channels allowed to open
	Channels map[string]string
	// MaxTTL is a maximum TTL
	MaxTTL time.Duration
}

/*
    {resource: {kind: "sessionList", actions: ["list", ""get"]}
	ActionGetSessions                       = "GetSessions"

	ActionGetSession                        = "GetSession"
	ActionViewSession                       = "ViewSession"
	ActionDeleteSession                     = "DeleteSession"
	ActionUpsertSession                     = "UpsertSession"

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
	ActionEmitEvents                        = "EmitEvents"
*/
