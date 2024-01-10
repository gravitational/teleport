/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
	"os/exec"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

// NewAppCLICommand creates CLI commands for app gateways.
func NewAppCLICommand(g gateway.Gateway) (*exec.Cmd, error) {
	app, err := gateway.AsApp(g)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if g.Protocol() == types.ApplicationProtocolTCP {
		return exec.Command(""), nil
	}

	cmd := exec.Command("curl", app.LocalProxyURL())
	return cmd, nil
}
