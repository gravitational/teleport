/*
Copyright 2022 Gravitational, Inc.

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
	"time"

	"github.com/stretchr/testify/require"
)

func TestInstanceControlLogExpiry(t *testing.T) {
	const ttl = time.Minute
	now := time.Now()
	instance, err := NewInstance("test-instance", InstanceSpecV1{
		LastSeen: now,
	})
	require.NoError(t, err)

	instance.AppendControlLog(
		InstanceControlLogEntry{
			Type: "foo",
			Time: now,
			TTL:  ttl,
		},
		InstanceControlLogEntry{
			Type: "bar",
			Time: now.Add(-ttl / 2),
			TTL:  ttl,
		},
		InstanceControlLogEntry{
			Type: "bin",
			Time: now.Add(-ttl * 2),
			TTL:  ttl,
		},
		InstanceControlLogEntry{
			Type: "baz",
			Time: now,
			TTL:  time.Hour,
		},
	)

	require.Len(t, instance.GetControlLog(), 4)

	instance.SyncLogAndResourceExpiry(ttl)

	require.Len(t, instance.GetControlLog(), 3)
	require.Equal(t, now.Add(time.Hour).UTC(), instance.Expiry())

	instance.SetLastSeen(now.Add(ttl))

	instance.SyncLogAndResourceExpiry(ttl)

	require.Len(t, instance.GetControlLog(), 2)
	require.Equal(t, now.Add(time.Hour).UTC(), instance.Expiry())

	instance.AppendControlLog(
		InstanceControlLogEntry{
			Type: "long-lived",
			Time: now,
			TTL:  time.Hour * 2,
		},
	)

	instance.SyncLogAndResourceExpiry(ttl)

	require.Len(t, instance.GetControlLog(), 3)
	require.Equal(t, now.Add(time.Hour*2).UTC(), instance.Expiry())
}
