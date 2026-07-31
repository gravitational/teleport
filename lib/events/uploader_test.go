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

package events

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestValidateReplayObjectName covers every object name the beam replay sink
// writes, and the traversal attempts the check exists to stop.
func TestValidateReplayObjectName(t *testing.T) {
	valid := []string{
		"manifest",
		"blob.0",
		"blob.12",
		"index.0",
		"commands.3",
		"agents.0",
		"activities.0",
		"conversations.7",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, ValidateReplayObjectName(name))
		})
	}

	invalid := []string{
		"",
		"manifest.json",
		"manifests",
		"blob",
		"blob.",
		"blob.x",
		"blob.0.0",
		"../other-session.summary.json",
		"blob.0/../../etc/passwd",
		"/manifest",
		"blob.0\n",
		"activities.0 ",
		"summary",
	}
	for _, name := range invalid {
		t.Run("rejects "+name, func(t *testing.T) {
			err := ValidateReplayObjectName(name)
			require.Error(t, err)
			require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
		})
	}
}
