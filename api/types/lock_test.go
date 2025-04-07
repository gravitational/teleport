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
	"reflect"
	"strings"
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
	// Test that we still support old locks with only Node field set and that
	// it only applies to nodes.
	// For Nodes, LockTarget Node and ServerID fields are both set at the same
	// time.
	targetNode := LockTarget{
		ServerID: "node-uuid",
		Node:     "node-uuid",
	}
	// Create a lock with only Node field set (old lock).
	lockNode, err := NewLock("some-lock", LockSpecV2{
		Target: LockTarget{
			Node: "node-uuid",
		},
	},
	)
	require.NoError(t, err)
	// Test that the old lock with only Node field set matches a target generated
	// from a Node identity (Node and ServerID fields set)
	require.True(t, targetNode.Match(lockNode))

	// Old locks with Node field should not match new lock targets with ServerID field
	// set but Node field unset.
	targetServerID := LockTarget{
		ServerID: "node-uuid",
	}

	require.False(t, targetServerID.Match(lockNode))

	// Test if locks with ServerID apply to nodes and other locks with ServerID.
	lockServerID, err := NewLock("some-lock", LockSpecV2{
		Target: LockTarget{
			ServerID: "node-uuid",
		},
	},
	)
	require.NoError(t, err)
	// Test that a lock with ServerID field set matches a target generated from a
	// Node identity (Node and ServerID fields set)
	require.True(t, targetNode.Match(lockServerID))
	// Test that a lock with ServerID field set matches any target with ServerID.
	require.True(t, targetServerID.Match(lockServerID))
}

// TestLockTargetIsEmpty checks that the implementation of [LockTarget.IsEmpty]
// is correct by filling one field at a time and expecting IsEmpty to return
// false. Only the public fields that don't start with `XXX_` are checked (as
// those are gogoproto-internal fields).
func TestLockTargetIsEmpty(t *testing.T) {
	require.True(t, (LockTarget{}).IsEmpty())

	for i, field := range reflect.VisibleFields(reflect.TypeOf(LockTarget{})) {
		if strings.HasPrefix(field.Name, "XXX_") {
			continue
		}

		var lt LockTarget
		// if we add non-string fields to LockTarget we need a type switch here
		reflect.ValueOf(&lt).Elem().Field(i).SetString("nonempty")
		require.False(t, lt.IsEmpty(), "field name: %v", field.Name)
	}
}

// TestLockTargetEquals checks that the implementation of [LockTarget.Equals]
// is correct by filling one field at a time in for two LockTargets and expecting
// Equals to return the appropriate value. Only the public fields that don't start with
// `XXX_` are checked (as those are gogoproto-internal fields).
func TestLockTargetEquals(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		require.True(t, (LockTarget{}).Equals(LockTarget{}), "empty targets equal")

		for i, field := range reflect.VisibleFields(reflect.TypeOf(LockTarget{})) {
			if strings.HasPrefix(field.Name, "XXX_") {
				continue
			}

			var a, b LockTarget
			// if we add non-string fields to LockTarget we need a type switch here
			reflect.ValueOf(&a).Elem().Field(i).SetString("nonempty")
			reflect.ValueOf(&b).Elem().Field(i).SetString("nonempty")
			require.True(t, a.Equals(b), "field name: %v", field.Name)
		}
	})

	t.Run("not equal", func(t *testing.T) {
		for i, field := range reflect.VisibleFields(reflect.TypeOf(LockTarget{})) {
			if strings.HasPrefix(field.Name, "XXX_") {
				continue
			}

			var a, b LockTarget
			// if we add non-string fields to LockTarget we need a type switch here
			reflect.ValueOf(&a).Elem().Field(i).SetString("nonempty")
			reflect.ValueOf(&b).Elem().Field(i).SetString("other")
			require.False(t, a.Equals(b), "field name: %v", field.Name)
		}
	})
}
