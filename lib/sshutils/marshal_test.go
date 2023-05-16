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
)

func TestMarshalKnownHost(t *testing.T) {
	var file [][]byte
	for _, kh := range knownHosts {
		line, err := MarshalKnownHost(kh)
		require.NoError(t, err)
		file = append(file, []byte(line))
	}
	require.Equal(t, knownHostsFile, file)
}

func TestUnmarshalKnownHosts(t *testing.T) {
	parsedKnownHosts, err := UnmarshalKnownHosts(knownHostsFile)
	require.NoError(t, err)
	for i, parsed := range parsedKnownHosts {
		require.Equal(t, knownHosts[i].AuthorizedKey, parsed.AuthorizedKey)
		require.Equal(t, knownHosts[i].ProxyHost, parsed.ProxyHost)
		require.Equal(t, knownHosts[i].Hostname, parsed.Hostname)
		for key, val := range knownHosts[i].Comment {
			require.Equal(t, knownHosts[i].Comment[key], val)
		}
	}
}

var knownHosts = []KnownHost{
	{
		AuthorizedKey: []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEk4cVIiydp9xSPIb8UqXpShY8zPlk/lpR69UL+0+RnNXtQl7GcQUZsrXDB2gOCfj+doKZj8Pt8oQVSDJF/vKhr+KS2Z+LC2Gyt8D5IY/acyyhSN5VoIo0JzIOr5CPGJNpLChREFuveV30hLihSfY52cqSvu7N5u34BlZ29WTLeBD9WssAG5HZUES8Xo3neHBl4SOck+mdiUvOIPhcnPiYRmYltOI3GJRu5y1xGemoPU3MnMziQMqnKCc2+To6IC8CkeQqa8D//BxLjenjSgn1K/SLUHraMb5qCmf77fyshj6A9jamgo0UOaOqem+jyg8idnz6JbVfXwW0nEaSyPzX\n"),
		ProxyHost:     "proxy.example.com",
		Hostname:      "cluster1",
		Comment: map[string][]string{
			"logins": {"root"},
		},
	}, {
		AuthorizedKey: []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEk4cVIiydp9xSPIb8UqXpShY8zPlk/lpR69UL+0+RnNXtQl7GcQUZsrXDB2gOCfj+doKZj8Pt8oQVSDJF/vKhr+KS2Z+LC2Gyt8D5IY/acyyhSN5VoIo0JzIOr5CPGJNpLChREFuveV30hLihSfY52cqSvu7N5u34BlZ29WTLeBD9WssAG5HZUES8Xo3neHBl4SOck+mdiUvOIPhcnPiYRmYltOI3GJRu5y1xGemoPU3MnMziQMqnKCc2+To6IC8CkeQqa8D//BxLjenjSgn1K/SLUHraMb5qCmf77fyshj6A9jamgo0UOaOqem+jyg8idnz6JbVfXwW0nEaSyPzX\n"),
		ProxyHost:     "proxy.example.com",
		Hostname:      "cluster2",
	}, {
		AuthorizedKey: []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEk4cVIiydp9xSPIb8UqXpShY8zPlk/lpR69UL+0+RnNXtQl7GcQUZsrXDB2gOCfj+doKZj8Pt8oQVSDJF/vKhr+KS2Z+LC2Gyt8D5IY/acyyhSN5VoIo0JzIOr5CPGJNpLChREFuveV30hLihSfY52cqSvu7N5u34BlZ29WTLeBD9WssAG5HZUES8Xo3neHBl4SOck+mdiUvOIPhcnPiYRmYltOI3GJRu5y1xGemoPU3MnMziQMqnKCc2+To6IC8CkeQqa8D//BxLjenjSgn1K/SLUHraMb5qCmf77fyshj6A9jamgo0UOaOqem+jyg8idnz6JbVfXwW0nEaSyPzX\n"),
		Hostname:      "cluster3",
	},
}

var knownHostsFile = [][]byte{
	[]byte("@cert-authority proxy.example.com,cluster1,*.cluster1 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEk4cVIiydp9xSPIb8UqXpShY8zPlk/lpR69UL+0+RnNXtQl7GcQUZsrXDB2gOCfj+doKZj8Pt8oQVSDJF/vKhr+KS2Z+LC2Gyt8D5IY/acyyhSN5VoIo0JzIOr5CPGJNpLChREFuveV30hLihSfY52cqSvu7N5u34BlZ29WTLeBD9WssAG5HZUES8Xo3neHBl4SOck+mdiUvOIPhcnPiYRmYltOI3GJRu5y1xGemoPU3MnMziQMqnKCc2+To6IC8CkeQqa8D//BxLjenjSgn1K/SLUHraMb5qCmf77fyshj6A9jamgo0UOaOqem+jyg8idnz6JbVfXwW0nEaSyPzX logins=root&type=host\n"),
	[]byte("@cert-authority proxy.example.com,cluster2,*.cluster2 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEk4cVIiydp9xSPIb8UqXpShY8zPlk/lpR69UL+0+RnNXtQl7GcQUZsrXDB2gOCfj+doKZj8Pt8oQVSDJF/vKhr+KS2Z+LC2Gyt8D5IY/acyyhSN5VoIo0JzIOr5CPGJNpLChREFuveV30hLihSfY52cqSvu7N5u34BlZ29WTLeBD9WssAG5HZUES8Xo3neHBl4SOck+mdiUvOIPhcnPiYRmYltOI3GJRu5y1xGemoPU3MnMziQMqnKCc2+To6IC8CkeQqa8D//BxLjenjSgn1K/SLUHraMb5qCmf77fyshj6A9jamgo0UOaOqem+jyg8idnz6JbVfXwW0nEaSyPzX type=host\n"),
	[]byte("@cert-authority cluster3,*.cluster3 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDEk4cVIiydp9xSPIb8UqXpShY8zPlk/lpR69UL+0+RnNXtQl7GcQUZsrXDB2gOCfj+doKZj8Pt8oQVSDJF/vKhr+KS2Z+LC2Gyt8D5IY/acyyhSN5VoIo0JzIOr5CPGJNpLChREFuveV30hLihSfY52cqSvu7N5u34BlZ29WTLeBD9WssAG5HZUES8Xo3neHBl4SOck+mdiUvOIPhcnPiYRmYltOI3GJRu5y1xGemoPU3MnMziQMqnKCc2+To6IC8CkeQqa8D//BxLjenjSgn1K/SLUHraMb5qCmf77fyshj6A9jamgo0UOaOqem+jyg8idnz6JbVfXwW0nEaSyPzX type=host\n"),
}
