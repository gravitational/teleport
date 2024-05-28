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

package handler

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/cmd"
)

func Test_makeGatewayCLICommand(t *testing.T) {
	absPath, err := filepath.Abs("test-binary")
	require.NoError(t, err)

	// Call exec.Command with a relative path so that command.Args[0] is a relative path.
	// Then replace command.Path with an absolute path to simulate binary being resolved to
	// an absolute path. This way we can later verify that gateway.CLICommand doesn't use the absolute
	// path.
	//
	// This also ensures that exec.Command behaves the same way on different devices, no matter
	// whether a command like postgres is installed on the system or not.
	command := exec.Command("test-binary", "arg1", "arg2")
	command.Path = absPath
	command.Env = []string{"FOO=bar"}

	gatewayCmd := makeGatewayCLICommand(cmd.Cmds{Exec: command, Preview: command})

	require.Equal(t, &api.GatewayCLICommand{
		Path:    absPath,
		Args:    []string{"test-binary", "arg1", "arg2"},
		Env:     []string{"FOO=bar"},
		Preview: "FOO=bar test-binary arg1 arg2",
	}, gatewayCmd)
}
