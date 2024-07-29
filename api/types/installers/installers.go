/*
Copyright 2022 Gravitational, Inc.

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

package installers

import (
	_ "embed"

	"github.com/gravitational/teleport/api/types"
)

//go:embed agentless-installer.sh.tmpl
var defaultAgentlessInstallScript string

// InstallerScriptName is the name of the by default populated, EC2
// installer script
const InstallerScriptName = types.DefaultInstallerScriptName

// InstallerScriptName is the name of the by default populated, EC2
// installer script when agentless mode is enabled for a matcher
const InstallerScriptNameAgentless = types.DefaultInstallerScriptNameAgentless

// DefaultAgentlessInstaller represents a the default agentless installer script provided
// by teleport
var DefaultAgentlessInstaller = types.MustNewInstallerV1(InstallerScriptNameAgentless, defaultAgentlessInstallScript)

// Template is used to fill proxy address and version information into
// the installer script
type Template struct {
	// PublicProxyAddr is public address of the proxy
	PublicProxyAddr string
	// MajorVersion is the major version of the Teleport auth node
	MajorVersion string
	// TeleportPackage is the teleport package to use. `teleport` or
	// `teleport-ent` depending on if the cluster is enterprise or not.
	TeleportPackage string
	// RepoChannel is the repo's channel name to install.
	RepoChannel string
	// AutomaticUpgrades indicates whether Automatic Upgrades are enabled or disabled.
	// Its value is either `true` or `false`.
	AutomaticUpgrades string
	// AzureClientID is the client ID of the managed identity to use when joining
	// the cluster. Only applicable for the azure join method.
	AzureClientID string
}
