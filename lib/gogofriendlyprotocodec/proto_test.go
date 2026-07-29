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

package gogofriendlyprotocodec

import (
	"testing"

	"github.com/stretchr/testify/require"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
)

type ExportedCodecV2 = codecV2 // exported for init_test.go

var _ gogoMarshaler = (*types.RoleV6)(nil)
var _ gogoUnmarshaler = (*types.RoleV6)(nil)

func TestImplements(t *testing.T) {
	// gogoproto messages should match our type assertions (same check as the
	// static assertion above, for completion)
	require.Implements(t, (*gogoMarshaler)(nil), (*types.RoleV6)(nil))
	require.Implements(t, (*gogoUnmarshaler)(nil), (*types.RoleV6)(nil))

	// any new protobuf codegen should not match the assertions
	require.NotImplements(t, (*gogoMarshaler)(nil), (*presencev1.RelayServer)(nil))
	require.NotImplements(t, (*gogoUnmarshaler)(nil), (*presencev1.RelayServer)(nil))
}
