// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package servicecfg

import (
	"github.com/gravitational/teleport/lib/utils"
)

type OpenSSHConfig struct {
	Enabled bool
	// SSHDConfigPath is the path to the OpenSSH config file.
	SSHDConfigPath string
	// RestartSSHD is true if sshd should be restarted after config updates.
	RestartSSHD bool
	// RestartCommand is the command to use when restarting sshd.
	RestartCommand string
	// CheckCommand is the command to use when validating sshd config.
	CheckCommand string
	// AdditionalPrincipals is a list of additional principals to be included.
	AdditionalPrincipals []string
	// InstanceAddr is the connectable address of the OpenSSh instance.
	InstanceAddr string
	// ProxyServer is the address of the teleport proxy.
	ProxyServer *utils.NetAddr
	// Labels are labels to set on the instance.
	Labels map[string]string
}
