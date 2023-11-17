/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/config/openssh"
)

// TestWriteSSHConfig tests the writeSSHConfig template output.
func TestWriteSSHConfig(t *testing.T) {
	t.Parallel()

	want := `# Begin generated Teleport configuration for localhost by tsh

# Common flags for all test-cluster hosts
Host *.test-cluster localhost
    UserKnownHostsFile "/tmp/know_host"
    IdentityFile "/tmp/alice"
    CertificateFile "/tmp/localhost-cert.pub"
    HostKeyAlgorithms rsa-sha2-512-cert-v01@openssh.com,rsa-sha2-256-cert-v01@openssh.com,ssh-rsa-cert-v01@openssh.com

# Flags for all test-cluster hosts except the proxy
Host *.test-cluster !localhost
    Port 3022
    ProxyCommand "/bin/tsh" proxy ssh --cluster=test-cluster --proxy=localhost:3080 %r@%h:%p

# End generated Teleport configuration
`

	var sb strings.Builder
	err := writeSSHConfig(&sb, &openssh.SSHConfigParameters{
		AppName:             "tsh",
		ClusterNames:        []string{"test-cluster"},
		KnownHostsPath:      "/tmp/know_host",
		IdentityFilePath:    "/tmp/alice",
		CertificateFilePath: "/tmp/localhost-cert.pub",
		ProxyHost:           "localhost",
		ProxyPort:           "3080",
		ExecutablePath:      "/bin/tsh",
	}, func() (*semver.Version, error) {
		return semver.New("9.0.0"), nil
	})
	require.NoError(t, err)
	require.Equal(t, want, sb.String())
}
