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

package sshutils

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// TestHostKeyCallback verifies that host key callback properly validates
// host certificates.
func TestHostKeyCallback(t *testing.T) {
	ca, err := MakeTestSSHCA()
	require.NoError(t, err)

	realCert, err := MakeRealHostCert(ca)
	require.NoError(t, err)

	spoofedCert, err := MakeSpoofedHostCert(ca)
	require.NoError(t, err)

	hostKeyCallback, err := HostKeyCallback([][]byte{
		[]byte(makeKnownHostsLine("127.0.0.1", ca.PublicKey())),
	}, false)
	require.NoError(t, err)

	err = hostKeyCallback("127.0.0.1:3022", nil, realCert.PublicKey())
	require.NoError(t, err, "host key callback rejected valid host certificate")

	err = hostKeyCallback("127.0.0.1:3022", nil, spoofedCert.PublicKey())
	require.Error(t, err, "host key callback accepted spoofed host certificate")
}

func makeKnownHostsLine(host string, key ssh.PublicKey) string {
	return fmt.Sprintf("%v %v %v", host, key.Type(),
		base64.StdEncoding.EncodeToString(key.Marshal()))
}
