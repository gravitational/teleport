/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package workloadattest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerAttestorConfig_CheckAndSetDefaults(t *testing.T) {
	validCases := map[string]DockerAttestorConfig{
		"attestor disabled": {Enabled: false, Addr: ""},
		"unix socket":       {Enabled: true, Addr: "unix:///path/to/socket"},
	}
	for name, cfg := range validCases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, cfg.CheckAndSetDefaults())
		})
	}

	t.Run("default addr", func(t *testing.T) {
		cfg := DockerAttestorConfig{Enabled: true, Addr: ""}
		require.NoError(t, cfg.CheckAndSetDefaults())
		require.Equal(t, DefaultDockerAddr, cfg.Addr)
	})

	invalidCases := map[string]struct {
		cfg DockerAttestorConfig
		err string
	}{
		"not a UDS": {
			cfg: DockerAttestorConfig{Enabled: true, Addr: "https://localhost:1234"},
			err: "must be in the form `unix://path/to/socket`",
		},
		"missing path": {
			cfg: DockerAttestorConfig{Enabled: true, Addr: "unix://"},
			err: "must be in the form `unix://path/to/socket`",
		},
		"missing leading slash": {
			cfg: DockerAttestorConfig{Enabled: true, Addr: "unix://path/to/file"},
			err: "host segment must be empty, did you forget a leading slash in the socket path? (i.e. `unix:///path/to/file`)",
		},
	}
	for name, tc := range invalidCases {
		t.Run(name, func(t *testing.T) {
			require.ErrorContains(t, tc.cfg.CheckAndSetDefaults(), tc.err)
		})
	}
}
