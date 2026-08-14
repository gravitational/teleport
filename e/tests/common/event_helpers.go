package common

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// LogScope represents an interval of time over a Teleport Audit Log.
type LogScope[T apievents.AuditEvent] struct {
	t0       time.Time
	sut      *SUT
	auditLog events.AuditLogSessionStreamer
}

// NewLogScope creates a new log scope starting at the current time.
func NewLogScope[T apievents.AuditEvent](sut *SUT) LogScope[T] {
	return LogScope[T]{
		t0:       sut.Clock.Now(),
		sut:      sut,
		auditLog: sut.Teleport.Process.GetAuditLog(),
	}
}

// Reset sets the start of the log scope to the current time
func (s *LogScope[T]) Reset() {
	s.t0 = s.sut.Clock.Now()
}

// RequireNoEvent asserts that no events of the target type have been emitted
// since the start of the scope.
func (s *LogScope[T]) RequireNoEvent(t *testing.T, eventType string) {
	const waitFor = 500 * time.Millisecond
	const tick = 20 * time.Millisecond

	t.Helper()
	query := events.SearchEventsRequest{
		From:       s.t0,
		EventTypes: []string{eventType},
		Limit:      1,
		Order:      types.EventOrderDescending,
	}

	ctx := context.Background()

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		query.To = time.Now()

		events, _, err := s.auditLog.SearchEvents(ctx, query)
		require.NoError(t, err, "Fetching audit log messages must succeed")
		require.Empty(t, events, "Expect no events events found")

		select {
		case <-timer.C:
			return

		case <-ticker.C:
			continue
		}
	}
}

// RequireEvent waits for an event of the desired type to be emitted. If the
// required event is emitted then the supplied assertions are run over it.
func (s *LogScope[T]) RequireEvent(t *testing.T, eventType string, assertions ...func(require.TestingT, T)) {
	t.Helper()

	query := events.SearchEventsRequest{
		From:       s.t0,
		EventTypes: []string{eventType},
		Limit:      1,
		Order:      types.EventOrderDescending,
	}

	ctx := t.Context()

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			query.To = time.Now()

			events, _, err := s.auditLog.SearchEvents(ctx, query)
			require.NoError(t, err, "Fetching audit log messages must succeed")
			require.NotEmpty(t, events, "No events found")

			event, ok := events[0].(T)
			require.True(t, ok, "Unexpected event type: %T", events[0])
			for _, assertFn := range assertions {
				assertFn(t, event)
			}
		},
		3*time.Second,
		20*time.Millisecond)
}

// EventMetadataAssertion is is an assertion function that asserts some property
// of an audit event's metadata.
type EventMetadataAssertion func(require.TestingT, *apievents.Metadata)

// WithEventCode returns an EventMetadataAssertion that checks the event code
// matches the supplied value.
func WithEventCode(code string) EventMetadataAssertion {
	return func(t require.TestingT, event *apievents.Metadata) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, code, event.GetCode())
	}
}

// EventStatusAssertion is an assertion function that asserts some property of
// an audit event's status.
type EventStatusAssertion func(require.TestingT, *apievents.Status)

// WithSuccess returns an EventStatusAssertion that checks the event success
// flag matches the supplied value.
func WithSuccess(success bool) EventStatusAssertion {
	return func(t require.TestingT, eventStatus *apievents.Status) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, success, eventStatus.Success)
	}
}

// WithError is an EventStatusAssertion that asserts the event contains a
// non-empty error message.
func WithError(t require.TestingT, eventStatus *apievents.Status) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	require.NotEmpty(t, eventStatus.Error)
}

// WithNoError is an EventStatusAssertion that asserts the event contains an
// empty error message.
func WithNoError(t require.TestingT, eventStatus *apievents.Status) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	require.Empty(t, eventStatus.Error)
}

// WithErrorMatching returns an EventStatusAssertion that checks the event's
// error message matches the supplied regular expression pattern.
func WithErrorMatching(pattern string) EventStatusAssertion {
	return func(t require.TestingT, eventStatus *apievents.Status) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Regexp(t, regexp.MustCompile(pattern), eventStatus.Error)
	}
}
