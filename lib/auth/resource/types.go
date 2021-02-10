package resource

import "github.com/gravitational/teleport/api/types"

// TODO(dmitri): temporarily aliased types for progressive migration.
// Remove when source code had been migrated to use the api/types package directly
type (
	AccessRequest               = types.AccessRequest
	AccessRequestV3             = types.AccessRequestV3
	AuthPreference              = types.AuthPreference
	AuthPreferenceV2            = types.AuthPreferenceV2
	Metadata                    = types.Metadata
	CertAuthority               = types.CertAuthority
	CertAuthorityV2             = types.CertAuthorityV2
	CertRoles                   = types.CertRoles
	ClusterConfig               = types.ClusterConfig
	ClusterConfigV3             = types.ClusterConfigV3
	ClusterName                 = types.ClusterName
	ClusterNameV2               = types.ClusterNameV2
	Resource                    = types.Resource
	ResourceHeader              = types.ResourceHeader //nolint:golint
	GithubConnector             = types.GithubConnector
	GithubConnectorV3           = types.GithubConnectorV3
	GithubConnectorSpecV3       = types.GithubConnectorSpecV3
	OIDCConnector               = types.OIDCConnector
	OIDCConnectorV2             = types.OIDCConnectorV2
	OIDCConnectorSpecV2         = types.OIDCConnectorSpecV2
	SAMLConnector               = types.SAMLConnector
	SAMLConnectorV2             = types.SAMLConnectorV2
	SAMLConnectorSpecV2         = types.SAMLConnectorSpecV2
	License                     = types.License
	LicenseV3                   = types.LicenseV3
	LicenseSpecV3               = types.LicenseSpecV3
	Namespace                   = types.Namespace
	PluginData                  = types.PluginData
	PluginDataV3                = types.PluginDataV3
	ProvisionToken              = types.ProvisionToken
	ProvisionTokenV1            = types.ProvisionTokenV1
	ProvisionTokenV2            = types.ProvisionTokenV2
	RemoteCluster               = types.RemoteCluster
	RemoteClusterV3             = types.RemoteClusterV3
	ResetPasswordToken          = types.ResetPasswordToken
	ResetPasswordTokenV3        = types.ResetPasswordTokenV3
	ResetPasswordTokenSpecV3    = types.ResetPasswordTokenSpecV3
	ResetPasswordTokenSecrets   = types.ResetPasswordTokenSecrets
	ResetPasswordTokenSecretsV3 = types.ResetPasswordTokenSecretsV3
	KubernetesCluster           = types.KubernetesCluster
	Role                        = types.Role
	RoleV3                      = types.RoleV3
	RoleSpecV3                  = types.RoleSpecV3
	RoleConditions              = types.RoleConditions
	RoleOptions                 = types.RoleOptions
	Rule                        = types.Rule
	Labels                      = types.Labels
	Semaphore                   = types.Semaphore
	SemaphoreV3                 = types.SemaphoreV3
	Server                      = types.Server
	ServerV2                    = types.ServerV2
	ServerSpecV2                = types.ServerSpecV2
	StaticTokens                = types.StaticTokens
	StaticTokensV2              = types.StaticTokensV2
	ReverseTunnel               = types.ReverseTunnel
	ReverseTunnelV2             = types.ReverseTunnelV2
	TunnelConnection            = types.TunnelConnection
	TunnelConnectionV2          = types.TunnelConnectionV2
	TeamMapping                 = types.TeamMapping
	TrustedCluster              = types.TrustedCluster
	TrustedClusterV2            = types.TrustedClusterV2
	User                        = types.User
	UserV2                      = types.UserV2
	UserSpecV2                  = types.UserSpecV2
	WebSession                  = types.WebSession
	WebSessionV2                = types.WebSessionV2
)

const (
	KindRole                      = types.KindRole
	KindAccessRequest             = types.KindAccessRequest
	KindPluginData                = types.KindPluginData
	KindOIDC                      = types.KindOIDC
	KindSAML                      = types.KindSAML
	KindGithub                    = types.KindGithub
	KindOIDCRequest               = types.KindOIDCRequest
	KindSAMLRequest               = types.KindSAMLRequest
	KindGithubRequest             = types.KindGithubRequest
	KindSession                   = types.KindSession
	KindSSHSession                = types.KindSSHSession
	KindWebSession                = types.KindWebSession
	KindWebToken                  = types.KindWebToken
	KindAppSession                = types.KindAppSession
	KindEvent                     = types.KindEvent
	KindAuthServer                = types.KindAuthServer
	KindProxy                     = types.KindProxy
	KindNode                      = types.KindNode
	KindAppServer                 = types.KindAppServer
	KindToken                     = types.KindToken
	KindCertAuthority             = types.KindCertAuthority
	KindReverseTunnel             = types.KindReverseTunnel
	KindOIDCConnector             = types.KindOIDCConnector
	KindSAMLConnector             = types.KindSAMLConnector
	KindGithubConnector           = types.KindGithubConnector
	KindConnectors                = types.KindConnectors
	KindClusterAuthPreference     = types.KindClusterAuthPreference
	MetaNameClusterAuthPreference = types.MetaNameClusterAuthPreference
	KindClusterConfig             = types.KindClusterConfig
	KindSemaphore                 = types.KindSemaphore
	MetaNameClusterConfig         = types.MetaNameClusterConfig
	KindClusterName               = types.KindClusterName
	MetaNameClusterName           = types.MetaNameClusterName
	KindStaticTokens              = types.KindStaticTokens
	MetaNameStaticTokens          = types.MetaNameStaticTokens
	KindTrustedCluster            = types.KindTrustedCluster
	KindAuthConnector             = types.KindAuthConnector
	KindTunnelConnection          = types.KindTunnelConnection
	KindRemoteCluster             = types.KindRemoteCluster
	KindResetPasswordToken        = types.KindResetPasswordToken
	KindResetPasswordTokenSecrets = types.KindResetPasswordTokenSecrets
	KindIdentity                  = types.KindIdentity
	KindState                     = types.KindState
	KindKubeService               = types.KindKubeService
	KindNamespace                 = types.KindNamespace
	KindUser                      = types.KindUser

	Wildcard = types.Wildcard

	VerbRead   = types.VerbRead
	VerbList   = types.VerbList
	VerbDelete = types.VerbDelete

	V1 = types.V1
	V2 = types.V2
	V3 = types.V3
)

var (
	NewGithubConnector = types.NewGithubConnector
	NewLicense         = types.NewLicense
	NewUser            = types.NewUser

	NewBool       = types.NewBool
	NewDuration   = types.NewDuration
	NewBoolOption = types.NewBoolOption
)
