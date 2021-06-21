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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Check that default constructors never return nil and pass validation checks.
func TestDefaultConstructors(t *testing.T) {
	require.NotNil(t, DefaultAuthPreference())
	require.NoError(t, DefaultAuthPreference().CheckAndSetDefaults())
	require.NotNil(t, DefaultClusterAuditConfig())
	require.NoError(t, DefaultClusterAuditConfig().CheckAndSetDefaults())
	require.NotNil(t, DefaultClusterConfig())
	require.NoError(t, DefaultClusterConfig().CheckAndSetDefaults())
	require.NotNil(t, DefaultClusterNetworkingConfig())
	require.NoError(t, DefaultClusterNetworkingConfig().CheckAndSetDefaults())
	require.NotNil(t, DefaultSessionRecordingConfig())
	require.NoError(t, DefaultSessionRecordingConfig().CheckAndSetDefaults())
	require.NotNil(t, DefaultStaticTokens())
	require.NoError(t, DefaultStaticTokens().CheckAndSetDefaults())

	ns := DefaultNamespace()
	require.NotEmpty(t, ns.GetName())
	require.NoError(t, ns.CheckAndSetDefaults())
}
