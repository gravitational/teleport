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

package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

func TestSearchEventsLimiter(t *testing.T) {
	t.Parallel()
	t.Run("emitting events happen without any limiting", func(t *testing.T) {
		s, err := events.NewSearchEventLimiter(events.SearchEventsLimiterConfig{
			RefillAmount: 1,
			Burst:        1,
			AuditLogger: &mockAuditLogger{
				emitAuditEventRespFn: func() error { return nil },
			},
		})
		require.NoError(t, err)
		for i := 0; i < 20; i++ {
			require.NoError(t, s.EmitAuditEvent(context.Background(), &apievents.AccessRequestCreate{}))
		}
	})

	t.Run("with limiter", func(t *testing.T) {
		burst := 20
		s, err := events.NewSearchEventLimiter(events.SearchEventsLimiterConfig{
			RefillTime:   20 * time.Millisecond,
			RefillAmount: 1,
			Burst:        burst,
			AuditLogger: &mockAuditLogger{
				searchEventsRespFn: func() ([]apievents.AuditEvent, string, error) { return nil, "", nil },
			},
		})
		require.NoError(t, err)

		someDate := clockwork.NewFakeClock().Now().UTC()

		ctx := context.Background()
		for i := 0; i < burst; i++ {
			var err error
			// rate limit is shared between both search endpoints.
			if i%2 == 0 {
				_, _, err = s.SearchEvents(ctx, events.SearchEventsRequest{
					From:  someDate,
					To:    someDate,
					Limit: 100,
					Order: types.EventOrderAscending,
				})
			} else {
				_, _, err = s.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
					From:  someDate,
					To:    someDate,
					Limit: 100,
					Order: types.EventOrderAscending,
				})
			}
			require.NoError(t, err)
		}
		// Now all tokens from rate limit should be used
		_, _, err = s.SearchEvents(ctx, events.SearchEventsRequest{
			From:  someDate,
			To:    someDate,
			Limit: 100,
			Order: types.EventOrderAscending,
		})
		require.True(t, trace.IsLimitExceeded(err))
		// Also on SearchSessionEvents
		_, _, err = s.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
			From:  someDate,
			To:    someDate,
			Limit: 100,
			Order: types.EventOrderAscending,
		})
		require.True(t, trace.IsLimitExceeded(err))

		// After 20ms 1 token should be added according to rate.
		require.Eventually(t, func() bool {
			_, _, err := s.SearchEvents(ctx, events.SearchEventsRequest{
				From:  someDate,
				To:    someDate,
				Limit: 100,
				Order: types.EventOrderAscending,
			})
			return err == nil
		}, 1*time.Second, 5*time.Millisecond)
	})
}

func TestSearchEventsLimiterConfig(t *testing.T) {
	tests := []struct {
		name   string
		cfg    events.SearchEventsLimiterConfig
		wantFn func(t *testing.T, err error, cfg events.SearchEventsLimiterConfig)
	}{
		{
			name: "valid config",
			cfg: events.SearchEventsLimiterConfig{
				AuditLogger:  &mockAuditLogger{},
				RefillAmount: 1,
				Burst:        1,
			},
			wantFn: func(t *testing.T, err error, cfg events.SearchEventsLimiterConfig) {
				require.NoError(t, err)
				require.Equal(t, time.Second, cfg.RefillTime)
			},
		},
		{
			name: "empty rate in config",
			cfg: events.SearchEventsLimiterConfig{
				AuditLogger: &mockAuditLogger{},
				Burst:       1,
			},
			wantFn: func(t *testing.T, err error, cfg events.SearchEventsLimiterConfig) {
				require.ErrorContains(t, err, "RefillAmount cannot be less or equal to 0")
			},
		},

		{
			name: "empty burst in config",
			cfg: events.SearchEventsLimiterConfig{
				AuditLogger:  &mockAuditLogger{},
				RefillAmount: 1,
			},
			wantFn: func(t *testing.T, err error, cfg events.SearchEventsLimiterConfig) {
				require.ErrorContains(t, err, "Burst cannot be less or equal to 0")
			},
		},
		{
			name: "empty logger",
			cfg: events.SearchEventsLimiterConfig{
				RefillAmount: 1,
				Burst:        1,
			},
			wantFn: func(t *testing.T, err error, cfg events.SearchEventsLimiterConfig) {
				require.ErrorContains(t, err, "empty auditLogger")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.CheckAndSetDefaults()
			tt.wantFn(t, err, tt.cfg)
		})
	}
}

type mockAuditLogger struct {
	searchEventsRespFn   func() ([]apievents.AuditEvent, string, error)
	emitAuditEventRespFn func() error
	events.AuditLogger
}

func (m *mockAuditLogger) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	return m.searchEventsRespFn()
}

func (m *mockAuditLogger) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	return m.searchEventsRespFn()
}

func (m *mockAuditLogger) EmitAuditEvent(context.Context, apievents.AuditEvent) error {
	return m.emitAuditEventRespFn()
}
