/*
Copyright 2022 Gravitational, Inc.

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
