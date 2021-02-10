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

package services

import (
	"github.com/gravitational/teleport/api/types"
)

// The following types, functions, and constants have been moved to /api/types, and are now imported here
// for backwards compatibility. These can be removed in a future version.
// DELETE IN 7.0.0

// types.pb.go
type (
	KeepAlive                          = types.KeepAlive
	Metadata                           = types.Metadata
	Rotation                           = types.Rotation
	RotationSchedule                   = types.RotationSchedule
	ResourceHeader                     = types.ResourceHeader
	ServerV2                           = types.ServerV2
	ServerSpecV2                       = types.ServerSpecV2
	App                                = types.App
	Rewrite                            = types.Rewrite
	CommandLabelV2                     = types.CommandLabelV2
	TLSKeyPair                         = types.TLSKeyPair
	JWTKeyPair                         = types.JWTKeyPair
	CertAuthorityV2                    = types.CertAuthorityV2
	CertAuthoritySpecV2                = types.CertAuthoritySpecV2
	RoleMapping                        = types.RoleMapping
	ProvisionTokenV1                   = types.ProvisionTokenV1
	ProvisionTokenV2                   = types.ProvisionTokenV2
	ProvisionTokenSpecV2               = types.ProvisionTokenSpecV2
	StaticTokensV2                     = types.StaticTokensV2
	StaticTokensSpecV2                 = types.StaticTokensSpecV2
	ClusterNameV2                      = types.ClusterNameV2
	ClusterNameSpecV2                  = types.ClusterNameSpecV2
	ClusterConfigV3                    = types.ClusterConfigV3
	ClusterConfigSpecV3                = types.ClusterConfigSpecV3
	AuditConfig                        = types.AuditConfig
	Namespace                          = types.Namespace
	NamespaceSpec                      = types.NamespaceSpec
	ResetPasswordTokenV3               = types.ResetPasswordTokenV3
	ResetPasswordTokenSpecV3           = types.ResetPasswordTokenSpecV3
	ResetPasswordTokenSecretsV3        = types.ResetPasswordTokenSecretsV3
	ResetPasswordTokenSecretsSpecV3    = types.ResetPasswordTokenSecretsSpecV3
	AccessRequestV3                    = types.AccessRequestV3
	AccessRequestSpecV3                = types.AccessRequestSpecV3
	AccessRequestFilter                = types.AccessRequestFilter
	PluginDataV3                       = types.PluginDataV3
	PluginDataEntry                    = types.PluginDataEntry
	PluginDataSpecV3                   = types.PluginDataSpecV3
	PluginDataFilter                   = types.PluginDataFilter
	PluginDataUpdateParams             = types.PluginDataUpdateParams
	RoleV3                             = types.RoleV3
	RoleSpecV3                         = types.RoleSpecV3
	RoleOptions                        = types.RoleOptions
	RoleConditions                     = types.RoleConditions
	AccessRequestConditions            = types.AccessRequestConditions
	AccessRequestClaimMapping          = types.ClaimMapping
	Rule                               = types.Rule
	BoolValue                          = types.BoolValue
	UserV2                             = types.UserV2
	UserSpecV2                         = types.UserSpecV2
	ExternalIdentity                   = types.ExternalIdentity
	LoginStatus                        = types.LoginStatus
	CreatedBy                          = types.CreatedBy
	U2FRegistrationData                = types.U2FRegistrationData
	LocalAuthSecrets                   = types.LocalAuthSecrets
	ConnectorRef                       = types.ConnectorRef
	UserRef                            = types.UserRef
	ReverseTunnelV2                    = types.ReverseTunnelV2
	ReverseTunnelSpecV2                = types.ReverseTunnelSpecV2
	TunnelConnectionV2                 = types.TunnelConnectionV2
	TunnelConnectionSpecV2             = types.TunnelConnectionSpecV2
	SemaphoreFilter                    = types.SemaphoreFilter
	AcquireSemaphoreRequest            = types.AcquireSemaphoreRequest
	SemaphoreLease                     = types.SemaphoreLease
	SemaphoreLeaseRef                  = types.SemaphoreLeaseRef
	SemaphoreV3                        = types.SemaphoreV3
	SemaphoreSpecV3                    = types.SemaphoreSpecV3
	WebSessionV2                       = types.WebSessionV2
	WebSessionSpecV2                   = types.WebSessionSpecV2
	RemoteClusterV3                    = types.RemoteClusterV3
	RemoteClusterStatusV3              = types.RemoteClusterStatusV3
	KubernetesCluster                  = types.KubernetesCluster
	CertAuthoritySpecV2_SigningAlgType = types.CertAuthoritySpecV2_SigningAlgType //nolint
)

