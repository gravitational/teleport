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

	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestTeleportProcessIntegrationsOnly(t *testing.T) {
	for _, tt := range []struct {
		name              string
		inputFeatureCloud bool
		inputAuthEnabled  bool
		integrationOnly   bool
	}{
		{
			name:              "self-hosted",
			inputFeatureCloud: false,
			inputAuthEnabled:  false,
			integrationOnly:   false,
		},
		{
			name:              "cloud but discovery service is not running side-by-side with Auth",
			inputFeatureCloud: false,
			inputAuthEnabled:  true,
			integrationOnly:   false,
		},
		{
			name:              "cloud and discovery service is not running side-by-side with Auth",
			inputFeatureCloud: false,
			inputAuthEnabled:  true,
			integrationOnly:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := TeleportProcess{
				Config: &servicecfg.Config{
					Auth: servicecfg.AuthConfig{
						Enabled: tt.inputAuthEnabled,
					},
				},
			}

			modules.SetTestModules(t, &modules.TestModules{TestFeatures: modules.Features{
				Cloud: tt.inputFeatureCloud,
			}})

			require.Equal(t, tt.integrationOnly, p.integrationOnlyCredentials())
		})
	}
}
