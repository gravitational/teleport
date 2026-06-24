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

package delegation_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/lib/delegation"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestEncodingRoundTrip(t *testing.T) {
	orig := delegationv1.Delegation_builder{
		Bot: delegationv1.BotDelegator_builder{
			Name:  "claude",
			Scope: "/dev",
		}.Build(),
		Previous: delegationv1.Delegation_builder{
			User: delegationv1.UserDelegator_builder{
				Username: "alice",
			}.Build(),
		}.Build(),
	}.Build()

	encoded, err := delegation.Encode(orig)
	require.NoError(t, err)

	decoded, err := delegation.Decode(encoded)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(orig, decoded, protocmp.Transform()))
}
