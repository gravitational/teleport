/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
