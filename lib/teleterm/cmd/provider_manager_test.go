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

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func TestProviderManager(t *testing.T) {
	manager, err := NewProviderManager(ProviderManagerConfig{
		Storage: fakeStorage{},
	})
	require.NoError(t, err)

	// Not supported type.
	_, err = manager.Get(uri.NewClusterURI("foo"))
	require.Error(t, err)

	// Database gateway.
	provider, err := manager.Get(uri.NewClusterURI("foo").AppendDB("db"))
	require.NoError(t, err)
	require.IsType(t, DbcmdCLICommandProvider{}, provider)

	// Kube gateway.
	provider, err = manager.Get(uri.NewClusterURI("foo").AppendKube("kube"))
	require.NoError(t, err)
	require.IsType(t, KubeCLICommandProvider{}, provider)
}