var (
	RequestState_NONE                = types.RequestState_NONE                //nolint
	RequestState_PENDING             = types.RequestState_PENDING             //nolint
	RequestState_APPROVED            = types.RequestState_APPROVED            //nolint
	RequestState_DENIED              = types.RequestState_DENIED              //nolint
	CertAuthoritySpecV2_UNKNOWN      = types.CertAuthoritySpecV2_UNKNOWN      //nolint
	CertAuthoritySpecV2_RSA_SHA1     = types.CertAuthoritySpecV2_RSA_SHA1     //nolint
	CertAuthoritySpecV2_RSA_SHA2_256 = types.CertAuthoritySpecV2_RSA_SHA2_256 //nolint
	CertAuthoritySpecV2_RSA_SHA2_512 = types.CertAuthoritySpecV2_RSA_SHA2_512 //nolint
	KeepAlive_NODE                   = types.KeepAlive_NODE                   //nolint
	KeepAlive_APP                    = types.KeepAlive_APP                    //nolint
)

// access_request.go
type (
	AccessRequest             = types.AccessRequest
	AccessRequestUpdate       = types.AccessRequestUpdate
	RequestStrategy           = types.RequestStrategy
	RequestState              = types.RequestState
	AccessCapabilitiesRequest = types.AccessCapabilitiesRequest
	AccessCapabilities        = types.AccessCapabilities
)

var (
	RequestStrategyOptional = types.RequestStrategyOptional
	RequestStrategyReason   = types.RequestStrategyReason
	RequestStrategyAlways   = types.RequestStrategyAlways
)

// authentication.go
type (
	AuthPreference       = types.AuthPreference
	AuthPreferenceV2     = types.AuthPreferenceV2
	AuthPreferenceSpecV2 = types.AuthPreferenceSpecV2
	U2F                  = types.U2F
)

var (
	NewAuthPreference     = types.NewAuthPreference
	DefaultAuthPreference = types.DefaultAuthPreference
)

// authority.go
type (
	CertAuthority = types.CertAuthority
	CertRoles     = types.CertRoles
)

var (
	GenerateSchedule = types.GenerateSchedule

	RotationStateStandby       = types.RotationStateStandby
	RotationStateInProgress    = types.RotationStateInProgress
	RotationPhaseStandby       = types.RotationPhaseStandby
	RotationPhaseInit          = types.RotationPhaseInit
	RotationPhaseUpdateClients = types.RotationPhaseUpdateClients
	RotationPhaseUpdateServers = types.RotationPhaseUpdateServers
	RotationPhaseRollback      = types.RotationPhaseRollback
	RotationModeManual         = types.RotationModeManual
	RotationModeAuto           = types.RotationModeAuto

	RotatePhases    = types.RotatePhases
	RemoveCASecrets = types.RemoveCASecrets
)

// clusterconfig.go
type ClusterConfig = types.ClusterConfig

var (
	NewClusterConfig = types.NewClusterConfig

	RecordAtNode      = types.RecordAtNode
	RecordAtProxy     = types.RecordAtProxy
	RecordOff         = types.RecordOff
	RecordAtNodeSync  = types.RecordAtNodeSync
	RecordAtProxySync = types.RecordAtProxySync

	HostKeyCheckYes = types.HostKeyCheckYes
	HostKeyCheckNo  = types.HostKeyCheckNo
)

// clustername.go
type ClusterName = types.ClusterName

var (
	NewClusterName = types.NewClusterName
)

// duration.go
type Duration = types.Duration

var (
	MaxDuration = types.MaxDuration
	NewDuration = types.NewDuration
)

// event.go
type (
	Event     = types.Event
	Watch     = types.Watch
	WatchKind = types.WatchKind
	Events    = types.Events
	Watcher   = types.Watcher
)

// github.go
type (
	GithubConnector       = types.GithubConnector
	GithubConnectorV3     = types.GithubConnectorV3
	GithubConnectorSpecV3 = types.GithubConnectorSpecV3
	TeamMapping           = types.TeamMapping
	GithubClaims          = types.GithubClaims
)

var (
	NewGithubConnector = types.NewGithubConnector
)

// license.go
type (
	License       = types.License
	LicenseV3     = types.LicenseV3
	LicenseSpecV3 = types.LicenseSpecV3
)

var (
	NewLicense = types.NewLicense
)

// namespace.go
type SortedNamespaces = types.SortedNamespaces

var (
	IsValidNamespace = types.IsValidNamespace
)

// oidc.go
type (
	OIDCConnector       = types.OIDCConnector
	OIDCConnectorV2     = types.OIDCConnectorV2
	OIDCConnectorSpecV2 = types.OIDCConnectorSpecV2
	ClaimMapping        = types.ClaimMapping
)

var NewOIDCConnector = types.NewOIDCConnector

// plugin_data.go
type PluginData = types.PluginData

var (
	NewPluginData = types.NewPluginData
)

// presence.go
type (
	ProxyGetter = types.ProxyGetter
	Site        = types.Site
	KeepAliver  = types.KeepAliver
)

var NewNamespace = types.NewNamespace

// provisioning.go
type (
	ProvisionToken = types.ProvisionToken
)

var (
	NewProvisionToken = types.NewProvisionToken

	ProvisionTokensToV1   = types.ProvisionTokensToV1
	ProvisionTokensFromV1 = types.ProvisionTokensFromV1
)

// remotecluster.go
type RemoteCluster = types.RemoteCluster

var (
	NewRemoteCluster = types.NewRemoteCluster
)

