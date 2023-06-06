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

package openssh

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type testSSHDBackend struct {
	didRestart bool
}

func (b *testSSHDBackend) restart() error {
	b.didRestart = true
	return nil
}

func (b *testSSHDBackend) checkConfig(path string) error {
	return nil
}

func TestSSHD(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string

		initialSSHDConfig          string
		expectedSSHDConfigPrefix   string
		expectedTeleportSSHDConfig string
		restart                    bool
	}{
		{
			name:                     "sshd config update with restart",
			initialSSHDConfig:        "SomeSSHConfig Hello",
			expectedSSHDConfigPrefix: "Include %s/sshd.conf",
			expectedTeleportSSHDConfig: `# Created by 'teleport join openssh', do not edit
TrustedUserCAKeys %s/teleport_openssh_ca.pub
HostKey %s/ssh_host_teleport_key
HostCertificate %s/ssh_host_teleport_key-cert.pub
`,
			restart: true,
		},
		{
			name:                     "sshd config update without restart",
			initialSSHDConfig:        "SomeSSHConfig Hello",
			expectedSSHDConfigPrefix: "Include %s/sshd.conf",
			expectedTeleportSSHDConfig: `# Created by 'teleport join openssh', do not edit
TrustedUserCAKeys %s/teleport_openssh_ca.pub
HostKey %s/ssh_host_teleport_key
HostCertificate %s/ssh_host_teleport_key-cert.pub
`,
			restart: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testDir := t.TempDir()
			backend := &testSSHDBackend{}
			sshd := SSHD{
				sshd: backend,
			}

			openSSHConfigFile := filepath.Join(testDir, "sshd_config")
			if tc.initialSSHDConfig != "" {
				require.NoError(t, os.WriteFile(openSSHConfigFile, []byte(tc.initialSSHDConfig), 0o700))
			}

			dataDir := filepath.Join(testDir, "teleport")
			require.NoError(t, os.MkdirAll(dataDir, 0o700))

			err := sshd.UpdateConfig(SSHDConfigUpdate{
				SSHDConfigPath: openSSHConfigFile,
				DataDir:        dataDir,
			}, tc.restart)
			require.NoError(t, err)

			teleportSSHDPath := filepath.Join(dataDir, "sshd.conf")

			actualSSHDConfig, err := os.ReadFile(openSSHConfigFile)
			require.NoError(t, err)
			expectedPrefix := fmt.Sprintf(tc.expectedSSHDConfigPrefix+"\n", dataDir)
			require.Equal(t, expectedPrefix+tc.initialSSHDConfig, string(actualSSHDConfig))

			actualTeleportSSHDConfig, err := os.ReadFile(teleportSSHDPath)
			require.NoError(t, err)
			openSSHKeyDir := filepath.Join(dataDir, "openssh")
			expectedTeleportSSHDConfig := fmt.Sprintf(tc.expectedTeleportSSHDConfig, openSSHKeyDir, openSSHKeyDir, openSSHKeyDir)

			require.Equal(t, expectedTeleportSSHDConfig, string(actualTeleportSSHDConfig))

			require.Equal(t, tc.restart, backend.didRestart)

		})
	}

}
