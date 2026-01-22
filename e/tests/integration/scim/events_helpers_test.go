package scim

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/events"
)

type logscope[T apievents.AuditEvent] struct {
	t0       time.Time
	auditLog events.AuditLogSessionStreamer
}

func newLogScope[T apievents.AuditEvent](sut *common.SUT) logscope[T] {
	return logscope[T]{t0: time.Now(), auditLog: sut.Teleport.Process.GetAuditLog()}
}

// TODO(tcsc): revisit and see if there is a nicer way to handle duplicated assertions

type metadatataAssertion func(require.TestingT, *apievents.Metadata)

func withResourceMetadata(assertions ...metadatataAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Metadata)
		}
	}
}

func withListingMetadata(assertions ...metadatataAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Metadata)
		}
	}
}

func withEventCode(code string) metadatataAssertion {
	return func(t require.TestingT, event *apievents.Metadata) {
		require.Equal(t, code, event.GetCode())
	}
}

type statusAssertion func(require.TestingT, *apievents.Status)

func withResourceStatus(assertions ...statusAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Status)
		}
	}
}

func withListingStatus(assertions ...statusAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Status)
		}
	}
}

func withSuccess(success bool) statusAssertion {
	return func(t require.TestingT, event *apievents.Status) {
		require.Equal(t, success, event.Success)
	}
}

func withError(t require.TestingT, event *apievents.Status) {
	require.NotEmpty(t, event.Error)
}

func withNoError(t require.TestingT, event *apievents.Status) {
	require.Empty(t, event.Error)
}

func withErrorMatching(pattern string) statusAssertion {
	return func(t require.TestingT, event *apievents.Status) {
		require.Regexp(t, regexp.MustCompile(pattern), event.Error)
	}
}

type commonDataAssertion func(require.TestingT, *apievents.SCIMCommonData)

func withResourceCommonData(assertions ...commonDataAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		for _, assertionFn := range assertions {
			assertionFn(t, &event.SCIMCommonData)
		}
	}
}

func withListingCommonData(assertions ...commonDataAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		for _, assertionFn := range assertions {
			assertionFn(t, &event.SCIMCommonData)
		}
	}
}

func withResourceType(resourceType string) commonDataAssertion {
	return func(t require.TestingT, event *apievents.SCIMCommonData) {
		require.Equal(t, resourceType, event.ResourceType)
	}
}

func withIntegration(plugin string) commonDataAssertion {
	return func(t require.TestingT, event *apievents.SCIMCommonData) {
		require.Equal(t, plugin, event.Integration)
	}
}

func withResourceCount(n uint32) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		require.Equal(t, n, event.ResourceCount)
	}
}

func withFilter(filter string) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		require.Equal(t, filter, event.Filter)
	}
}

func withTeleportID(name string) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		require.Equal(t, name, event.TeleportID, "Teleport ID mismatch")
	}
}

func withExternalID(id string) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		require.Equal(t, id, event.ExternalID, "External ID mismatch")
	}
}

func withDisplayName(id string) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		require.Equal(t, id, event.Display, "Display name mismatch")
	}
}

func withBody(expected map[string]any) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		body := event.Request.Body
		if expected == nil {
			require.Nil(t, body)
			return
		}

		require.NotNil(t, body)
		actual, err := apievents.DecodeToMap(body)
		require.NoError(t, err)

		require.Equal(t, expected, actual)
	}
}

func (s *logscope[T]) requireNoEvent(t *testing.T, eventType string) {
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

// requireEvent waits for a resource sync event to arrive in the
// emitter channel
func (s *logscope[T]) requireEvent(t *testing.T, eventType string, assertions ...func(require.TestingT, T)) {
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
