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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestLockTargetMapConversions(t *testing.T) {
	lt := LockTarget{
		User:      "user@sso.tld",
		ServerID:  "node-uuid",
		MFADevice: "mfa-device-uuid",
	}
	m := map[string]string{
		"user":       "user@sso.tld",
		"server_id":  "node-uuid",
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
		ServerID:  "node-uuid",
		MFADevice: "mfa-device-uuid",
	}
	lock, err := NewLock("some-lock", LockSpecV2{Target: target})
	require.NoError(t, err)

	require.True(t, target.Match(lock))

	target.ServerID = "node-uuid-2"
	require.False(t, target.Match(lock))

	target.ServerID = ""
	require.True(t, target.Match(lock))

	disjointTarget := LockTarget{
		Login: "root",
	}
	require.False(t, disjointTarget.Match(lock))

	// Empty target should match no lock.
	emptyTarget := LockTarget{}
	require.False(t, emptyTarget.Match(lock))
}

func TestLockTargetIsSimple(t *testing.T) {
	ty := reflect.TypeFor[LockTarget]()
	for f := range ty.Fields() {
		// A struct embedded by value that is also "simple" in this sense would
		// work too, so if we need something like that we can extend this check.
		// Arrays (of likewise "simple" types) would also work but outside of
		// nasty gogoproto shenanigans (which we should not make use of) it's
		// not possible to have an array in a protobuf message struct, so it's a
		// moot point. Pointers are a no-no, since they are compared by address
		// rather than by checking the value that they point to.
		require.Containsf(t, []reflect.Kind{
			reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Uintptr,
			reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
			reflect.String,
		}, f.Type.Kind(), "field %+q (%#d) is not of a scalar kind (%s)", f.Name, f.Index[0], f.Type.Kind())
		require.NotEqualf(t, "_", f.Name, "field %+q (#%d) is padding", f.Name, f.Index[0])
		require.Truef(t, f.IsExported(), "field %+q (#%d) is unexported", f.Name, f.Index[0])
		// embedding scalar newtypes is weird but technically possible
		require.Falsef(t, f.Anonymous, "field %+q (#%d) is embedded", f.Name, f.Index[0])
	}
}
