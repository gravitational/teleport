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

// Tests symmetric equality of key equality.
func TestSSHKeysEqual(t *testing.T) {
	rsaKey1, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQClwXUKOp/S4XEtFjgr8mfaCy4OyI7N9ZMibdCGxvk2VHP9+Vn8Al1lUSVwuBxHI7EHiq42RCTBetIpTjzn6yiPNAeGNL5cfl9i6r+P5k7og1hz+2oheWveGodx6Dp+Z4o2dw65NGf5EPaotXF8AcHJc3+OiMS5yp/x2A3tu2I1SPQ6dtPa067p8q1L49BKbFwrFRBCVwkr6kpEQAIjnMESMPGD5Buu/AtyAdEZQSLTt8RZajJZDfXFKMEtQm2UF248NFl3hSMAcbbTxITBbZxX7THbwQz22Yuw7422G5CYBPf6WRXBY84Rs6jCS4I4GMxj+3rF4mGtjvuz0wOE32s3w4eMh9h3bPuEynufjE8henmPCIW49+kuZO4LZut7Zg5BfVDQnZYclwokEIMz+gR02YpyflxQOa98t/0mENu+t4f0LNAdkQEBpYtGKKDth5kLphi2Sdi9JpGO2sTivlxMsGyBqdd0wT9VwQpWf4wro6t09HdZJX1SAuEi/0tNI10= friel@test"))
	require.NoError(t, err)
	// Same as above, but different comment
	rsaKey1Alt, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQClwXUKOp/S4XEtFjgr8mfaCy4OyI7N9ZMibdCGxvk2VHP9+Vn8Al1lUSVwuBxHI7EHiq42RCTBetIpTjzn6yiPNAeGNL5cfl9i6r+P5k7og1hz+2oheWveGodx6Dp+Z4o2dw65NGf5EPaotXF8AcHJc3+OiMS5yp/x2A3tu2I1SPQ6dtPa067p8q1L49BKbFwrFRBCVwkr6kpEQAIjnMESMPGD5Buu/AtyAdEZQSLTt8RZajJZDfXFKMEtQm2UF248NFl3hSMAcbbTxITBbZxX7THbwQz22Yuw7422G5CYBPf6WRXBY84Rs6jCS4I4GMxj+3rF4mGtjvuz0wOE32s3w4eMh9h3bPuEynufjE8henmPCIW49+kuZO4LZut7Zg5BfVDQnZYclwokEIMz+gR02YpyflxQOa98t/0mENu+t4f0LNAdkQEBpYtGKKDth5kLphi2Sdi9JpGO2sTivlxMsGyBqdd0wT9VwQpWf4wro6t09HdZJX1SAuEi/0tNI10= other@foo"))
	require.NoError(t, err)
	rsaKey2, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCxNlAqagvmYJSKcSZeTfAJj8ch/wKeTuwOjUOvuudXelAl6ZNslu2a9IrRyu0SIKtHSxgJG7BPuz4bPSkh7/unjCDIRjuxzIEyA2Dud+MDG2QgFrgQqSuDQS/1CJwlqm9i/UdlbrQOrkX7dwIoY2f+bzW1JR3tLjhCQkCZzkViGkY3ELew+Pdu+aagS/BeR+fJNLKvGx8NwAFjZe/YoTJ+vwDNhRP2NyDe1ISSX9PWGvqy4cCQ3WBOsuocpX+XR01tTuxVFsw5fQ+U4vb52aIDUv5VYU5ioG9TLpR1HL9Lu8l0mDI1lGtzHl0/uKEyAFUghyD0ow25vLQzkG/3bjOla6ehQKvSrXly5TbGed+QfphK27hVVctnehO8PyP3ANJK4bzy3fKdq7EmzoGwBIc2QvX3RjdzWJw37kNw44cAsEYVJRJdtaFOB9nFvtCRSPYM0CJj081Sjgqwvnd4cGgUfQjGYdQiMmzLA11PryUQ6ahZVeVhu3TE809ZrJI8HIc= friel@test"))
	require.NoError(t, err)

	ed25519Key1, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGtqQKEkGIY5+Bc4EmEv7NeSn6aA7KMl5eiNEAOqwTBl friel@test"))
	require.NoError(t, err)
	ed25519Key1Alt, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGtqQKEkGIY5+Bc4EmEv7NeSn6aA7KMl5eiNEAOqwTBl other@foo"))
	require.NoError(t, err)
	ed25519Key2, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAhOF1Yw9LyTcIM1ku2hrqYcJ4e+784zp2XX4oIAWRuZ friel@test"))
	require.NoError(t, err)

	// The key material says this is RSA, the prefix is irrelevant.
	disguisedRsaKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 AAAAB3NzaC1yc2EAAAADAQABAAABgQClwXUKOp/S4XEtFjgr8mfaCy4OyI7N9ZMibdCGxvk2VHP9+Vn8Al1lUSVwuBxHI7EHiq42RCTBetIpTjzn6yiPNAeGNL5cfl9i6r+P5k7og1hz+2oheWveGodx6Dp+Z4o2dw65NGf5EPaotXF8AcHJc3+OiMS5yp/x2A3tu2I1SPQ6dtPa067p8q1L49BKbFwrFRBCVwkr6kpEQAIjnMESMPGD5Buu/AtyAdEZQSLTt8RZajJZDfXFKMEtQm2UF248NFl3hSMAcbbTxITBbZxX7THbwQz22Yuw7422G5CYBPf6WRXBY84Rs6jCS4I4GMxj+3rF4mGtjvuz0wOE32s3w4eMh9h3bPuEynufjE8henmPCIW49+kuZO4LZut7Zg5BfVDQnZYclwokEIMz+gR02YpyflxQOa98t/0mENu+t4f0LNAdkQEBpYtGKKDth5kLphi2Sdi9JpGO2sTivlxMsGyBqdd0wT9VwQpWf4wro6t09HdZJX1SAuEi/0tNI10= friel@Zing"))
	require.NoError(t, err)

	// The key material says this is Ed25519, the prefix is irrelevant.
	disguisedEd25519Key, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAIGtqQKEkGIY5+Bc4EmEv7NeSn6aA7KMl5eiNEAOqwTBl friel@test"))
	require.NoError(t, err)

	type test struct {
		name    string
		variant string
		key     ssh.PublicKey
	}

	keys := []test{
		{name: "rsaKey1", key: rsaKey1},
		{name: "rsaKey1", variant: "-alt", key: rsaKey1Alt},
		{name: "rsaKey2", key: rsaKey2},
		{name: "ed25519Key1", variant: "-rsa-prefixed", key: disguisedEd25519Key},
		{name: "ed25519Key1", key: ed25519Key1},
		{name: "ed25519Key1", variant: "-alt", key: ed25519Key1Alt},
		{name: "ed25519Key2", key: ed25519Key2},
		{name: "rsaKey1", variant: "-ed25519-prefixed", key: disguisedRsaKey},
	}

	for _, ak := range keys {
		for _, bk := range keys {
			expected := ak.name == bk.name

			var op string
			if expected {
				op = "=="
			} else {
				op = "!="
			}

			t.Run(fmt.Sprintf("%v%v%v%v%v", ak.name, ak.variant, op, bk.name, bk.variant), func(t *testing.T) {
				actual := KeysEqual(ak.key, bk.key)
				require.Equal(t, expected, actual)
			})
		}
	}
}

