/*
Copyright 2020 Gravitational, Inc.

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

package services

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"

	"github.com/stretchr/testify/require"
)

func TestAcquireSemaphoreRequest(t *testing.T) {
	ok := types.AcquireSemaphoreRequest{
		SemaphoreKind: "foo",
		SemaphoreName: "bar",
		MaxLeases:     1,
		Expires:       time.Now(),
	}
	ok2 := ok
	require.Nil(t, ok.Check())
	require.Nil(t, ok2.Check())

	// Check that all the required fields have their
	// zero values rejected.
	bad := ok
	bad.SemaphoreKind = ""
	require.NotNil(t, bad.Check())
	bad = ok
	bad.SemaphoreName = ""
	require.NotNil(t, bad.Check())
	bad = ok
	bad.MaxLeases = 0
	require.NotNil(t, bad.Check())
	bad = ok
	bad.Expires = time.Time{}
	require.NotNil(t, bad.Check())

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
	require.Nil(t, sem.KeepAlive(newLease))
	require.Equal(t, newLease.Expires, sem.Expiry())

	require.Nil(t, sem.Cancel(newLease))
	require.False(t, sem.Contains(newLease))
}
