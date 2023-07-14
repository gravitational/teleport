/*
Copyright 2018-2019 Gravitational, Inc.

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

package lite

import (
	"context"
	"os"
	"testing"
	"time"

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

func TestLite(t *testing.T) {
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		clock := clockwork.NewFakeClock()

		cfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, err
		}

		if cfg.ConcurrentBackend != nil {
			return nil, nil, test.ErrConcurrentAccessNotSupported
		}

		if cfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		backend, err := NewWithConfig(context.Background(), Config{
			Path:             t.TempDir(),
			PollStreamPeriod: 300 * time.Millisecond,
			Clock:            clock,
		})

		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return backend, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

// newBackend creates a backend instance that automatically deletes itself
// at the end of the supplied test
func newBackend(t *testing.T) *Backend {
	clock := clockwork.NewFakeClock()
	backend, err := NewWithConfig(context.Background(), Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 300 * time.Millisecond,
		Clock:            clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { backend.Close() })
	return backend
}

// Import tests importing values
func TestImport(t *testing.T) {
	ctx := context.Background()
	prefix := test.MakePrefix()

	uut := newBackend(t)

	imported, err := uut.Imported(ctx)
	require.NoError(t, err)
	require.False(t, imported)

	// add one element that should not show up
	items := []backend.Item{
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
		{Key: prefix("/prefix/b"), Value: []byte("val b")},
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
	}
	err = uut.Import(ctx, items)
	require.NoError(t, err)

	// prefix range fetch
	result, err := uut.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), backend.NoLimit)
	require.NoError(t, err)
	expected := []backend.Item{
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
		{Key: prefix("/prefix/b"), Value: []byte("val b")},
	}
	test.RequireItems(t, result.Items, expected)

	imported, err = uut.Imported(ctx)
	require.NoError(t, err)
	require.True(t, imported)

	err = uut.Import(ctx, items)
	require.True(t, trace.IsAlreadyExists(err))

	imported, err = uut.Imported(ctx)
	require.NoError(t, err)
	require.True(t, imported)
}

func TestConnectionURIGeneration(t *testing.T) {
	fileNameAndParams := "/sqlite.db?_busy_timeout=0&_txlock=immediate"
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			path:     "/Users/testuser/data_dir",
			expected: "file:/Users/testuser/data_dir" + fileNameAndParams,
		}, {
			name:     "relative path",
			path:     "./data_dir",
			expected: "file:data_dir" + fileNameAndParams,
		}, {
			name:     "path with space",
			path:     "/Users/testuser/dir with spaces/data_dir",
			expected: "file:/Users/testuser/dir%20with%20spaces/data_dir" + fileNameAndParams,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := Config{Path: tt.path}
			require.Equal(t, tt.expected, conf.ConnectionURI())
		})
	}
}
