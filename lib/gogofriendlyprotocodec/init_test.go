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

package gogofriendlyprotocodec_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/encoding"

	"github.com/gravitational/teleport/lib/gogofriendlyprotocodec"
)

func TestInitOrder(t *testing.T) {
	require.Nil(t, encoding.GetCodec("proto"))
	require.IsType(t, gogofriendlyprotocodec.ExportedCodecV2{}, encoding.GetCodecV2("proto"))
}
