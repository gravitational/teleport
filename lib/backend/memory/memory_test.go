/*
Copyright 2019 Gravitational, Inc.

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

package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestMemory(t *testing.T) {
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		cfg, err := test.ApplyOptions(options)

		if err != nil {
			return nil, nil, err
		}

		if cfg.ConcurrentBackend != nil {
			if _, ok := cfg.ConcurrentBackend.(*Memory); !ok {
				return nil, nil, trace.BadParameter("target is not a Memory backend")
			}
			return cfg.ConcurrentBackend, nil, nil
		}

		clock := clockwork.NewFakeClock()
		mem, err := New(Config{
			Clock:  clock,
			Mirror: cfg.MirrorMode,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return mem, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

func TestIterateRange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := New(Config{})
	require.NoError(t, err)

	// set up a generic bulk range to iterate
	expectedKeys := make(map[string]struct{})
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("/bulk/%d", i)
		expectedKeys[key] = struct{}{}

		_, err = bk.Put(ctx, backend.Item{
			Key:   []byte(key),
			Value: []byte("v"),
		})
		require.NoError(t, err)
	}

	// check that observed range members match expectations
	observedKeys := make(map[string]struct{})
	err = backend.IterateRange(ctx, bk, []byte("/bulk/"), backend.RangeEnd([]byte("/bulk/")), 3, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			key := string(item.Key)

			_, dup := observedKeys[key]
			require.False(t, dup, "duplicate of %q", key)

			_, exp := expectedKeys[key]
			require.True(t, exp, "unexpected key %q", key)

			observedKeys[key] = struct{}{}
		}

		return false, nil
	})
	require.NoError(t, err)
	require.Equal(t, expectedKeys, observedKeys)

	// set up a collection of keys that are suffixes of one another (ensures we aren't suffering from the classic 'pagination bug', where
	// page breaks landing on some key K skip subsequent keys with prefix K).
	for i := 0; i < 20; i++ {
		_, err = bk.Put(ctx, backend.Item{
			Key:   []byte("/suff/" + strings.Repeat("s", i+1)),
			Value: []byte("s"),
		})

		require.NoError(t, err)
	}

	var scount int
	err = backend.IterateRange(ctx, bk, []byte("/suff/"), backend.RangeEnd([]byte("/suff/")), 2, func(items []backend.Item) (stop bool, err error) {
		scount += len(items)
		return false, nil
	})
	require.NoError(t, err)
	require.Equal(t, 20, scount)
}
