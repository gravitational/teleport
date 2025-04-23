// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package objects

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadEnvVar(t *testing.T) {
	testCases := []struct {
		name             string
		envValue         string
		expectedFound    bool
		expectedDuration time.Duration
	}{
		{"empty", "", false, 0},
		{"never", "never", true, 0},
		{"valid duration", "1h", true, time.Hour},
		{"invalid duration", "invalid", true, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEST_ENV_VAR", tc.envValue)
			c := &Config{Log: slog.Default()}

			found, duration := c.loadEnvVar(context.Background(), "TEST_ENV_VAR")

			require.Equal(t, tc.expectedFound, found)
			require.Equal(t, tc.expectedDuration, duration)
		})
	}
}

func TestLoadEnvVarOverrides(t *testing.T) {
	testCases := []struct {
		name              string
		envVars           map[string]string
		expectedScanInt   time.Duration
		expectedObjectTTL time.Duration
		expectedRefreshTh time.Duration
	}{
		{
			name:              "no overrides",
			envVars:           map[string]string{},
			expectedScanInt:   0,
			expectedObjectTTL: 0,
			expectedRefreshTh: 0,
		},
		{
			name: "scan interval only",
			envVars: map[string]string{
				"TELEPORT_UNSTABLE_DB_OBJECTS_SCAN_INTERVAL": "1h",
			},
			expectedScanInt:   time.Hour,
			expectedObjectTTL: 12 * time.Hour,
			expectedRefreshTh: 3 * time.Hour,
		},
		{
			name: "object ttl only",
			envVars: map[string]string{
				"TELEPORT_UNSTABLE_DB_OBJECTS_OBJECT_TTL": "2h",
			},
			expectedScanInt:   0,
			expectedObjectTTL: 2 * time.Hour,
			expectedRefreshTh: 0,
		},
		{
			name: "refresh threshold only",
			envVars: map[string]string{
				"TELEPORT_UNSTABLE_DB_OBJECTS_REFRESH_THRESHOLD": "3h",
			},
			expectedScanInt:   0,
			expectedObjectTTL: 0,
			expectedRefreshTh: 3 * time.Hour,
		},
		{
			name: "all overrides",
			envVars: map[string]string{
				"TELEPORT_UNSTABLE_DB_OBJECTS_SCAN_INTERVAL":     "1h",
				"TELEPORT_UNSTABLE_DB_OBJECTS_OBJECT_TTL":        "2h",
				"TELEPORT_UNSTABLE_DB_OBJECTS_REFRESH_THRESHOLD": "3h",
			},
			expectedScanInt:   time.Hour,
			expectedObjectTTL: 2 * time.Hour,
			expectedRefreshTh: 3 * time.Hour,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			c := &Config{Log: slog.Default()}
			c.loadEnvVarOverrides(context.Background())

			require.Equal(t, tc.expectedScanInt, c.ScanInterval)
			require.Equal(t, tc.expectedObjectTTL, c.ObjectTTL)
			require.Equal(t, tc.expectedRefreshTh, c.RefreshThreshold)
		})
	}
}

func TestConfigDisabled(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *Config
		expected bool
	}{
		{
			name: "default config",
			cfg: &Config{
				ScanInterval:     time.Minute * 15,
				ObjectTTL:        time.Minute * 180,
				RefreshThreshold: time.Minute * 45,
			},
			expected: false,
		},
		{
			name: "disabled through object TTL",
			cfg: &Config{
				ScanInterval:     time.Minute * 15,
				ObjectTTL:        0,
				RefreshThreshold: time.Minute * 45,
			},
			expected: true,
		},
		{
			name: "disabled through scan interval",
			cfg: &Config{
				ScanInterval:     0,
				ObjectTTL:        time.Minute * 180,
				RefreshThreshold: time.Minute * 45,
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.cfg.disabled())
		})
	}
}
