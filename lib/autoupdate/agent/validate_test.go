/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package agent

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidator_IsValidBinary(t *testing.T) {
	for _, tt := range []struct {
		name     string
		mode     os.FileMode
		contents string

		valid    bool
		errMatch string
		logMatch string
	}{
		{
			name:     "missing",
			errMatch: "no such",
		},
		{
			name:     "non-executable",
			contents: "test",
			mode:     0666,
			logMatch: "non-executable",
		},
		{
			name:     "shell script",
			contents: "  #!bash  ",
			mode:     0777,
			logMatch: "unexpected shell",
		},
		{
			name:     "unqualified shell script",
			contents: "  #!bash" + string([]byte{0x0B}),
			mode:     0777,
			errMatch: "validating binary",
		},
		{
			name:     "exit 0",
			contents: "#!/bin/sh\nexit 0\n" + string([]byte{0x0B}),
			mode:     0777,
			valid:    true,
		},
		{
			name:     "exit 1",
			contents: "#!/bin/sh\nexit 1\n" + string([]byte{0x0B}),
			mode:     0777,
			valid:    true,
			logMatch: "version command",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			opts := &slog.HandlerOptions{AddSource: true}
			log := slog.New(slog.NewTextHandler(&buf, opts))
			v := Validator{Log: log}
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "file")
			if tt.contents != "" {
				os.WriteFile(path, []byte(tt.contents), tt.mode)
			}
			val, err := v.IsValidBinary(ctx, path)
			if tt.logMatch != "" {
				require.Contains(t, buf.String(), tt.logMatch)
			}
			if tt.errMatch != "" {
				require.Error(t, err)
				require.False(t, val)
				require.Contains(t, err.Error(), tt.errMatch)
				return
			}
			require.Equal(t, tt.valid, val)
			require.NoError(t, err)
		})
	}
}
