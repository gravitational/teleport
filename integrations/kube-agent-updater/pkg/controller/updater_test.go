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

package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/distribution/reference"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	"github.com/gravitational/teleport/lib/automaticupgrades/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

const (
	defaultTestRegistry = "registry.example.com"
	defaultTestPath     = "path/img"
	versionLow          = "11.3.2"
	versionMid          = "11.5.4"
	versionHigh         = "12.2.1"
	defaultImageDigest  = digest.Digest("sha256:fac4209c2267dbf3517e1211dc3bd1c9b6e942fc3e7adcad47acb92a92c23f94")
)

var (
	alwaysTrigger = maintenance.NewMaintenanceStaticTrigger("always trigger", true)
	neverTrigger  = maintenance.NewMaintenanceStaticTrigger("never trigger", false)
	alwaysValid   = img.NewImageValidatorMock(
		"always",
		true,
		img.NewImageRef(defaultTestRegistry, defaultTestPath, versionHigh, defaultImageDigest),
	)
	neverValid = img.NewImageValidatorMock("never", false, nil)
)

// errorIsType is a helper that takes an error and yields an ErrorAssertionFunc.
func errorIsType(errType any) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, i ...any) {
		require.Error(t, err)
		err = trace.Unwrap(err)
		require.IsType(t, errType, err)
	}
}

func mustNewStaticGetter(t *testing.T, versionMock string, errMock error) version.Getter {
	t.Helper()
	getter, err := version.NewStaticGetter(versionMock, errMock)
	require.NoError(t, err)
	return getter
}

func Test_VersionUpdater_GetVersion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                string
		releaseRegistry     string
		releasePath         string
		currentVersion      *semver.Version
		versionGetter       version.Getter
		maintenanceTriggers []maintenance.Trigger
		imageCheckers       []img.Validator
		assertErr           require.ErrorAssertionFunc
		expectedImage       string
	}{
		{
			name:                "all good",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      semver.Must(version.EnsureSemver(versionMid)),
			versionGetter:       mustNewStaticGetter(t, versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           require.NoError,
			expectedImage:       fmt.Sprintf("%s/%s:%s@%s", defaultTestRegistry, defaultTestPath, versionHigh, defaultImageDigest),
		},
		{
			name:                "all good but no current version",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      nil,
			versionGetter:       mustNewStaticGetter(t, versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           require.NoError,
			expectedImage:       fmt.Sprintf("%s/%s:%s@%s", defaultTestRegistry, defaultTestPath, versionHigh, defaultImageDigest),
		},
		{
			name:                "same version",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      semver.Must(version.EnsureSemver(versionMid)),
			versionGetter:       mustNewStaticGetter(t, versionMid, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           errorIsType(&version.NoNewVersionError{}),
			expectedImage:       "",
		},
		{
			name:                "no version",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      semver.Must(version.EnsureSemver(versionMid)),
			versionGetter:       mustNewStaticGetter(t, "", &version.NoNewVersionError{Message: "version server did not advertise a version"}),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           errorIsType(&version.NoNewVersionError{}),
			expectedImage:       "",
		},
		{
			name:                "no maintenance triggered",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      semver.Must(version.EnsureSemver(versionMid)),
			versionGetter:       mustNewStaticGetter(t, versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{neverTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           errorIsType(&MaintenanceNotTriggeredError{}),
			expectedImage:       "",
		},
		{
			name:                "invalid signature",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      semver.Must(version.EnsureSemver(versionMid)),
			versionGetter:       mustNewStaticGetter(t, versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{neverValid},
			assertErr:           errorIsType(&trace.TrustError{}),
			expectedImage:       "",
		},
		{
			name:                "error getting version",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      semver.Must(version.EnsureSemver(versionMid)),
			versionGetter:       mustNewStaticGetter(t, "", &trace.ConnectionProblemError{}),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{neverValid},
			assertErr:           errorIsType(&trace.ConnectionProblemError{}),
			expectedImage:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test setup
			baseImage, err := reference.ParseNamed(tt.releaseRegistry + "/" + tt.releasePath)
			require.NoError(t, err)

			updater := VersionUpdater{
				versionGetter:       tt.versionGetter,
				imageValidators:     tt.imageCheckers,
				maintenanceTriggers: tt.maintenanceTriggers,
				baseImage:           baseImage,
			}

			// We need a dummy Kubernetes object, it is not used by the StaticTrigger
			obj := &core.Pod{}

			// Doing the test
			image, err := updater.GetVersion(ctx, obj, tt.currentVersion)
			tt.assertErr(t, err)
			if tt.expectedImage == "" {
				require.Nil(t, image)
			} else {
				require.NotNil(t, image)
				require.Equal(t, tt.expectedImage, image.String())
			}
		})
	}
}