// resetpasswordtoken.go
type ResetPasswordToken = types.ResetPasswordToken

var (
	NewResetPasswordToken = types.NewResetPasswordToken
)

// resetpasswordtokensecrets.go

type ResetPasswordTokenSecrets = types.ResetPasswordTokenSecrets

var (
	NewResetPasswordTokenSecrets = types.NewResetPasswordTokenSecrets
)

// resource.go
type (
	Resource            = types.Resource
	ResourceWithSecrets = types.ResourceWithSecrets
)

var (
	IsValidLabelKey = types.IsValidLabelKey
)

// role.go
type (
	Role              = types.Role
	RoleConditionType = types.RoleConditionType
	Labels            = types.Labels
	Bool              = types.Bool
)

var (
	NewRole          = types.NewRole
	NewRule          = types.NewRule
	CopyRulesSlice   = types.CopyRulesSlice
	RuleSlicesEqual  = types.RuleSlicesEqual
	NewBool          = types.NewBool
	NewBoolOption    = types.NewBoolOption
	BoolDefaultTrue  = types.BoolDefaultTrue
	ProcessNamespace = types.ProcessNamespace
)

// saml.go
type (
	SAMLConnector       = types.SAMLConnector
	SAMLConnectorV2     = types.SAMLConnectorV2
	SAMLConnectorSpecV2 = types.SAMLConnectorSpecV2
	AttributeMapping    = types.AttributeMapping
	AsymmetricKeyPair   = types.AsymmetricKeyPair
)

var (
	NewSAMLConnector = types.NewSAMLConnector
)

// semaphore.go
type (
	Semaphore  = types.Semaphore
	Semaphores = types.Semaphores
)

var (
	SemaphoreKindConnection = types.SemaphoreKindConnection
)

// server.go
type (
	Server       = types.Server
	CommandLabel = types.CommandLabel
)

var (
	CombineLabels  = types.CombineLabels
	LabelsAsString = types.LabelsAsString
	V2ToLabels     = types.V2ToLabels
	LabelsToV2     = types.LabelsToV2
)

// session.go
type (
	WebSession              = types.WebSession
	GetAppSessionRequest    = types.GetAppSessionRequest
	CreateAppSessionRequest = types.CreateAppSessionRequest
	DeleteAppSessionRequest = types.DeleteAppSessionRequest
)

var (
	NewWebSession = types.NewWebSession
)

// statictokens.go
type StaticTokens = types.StaticTokens

var (
	NewStaticTokens = types.NewStaticTokens
)

// traits.go
type (
	TraitMapping    = types.TraitMapping
	TraitMappingSet = types.TraitMappingSet
)

// trust.go
type (
	CertAuthType = types.CertAuthType
	CertAuthID   = types.CertAuthID
)

var (
	HostCA    = types.HostCA
	UserCA    = types.UserCA
	JWTSigner = types.JWTSigner
)

// trustedcluster.go
type (
	TrustedCluster       = types.TrustedCluster
	TrustedClusterV2     = types.TrustedClusterV2
	TrustedClusterSpecV2 = types.TrustedClusterSpecV2
	RoleMap              = types.RoleMap
	SortedTrustedCluster = types.SortedTrustedCluster
)

var (
	NewTrustedCluster = types.NewTrustedCluster
)

// tunnel.go
type (
	ReverseTunnel = types.ReverseTunnel
	TunnelType    = types.TunnelType
)

var (
	NewReverseTunnel = types.NewReverseTunnel

	NodeTunnel  = types.NodeTunnel
	ProxyTunnel = types.ProxyTunnel
	AppTunnel   = types.AppTunnel
	KubeTunnel  = types.KubeTunnel
)

// tunnelconn.go
type (
	TunnelConnection = types.TunnelConnection
)

var (
	NewTunnelConnection = types.NewTunnelConnection
)

// user.go
type User = types.User

var (
	NewUser = types.NewUser
)

// The following constants are imported from api/constants to simplify
// refactoring. These could be removed and their references updated.
const (
	DefaultAPIGroup               = types.DefaultAPIGroup
	ActionRead                    = types.ActionRead
	ActionWrite                   = types.ActionWrite
	Wildcard                      = types.Wildcard
	KindNamespace                 = types.KindNamespace
	KindUser                      = types.KindUser
	KindKeyPair                   = types.KindKeyPair
	KindHostCert                  = types.KindHostCert
	KindJWT                       = types.KindJWT
	KindLicense                   = types.KindLicense
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
	KindBilling                   = types.KindBilling
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
	V3                            = types.V3
	V2                            = types.V2
	V1                            = types.V1
	VerbList                      = types.VerbList
	VerbCreate                    = types.VerbCreate
	VerbRead                      = types.VerbRead
	VerbReadNoSecrets             = types.VerbReadNoSecrets
	VerbUpdate                    = types.VerbUpdate
	VerbDelete                    = types.VerbDelete
	VerbRotate                    = types.VerbRotate
)

const (
	// Allow is the set of conditions that allow access.
	Allow RoleConditionType = true
	// Deny is the set of conditions that prevent access.
	Deny RoleConditionType = false
)
