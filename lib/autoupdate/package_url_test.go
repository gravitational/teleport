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

package autoupdate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInstallFlagsYAML(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		yaml     string
		flags    InstallFlags
		skipYAML bool
	}{
		{
			name:  "both",
			yaml:  `["Enterprise", "FIPS"]`,
			flags: FlagEnterprise | FlagFIPS,
		},
		{
			name:     "order",
			yaml:     `["FIPS", "Enterprise"]`,
			flags:    FlagEnterprise | FlagFIPS,
			skipYAML: true,
		},
		{
			name:     "extra",
			yaml:     `["FIPS", "Enterprise", "bad"]`,
			flags:    FlagEnterprise | FlagFIPS,
			skipYAML: true,
		},
		{
			name:  "enterprise",
			yaml:  `["Enterprise"]`,
			flags: FlagEnterprise,
		},
		{
			name:  "fips",
			yaml:  `["FIPS"]`,
			flags: FlagFIPS,
		},
		{
			name: "empty",
			yaml: `[]`,
		},
		{
			name:     "nil",
			skipYAML: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var flags InstallFlags
			err := yaml.Unmarshal([]byte(tt.yaml), &flags)
			require.NoError(t, err)
			require.Equal(t, tt.flags, flags)

			// verify test YAML
			var v any
			err = yaml.Unmarshal([]byte(tt.yaml), &v)
			require.NoError(t, err)
			res, err := yaml.Marshal(v)
			require.NoError(t, err)

			// compare verified YAML to flag output
			out, err := yaml.Marshal(flags)
			require.NoError(t, err)

			if !tt.skipYAML {
				require.Equal(t, string(res), string(out))
			}
		})
	}
}
