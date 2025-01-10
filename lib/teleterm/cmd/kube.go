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

package cmd

import (
	"fmt"
	"os/exec"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

// NewKubeCLICommand creates CLI commands for kube gateways.
func NewKubeCLICommand(g gateway.Gateway) (Cmds, error) {
	kube, err := gateway.AsKube(g)
	if err != nil {
		return Cmds{}, trace.Wrap(err)
	}

	// Use kubectl version as placeholders. Only env should be used.
	cmd := exec.Command("kubectl", "version")
	cmd.Env = []string{fmt.Sprintf("%v=%v", teleport.EnvKubeConfig, kube.KubeconfigPath())}
	return Cmds{Exec: cmd, Preview: cmd}, nil
}
