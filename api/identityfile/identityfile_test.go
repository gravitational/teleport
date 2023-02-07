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

package identityfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// TestIdentityFileBasics verifies basic profile operations such as
// load/store and setting current.
func TestIdentityFileBasics(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "file")
	writeIDFile := &IdentityFile{
		PrivateKey: []byte("-----BEGIN RSA PRIVATE KEY-----\nkey\n-----END RSA PRIVATE KEY-----\n"),
		Certs: Certs{
			SSH: append([]byte(ssh.CertAlgoRSAv01), '\n'),
			TLS: []byte("-----BEGIN CERTIFICATE-----\ntls-cert\n-----END CERTIFICATE-----\n"),
		},
		CACerts: CACerts{
			SSH: [][]byte{[]byte("@cert-authority ssh-cacerts\n")},
			TLS: [][]byte{[]byte("-----BEGIN CERTIFICATE-----\ntls-cacerts\n-----END CERTIFICATE-----\n")},
		},
	}

	// Write identity file
	err := Write(writeIDFile, path)
	require.NoError(t, err)

	// Read identity file from file
	readIDFile, err := ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, writeIDFile, readIDFile)

	// Read identity file from string
	s, err := os.ReadFile(path)
	require.NoError(t, err)
	fromStringIDFile, err := FromString(string(s))
	require.NoError(t, err)
	require.Equal(t, writeIDFile, fromStringIDFile)
}

func TestIsSSHCert(t *testing.T) {
	for _, tc := range []struct {
		certType   string
		expectBool bool
	}{
		{
			certType:   "opensesame@openssh.com",
			expectBool: false,
		}, {
			certType:   ssh.CertAlgoRSAv01,
			expectBool: true,
		}, {
			certType:   ssh.CertAlgoECDSA256v01,
			expectBool: true,
		}, {
			certType:   ssh.CertAlgoED25519v01,
			expectBool: true,
		},
	} {
		t.Run(tc.certType, func(t *testing.T) {
			certData := append([]byte(tc.certType), []byte(" AAAA...")...)
			isSSHCert := isSSHCert(certData)
			require.Equal(t, tc.expectBool, isSSHCert)
		})
	}
}
