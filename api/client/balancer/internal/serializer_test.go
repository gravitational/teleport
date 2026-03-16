// Copyright 2025 Gravitational, Inc
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

package internal

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCallbackSerializerParallelPuts(t *testing.T) {
	wantCount := 1000
	gotCount := 0
	cs := NewCallbackSerializer()
	wg := &sync.WaitGroup{}
	wait := make(chan struct{})
	for i := 0; i < wantCount; i++ {
		wg.Add(1)
		go func() {
			cs.Put(func() {
				<-wait
				gotCount++
				wg.Done()
			})
		}()
	}
	close(wait)
	wg.Wait()
	cs.Close()
	require.Equal(t, wantCount, gotCount)
}

func TestCallbackSerializerOrder(t *testing.T) {
	cs := NewCallbackSerializer()
	len := 1000
	s := make([]int, 0, len)
	wg := &sync.WaitGroup{}
	wait := make(chan struct{})
	for i := 0; i < len; i++ {
		wg.Add(1)
		cs.Put(func() {
			<-wait
			s = append(s, i)
			wg.Done()
		})
	}
	close(wait)
	wg.Wait()
	cs.Close()
	require.Len(t, s, len)
	for i, val := range s {
		require.Equal(t, i, val)
	}
}
