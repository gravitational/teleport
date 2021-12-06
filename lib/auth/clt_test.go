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

package auth

import (
	"crypto/tls"
	"testing"
	"time"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/session"
	"github.com/stretchr/testify/require"
)

func TestClient_DialTimeout(t *testing.T) {
	cases := []struct {
		desc    string
		timeout time.Duration
	}{
		{
			desc:    "dial timeout set to valid value",
			timeout: 1 * time.Second,
		},
		{
			desc: "defaults prevent infinite timeout",
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			// create a client that will attempt to connect a blackholed address. The address is reserved
			// for benchmarking by RFC 6890.
			clt, err := NewClient(apiclient.Config{
				DialTimeout: tt.timeout,
				Addrs:       []string{"198.18.0.254:1234"},
				Credentials: []apiclient.Credentials{
					apiclient.LoadTLS(&tls.Config{}),
				},
			})
			require.NoError(t, err)

			// try to create a session - this will timeout after the DialTimeout threshold is exceeded
			require.Error(t, clt.CreateSession(session.Session{Namespace: "test"}))
		})
	}
}
