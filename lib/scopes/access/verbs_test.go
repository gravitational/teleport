// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package access

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestIsAllowedScopedRule_WorkloadIdentity asserts that scoped roles may grant
// the verb set defined for the workload_identity kind in RFD 0229c
// (create, update, delete, list, read_no_secrets), and that the secret-bearing
// "read" verb and unsupported verbs are rejected.
func TestIsAllowedScopedRule_WorkloadIdentity(t *testing.T) {
	allowed := []string{
		types.VerbCreate,
		types.VerbUpdate,
		types.VerbDelete,
		types.VerbList,
		types.VerbReadNoSecrets,
	}
	for _, verb := range allowed {
		require.Truef(t, isAllowedScopedRule(types.KindWorkloadIdentity, verb),
			"expected verb %q to be allowed for workload_identity", verb)
	}

	disallowed := []string{
		types.VerbRead,
		types.Wildcard,
		types.VerbRotate,
	}
	for _, verb := range disallowed {
		require.Falsef(t, isAllowedScopedRule(types.KindWorkloadIdentity, verb),
			"expected verb %q to be disallowed for workload_identity", verb)
	}
}
