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

package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestTeleportProcess_shouldInitDatabases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config servicecfg.DatabasesConfig
		want   bool
	}{
		{
			name: "disabled",
			config: servicecfg.DatabasesConfig{
				Enabled: false,
			},
			want: false,
		},
		{
			name: "enabled but no config",
			config: servicecfg.DatabasesConfig{
				Enabled: true,
			},
			want: false,
		},
		{
			name: "enabled with config",
			config: servicecfg.DatabasesConfig{
				Enabled: true,
				Databases: []servicecfg.Database{
					{
						Name: "foo",
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TeleportProcess{
				Config: &servicecfg.Config{
					Databases: tt.config,
				},
			}
			require.Equal(t, tt.want, p.shouldInitDatabases())
		})
	}
}
