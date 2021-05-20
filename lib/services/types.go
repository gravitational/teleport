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
