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
package config

import (
	"bytes"
	"testing"

	"github.com/gravitational/teleport/lib/utils/golden"
	"github.com/stretchr/testify/require"
)

func TestSystemd_Install(t *testing.T) {

	flags := SystemdSampleFlags{
		EnvironmentFile:          "/custom/env/dir/teleport",
		PIDFile:                  "/custom/pid/dir/teleport.pid",
		FileDescriptorLimit:      16384,
		TeleportInstallationFile: "/custom/install/dir/teleport",
	}

	t.Run("stdout", func(t *testing.T) {
		stdout := new(bytes.Buffer)
		out, err := MakeSystemdUnitFileString(flags)
		require.NoError(t, err)
		data := stdout.Bytes()
		if golden.ShouldSet() {
			golden.Set(t, data)
		}
		require.Equal(t, string(golden.Get(t)), string(out))
	})

}
