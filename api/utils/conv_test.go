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

package utils

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConv_IntToInt32(t *testing.T) {
	t.Run("simpleCast", func(t *testing.T) {
		for i := -10; i <= 10; i++ {
			castI, err := IntToInt32(i)
			require.Nil(t, err)
			require.Equal(t, int32(i), castI)
		}
	})
	// range tests for 32bit excluded to support 32bit platform test execution
}

func TestConv_IntToInt16(t *testing.T) {
	t.Run("simpleCast", func(t *testing.T) {
		for i := -10; i <= 10; i++ {
			castI, err := IntToInt16(i)
			require.Nil(t, err)
			require.Equal(t, int16(i), castI)
		}
	})
	t.Run("aboveRange", func(t *testing.T) {
		i, err := IntToInt16(math.MaxInt16 + 1)
		require.NotNil(t, err)
		require.Equal(t, int16(math.MaxInt16), i)
	})
	t.Run("belowRange", func(t *testing.T) {
		i, err := IntToInt16(math.MinInt16 - 1)
		require.NotNil(t, err)
		require.Equal(t, int16(math.MinInt16), i)
	})
}

func TestConv_IntToUInt32(t *testing.T) {
	t.Run("simpleCast", func(t *testing.T) {
		for i := 0; i <= 10; i++ {
			castI, err := IntToUInt32(i)
			require.Nil(t, err)
			require.Equal(t, uint32(i), castI)
		}
	})
	// above range test for 32bit excluded for 32bit platform capability
	t.Run("negative", func(t *testing.T) {
		i, err := IntToUInt32(-1)
		require.NotNil(t, err)
		require.Equal(t, uint32(0), i)
	})
}

func TestConv_IntToUInt16(t *testing.T) {
	t.Run("simpleCast", func(t *testing.T) {
		for i := 0; i <= 10; i++ {
			castI, err := IntToUInt16(i)
			require.Nil(t, err)
			require.Equal(t, uint16(i), castI)
		}
	})
	t.Run("aboveRange", func(t *testing.T) {
		i, err := IntToUInt16(math.MaxUint16 + 1)
		require.NotNil(t, err)
		require.Equal(t, uint16(math.MaxUint16), i)
	})
	t.Run("negative", func(t *testing.T) {
		i, err := IntToUInt16(-1)
		require.NotNil(t, err)
		require.Equal(t, uint16(0), i)
	})
}
