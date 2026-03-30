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

package reexec

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

func TestReadChildError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		readErr    error
		stderrIn   string
		wantStderr string
	}{
		{
			name:       "empty stderr",
			stderrIn:   "",
			wantStderr: "",
		},
		{
			name:       "has stderr",
			stderrIn:   "Failed to launch: test error.\r\n",
			wantStderr: "Failed to launch: test error.\r\n",
		},
		{
			name:       "stderr at max read limit",
			stderrIn:   strings.Repeat("a", maxRead),
			wantStderr: strings.Repeat("a", maxRead),
		},
		{
			name:       "stderr over max read limit is truncated",
			stderrIn:   strings.Repeat("a", maxRead) + "b",
			wantStderr: strings.Repeat("a", maxRead),
		},
		{
			name:    "read error",
			readErr: errors.New("read failure"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.readErr != nil {
				_, err := ReadChildError(iotest.ErrReader(tt.readErr))
				require.ErrorIs(t, err, tt.readErr)
				return
			}

			got, err := ReadChildError(strings.NewReader(tt.stderrIn))
			require.NoError(t, err)
			require.Equal(t, tt.wantStderr, got)
		})
	}
}
