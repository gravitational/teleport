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

package teleportassets

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/modules"
)

func TestDistrolessTeleportImageRepo(t *testing.T) {
	tests := []struct {
		desc      string
		buildType string
		version   string
		want      string
	}{
		{
			desc:      "ent release",
			buildType: modules.BuildEnterprise,
			version:   "16.0.0",
			want:      "public.ecr.aws/gravitational/teleport-ent-distroless:16.0.0",
		},
		{
			desc:      "oss release",
			buildType: modules.BuildOSS,
			version:   "16.0.0",
			want:      "public.ecr.aws/gravitational/teleport-distroless:16.0.0",
		},
		{
			desc:      "community release",
			buildType: modules.BuildCommunity,
			version:   "16.0.0",
			want:      "public.ecr.aws/gravitational/teleport-distroless:16.0.0",
		},
		{
			desc:      "ent pre-release",
			buildType: modules.BuildEnterprise,
			version:   "16.0.0-alpha.1",
			want:      "public.ecr.aws/gravitational-staging/teleport-ent-distroless:16.0.0-alpha.1",
		},
		{
			desc:      "oss pre-release",
			buildType: modules.BuildOSS,
			version:   "16.0.0-alpha.1",
			want:      "public.ecr.aws/gravitational-staging/teleport-distroless:16.0.0-alpha.1",
		},
		{
			desc:      "community pre-release",
			buildType: modules.BuildCommunity,
			version:   "16.0.0-alpha.1",
			want:      "public.ecr.aws/gravitational-staging/teleport-distroless:16.0.0-alpha.1",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			semVer, err := semver.NewVersion(test.version)
			require.NoError(t, err)
			modules.SetTestModules(t, &modules.TestModules{TestBuildType: test.buildType})
			require.Equal(t, test.want, DistrolessImage(*semVer))
		})
	}
}

func Test_cdnBaseURLForVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		artifactVersion string
		teleportVersion string
		want            string
	}{
		{
			name:            "both official releases",
			artifactVersion: "16.3.2",
			teleportVersion: "16.1.0",
			want:            TeleportReleaseCDN,
		},
		{
			name:            "both pre-releases",
			artifactVersion: "16.3.2-dev.1",
			teleportVersion: "16.1.0-foo.25",
			want:            teleportPreReleaseCDN,
		},
		{
			name:            "official teleport should not be able to install pre-release artifacts",
			artifactVersion: "16.3.2-dev.1",
			teleportVersion: "16.1.0",
			want:            TeleportReleaseCDN,
		},
		{
			name:            "pre-release teleport should be able to install official artifacts",
			artifactVersion: "16.3.2",
			teleportVersion: "16.1.0-dev.1",
			want:            TeleportReleaseCDN,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test setup: parse version.
			av, err := semver.NewVersion(tt.artifactVersion)
			require.NoError(t, err)
			tv, err := semver.NewVersion(tt.teleportVersion)
			require.NoError(t, err)

			// Test execution and validation.
			require.Equal(t, tt.want, cdnBaseURLForVersion(av, tv))
		})
	}
}
