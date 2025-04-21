// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hardwarekey_test

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

func TestPINCache(t *testing.T) {
	clock := clockwork.NewFakeClock()
	pinCache := hardwarekey.PINCache{Clock: clock}

	testPIN := "123467"

	smallTTL := time.Second
	mediumTTL := time.Minute
	largeTTL := time.Hour

	// Set the PIN with the medium TTL.
	pinCache.SetPIN(testPIN, mediumTTL)
	require.Equal(t, testPIN, pinCache.GetPIN(smallTTL))
	require.Equal(t, testPIN, pinCache.GetPIN(mediumTTL))
	require.Equal(t, testPIN, pinCache.GetPIN(largeTTL))

	// Advancing by the small TTL should only expire the pin for the small TTL.
	clock.Advance(smallTTL)
	require.Zero(t, pinCache.GetPIN(smallTTL))
	require.Equal(t, testPIN, pinCache.GetPIN(mediumTTL))
	require.Equal(t, testPIN, pinCache.GetPIN(largeTTL))

	// Setting the PIN with the small TTL should reset the PIN's set-at time.
	// The expiration time should remain tied to the medium TTL.
	pinCache.SetPIN(testPIN, smallTTL)
	require.Equal(t, testPIN, pinCache.GetPIN(smallTTL))
	require.Equal(t, testPIN, pinCache.GetPIN(mediumTTL))

	// Advancing by the medium TTL, used to set the initial cache, should expire the PIN cache.
	clock.Advance(mediumTTL)
	require.Zero(t, pinCache.GetPIN(smallTTL))
	require.Zero(t, pinCache.GetPIN(mediumTTL))
	require.Zero(t, pinCache.GetPIN(largeTTL))
}
