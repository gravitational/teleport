/*
Copyright 2018-2021 Gravitational, Inc.

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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCircularBuffer(t *testing.T) {
	buff, err := NewCircularBuffer(-1)
	require.Error(t, err)
	require.Nil(t, buff)

	buff, err = NewCircularBuffer(5)
	require.NoError(t, err)
	require.NotNil(t, buff)
	require.Len(t, buff.buf, 5)
}

func TestCircularBuffer_Data(t *testing.T) {
	buff, err := NewCircularBuffer(5)
	require.NoError(t, err)

	data := buff.Data(1)
	require.Nil(t, data)

	buff.Add(1.0)

	buff.Data(0)
	require.Nil(t, data)

	data = buff.Data(1)
	require.Equal(t, []float64{1}, data)

	data = buff.Data(2)
	require.Equal(t, []float64{1}, data)

	data = buff.Data(10)
	require.Equal(t, []float64{1}, data)

	buff.Add(2)
	buff.Add(3)
	buff.Add(4)

	data = buff.Data(1)
	require.Equal(t, []float64{4}, data)

	data = buff.Data(2)
	require.Equal(t, []float64{3, 4}, data)

	data = buff.Data(10)
	require.Equal(t, []float64{1, 2, 3, 4}, data)

	buff.Add(5)
	buff.Add(6)
	buff.Add(7)

	data = buff.Data(1)
	require.Equal(t, []float64{7}, data)

	data = buff.Data(2)
	require.Equal(t, []float64{6, 7}, data)

	data = buff.Data(10)
	require.Equal(t, []float64{3, 4, 5, 6, 7}, data)

}
