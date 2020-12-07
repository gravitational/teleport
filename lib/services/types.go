package services

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/proto/types"
)

// The following types are imported from the types package to
// simplify refactoring. These could be removed and their
// references updated.

// Metadata import
type Metadata = types.Metadata

// AccessRequestSpecV3 import
type AccessRequestSpecV3 = types.AccessRequestSpecV3

// RequestState import
type RequestState = types.RequestState

// PluginDataFilter import
type PluginDataFilter = types.PluginDataFilter

// PluginDataUpdateParams import
type PluginDataUpdateParams = types.PluginDataUpdateParams

// ProvisionTokenSpecV2 import
type ProvisionTokenSpecV2 = types.ProvisionTokenSpecV2

// WebSessionSpecV2 import
type WebSessionSpecV2 = types.WebSessionSpecV2

// The following types are all wrapper structs, used to add
// additional helper methods on top of the standard proto methods.
// These can't be removed unless all of their methods are moved to
// the /api/proto/types package.

// ResourceHeader wrapper
type ResourceHeader struct {
	types.ResourceHeader
}

// AccessRequestV3 wrapper
type AccessRequestV3 struct {
	types.AccessRequestV3
}

// AccessRequestFilter wrapper
type AccessRequestFilter struct {
	types.AccessRequestFilter
}

// AccessRequestConditions wrapper
type AccessRequestConditions struct {
	types.AccessRequestConditions
}

// ProvisionTokenV1 wrapper
type ProvisionTokenV1 struct {
	types.ProvisionTokenV1
}

// ProvisionTokenV2 wrapper
type ProvisionTokenV2 struct {
	types.ProvisionTokenV2
}

// WebSessionV2 wrapper
type WebSessionV2 struct {
	types.WebSessionV2
}

// RemoteClusterV3 wrapper
type RemoteClusterV3 struct {
	types.RemoteClusterV3
}

// Some functions and variables also need to be imported from the types package
var (
	IsValidLabelKey       = types.IsValidLabelKey
	MetadataSchema        = types.MetadataSchema
	RequestState_NONE     = types.RequestState_NONE
	RequestState_PENDING  = types.RequestState_PENDING
	RequestState_APPROVED = types.RequestState_APPROVED
	RequestState_DENIED   = types.RequestState_DENIED
)

// The following Constants are imported from teleport to simplify
// refactoring. These could be removed and their references updated.
const (
	DefaultAPIGroup               = teleport.DefaultAPIGroup
	ActionRead                    = teleport.ActionRead
	ActionWrite                   = teleport.ActionWrite
	Wildcard                      = teleport.Wildcard
	KindNamespace                 = teleport.KindNamespace
	KindUser                      = teleport.KindUser
	KindKeyPair                   = teleport.KindKeyPair
	KindHostCert                  = teleport.KindHostCert
	KindJWT                       = teleport.KindJWT
	KindLicense                   = teleport.KindLicense
	KindRole                      = teleport.KindRole
	KindAccessRequest             = teleport.KindAccessRequest
	KindPluginData                = teleport.KindPluginData
	KindOIDC                      = teleport.KindOIDC
	KindSAML                      = teleport.KindSAML
	KindGithub                    = teleport.KindGithub
	KindOIDCRequest               = teleport.KindOIDCRequest
	KindSAMLRequest               = teleport.KindSAMLRequest
	KindGithubRequest             = teleport.KindGithubRequest
	KindSession                   = teleport.KindSession
	KindSSHSession                = teleport.KindSSHSession
	KindWebSession                = teleport.KindWebSession
	KindAppSession                = teleport.KindAppSession
	KindEvent                     = teleport.KindEvent
	KindAuthServer                = teleport.KindAuthServer
	KindProxy                     = teleport.KindProxy
	KindNode                      = teleport.KindNode
	KindAppServer                 = teleport.KindAppServer
	KindToken                     = teleport.KindToken
	KindCertAuthority             = teleport.KindCertAuthority
	KindReverseTunnel             = teleport.KindReverseTunnel
	KindOIDCConnector             = teleport.KindOIDCConnector
	KindSAMLConnector             = teleport.KindSAMLConnector
	KindGithubConnector           = teleport.KindGithubConnector
	KindConnectors                = teleport.KindConnectors
	KindClusterAuthPreference     = teleport.KindClusterAuthPreference
	MetaNameClusterAuthPreference = teleport.MetaNameClusterAuthPreference
	KindClusterConfig             = teleport.KindClusterConfig
	KindSemaphore                 = teleport.KindSemaphore
	MetaNameClusterConfig         = teleport.MetaNameClusterConfig
	KindClusterName               = teleport.KindClusterName
	MetaNameClusterName           = teleport.MetaNameClusterName
	KindStaticTokens              = teleport.KindStaticTokens
	MetaNameStaticTokens          = teleport.MetaNameStaticTokens
	KindTrustedCluster            = teleport.KindTrustedCluster
	KindAuthConnector             = teleport.KindAuthConnector
	KindTunnelConnection          = teleport.KindTunnelConnection
	KindRemoteCluster             = teleport.KindRemoteCluster
	KindResetPasswordToken        = teleport.KindResetPasswordToken
	KindResetPasswordTokenSecrets = teleport.KindResetPasswordTokenSecrets
	KindIdentity                  = teleport.KindIdentity
	KindState                     = teleport.KindState
	KindKubeService               = teleport.KindKubeService
	V3                            = teleport.V3
	V2                            = teleport.V2
	V1                            = teleport.V1
	VerbList                      = teleport.VerbList
	VerbCreate                    = teleport.VerbCreate
	VerbRead                      = teleport.VerbRead
	VerbReadNoSecrets             = teleport.VerbReadNoSecrets
	VerbUpdate                    = teleport.VerbUpdate
	VerbDelete                    = teleport.VerbDelete
	VerbRotate                    = teleport.VerbRotate
)
