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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestTemplateSSHClient_Render(t *testing.T) {
	tests := []struct {
		Name    string
		Version string
	}{
		{
			Name:    "legacy OpenSSH",
			Version: "6.5.0",
		},
		{
			Name:    "latest OpenSSH",
			Version: "9.0.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()

			cfg, err := newTestConfig("example.com")
			require.NoError(t, err)

			// ident is passed in, but not used.
			var ident *identity.Identity
			dest := &DestinationDirectory{
				Path:     dir,
				Symlinks: botfs.SymlinksInsecure,
				ACLs:     botfs.ACLOff,
			}

			mockBot := newMockProvider(cfg)
			tmpl := templateSSHClient{
				getSSHVersion: func() (*semver.Version, error) {
					return semver.New(tc.Version), nil
				},
				executablePathGetter: fakeGetExecutablePath,
				destPath:             dest.Path,
			}

			err = tmpl.render(context.Background(), mockBot, ident, dest)
			require.NoError(t, err)

			replaceTestDir := func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte(dir), []byte("/test/dir"))
			}

			knownHostBytes, err := os.ReadFile(filepath.Join(dir, knownHostsName))
			require.NoError(t, err)
			knownHostBytes = replaceTestDir(knownHostBytes)
			sshConfigBytes, err := os.ReadFile(filepath.Join(dir, sshConfigName))
			require.NoError(t, err)
			sshConfigBytes = replaceTestDir(sshConfigBytes)
			if golden.ShouldSet() {
				golden.SetNamed(t, "known_hosts", knownHostBytes)
				golden.SetNamed(t, "ssh_config", sshConfigBytes)
			}
			require.Equal(
				t, string(golden.GetNamed(t, "known_hosts")), string(knownHostBytes),
			)
			require.Equal(
				t, string(golden.GetNamed(t, "ssh_config")), string(sshConfigBytes),
			)
		})
	}
}
