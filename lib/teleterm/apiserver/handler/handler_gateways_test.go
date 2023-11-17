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

package handler

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

func Test_makeGatewayCLICommand(t *testing.T) {
	absPath, err := filepath.Abs("test-binary")
	require.NoError(t, err)

	// Call exec.Command with a relative path so that cmd.Args[0] is a relative path.
	// Then replace cmd.Path with an absolute path to simulate binary being resolved to
	// an absolute path. This way we can later verify that gateway.CLICommand doesn't use the absolute
	// path.
	//
	// This also ensures that exec.Command behaves the same way on different devices, no matter
	// whether a command like postgres is installed on the system or not.
	cmd := exec.Command("test-binary", "arg1", "arg2")
	cmd.Path = absPath
	cmd.Env = []string{"FOO=bar"}

	command := makeGatewayCLICommand(cmd)

	require.Equal(t, &api.GatewayCLICommand{
		Path:    absPath,
		Args:    []string{"test-binary", "arg1", "arg2"},
		Env:     []string{"FOO=bar"},
		Preview: "FOO=bar test-binary arg1 arg2",
	}, command)
}
