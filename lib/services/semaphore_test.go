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

package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestAcquireSemaphoreRequest(t *testing.T) {
	ok := types.AcquireSemaphoreRequest{
		SemaphoreKind: "foo",
		SemaphoreName: "bar",
		MaxLeases:     1,
		Expires:       time.Now(),
	}
	ok2 := ok
	require.NoError(t, ok.Check())
	require.NoError(t, ok2.Check())

	// Check that all the required fields have their
	// zero values rejected.
	bad := ok
	bad.SemaphoreKind = ""
	require.Error(t, bad.Check())
	bad = ok
	bad.SemaphoreName = ""
	require.Error(t, bad.Check())
	bad = ok
	bad.MaxLeases = 0
	require.Error(t, bad.Check())
	bad = ok
	bad.Expires = time.Time{}
	require.Error(t, bad.Check())

	// ensure that well formed acquire params can configure
	// a well formed semaphore.
	sem, err := ok.ConfigureSemaphore()
	require.NoError(t, err)

	// verify acquisition works and semaphore state is
	// correctly updated.
	lease, err := sem.Acquire("sem-id", ok)
	require.NoError(t, err)
	require.True(t, sem.Contains(*lease))

	// verify keepalive succeeds and correctly updates
	// semaphore expiry.
	newLease := *lease
	newLease.Expires = sem.Expiry().Add(time.Second)
	require.NoError(t, sem.KeepAlive(newLease))
	require.Equal(t, newLease.Expires, sem.Expiry())

	require.NoError(t, sem.Cancel(newLease))
	require.False(t, sem.Contains(newLease))
}