func TestSSHMarshalEd25519(t *testing.T) {
	ak, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGtqQKEkGIY5+Bc4EmEv7NeSn6aA7KMl5eiNEAOqwTBl friel@test"))
	require.NoError(t, err)

	bk, _, _, _, err := ssh.ParseAuthorizedKey([]byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGtqQKEkGIY5+Bc4EmEv7NeSn6aA7KMl5eiNEAOqwTBl friel@test"))
	require.NoError(t, err)

	result := KeysEqual(ak, bk)
	require.True(t, result)
}

func TestMatchesWildcard(t *testing.T) {
	// Not a wildcard pattern.
	require.False(t, matchesWildcard("foo.example.com", "example.com"))

	// Not a match.
	require.False(t, matchesWildcard("foo.example.org", "*.example.com"))

	// Too many levels deep.
	require.False(t, matchesWildcard("a.b.example.com", "*.example.com"))

	// Single-part hostnames never match.
	require.False(t, matchesWildcard("example", "*.example.com"))
	require.False(t, matchesWildcard("example", "*.example"))
	require.False(t, matchesWildcard("example", "example"))
	require.False(t, matchesWildcard("example", "*."))

	// Valid wildcard matches.
	require.True(t, matchesWildcard("foo.example.com", "*.example.com"))
	require.True(t, matchesWildcard("bar.example.com", "*.example.com"))
	require.True(t, matchesWildcard("bar.example.com.", "*.example.com"))
	require.True(t, matchesWildcard("bar.foo", "*.foo"))
}
