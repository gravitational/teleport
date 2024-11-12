/*
Copyright 2021 Gravitational, Inc.

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

// Package keypaths defines several keypaths used by multiple Teleport services.
package keypaths_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keypaths"
)

func TestIsProfileKubeConfigPath(t *testing.T) {
	path := ""
	isKubeConfig, err := keypaths.IsProfileKubeConfigPath(path)
	require.NoError(t, err)
	require.False(t, isKubeConfig)

	path = keypaths.KubeCredPath("~/tsh", "proxy", "user", "cluster", "kube")
	isKubeConfig, err = keypaths.IsProfileKubeConfigPath(path)
	require.NoError(t, err)
	require.False(t, isKubeConfig)

	path = keypaths.KubeConfigPath("~/tsh", "proxy", "user", "cluster", "kube")
	isKubeConfig, err = keypaths.IsProfileKubeConfigPath(path)
	require.NoError(t, err)
	require.True(t, isKubeConfig)

	path = keypaths.KubeConfigPath("keys/keys/keys", "proxy", "user", "cluster", "kube")
	isKubeConfig, err = keypaths.IsProfileKubeConfigPath(path)
	require.NoError(t, err)
	require.True(t, isKubeConfig)
}
