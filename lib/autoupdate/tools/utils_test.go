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

package tools

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestTeleportPackageURLs(t *testing.T) {
	currentModules := modules.GetModules()
	t.Cleanup(func() { modules.SetModules(currentModules) })
	modules.SetModules(&modulestest.Modules{TestBuildType: modules.BuildCommunity})

	ctx := context.Background()

	type expected struct {
		archivePrefix string
		optional      bool
	}

	for _, tt := range []struct {
		name     string
		version  string
		expected func() []expected
	}{
		{
			name:    "v17",
			version: "17.0.0",
			expected: func() []expected {
				return []expected{{archivePrefix: "/teleport-", optional: false}}
			},
		},
		{
			name:    "v17-latest",
			version: "17.7.2",
			expected: func() []expected {
				if runtime.GOOS == constants.DarwinOS {
					return []expected{{archivePrefix: "/teleport-tools-", optional: false}}
				}
				return []expected{{archivePrefix: "/teleport-", optional: false}}
			},
		},
		{
			name:    "v18",
			version: "18.0.0",
			expected: func() []expected {
				return []expected{{archivePrefix: "/teleport-", optional: false}}
			},
		},
		{
			name:    "v18-latest",
			version: "18.1.5",
			expected: func() []expected {
				if runtime.GOOS == constants.DarwinOS {
					return []expected{{archivePrefix: "/teleport-tools-", optional: false}}
				}
				return []expected{{archivePrefix: "/teleport-", optional: false}}
			},
		},
		{
			name:    "v19",
			version: "19.0.0",
			expected: func() []expected {
				if runtime.GOOS == constants.DarwinOS {
					return []expected{{archivePrefix: "/teleport-tools-", optional: false}}
				}
				return []expected{{archivePrefix: "/teleport-", optional: false}}
			},
		},
		{
			name:    "v16",
			version: "16.0.0",
			expected: func() []expected {
				if runtime.GOOS == constants.DarwinOS {
					return []expected{
						{archivePrefix: "/teleport-", optional: false},
						{archivePrefix: "/tsh-", optional: true},
					}
				}
				return []expected{{archivePrefix: "/teleport-", optional: false}}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pkgURLs, err := teleportPackageURLs(ctx, autoupdate.DefaultCDNURITemplate, "", tt.version)
			require.NoError(t, err)

			exp := tt.expected()
			require.Len(t, pkgURLs, len(exp))
			for i := range pkgURLs {
				assert.True(t, strings.HasPrefix(pkgURLs[i].Archive, exp[i].archivePrefix))
				assert.Equal(t, exp[i].optional, pkgURLs[i].Optional)
				assert.Equal(t, tt.version, pkgURLs[i].Version)
			}
		})
	}
}

// TestFilterEnv verifies excluding environment variables by the list of the keys.
func TestFilterEnv(t *testing.T) {
	env := "TEST_ENV_WITHOUT_FILTER"
	env1 := "TEST_ENV_WITHOUT_FILTER1"
	env2 := "TEST_ENV_WITHOUT_FILTER2"
	env3 := "TEST_ENV_WITHOUT_FILTER3"

	source := []string{
		env3,
		env,
		fmt.Sprint(env1, "=", "test"),
		fmt.Sprint(teleportToolsVersionEnv, "=", teleportToolsVersionEnvDisabled),
		fmt.Sprint(env2, "=", "test"),
		env3,
		env,
		env3,
	}

	assert.Equal(t, []string{
		env,
		fmt.Sprint(env1, "=", "test"),
		fmt.Sprint(env2, "=", "test"),
		env,
	}, filterEnvs(source, []string{teleportToolsVersionEnv, env3}))
}
