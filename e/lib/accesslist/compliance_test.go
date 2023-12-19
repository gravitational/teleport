package accesslist

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

func TestReportCompliance(t *testing.T) {
	tests := []struct {
		name                          string
		accessLists                   []*accesslist.AccessList
		currentTime                   time.Time
		expectedTotalAccessLists      int32
		expectedAccessListsNeedReview int32
	}{
		{
			name: "no access lists",
		},
		{
			name: "all in compliance",
			accessLists: []*accesslist.AccessList{
				newAccessListWithNextAuditDate(t, "1", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "2", time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "3", time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "4", time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "5", time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC)),
			},
			currentTime:              time.Date(2022, 12, 31, 0, 0, 0, 0, time.UTC),
			expectedTotalAccessLists: 5,
		},
		{
			name: "some not in compliance",
			accessLists: []*accesslist.AccessList{
				newAccessListWithNextAuditDate(t, "1", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "2", time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "3", time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "4", time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "5", time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC)),
			},
			currentTime:                   time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC),
			expectedTotalAccessLists:      5,
			expectedAccessListsNeedReview: 2,
		},
		{
			name: "all not in compliance",
			accessLists: []*accesslist.AccessList{
				newAccessListWithNextAuditDate(t, "1", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "2", time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "3", time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "4", time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC)),
				newAccessListWithNextAuditDate(t, "5", time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC)),
			},
			currentTime:                   time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC),
			expectedTotalAccessLists:      5,
			expectedAccessListsNeedReview: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := initSvc(t)

			// Create all of the access lists.
			createAccessListsAndMembers(t, c.userCtx, c.svc, c.emitter, c.usageEvents,
				test.accessLists, nil)

			// Advance the time to the specified time.
			c.clock.Advance(test.currentTime.Sub(c.clock.Now()))

			require.NoError(t, c.svc.reportComplianceMetrics(c.userCtx))

			expectUsageReporterEvent(t, c.usageReporter, func(event *usagereporter.AccessListReviewComplianceEvent) {
				require.Equal(t, test.expectedTotalAccessLists, event.TotalAccessLists)
				require.Equal(t, test.expectedAccessListsNeedReview, event.AccessListsNeedReview)
			})
		})
	}
}

func expectUsageReporterEvent[T any](t *testing.T, usageReporter *usageReporter, fn func(T)) {
	t.Helper()

	require.NotEmpty(t, usageReporter.events)

	event := usageReporter.events[0]
	unwrapped, ok := event.(T)
	require.True(t, ok, "got unexpected type %T", event)

	// Remove the tp event
	usageReporter.events = usageReporter.events[1:]

	fn(unwrapped)
}
