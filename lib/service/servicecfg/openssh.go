/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
