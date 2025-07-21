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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMeetsMinVersion_emptyOrInvalid(t *testing.T) {
	// See TestMinVersions for more comprehensive tests.

	if !MeetsMinVersion("", "v1.2.3") {
		t.Error("MeetsMinVersion with an empty gotVer should always succeed")
	}

	if !MeetsMinVersion("banana", "v1.2.3") {
		t.Error("MeetsMinVersion with an invalid version should always succeed")
	}
	if !MeetsMinVersion("v1.2.3", "banana") {
		t.Error("MeetsMinVersion with an invalid version should always succeed")
	}
}

func TestMeetsMaxVersion_emptyOrInvalid(t *testing.T) {
	// See TestMaxVersions for more comprehensive tests.

	if !MeetsMaxVersion("", "v1.2.3") {
		t.Error("MeetsMaxVersion with an empty gotVer should always succeed")
	}

	if !MeetsMaxVersion("banana", "v1.2.3") {
		t.Error("banana with an invalid version should always succeed")
	}
	if !MeetsMaxVersion("v1.2.3", "banana") {
		t.Error("MeetsMaxVersion with an invalid version should always succeed")
	}
}

func TestMajorSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  string
		expected string
		want     bool
		wantErr  bool
	}{
		{
			name:     "simple semver",
			version:  "13.4.2",
			expected: "13.0.0",
			want:     true,
		},
		{
			name:     "ignores suffix",
			version:  "13.4.2-dev",
			expected: "13.0.0",
			want:     true,
		},
		{
			name:     "empty version is rejected",
			version:  "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "incorrect version is rejected",
			version:  "13.4", // missing patch
			expected: "13.0.0",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MajorSemver(tt.version)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestMinVerWithoutPreRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		currentVersion string
		minVersion     string
		want           bool
		wantErr        bool
	}{
		{
			name:           "compare equal versions",
			currentVersion: "9.0.0",
			minVersion:     "9.0.0",
			want:           true,
		},
		{
			name:           "compare greater minor versions",
			currentVersion: "9.1.0",
			minVersion:     "9.0.0",
			want:           true,
		},
		{
			name:           "compare greater major versions",
			currentVersion: "10.0.0",
			minVersion:     "9.0.0",
			want:           true,
		},
		{
			name:           "compare lower minor versions",
			currentVersion: "8.6.0",
			minVersion:     "9.0.0",
			want:           false,
		},
		{
			name:           "compare lower major versions",
			currentVersion: "8.0.0",
			minVersion:     "9.0.0",
			want:           false,
		},
		{
			name:           "ignores dev",
			currentVersion: "9.0.0-dev",
			minVersion:     "9.0.0",
			want:           true,
		},
		{
			name:           "ignores beta",
			currentVersion: "9.0.0-beta",
			minVersion:     "9.0.0",
			want:           true,
		},
		{
			name:           "ignores rc",
			currentVersion: "9.0.0-rc",
			minVersion:     "9.0.0",
			want:           true,
		},
		{
			name:           "empty version is rejected",
			currentVersion: "",
			minVersion:     "",
			wantErr:        true,
		},
		{
			name:           "incorrect version is rejected",
			currentVersion: "9.0", // missing patch
			minVersion:     "9.0.0",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := MinVerWithoutPreRelease(tt.currentVersion, tt.minVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("MinVerWithoutPreRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MinVerWithoutPreRelease() got = %v, want %v", got, tt.want)
			}
		})
	}
}
