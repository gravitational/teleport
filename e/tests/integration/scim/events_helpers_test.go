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
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Metadata)
		}
	}
}

func withListingMetadata(assertions ...metadatataAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Metadata)
		}
	}
}

func withEventCode(code string) metadatataAssertion {
	return func(t require.TestingT, event *apievents.Metadata) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, code, event.GetCode())
	}
}

type statusAssertion func(require.TestingT, *apievents.Status)

func withResourceStatus(assertions ...statusAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Status)
		}
	}
}

func withListingStatus(assertions ...statusAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Status)
		}
	}
}

func withSuccess(success bool) statusAssertion {
	return func(t require.TestingT, event *apievents.Status) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, success, event.Success)
	}
}

func withError(t require.TestingT, event *apievents.Status) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	require.NotEmpty(t, event.Error)
}

func withNoError(t require.TestingT, event *apievents.Status) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	require.Empty(t, event.Error)
}

func withErrorMatching(pattern string) statusAssertion {
	return func(t require.TestingT, event *apievents.Status) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Regexp(t, regexp.MustCompile(pattern), event.Error)
	}
}

type commonDataAssertion func(require.TestingT, *apievents.SCIMCommonData)

func withResourceCommonData(assertions ...commonDataAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.SCIMCommonData)
		}
	}
}

func withListingCommonData(assertions ...commonDataAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.SCIMCommonData)
		}
	}
}

func withResourceType(resourceType string) commonDataAssertion {
	return func(t require.TestingT, event *apievents.SCIMCommonData) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, resourceType, event.ResourceType)
	}
}

func withIntegration(plugin string) commonDataAssertion {
	return func(t require.TestingT, event *apievents.SCIMCommonData) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, plugin, event.Integration)
	}
}

func withResourceCount(n uint32) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, n, event.ResourceCount)
	}
}

func withFilter(filter string) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, filter, event.Filter)
	}
}

func withTeleportID(name string) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, name, event.TeleportID, "Teleport ID mismatch")
	}
}

func withExternalID(id string) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, id, event.ExternalID, "External ID mismatch")
	}
}

func withDisplayName(id string) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, id, event.Display, "Display name mismatch")
	}
}

func withNoResponse(t require.TestingT, event *apievents.SCIMResourceEvent) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	require.Nil(t, event.Response)
}

func withResponseStatusCode(expected uint32) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.NotNil(t, event.Response, "No response info supplied")
		require.Equal(t, expected, event.Response.StatusCode)
	}
}

type bodyAssertion func(require.TestingT, map[string]any)

func withExactly(expected map[string]any) bodyAssertion {
	return func(t require.TestingT, body map[string]any) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Equal(t, expected, body)
	}
}

func fieldNotEmpty(name string) bodyAssertion {
	return func(t require.TestingT, body map[string]any) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Contains(t, body, name)
		require.NotEmpty(t, body[name])
	}
}

func fieldLen(name string, expected int) bodyAssertion {
	return func(t require.TestingT, body map[string]any) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Contains(t, body, name)
		require.Len(t, body[name], expected)
	}
}

func fieldEquals(name string, expected any) bodyAssertion {
	return func(t require.TestingT, body map[string]any) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.Contains(t, body, name)
		require.Equal(t, expected, body[name])
	}
}

func withRequestBody(assertions ...bodyAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.NotNil(t, event.Request, "Request missing")
		assertBodyStruct(t, "Request", event.Request.Body, assertions)
	}
}

func withResponseBody(assertions ...bodyAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		require.NotNil(t, event.Response, "Response missing")
		assertBodyStruct(t, "Response", event.Response.Body, assertions)
	}
}

func assertBodyStruct(t require.TestingT, name string, bodyStruct *apievents.Struct, assertions []bodyAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	var body map[string]any

	if bodyStruct != nil {
		var err error
		body, err = apievents.DecodeToMap(bodyStruct)
		require.NoError(t, err, "%s body decoding failed", name)
	}

	for _, assertion := range assertions {
		assertion(t, body)
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
