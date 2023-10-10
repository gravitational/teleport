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

package utils

import "testing"

func TestMeetsVersion_emptyOrInvalid(t *testing.T) {
	// See TestVersions for more comprehensive tests.

	if !MeetsVersion("", "v1.2.3") {
		t.Error("MeetsVersion with an empty gotVer should always succeed")
	}

	if !MeetsVersion("banana", "v1.2.3") {
		t.Error("MeetsVersion with an invalid version should always succeed")
	}
	if !MeetsVersion("v1.2.3", "banana") {
		t.Error("MeetsVersion with an invalid version should always succeed")
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
		tt := tt
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
