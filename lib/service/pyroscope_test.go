// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/grafana/pyroscope-go"
	"github.com/stretchr/testify/require"
)

func TestPyroscopeConfig(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    pyroscope.Config
	}{
		{
			name:    "No environment vars configured",
			envVars: map[string]string{},
			want: pyroscope.Config{
				Tags: map[string]string{},
			},
		},
		{
			name: "Environment vars configured",
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_PROFILE_CPU_ENABLED":        "true",
				"TELEPORT_PYROSCOPE_PROFILE_GOROUTINES_ENABLED": "true",
				"TELEPORT_PYROSCOPE_PROFILE_MEMORY_ENABLED":     "true",
				"TELEPORT_PYROSCOPE_KUBE_COMPONENT":             "auth",
				"TELEPORT_PYROSCOPE_KUBE_NAMESPACE":             "test-namespace",
			},
			want: pyroscope.Config{
				ProfileTypes: []pyroscope.ProfileType{
					pyroscope.ProfileAllocObjects,
					pyroscope.ProfileAllocSpace,
					pyroscope.ProfileInuseObjects,
					pyroscope.ProfileInuseSpace,
					pyroscope.ProfileCPU,
					pyroscope.ProfileGoroutines,
				},
				Tags: map[string]string{
					"component": "auth",
					"namespace": "test-namespace",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			p := &TeleportProcess{
				logger: slog.Default(),
				Supervisor: &LocalSupervisor{
					exitContext: context.Background(),
				},
			}
			got, _ := createPyroscopeConfig(p.ExitContext(), "127.0.0.1")

			require.Equal(t, tt.want.ProfileTypes, got.ProfileTypes)
			require.Subset(t, got.Tags, tt.want.Tags)
		})
	}
}
