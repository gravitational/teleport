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

package approval

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestDecisionDenied(t *testing.T) {
	d := Decision{Approved: false, Approver: "ai-moderator", Reason: "blocked", Mode: ModeAI}
	require.False(t, d.Approved)
	require.Equal(t, "ai-moderator", d.Approver)
	require.Equal(t, ModeAI, d.Mode)
}

func TestCommandRequestKind(t *testing.T) {
	r := CommandRequest{Command: "ls", Kind: types.SSHSessionKind}
	require.Equal(t, types.SSHSessionKind, r.Kind)
}
