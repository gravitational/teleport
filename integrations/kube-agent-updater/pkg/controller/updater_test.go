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

package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/version"
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
	alwaysTrigger = maintenance.NewMaintenanceTriggerMock("always trigger", true)
	neverTrigger  = maintenance.NewMaintenanceTriggerMock("never trigger", false)
	alwaysValid   = img.NewImageValidatorMock(
		"always",
		true,
		img.NewImageRef(defaultTestRegistry, defaultTestPath, versionHigh, defaultImageDigest),
	)
	neverValid = img.NewImageValidatorMock("never", false, nil)
)

// errorIsType is a helper that takes an error and yields an ErrorAssertionFunc.
func errorIsType(errType interface{}) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, i ...interface{}) {
		require.Error(t, err)
		err = trace.Unwrap(err)
		require.IsType(t, errType, err)
	}
}

func Test_VersionUpdater_GetVersion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                string
		releaseRegistry     string
		releasePath         string
		currentVersion      string
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
			currentVersion:      versionMid,
			versionGetter:       version.NewGetterMock(versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           require.NoError,
			expectedImage:       fmt.Sprintf("%s/%s:%s@%s", defaultTestRegistry, defaultTestPath, versionHigh, defaultImageDigest),
		},
		{
			name:                "all good but no current version",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      "",
			versionGetter:       version.NewGetterMock(versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           require.NoError,
			expectedImage:       fmt.Sprintf("%s/%s:%s@%s", defaultTestRegistry, defaultTestPath, versionHigh, defaultImageDigest),
		},
		{
			name:                "same version",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      versionMid,
			versionGetter:       version.NewGetterMock(versionMid, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           errorIsType(&NoNewVersionError{}),
			expectedImage:       "",
		},
		{
			name:                "no maintenance triggered",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      versionMid,
			versionGetter:       version.NewGetterMock(versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{neverTrigger},
			imageCheckers:       []img.Validator{alwaysValid},
			assertErr:           errorIsType(&MaintenanceNotTriggeredError{}),
			expectedImage:       "",
		},
		{
			name:                "invalid signature",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      versionMid,
			versionGetter:       version.NewGetterMock(versionHigh, nil),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{neverValid},
			assertErr:           errorIsType(&trace.TrustError{}),
			expectedImage:       "",
		},
		{
			name:                "error getting version",
			releaseRegistry:     defaultTestRegistry,
			releasePath:         defaultTestPath,
			currentVersion:      versionMid,
			versionGetter:       version.NewGetterMock("", &trace.ConnectionProblemError{}),
			maintenanceTriggers: []maintenance.Trigger{alwaysTrigger},
			imageCheckers:       []img.Validator{neverValid},
			assertErr:           errorIsType(&trace.ConnectionProblemError{}),
			expectedImage:       "",
		},
	}

	for _, tt := range tests {
		tt := tt
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

			// We need a dummy Kubernetes object, it is not used by the TriggerMock
			obj := &core.Pod{}

			// Doing the test
			image, err := updater.GetVersion(ctx, obj, "v"+tt.currentVersion)
			tt.assertErr(t, err)
			if tt.expectedImage == "" {
				require.Nil(t, image)
			} else {
				require.NotNil(t, image)
				require.Equal(t, image.String(), tt.expectedImage)
			}
		})
	}
}
