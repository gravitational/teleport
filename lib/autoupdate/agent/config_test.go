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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/autoupdate"
)

func TestNewRevisionFromDir(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		dir      string
		rev      Revision
		errMatch string
	}{
		{
			name: "version",
			dir:  "1.2.3",
			rev: Revision{
				Version: "1.2.3",
			},
		},
		{
			name: "full",
			dir:  "1.2.3_ent_fips",
			rev: Revision{
				Version: "1.2.3",
				Flags:   autoupdate.FlagEnterprise | autoupdate.FlagFIPS,
			},
		},
		{
			name: "ent",
			dir:  "1.2.3_ent",
			rev: Revision{
				Version: "1.2.3",
				Flags:   autoupdate.FlagEnterprise,
			},
		},
		{
			name:     "empty",
			errMatch: "missing",
		},
		{
			name:     "trailing",
			dir:      "1.2.3_",
			errMatch: "invalid",
		},
		{
			name:     "more trailing",
			dir:      "1.2.3___",
			errMatch: "invalid",
		},
		{
			name:     "no version",
			dir:      "_fips",
			errMatch: "missing",
		},
		{
			name:     "fips no ent",
			dir:      "1.2.3_fips",
			errMatch: "invalid",
		},
		{
			name:     "unknown start fips",
			dir:      "1.2.3_test_fips",
			errMatch: "invalid",
		},
		{
			name:     "unknown start ent",
			dir:      "1.2.3_test_ent",
			errMatch: "invalid",
		},
		{
			name:     "unknown end fips",
			dir:      "1.2.3_fips_test",
			errMatch: "invalid",
		},
		{
			name:     "unknown end ent",
			dir:      "1.2.3_ent_test",
			errMatch: "invalid",
		},
		{
			name:     "bad order",
			dir:      "1.2.3_fips_ent",
			errMatch: "invalid",
		},
		{
			name:     "underscore",
			dir:      "_",
			errMatch: "missing",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rev, err := NewRevisionFromDir(tt.dir)
			if tt.errMatch != "" {
				require.ErrorContains(t, err, tt.errMatch)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.rev, rev)
			require.Equal(t, tt.dir, rev.Dir())
		})
	}
}

func TestValidateConfigSpec(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		config   UpdateSpec
		override UpdateSpec
		result   UpdateSpec
		errMatch string
	}{
		{
			name: "overrides",
			config: UpdateSpec{
				Proxy:   "proxy",
				Path:    "/path",
				Group:   "group",
				BaseURL: "https://example.com",
			},
			override: UpdateSpec{
				Enabled: true,
				Pinned:  true,
				Proxy:   "overrideProxy",
				Path:    "/overridePath",
				Group:   "group2",
				BaseURL: "https://example.com",
			},
			result: UpdateSpec{
				Enabled: true,
				Pinned:  true,
				Proxy:   "overrideProxy",
				Path:    "/overridePath",
				Group:   "group2",
				BaseURL: "https://example.com",
			},
		},
		{
			name: "default overrides",
			config: UpdateSpec{
				Proxy:   "proxy",
				Path:    "/path",
				Group:   "group",
				BaseURL: "https://example.com",
			},
			override: UpdateSpec{
				Proxy:   "default",
				Path:    "default",
				Group:   "default",
				BaseURL: "default",
			},
			result: UpdateSpec{
				Proxy: "default",
				Path:  "default",
			},
		},
		{
			name: "only overrides",
			override: UpdateSpec{
				Enabled: true,
				Pinned:  true,
				Proxy:   "overrideProxy",
				Path:    "/overridePath",
				Group:   "group2",
				BaseURL: "https://example.com",
			},
			result: UpdateSpec{
				Enabled: true,
				Pinned:  true,
				Proxy:   "overrideProxy",
				Path:    "/overridePath",
				Group:   "group2",
				BaseURL: "https://example.com",
			},
		},
		{
			name: "no overrides",
			config: UpdateSpec{
				Proxy:   "proxy",
				Path:    "/path",
				Group:   "group",
				BaseURL: "https://example.com",
			},
			result: UpdateSpec{
				Proxy:   "proxy",
				Path:    "/path",
				Group:   "group",
				BaseURL: "https://example.com",
			},
		},
		{
			name: "BaseURL validation fails",
			override: UpdateSpec{
				BaseURL: "http://example.com",
			},
			errMatch: "must use TLS",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := updateConfigSpec(&tt.config, OverrideConfig{UpdateSpec: tt.override})
			if tt.errMatch != "" {
				require.ErrorContains(t, err, tt.errMatch)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.result, tt.config)
		})
	}
}
