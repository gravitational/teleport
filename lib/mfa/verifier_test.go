// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mfa_test

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authcatest"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/mfa"
)

const testCluster = "test-cluster"

func TestVerifier(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClock()
	ca := newTestCA(t)

	v, err := mfa.NewVerifier(ca, fakeClock)
	require.NoError(t, err)

	for _, tc := range []struct {
		name    string
		token   string
		user    string
		advance time.Duration
		wantErr bool
	}{
		{
			name:  "valid token",
			token: signToken(t, ca, "alice", fakeClock),
			user:  "alice",
		},
		{
			name:    "expired token",
			token:   signToken(t, ca, "alice", fakeClock),
			user:    "alice",
			advance: 6 * time.Minute,
			wantErr: true,
		},
		{
			name:    "sub mismatch",
			token:   signToken(t, ca, "alice", fakeClock),
			user:    "bob",
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			user:    "alice",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeClock.Advance(tc.advance)

			got, err := v.Verify(tc.token, tc.user)

			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, got)

				return
			}

			require.NoError(t, err)
			require.Equal(t, "alice", got.Username)
		})
	}
}

func newTestCA(t *testing.T) *types.CertAuthorityV2 {
	t.Helper()

	ca, err := authcatest.NewCA(types.InBandCA, testCluster)
	require.NoError(t, err)

	return ca
}

func signToken(t *testing.T, ca *types.CertAuthorityV2, username string, clock clockwork.Clock) string {
	t.Helper()

	pairs := ca.GetTrustedJWTKeyPairs()
	require.Len(t, pairs, 1)

	priv, err := keys.ParsePrivateKey(pairs[0].PrivateKey)
	require.NoError(t, err)

	key, err := jwt.New(&jwt.Config{
		Clock:       clock,
		ClusterName: testCluster,
		PrivateKey:  priv,
	})
	require.NoError(t, err)

	token, err := key.Sign(jwt.SignParams{
		Issuer:   testCluster,
		Username: username,
		URI:      testCluster,
		Expires:  clock.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)

	return token
}
