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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestLockTargetMapConversions(t *testing.T) {
	lt := LockTarget{
		User:      "user@sso.tld",
		Node:      "node-uuid",
		MFADevice: "mfa-device-uuid",
	}
	m := map[string]string{
		"user":       "user@sso.tld",
		"node":       "node-uuid",
		"mfa_device": "mfa-device-uuid",
	}

	ltMap, err := lt.IntoMap()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(m, ltMap))

	lt2 := LockTarget{}
	err = lt2.FromMap(m)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(lt, lt2))
}

func TestLockTargetMatch(t *testing.T) {
	target := LockTarget{
		User:      "user@sso.tld",
		Node:      "node-uuid",
		MFADevice: "mfa-device-uuid",
	}
	lock, err := NewLock("some-lock", LockSpecV2{Target: target})
	require.NoError(t, err)

	require.True(t, target.Match(lock))

	target.Node = "node-uuid-2"
	require.False(t, target.Match(lock))

	target.Node = ""
	require.True(t, target.Match(lock))

	disjointTarget := LockTarget{
		Login: "root",
	}
	require.False(t, disjointTarget.Match(lock))

	// Empty target should match no lock.
	emptyTarget := LockTarget{}
	require.False(t, emptyTarget.Match(lock))
}
