/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestLite(t *testing.T) {
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
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
