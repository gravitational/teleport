package okta

import (
	"context"
	"crypto"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
)

func TestProcessAssignments(t *testing.T) {
	t.Parallel()

	const link = "link"
	hash := crypto.SHA256
	appName := func(name string) string {
		return mustAppName(t, hash, name, link)
	}
	startTime := time.Now().UTC()
	zero := time.Time{}
	timeout := startTime.Add(5 * time.Minute)
	testUser := "test-user@test.user"
	oktaUserID := "okta-user-id"

	type auditEventInfo struct {
		name           string
		event          string
		code           string
		startingStatus string
		endingStatus   string
	}

	tests := []struct {
		name                   string
		groups                 types.UserGroups
		groupsSkipAddToOkta    bool
		apps                   types.AppServers
		deleteOktaAppIDs       map[string]bool
		appsSkipAddToOkta      bool
		reconcile              bool
		assignments            types.OktaAssignments
		expected               types.OktaAssignments
		incrementTimeDuration  time.Duration
		skipAssignmentCreation bool
		oktaClientGroupMapping map[string]map[string]bool
		oktaClientAppMapping   map[string]map[string]bool
		expectedAuditEvents    []auditEventInfo
		errAssertionFunc       require.ErrorAssertionFunc
	}{
		{
			name:                   "empty",
			assignments:            types.OktaAssignments{},
			expected:               types.OktaAssignments{},
			oktaClientGroupMapping: map[string]map[string]bool{},
			oktaClientAppMapping:   map[string]map[string]bool{},
			errAssertionFunc:       require.NoError,
		},
		{
			name: "fail to process app, skip pending app for different org, skip successful group",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
				application(t, hash, "app2", link, types.OriginOkta, "different-org-url"),
			},
			deleteOktaAppIDs: map[string]bool{
				mustAppName(t, hash, "app1", link): true,
				mustAppName(t, hash, "app2", link): true,
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusPending, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app2")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusFailed, startTime.Add(time.Minute), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app2")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration: time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {
					oktaUserID: true,
				},
			},
			oktaClientAppMapping: map[string]map[string]bool{},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentProcessEvent,
					code:           events.OktaAssignmentProcessFailureCode,
					startingStatus: constants.OktaAssignmentStatusPending,
					endingStatus:   constants.OktaAssignmentStatusFailed,
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, fmt.Sprintf(`app_server %q does not have an Okta App ID`, appName("app1")))
			},
		},
		{
			name: "successfully process app, retry failed app, fail to process group",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			groupsSkipAddToOkta: true,
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
				application(t, hash, "app2", link, types.OriginOkta, testOrgURL),
			},
			reconcile: true,
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusFailed, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app2")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusFailed, startTime.Add(10*time.Minute), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app2")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration:  10 * time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {
					oktaUserID: true,
				},
				"app2": {
					oktaUserID: true,
				},
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentProcessEvent,
					code:           events.OktaAssignmentProcessFailureCode,
					startingStatus: constants.OktaAssignmentStatusFailed,
					endingStatus:   constants.OktaAssignmentStatusFailed,
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "assignments for group group1 not found")
			},
		},
		{
			name: "successfully process group",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusPending, startTime, false,
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(time.Minute), false,
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration: time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {
					oktaUserID: true,
				},
			},
			oktaClientAppMapping: map[string]map[string]bool{},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentProcessEvent,
					code:           events.OktaAssignmentProcessSuccessCode,
					startingStatus: constants.OktaAssignmentStatusPending,
					endingStatus:   constants.OktaAssignmentStatusSuccessful,
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "fail to process group due to no assignment in backend",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusPending, startTime, false,
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected:               types.OktaAssignments{},
			incrementTimeDuration:  time.Minute,
			skipAssignmentCreation: true,
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {},
			},
			oktaClientAppMapping: map[string]map[string]bool{},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, `"assignment1" doesn't exist`)
			},
		},
		{
			name: "cleanup failure",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			groupsSkipAddToOkta: true,
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			appsSkipAddToOkta: true,
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusPending, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusFailed, startTime.Add(time.Minute), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration:  time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{},
			oktaClientAppMapping:   map[string]map[string]bool{},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentCleanupEvent,
					code:           events.OktaAssignmentCleanupFailureCode,
					startingStatus: constants.OktaAssignmentStatusPending,
					endingStatus:   constants.OktaAssignmentStatusFailed,
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, `assignments for app app1 not found`)
				require.ErrorContains(t, err, `assignments for group group1 not found`)
			},
		},
		{
			name: "cleanup success",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusPending, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusSuccessful, startTime.Add(time.Minute), true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration: time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {},
			},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {},
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentCleanupEvent,
					code:           events.OktaAssignmentCleanupSuccessCode,
					startingStatus: constants.OktaAssignmentStatusPending,
					endingStatus:   constants.OktaAssignmentStatusSuccessful,
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "finalized assignment not reprocessed",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusSuccessful, startTime, true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusSuccessful, startTime, true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration: time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {},
			},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "finalized assignment reprocessed due to later cleanup time",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime, true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(time.Minute), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration: time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {"okta-user-id": true},
			},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {"okta-user-id": true},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "processing timeout, app retried, group retried",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			reconcile: true,
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusProcessing, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(5*time.Minute), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration: 5 * time.Minute,
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {
					oktaUserID: true,
				},
			},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {
					oktaUserID: true,
				},
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentProcessEvent,
					code:           events.OktaAssignmentProcessSuccessCode,
					startingStatus: constants.OktaAssignmentStatusProcessing,
					endingStatus:   constants.OktaAssignmentStatusSuccessful,
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "cleanup due to assignment cleanup time passed",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			// The cleanup time is 1 minute ahead of the last transition time, which puts it in the window of
			// not immediately transitioning. However, assignments should be cleaned up immediately even if they're
			// within the retry window.
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusPending, timeout.Add(-time.Minute), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			// The expected last transition time is 2 minutes ahead of the timeout time.
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusSuccessful, timeout.Add(time.Minute), true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {},
			},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {},
			},
			// 6 minutes pass from the start time, which should trigger an immediate cleanup.
			incrementTimeDuration: 6 * time.Minute,
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentCleanupEvent,
					code:           events.OktaAssignmentCleanupSuccessCode,
					startingStatus: constants.OktaAssignmentStatusPending,
					endingStatus:   constants.OktaAssignmentStatusSuccessful,
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "cleanup retry",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusFailed, timeout.Add(1*time.Minute), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime.Add(15*time.Minute), true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			oktaClientGroupMapping: map[string]map[string]bool{
				"group1": {},
			},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {},
			},
			incrementTimeDuration: 15 * time.Minute,
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentCleanupEvent,
					code:           events.OktaAssignmentCleanupSuccessCode,
					startingStatus: constants.OktaAssignmentStatusFailed,
					endingStatus:   constants.OktaAssignmentStatusSuccessful,
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "still assigned because two assignments refer to the same target",
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			reconcile: true,
			assignments: types.OktaAssignments{
				assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				)},
			expected: types.OktaAssignments{
				assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(10*time.Minute), false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime.Add(10*time.Minute), true,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				)},
			oktaClientGroupMapping: map[string]map[string]bool{},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {
					oktaUserID: true,
				},
			},
			incrementTimeDuration: 10 * time.Minute,
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment2",
					event:          events.OktaAssignmentCleanupEvent,
					code:           events.OktaAssignmentCleanupSuccessCode,
					startingStatus: constants.OktaAssignmentStatusSuccessful,
					endingStatus:   constants.OktaAssignmentStatusSuccessful,
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "still assigned because two assignments refer to the same target (reverse order)",
			apps: types.AppServers{
				application(t, hash, "app1", link, types.OriginOkta, testOrgURL),
			},
			reconcile: true,
			assignments: types.OktaAssignments{
				assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				)},
			expected: types.OktaAssignments{
				assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime.Add(10*time.Minute), true,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(10*time.Minute), false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				)},
			oktaClientGroupMapping: map[string]map[string]bool{},
			oktaClientAppMapping: map[string]map[string]bool{
				"app1": {
					oktaUserID: true,
				},
			},
			incrementTimeDuration: 10 * time.Minute,
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentCleanupEvent,
					code:           events.OktaAssignmentCleanupSuccessCode,
					startingStatus: constants.OktaAssignmentStatusSuccessful,
					endingStatus:   constants.OktaAssignmentStatusSuccessful,
				},
			},
			errAssertionFunc: require.NoError,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clock := clockwork.NewFakeClockAt(startTime)
			ctx := context.Background()
			ap := newTestAccessPoint(t, clock)
			svc, oktaClient, emitter := newTestService(t, ap)
			svc.SetLeader(true)
			svc.clock = clock
			a := newAssignmentProcessor(svc, func() types.OktaAssignments {
				return test.assignments
			})

			oktaClient.addUserID(testUser, oktaUserID)

			// Make sure the rate limit is not in effect here.
			a.rateLimiter = rate.NewLimiter(rate.Inf, 1)

			for _, group := range test.groups {
				require.NoError(t, ap.CreateUserGroup(ctx, group))

				if !test.groupsSkipAddToOkta {
					oktaClient.addGroupToMapping(group.GetName())
				}
			}

			for _, app := range test.apps {
				if test.deleteOktaAppIDs != nil && test.deleteOktaAppIDs[app.GetName()] {
					labels := app.GetStaticLabels()
					delete(labels, teleport.OktaAppIDLabel)
					app.SetStaticLabels(labels)
				}
				_, err := ap.UpsertApplicationServer(ctx, app)
				require.NoError(t, err)

				if !test.appsSkipAddToOkta {
					oktaAppID, ok := app.GetLabel(teleport.OktaAppIDLabel)
					if ok {
						oktaClient.addApplicationToMapping(oktaAppID)
					}
				}
			}

			if !test.skipAssignmentCreation {
				for _, assignment := range test.assignments {
					_, err := ap.CreateOktaAssignment(ctx, assignment)
					require.NoError(t, err)
				}
			}

			clock.Advance(test.incrementTimeDuration)

			err := a.processAssignments(ctx, test.reconcile)
			test.errAssertionFunc(t, err)

			actual, _, err := ap.ListOktaAssignments(ctx, 0, "")
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(test.expected, types.OktaAssignments(actual),
				cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

			require.Empty(t, cmp.Diff(test.oktaClientGroupMapping, oktaClient.groupsToUsers))
			require.Empty(t, cmp.Diff(test.oktaClientAppMapping, oktaClient.appsToUsers))

			for _, expectedEvent := range test.expectedAuditEvents {
				expectAuditEvent(t, emitter, func(event *apievents.OktaAssignmentResult) {
					require.Equal(t, expectedEvent.name, event.Name)
					require.Equal(t, expectedEvent.event, event.GetType())
					require.Equal(t, expectedEvent.code, event.GetCode())
					require.Equal(t, expectedEvent.startingStatus, event.StartingStatus)
					require.Equal(t, expectedEvent.endingStatus, event.EndingStatus)
				})
			}
		})
	}
}
