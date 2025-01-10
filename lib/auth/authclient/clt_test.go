/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package authclient

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

func TestClient_DialTimeout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		desc    string
		timeout time.Duration
	}{
		{
			desc:    "dial timeout set to valid value",
			timeout: 500 * time.Millisecond,
		},
		{
			desc:    "defaults prevent infinite timeout",
			timeout: 0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			t.Parallel()

			// create a client that will attempt to connect to a blackholed address. The address is reserved
			// for benchmarking by RFC 6890.
			cfg := apiclient.Config{
				DialTimeout: tt.timeout,
				Addrs:       []string{"198.18.0.254:1234"},
				Credentials: []apiclient.Credentials{
					apiclient.LoadTLS(&tls.Config{}),
				},
				CircuitBreakerConfig: breaker.NoopBreakerConfig(),
			}
			clt, err := NewClient(cfg)
			require.NoError(t, err)

			// call this so that the DialTimeout gets updated, if necessary, so that we know how long to
			// wait before failing this test
			require.NoError(t, cfg.CheckAndSetDefaults())

			errChan := make(chan error, 1)
			go func() {
				// try to create a session - this will timeout after the DialTimeout threshold is exceeded
				_, err := clt.CreateSessionTracker(context.Background(), &types.SessionTrackerV1{})
				errChan <- err
			}()

			select {
			case err := <-errChan:
				require.Error(t, err)
			case <-time.After(cfg.DialTimeout + (cfg.DialTimeout / 2)):
				t.Fatal("Timed out waiting for dial to complete")
			}
		})
	}
}
