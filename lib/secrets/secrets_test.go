/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
ITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secrets

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func secretsTestSuite(t *testing.T, createFunc func(context.Context) (Secrets, error)) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	secrets, err := createFunc(ctx)
	require.NoError(t, err)
	require.NotNil(t, secrets)

	key := "aa/bb/cc"
	firstValue := "first value"
	secondValue := "second value"

	// Note that subtests need to be run in correct sequence.
	t.Run("Create", func(t *testing.T) {
		require.NoError(t, secrets.Create(ctx, key, firstValue))
	})

	t.Run("PutValue", func(t *testing.T) {
		first, err := secrets.GetValue(ctx, key, CurrentVersion)
		require.NoError(t, err)

		// First caller succeeds.
		require.NoError(t, secrets.PutValue(ctx, key, secondValue, first.Version))

		// Simulate a case two callers try to PutValue at the same time. Other
		// caller succeeds so now the lastest version is 2nd. This caller still
		// thinks lastest is 1st so PutValue will fail.
		require.Error(t, secrets.PutValue(ctx, key, secondValue, first.Version))
	})

	t.Run("GetValue CurrentVersion", func(t *testing.T) {
		second, err := secrets.GetValue(ctx, key, CurrentVersion)
		require.NoError(t, err)
		require.Equal(t, second.Value, secondValue)
	})

	t.Run("GetValue PreviousVersion", func(t *testing.T) {
		first, err := secrets.GetValue(ctx, key, PreviousVersion)
		require.NoError(t, err)
		require.Equal(t, first.Value, firstValue)
	})

	t.Run("GetValue version string", func(t *testing.T) {
		first, err := secrets.GetValue(ctx, key, PreviousVersion)
		require.NoError(t, err)

		firstByVersionString, err := secrets.GetValue(ctx, key, first.Version)
		require.NoError(t, err)
		require.Equal(t, first, firstByVersionString)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, secrets.Delete(ctx, key))

		_, err := secrets.GetValue(ctx, key, CurrentVersion)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}
