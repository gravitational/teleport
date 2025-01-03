/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestRun_Configure(t *testing.T) {
	t.Parallel()

	// This is slightly rubbish, but due to the global nature of `botfs`,
	// it's difficult to configure the default acl and symlink values to be
	// the same across dev laptops and GCB.
	// If we switch to a more dependency injected model for botfs, we can
	// ensure that the test one returns the same value across operating systems.
	normalizeOSDependentValues := func(data []byte) []byte {
		cpy := append([]byte{}, data...)
		cpy = bytes.ReplaceAll(
			cpy, []byte("symlinks: try-secure"), []byte("symlinks: secure"),
		)
		cpy = bytes.ReplaceAll(
			cpy, []byte(`acls: "off"`), []byte("acls: try"),
		)
		return cpy
	}

	baseArgs := []string{"configure", "--join-method", "token"}
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "no parameters provided",
			args: baseArgs,
		},
		{
			name: "all parameters provided",
			args: append(baseArgs, []string{
				"-a", "example.com",
				"--token", "xxyzz",
				"--ca-pin", "sha256:capindata",
				"--data-dir", "/custom/data/dir",
				"--join-method", "token",
				"--oneshot",
				"--certificate-ttl", "42m",
				"--renewal-interval", "21m",
				"--fips",
			}...),
		},
		{
			name: "all parameters provided",
			args: append(baseArgs, []string{
				"--proxy-server", "proxy.example.com:443",
			}...),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Run("file", func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "config.yaml")
				args := append(tt.args, []string{"-o", path}...)
				err := Run(args, nil)
				require.NoError(t, err)

				data, err := os.ReadFile(path)
				data = normalizeOSDependentValues(data)
				require.NoError(t, err)
				if golden.ShouldSet() {
					golden.Set(t, data)
				}
				require.Equal(t, string(golden.Get(t)), string(data))
			})

			t.Run("stdout", func(t *testing.T) {
				stdout := new(bytes.Buffer)
				err := Run(tt.args, stdout)
				require.NoError(t, err)
				data := normalizeOSDependentValues(stdout.Bytes())
				if golden.ShouldSet() {
					golden.Set(t, data)
				}
				require.Equal(t, string(golden.Get(t)), string(data))
			})
		})
	}
}
