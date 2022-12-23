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
package sshutils

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/native"
)

func TestMarshalUnmarshalKnownHosts(t *testing.T) {
	priv, err := native.GeneratePrivateKey()
	require.NoError(t, err)
	authorizedKey := priv.MarshalSSHPublicKey()
	knownHosts := []KnownHost{
		{
			AuthorizedKey: authorizedKey,
			ProxyHost:     "proxy.example.com",
			Hostname:      "cluster1",
			Comment: map[string][]string{
				"logins": {"root"},
			},
		}, {
			AuthorizedKey: authorizedKey,
			ProxyHost:     "proxy.example.com",
			Hostname:      "cluster2",
		}, {
			AuthorizedKey: authorizedKey,
			Hostname:      "cluster3",
		},
	}

	var knownHostsFile [][]byte
	for _, kh := range knownHosts {
		line, err := MarshalKnownHost(kh)
		require.NoError(t, err)
		knownHostsFile = append(knownHostsFile, []byte(line))
	}

	parsedKnownHosts, err := UnmarshalKnownHosts(knownHostsFile)
	require.NoError(t, err)
	for i, parsed := range parsedKnownHosts {
		// type comment should default to host
		if knownHosts[i].Comment == nil {
			knownHosts[i].Comment = map[string][]string{"type": {"host"}}
		}
		if knownHosts[i].Comment["type"] == nil {
			knownHosts[i].Comment["type"] = []string{"host"}
		}
		require.Equal(t, knownHosts[i], parsed)
	}
}
