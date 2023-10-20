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

package reversetunnel

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentStoreRace(t *testing.T) {
	store := newAgentStore()
	agents := []*agent{{}, {}, {}, {}, {}}

	wg := &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			store.add(agents[i])
			wg.Done()
		}(i)
	}

	wg.Wait()

	wg = &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			ok := store.remove(agents[i])
			require.True(t, ok)
			wg.Done()
		}(i)
	}

	wg.Wait()
}
