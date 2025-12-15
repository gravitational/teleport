/*
Copyright 2025 Gravitational, Inc.

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

package structfill_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/testutils/structfill"
)

type InnerObject struct {
	D  time.Duration
	T  time.Time
	DP *time.Duration
	TP *time.Time
}
type Nested struct {
	NI int
}

type object struct {
	B    bool
	I    int64
	U    uint32
	F    float64
	C    complex128
	S    string
	A    [3]int
	Sl   []InnerObject
	SlP  []*InnerObject
	M    map[string]*InnerObject
	P    *int
	Sub  InnerObject
	SubP *InnerObject
	SP   *string

	*Nested
}

func TestRandFill(t *testing.T) {
	var v object
	err := structfill.Fill(&v)
	require.NoError(t, err)

	require.True(t, v.B)
	require.NotZero(t, v.I)
	require.NotZero(t, v.U)
	require.NotZero(t, v.F)
	require.NotZero(t, v.C)
	require.NotEmpty(t, v.S)

	require.Len(t, v.A, 3)
	require.NotZero(t, v.A[0])
	require.NotZero(t, v.A[1])
	require.NotZero(t, v.A[2])

	require.NotEmpty(t, v.Sl)
	require.NotZero(t, v.Sl[0].D)
	require.False(t, v.Sl[0].T.IsZero())
	require.NotNil(t, v.Sl[0].DP)
	require.NotZero(t, *v.Sl[0].DP)
	require.NotNil(t, v.Sl[0].TP)
	require.False(t, v.Sl[0].TP.IsZero())

	require.NotEmpty(t, v.SlP)
	require.NotNil(t, v.SlP[0])
	require.NotZero(t, v.SlP[0].D)
	require.False(t, v.SlP[0].T.IsZero())
	require.NotNil(t, v.SlP[0].DP)
	require.NotZero(t, *v.SlP[0].DP)
	require.NotNil(t, v.SlP[0].TP)
	require.False(t, v.SlP[0].TP.IsZero())

	require.NotEmpty(t, v.M)
	for k, val := range v.M {
		require.NotEmpty(t, k)
		require.NotNil(t, val)
		require.NotZero(t, val.D)
		require.False(t, val.T.IsZero())
		require.NotNil(t, val.DP)
		require.NotZero(t, *val.DP)
		require.NotNil(t, val.TP)
		require.False(t, val.TP.IsZero())
	}

	require.NotNil(t, v.P)
	require.NotZero(t, *v.P)

	require.NotZero(t, v.Sub.D)
	require.False(t, v.Sub.T.IsZero())
	require.NotNil(t, v.Sub.DP)
	require.NotZero(t, *v.Sub.DP)
	require.NotNil(t, v.Sub.TP)
	require.False(t, v.Sub.TP.IsZero())

	require.NotNil(t, v.SubP)
	require.NotZero(t, v.SubP.D)
	require.False(t, v.SubP.T.IsZero())
	require.NotNil(t, v.SubP.DP)
	require.NotZero(t, *v.SubP.DP)
	require.NotNil(t, v.SubP.TP)
	require.False(t, v.SubP.TP.IsZero())

	require.NotNil(t, v.SP)
	require.NotEmpty(t, *v.SP)

	require.NotNil(t, v.Nested)
	require.NotEmpty(t, v.Nested.NI)
}
