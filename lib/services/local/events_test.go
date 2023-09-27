/*
Copyright 2023 Gravitational, Inc.

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

package local

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestVerifyEventWatcherPrefxies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		expectedPrefixes [][]byte
		assertErr        require.ErrorAssertionFunc
	}{
		{
			name: "no overlap",
			expectedPrefixes: [][]byte{
				backend.Key("one"),
				backend.Key("two"),
				backend.Key("three"),
			},
			assertErr: require.NoError,
		},
		{
			name: "overlap",
			expectedPrefixes: [][]byte{
				backend.Key("one"),
				backend.Key("oneoverlap"),
				backend.Key("two"),
				backend.Key("three"),
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			mem, err := memory.New(memory.Config{})
			require.NoError(t, err)

			w, err := mem.NewWatcher(ctx, backend.Watch{
				Name:            "test-watcher",
				Prefixes:        test.expectedPrefixes,
				QueueSize:       10,
				MetricComponent: "component",
			})
			require.NoError(t, err)

			test.assertErr(t, verifyEventWatcherPrefixes(test.expectedPrefixes, w))
		})
	}
}
