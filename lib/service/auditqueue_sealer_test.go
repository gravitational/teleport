/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package service

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type flakySRCGetter struct {
	failures int
	calls    int
}

func (f *flakySRCGetter) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	f.calls++
	if f.calls <= f.failures {
		return nil, trace.ConnectionProblem(nil, "auth unavailable")
	}
	return &types.SessionRecordingConfigV2{}, nil
}

func TestNewAuditQueueSealerWithRetry(t *testing.T) {
	t.Run("recovers from transient failures", func(t *testing.T) {
		getter := &flakySRCGetter{failures: 2}
		sealer, err := newAuditQueueSealerWithRetry(t.Context(), auditQueueSealerRetryConfig{
			getter:       getter,
			logger:       logtest.NewLogger(),
			attempts:     5,
			initialDelay: time.Millisecond,
			maxDelay:     time.Millisecond,
		})
		require.NoError(t, err)
		require.NotNil(t, sealer)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })
		require.Equal(t, 3, getter.calls)
	})

	t.Run("gives up after max attempts", func(t *testing.T) {
		getter := &flakySRCGetter{failures: 10}
		_, err := newAuditQueueSealerWithRetry(t.Context(), auditQueueSealerRetryConfig{
			getter:       getter,
			logger:       logtest.NewLogger(),
			attempts:     3,
			initialDelay: time.Millisecond,
			maxDelay:     time.Millisecond,
		})
		require.Error(t, err)
		require.Equal(t, 3, getter.calls)
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		getter := &flakySRCGetter{failures: 10}
		_, err := newAuditQueueSealerWithRetry(ctx, auditQueueSealerRetryConfig{
			getter:       getter,
			logger:       logtest.NewLogger(),
			attempts:     5,
			initialDelay: time.Hour,
			maxDelay:     time.Hour,
		})
		require.Error(t, err)
		require.Equal(t, 1, getter.calls)
	})
}
