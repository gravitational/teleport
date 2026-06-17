/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package installer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoinFailureErrorString(t *testing.T) {
	t.Parallel()
	joinFailureMessage := fmt.Sprintf("node did not become ready (join cluster) within %s", JoinFailureTimeout)

	tests := []struct {
		name     string
		err      *JoinFailureError
		contains []string
	}{
		{
			name: "all fields populated",
			err: &JoinFailureError{
				Message:            "node did not become ready (join cluster) within 10s",
				ServiceDiagnostics: `systemd service state: ActiveState="failed", SubState="exited", Result="exit-code"`,
				JournalOutput:      "error: token expired",
			},
			contains: []string{
				"node did not become ready (join cluster) within 10s",
				`ActiveState="failed"`,
				"Journal output:\nerror: token expired",
				"agent failed to join the cluster",
			},
		},
		{
			name: "no journal output",
			err: &JoinFailureError{
				Message:            joinFailureMessage,
				ServiceDiagnostics: "systemd service state: unavailable",
			},
			contains: []string{
				joinFailureMessage,
				"systemd service state: unavailable",
				"agent failed to join the cluster",
			},
		},
		{
			name: "message only",
			err: &JoinFailureError{
				Message: "node did not become ready (join cluster) within 10s",
			},
			contains: []string{
				"node did not become ready (join cluster) within 10s",
				"agent failed to join the cluster",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.contains {
				require.Contains(t, got, want)
			}
		})
	}
}
