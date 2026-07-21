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

package jointest

import (
	"testing"

	"github.com/stretchr/testify/require"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/tlsca"
)

// RequireScopedHostResult verifies the scoped controls returned by a
// successful host join instead of treating a nil error as sufficient coverage.
func RequireScopedHostResult(t testing.TB, result *joinclient.JoinResult, token *joiningv1.ScopedToken) {
	t.Helper()

	require.NotNil(t, result)
	require.NotNil(t, result.Certs)

	cert, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
	require.NoError(t, err)
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	require.NoError(t, err)

	require.Equal(t, token.GetSpec().GetAssignedScope(), identity.AgentScope)
	require.Equal(t, joining.HashImmutableLabels(token.GetSpec().GetImmutableLabels()), identity.ImmutableLabelHash)
	require.Equal(t, token.GetSpec().GetImmutableLabels().GetSsh(), result.ImmutableLabels.GetSsh())
}
