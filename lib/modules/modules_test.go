/*
Copyright 2017-2021 Gravitational, Inc.

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

package modules_test

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"

	"github.com/stretchr/testify/require"
)

func TestOSSModules(t *testing.T) {
	require.False(t, modules.GetModules().IsBoringBinary())
	require.Equal(t, modules.BuildOSS, modules.GetModules().BuildType())
}

func TestValidateAuthPreferenceOnCloud(t *testing.T) {
	ctx := context.Background()
	testServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	authPref := types.DefaultAuthPreference()
	err = testServer.AuthServer.SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	authPref.SetSecondFactor(constants.SecondFactorOff)
	err = testServer.AuthServer.SetAuthPreference(ctx, authPref)
	require.EqualError(t, err, "cannot disable two-factor authentication on Cloud")
}

func TestValidateSessionRecordingConfigOnCloud(t *testing.T) {
	ctx := context.Background()

	testServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
		},
	})

	recConfig := types.DefaultSessionRecordingConfig()
	err = testServer.AuthServer.SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	recConfig.SetMode(types.RecordAtProxy)
	err = testServer.AuthServer.SetSessionRecordingConfig(ctx, recConfig)
	require.EqualError(t, err, "cannot set proxy recording mode on Cloud")
}
