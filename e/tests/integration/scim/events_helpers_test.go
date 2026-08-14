package scim

import (
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/tests/common"
)

// TODO(tcsc): revisit and see if there is a nicer way to handle duplicated assertions

func withResourceMetadata(assertions ...common.EventMetadataAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Metadata)
		}
	}
}

func withListingMetadata(assertions ...common.EventMetadataAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Metadata)
		}
	}
}

func withResourceStatus(assertions ...common.EventStatusAssertion) func(require.TestingT, *apievents.SCIMResourceEvent) {
	return func(t require.TestingT, event *apievents.SCIMResourceEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Status)
		}
	}
}

func withListingStatus(assertions ...common.EventStatusAssertion) func(require.TestingT, *apievents.SCIMListingEvent) {
	return func(t require.TestingT, event *apievents.SCIMListingEvent) {
		if h, ok := t.(interface{ Helper() }); ok {
			h.Helper()
		}
		for _, assertionFn := range assertions {
			assertionFn(t, &event.Status)
		}
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
