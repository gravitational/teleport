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
	t.Run("CreateOrUpdate", func(t *testing.T) {
		require.NoError(t, secrets.CreateOrUpdate(ctx, key, firstValue))
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
