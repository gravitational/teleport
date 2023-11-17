package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/secreports/limiter"
)

type mockGetter struct {
	details *limiter.Details
}

func (m *mockGetter) GetDetails(ctx context.Context) (*limiter.Details, error) {
	return m.details, nil
}

func TestScheduler(t *testing.T) {
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	day := time.Hour * 24

	tests := []struct {
		name     string
		details  *limiter.Details
		expected time.Time
		now      time.Time
	}{
		{
			name: "10% cap used -  30 days span",
			now:  start.Add(day),
			details: &limiter.Details{
				Current: 1,
				Limit:   10,
				Start:   start,
				End:     start.Add(30 * day),
			},
			expected: time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "50% cap used - 30 days span",
			now:  time.Date(2021, 1, 1, 1, 0, 0, 0, time.UTC),
			details: &limiter.Details{
				Current: 5,
				Limit:   10,
				Start:   start,
				End:     start.Add(30 * day),
			},
			expected: time.Date(2021, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "0% cap used - next after default interval",
			now:  time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			details: &limiter.Details{
				Current: 0,
				Limit:   10,
				Start:   start,
				End:     start.Add(30 * day),
			},
			expected: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "100% cap used - next after end time",
			now:  time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			details: &limiter.Details{
				Current: 11,
				Limit:   10,
				Start:   start,
				End:     start.Add(30 * day),
			},
			expected: time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC),
		},

		{
			name: "now after end time",
			now:  time.Date(2021, 1, 31, 1, 0, 0, 0, time.UTC),
			details: &limiter.Details{
				Current: 11,
				Limit:   10,
				Start:   start,
				End:     start.Add(30 * day),
			},
			expected: time.Date(2021, 1, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			clock := clockwork.NewFakeClockAt(tc.now)
			s := Scheduler{
				Config: Config{
					Limiter: &mockGetter{
						details: tc.details,
					},
					Clock:       clock,
					MinInterval: time.Hour,
				},
			}
			got, err := s.Next(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.expected, got)
		})
	}
}
