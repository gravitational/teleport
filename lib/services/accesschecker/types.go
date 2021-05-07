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

package accesschecker

import "github.com/gravitational/teleport/api/types"

const (
	Allow                     = types.Allow
	Deny                      = types.Deny
	KindAppServer             = types.KindAppServer
	KindAuthServer            = types.KindAuthServer
	KindCertAuthority         = types.KindCertAuthority
	KindClusterAuthPreference = types.KindClusterAuthPreference
	KindClusterName           = types.KindClusterName
	KindDatabaseServer        = types.KindDatabaseServer
	KindKubeService           = types.KindKubeService
	KindNode                  = types.KindNode
	KindProxy                 = types.KindProxy
	KindRemoteCluster         = types.KindRemoteCluster
	KindReverseTunnel         = types.KindReverseTunnel
	KindRole                  = types.KindRole
	KindSession               = types.KindSession
	KindSSHSession            = types.KindSSHSession
	V3                        = types.V3
	VerbCreate                = types.VerbCreate
	VerbDelete                = types.VerbDelete
	VerbList                  = types.VerbList
	VerbRead                  = types.VerbRead
	VerbReadNoSecrets         = types.VerbReadNoSecrets
	VerbUpdate                = types.VerbUpdate
	Wildcard                  = types.Wildcard
)

type (
	RemoteCluster         = types.RemoteCluster
	App                   = types.App
	CertAuthority         = types.CertAuthority
	DatabaseServer        = types.DatabaseServer
	ImpersonateConditions = types.ImpersonateConditions
	KubernetesCluster     = types.KubernetesCluster
	Labels                = types.Labels
	Metadata              = types.Metadata
	RemoveCluster         = types.RemoteCluster
	Resource              = types.Resource
	Role                  = types.Role
	RoleConditions        = types.RoleConditions
	RoleConditionType     = types.RoleConditionType
	RoleOptions           = types.RoleOptions
	RoleSpecV3            = types.RoleSpecV3
	RoleV3                = types.RoleV3
	Rule                  = types.Rule
	Server                = types.Server
	User                  = types.User
	UserV2                = types.UserV2
)

var (
	ProcessNamespace = types.ProcessNamespace
	NewRole          = types.NewRole
	CombineLabels    = types.CombineLabels
	MaxDuration      = types.MaxDuration
	NewBoolOption    = types.NewBoolOption
	CopyRulesSlice   = types.CopyRulesSlice
	BoolDefaultTrue  = types.BoolDefaultTrue
	NewRule          = types.NewRule
)
