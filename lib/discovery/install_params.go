/*
Copyright 2023 Gravitational, Inc.

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

package discovery

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// InstallParams sets join method to use on discovered nodes
type InstallParams struct {
	// JoinParams sets the token and method to use when generating
	// config on cloud instances
	JoinParams types.JoinParams `yaml:"join_params,omitempty"`
	// ScriptName is the name of the teleport installer script
	// resource for the cloud instance to execute
	ScriptName string `yaml:"script_name,omitempty"`
	// InstallTeleport disables agentless discovery
	InstallTeleport string `yaml:"install_teleport,omitempty"`
	// SSHDConfig provides the path to write sshd configuration changes
	SSHDConfig string `yaml:"sshd_config,omitempty"`
	// PublicProxyAddr is the address of the proxy the discovered node should use
	// to connect to the cluster. Used ony in Azure.
	PublicProxyAddr string `yaml:"public_proxy_addr,omitempty"`
}

func (ip *InstallParams) Parse() (types.InstallerParams, error) {
	install := types.InstallerParams{
		JoinMethod:      ip.JoinParams.Method,
		JoinToken:       ip.JoinParams.TokenName,
		ScriptName:      ip.ScriptName,
		InstallTeleport: true,
		SSHDConfig:      ip.SSHDConfig,
	}

	if ip.InstallTeleport == "" {
		return install, nil
	}

	var err error
	install.InstallTeleport, err = apiutils.ParseBool(ip.InstallTeleport)
	if err != nil {
		return types.InstallerParams{}, trace.Wrap(err)
	}

	return install, nil
}
