// Copyright 2024 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGithubAuthRequestCheck(t *testing.T) {
	tests := []struct {
		request *GithubAuthRequest
		check   require.ErrorAssertionFunc
	}{
		{
			request: &GithubAuthRequest{
				ConnectorID: "valid",
				StateToken:  "state-token",
			},
			check: require.NoError,
		},
		{
			request: &GithubAuthRequest{
				ConnectorID:   "invalid-connector-spec-set-for-regular-flow",
				StateToken:    "state-token",
				ConnectorSpec: &GithubConnectorSpecV3{},
			},
			check: require.Error,
		},
		{
			request: &GithubAuthRequest{
				ConnectorID:   "sso-test",
				StateToken:    "state-token",
				SSOTestFlow:   true,
				ConnectorSpec: &GithubConnectorSpecV3{},
			},
			check: require.NoError,
		},
		{
			request: &GithubAuthRequest{
				ConnectorID: "connector-spec-missing-for-sso-test",
				StateToken:  "state-token",
				SSOTestFlow: true,
			},
			check: require.Error,
		},
		{
			request: &GithubAuthRequest{
				ConnectorID:       "authenticated-user",
				StateToken:        "state-token",
				AuthenticatedUser: "alice",
				ConnectorSpec:     &GithubConnectorSpecV3{},
			},
			check: require.NoError,
		},
		{
			request: &GithubAuthRequest{
				ConnectorID:       "connector-spec-missing-for-authenticated-user",
				StateToken:        "state-token",
				AuthenticatedUser: "alice",
			},
			check: require.Error,
		},
		{
			request: &GithubAuthRequest{
				ConnectorID:  "both-new-and-deprecated-keys-are-set",
				StateToken:   "state-token",
				PublicKey:    []byte("deprecated"),
				SshPublicKey: []byte("ssh-key"),
				TlsPublicKey: []byte("tls-key"),
			},
			check: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.request.ConnectorID, func(t *testing.T) {
			test.check(t, test.request.Check())
		})
	}
}
