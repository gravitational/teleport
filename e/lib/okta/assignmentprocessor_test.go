package okta

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaapitest "github.com/gravitational/teleport/e/lib/okta/api/apitest"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils/set"
)

func TestProcessAssignments(t *testing.T) {
	t.Parallel()

	const link = "link"
	appName := func(name string) string {
		return mustAppName(t, name, link)
	}
	startTime := time.Now().UTC()
	zero := time.Time{}
	timeout := startTime.Add(1 * time.Minute)
	testUser := userName("test-user@test.user")
	testOktaUserID := oktaUserID("okta-user-id")

	type auditEventInfo struct {
		name             string
		event            string
		code             string
		startingStatus   string
		endingStatus     string
		errorAssertionFn func(t require.TestingT, s any, msgAndArgs ...any)
	}

	tests := []struct {
		name                   string
		groups                 types.UserGroups
		groupsSkipAddToOkta    bool
		apps                   types.AppServers
		deleteOktaAppIDs       map[string]bool
		appsSkipAddToOkta      bool
		assignments            types.OktaAssignments
		expected               types.OktaAssignments
		incrementTimeDuration  time.Duration
		skipAssignmentCreation bool
		oktaClientGroupMapping map[oktaapi.OktaGroupID]set.Set[oktaapi.OktaUserID]
		oktaClientAppMapping   map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]
		expectedAuditEvents    []auditEventInfo
	}{
		{
			name:        "empty",
			assignments: types.OktaAssignments{},
		},
		{
			name: "fail to process app, skip pending app for different org, skip successful group",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
				application(t, "app2", link, types.OriginOkta, "different-org-url"),
			},
			deleteOktaAppIDs: map[string]bool{
				mustAppName(t, "app1", link): true,
				mustAppName(t, "app2", link): true,
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
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New(testOktaUserID),
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentProcessEvent,
					code:           events.OktaAssignmentProcessFailureCode,
					startingStatus: constants.OktaAssignmentStatusPending,
					endingStatus:   constants.OktaAssignmentStatusFailed,
					errorAssertionFn: func(t require.TestingT, s any, msgAndArgs ...any) {
						require.Contains(t, s, fmt.Sprintf(`app_server %q does not have an Okta App ID`, appName("app1")), msgAndArgs...)
					},
				},
			},
		},
		{
			name: "successfully process app, retry failed app, fail to process group",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			groupsSkipAddToOkta: true,
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
				application(t, "app2", link, types.OriginOkta, testOrgURL),
			},
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
			incrementTimeDuration: 10 * time.Minute,
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New(oktaapi.AppAssignment{UserID: string(testOktaUserID), Scope: oktaapi.UserScope}),
				"app2": set.New(oktaapi.AppAssignment{UserID: string(testOktaUserID), Scope: oktaapi.UserScope}),
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentProcessEvent,
					code:           events.OktaAssignmentProcessFailureCode,
					startingStatus: constants.OktaAssignmentStatusFailed,
					endingStatus:   constants.OktaAssignmentStatusFailed,
					errorAssertionFn: func(t require.TestingT, s any, msgAndArgs ...any) {
						require.Contains(t, s, "assignments for group group1 not found")
					},
				},
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
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New(testOktaUserID),
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:             "assignment1",
					event:            events.OktaAssignmentProcessEvent,
					code:             events.OktaAssignmentProcessSuccessCode,
					startingStatus:   constants.OktaAssignmentStatusPending,
					endingStatus:     constants.OktaAssignmentStatusSuccessful,
					errorAssertionFn: require.Empty,
				},
			},
		},
		{
			name: "fail to process group due to no assignment in backend",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusPending, startTime, false,
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration:  time.Minute,
			skipAssignmentCreation: true,
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New[oktaUserID](),
			},
		},
		{
			name: "cleanup failure",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			groupsSkipAddToOkta: true,
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
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
			incrementTimeDuration: time.Minute,
			expectedAuditEvents: []auditEventInfo{
				{
					name:           "assignment1",
					event:          events.OktaAssignmentCleanupEvent,
					code:           events.OktaAssignmentCleanupFailureCode,
					startingStatus: constants.OktaAssignmentStatusPending,
					endingStatus:   constants.OktaAssignmentStatusFailed,
					errorAssertionFn: func(t require.TestingT, s any, msgAndArgs ...any) {
						require.Contains(t, s, `assignments for app app1 not found`)
						require.Contains(t, s, `assignments for group group1 not found`)
					},
				},
			},
		},
		{
			name: "cleanup success",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
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
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New[oktaUserID](),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New[oktaapi.AppAssignment](),
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:             "assignment1",
					event:            events.OktaAssignmentCleanupEvent,
					code:             events.OktaAssignmentCleanupSuccessCode,
					startingStatus:   constants.OktaAssignmentStatusPending,
					endingStatus:     constants.OktaAssignmentStatusSuccessful,
					errorAssertionFn: require.Empty,
				},
			},
		},
		{
			name: "finalized assignment will be deleted",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusSuccessful, startTime, true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected:              nil,
			incrementTimeDuration: time.Minute,
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New[oktaUserID](),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New[oktaapi.AppAssignment](),
			},
		},
		{
			name: "finalized assignment reprocessed due to later cleanup time",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
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
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New(testOktaUserID),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New(oktaapi.AppAssignment{UserID: "okta-user-id", Scope: oktaapi.UserScope}),
			},
		},
		{
			name: "processing, app retried, group retried right away",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusProcessing, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(1*time.Second), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			incrementTimeDuration: 1 * time.Second,
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New(testOktaUserID),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New(oktaapi.AppAssignment{UserID: string(testOktaUserID), Scope: oktaapi.UserScope}),
			},
			expectedAuditEvents: []auditEventInfo{
				{
					name:             "assignment1",
					event:            events.OktaAssignmentProcessEvent,
					code:             events.OktaAssignmentProcessSuccessCode,
					startingStatus:   constants.OktaAssignmentStatusProcessing,
					endingStatus:     constants.OktaAssignmentStatusSuccessful,
					errorAssertionFn: require.Empty,
				},
			},
		},
		{
			name: "cleanup due to assignment cleanup time passed",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
			},
			// The cleanup time is the same time as the last transition time, which puts it in the window of
			// not immediately transitioning. However, assignments should be cleaned up immediately even if they're
			// within the retry window.
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusPending, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusSuccessful, startTime.Add(1*time.Second), true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New[oktaUserID](),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New[oktaapi.AppAssignment](),
			},
			incrementTimeDuration: 1 * time.Second,
			expectedAuditEvents: []auditEventInfo{
				{
					name:             "assignment1",
					event:            events.OktaAssignmentCleanupEvent,
					code:             events.OktaAssignmentCleanupSuccessCode,
					startingStatus:   constants.OktaAssignmentStatusPending,
					endingStatus:     constants.OktaAssignmentStatusSuccessful,
					errorAssertionFn: require.Empty,
				},
			},
		},
		{
			name: "failed cleanup retry",
			groups: types.UserGroups{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusFailed, startTime, false,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			expected: types.OktaAssignments{assignment(t, "assignment1", testUser, startTime, constants.OktaAssignmentStatusSuccessful, startTime.Add(oktaplugin.DefaultTimeBetweenAssignmentProcessLoops), true,
				target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			)},
			oktaClientGroupMapping: map[oktaGroupID]set.Set[oktaUserID]{
				"group1": set.New[oktaUserID](),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New[oktaapi.AppAssignment](),
			},
			incrementTimeDuration: oktaplugin.DefaultTimeBetweenAssignmentProcessLoops,
			expectedAuditEvents: []auditEventInfo{
				{
					name:             "assignment1",
					event:            events.OktaAssignmentCleanupEvent,
					code:             events.OktaAssignmentCleanupSuccessCode,
					startingStatus:   constants.OktaAssignmentStatusFailed,
					endingStatus:     constants.OktaAssignmentStatusSuccessful,
					errorAssertionFn: require.Empty,
				},
			},
		},
		{
			name: "still assigned because two assignments refer to the same target",
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{
				assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
			},
			expected: types.OktaAssignments{
				assignment(t, "assignment1", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(oktaplugin.DefaultTimeBetweenAssignmentProcessLoops), false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime.Add(oktaplugin.DefaultTimeBetweenAssignmentProcessLoops), true,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New(oktaapi.AppAssignment{UserID: string(testOktaUserID), Scope: oktaapi.UserScope}),
			},
			incrementTimeDuration: oktaplugin.DefaultTimeBetweenAssignmentProcessLoops,
			expectedAuditEvents: []auditEventInfo{
				{
					name:             "assignment2",
					event:            events.OktaAssignmentCleanupEvent,
					code:             events.OktaAssignmentCleanupSuccessCode,
					startingStatus:   constants.OktaAssignmentStatusSuccessful,
					endingStatus:     constants.OktaAssignmentStatusSuccessful,
					errorAssertionFn: require.Empty,
				},
			},
		},
		{
			name: "still assigned because two assignments refer to the same target (reverse order)",
			apps: types.AppServers{
				application(t, "app1", link, types.OriginOkta, testOrgURL),
			},
			assignments: types.OktaAssignments{
				assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime, false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
			},
			expected: types.OktaAssignments{
				assignment(t, "assignment1", testUser, timeout, constants.OktaAssignmentStatusSuccessful, startTime.Add(oktaplugin.DefaultTimeBetweenAssignmentProcessLoops), true,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
				assignment(t, "assignment2", testUser, zero, constants.OktaAssignmentStatusSuccessful, startTime.Add(oktaplugin.DefaultTimeBetweenAssignmentProcessLoops), false,
					target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
				),
			},
			oktaClientAppMapping: map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]{
				"app1": set.New(oktaapi.AppAssignment{UserID: string(testOktaUserID), Scope: oktaapi.UserScope}),
			},
			incrementTimeDuration: oktaplugin.DefaultTimeBetweenAssignmentProcessLoops,
			expectedAuditEvents: []auditEventInfo{
				{
					name:             "assignment1",
					event:            events.OktaAssignmentCleanupEvent,
					code:             events.OktaAssignmentCleanupSuccessCode,
					startingStatus:   constants.OktaAssignmentStatusSuccessful,
					endingStatus:     constants.OktaAssignmentStatusSuccessful,
					errorAssertionFn: require.Empty,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clock := clockwork.NewFakeClockAt(startTime)
			ctx := context.Background()
			ap := newTestAccessPoint(t, clock)
			oktaClient := newTestOktaClient()
			svc, emitter := newTestService(t, ap, oktaClient)
			svc.clock = clock
			a := newAssignmentProcessor(svc)

			oktaClient.AddUserID(testUser, testOktaUserID)

			// Make sure the rate limit is not in effect here.
			a.rateLimiter = rate.NewLimiter(rate.Inf, 1)

			for _, group := range test.groups {
				a.syncedUserGroups.Store(group.GetName(), group)

				if !test.groupsSkipAddToOkta {
					oktaClient.AddGroupToMapping(group.GetName())
				}
			}

			for _, app := range test.apps {
				if test.deleteOktaAppIDs != nil && test.deleteOktaAppIDs[app.GetName()] {
					labels := app.GetStaticLabels()
					delete(labels, teleport.OktaAppIDLabel)
					app.SetStaticLabels(labels)
				}
				a.syncedAppServers.Store(app.GetName(), app)

				if !test.appsSkipAddToOkta {
					oktaAppID, ok := app.GetLabel(teleport.OktaAppIDLabel)
					if ok {
						oktaClient.AddApplicationToMapping(oktaAppID)
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

			a.processTimerEvent(ctx)

			actual, _, err := ap.ListOktaAssignments(ctx, 0, "")
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(test.expected, types.OktaAssignments(actual),
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			oktaClient.GroupsToUsers.Read(func(m map[oktaGroupID]set.Set[oktaUserID]) {
				require.Empty(t, cmp.Diff(test.oktaClientGroupMapping, m))
			})

			oktaClient.AppsToUsers.Read(func(m map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]) {
				require.Empty(t, cmp.Diff(test.oktaClientAppMapping, m))
			})

			var events []*apievents.OktaAssignmentResult
			collectAllEvents(t, emitter, &events)

			require.Len(t, events, len(test.expectedAuditEvents))
			for i, event := range events {
				expectedEvent := test.expectedAuditEvents[i]
				require.Equal(t, expectedEvent.name, event.Name)
				require.Equal(t, expectedEvent.event, event.GetType())
				require.Equal(t, expectedEvent.code, event.GetCode())
				require.Equal(t, expectedEvent.startingStatus, event.StartingStatus)
				require.Equal(t, expectedEvent.endingStatus, event.EndingStatus)
				expectedEvent.errorAssertionFn(t, event.Error)
			}
		})
	}
}

// Test_assignmentProcessor_targetProcessingTimeout ensures at most processAssignmentTargetsTimeout
// can be spent processing okta_assignment's targets. It also makes sure the status is updated and
// audit event emitted even if targets processing timed out.
func Test_assignmentProcessor_targetProcessingTimeout(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		clock := clockwork.NewRealClock()
		ap := newTestAccessPoint(t, clock)
		oktaClient, oktaData := oktaapitest.NewLocalDataClient(t)
		oktaClient.OrgURLFunc = func(t *testing.T) string { return oktaapitest.TestOrgURL }
		svc, emitter := newTestService(t, ap, oktaClient, withClock(clock))

		const group1, group2 = "group1", "group2"
		const user = "test-user@test.user"

		oktaData.UpsertUserForId(user, "okta-user-id")
		oktaData.UpsertGroupForId(group1)
		oktaData.UpsertGroupForId(group2)

		// Synchronize to populate newGroups.
		require.NoError(t, svc.synchronize(ctx))
		// Drain events.
		collectAllEvents(t, emitter, new([]apievents.AuditEvent))

		// Simulate GetGroupAssignments timing out for group2.
		getGroupAssignments := oktaClient.GetGroupAssignmentsFunc
		oktaClient.GetGroupAssignmentsFunc = func(t *testing.T, ctx context.Context, groupID oktaapi.OktaGroupID) ([]oktaapi.OktaUserID, error) {
			if groupID != group2 {
				return getGroupAssignments(t, ctx, groupID)
			}
			<-ctx.Done() // we are inside synctest bubble so this will not block
			return nil, ctx.Err()
		}

		processor := svc.assignmentReconciler.assignmentProcessor
		assignment1 := assignment(t, "test_assignment_1", user, time.Time{}, constants.OktaAssignmentStatusPending, time.Now(), false,
			target(types.OktaAssignmentTargetV1_GROUP, group1),
			target(types.OktaAssignmentTargetV1_GROUP, group2),
		)

		assignment1, err := ap.CreateOktaAssignment(ctx, assignment1)
		require.NoError(t, err)

		timeBeforeProcessing := time.Now()

		result := processor.processAssignment(ctx, processor.logger, assignment1)
		require.Equal(t, processAssignmentProcessed, result)

		timeAfterProcessing := time.Now()
		require.Equal(t, timeBeforeProcessing.Add(processAssignmentTargetsTimeout), timeAfterProcessing)

		// Verify the assignment is in "failed" state.
		assignment1Updated, err := ap.GetOktaAssignment(ctx, assignment1.GetName())
		require.NoError(t, err)
		require.Equal(t, constants.OktaAssignmentStatusFailed, assignment1Updated.GetStatus())

		// Verify the audit event for the assignment going from "pending" -> "failed" and
		// has expected errors.
		event := requireAuditEvent[*apievents.OktaAssignmentResult](t, emitter)
		require.Equal(t, events.OktaAssignmentProcessEvent, event.GetType())
		require.Equal(t, events.OktaAssignmentProcessFailureCode, event.GetCode())
		require.Equal(t, assignment1.GetName(), event.ResourceMetadata.Name)
		require.Equal(t, constants.OktaAssignmentStatusPending, event.StartingStatus)
		require.Equal(t, constants.OktaAssignmentStatusFailed, event.EndingStatus)
		require.Contains(t, event.Error, `failed to provision target "group:group2": context deadline exceeded`)
		require.Contains(t, event.Error, "assignment targets processing timed out")
	})
}

// Test_assignmentProcessor_GetAccessListMember_error checks that when AccessList service fails to
// get a member for reasons different that NotFound, it results in processing failure and and audit
// event.
//
// If it isn't a critical error, then it may lead to race when an Okta-side managed assignment is
// deleted resulting in re-creating the assignment.
func Test_assignmentProcessor_GetAccessListMember_error(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewFakeClock()
	ap := newTestAccessPoint(t, clock)
	oktaClient, oktaData := oktaapitest.NewLocalDataClient(t)
	oktaClient.OrgURLFunc = func(t *testing.T) string { return oktaapitest.TestOrgURL }
	svc, emitter := newTestService(t, ap, oktaClient, withClock(clock))
	processor := svc.assignmentReconciler.assignmentProcessor

	const groupName = "group1"
	const testUser = "test-user@test.user"
	const testUserOktaID = "okta-user-id"

	oktaData.UpsertUserForId(testUser, testUserOktaID)
	oktaData.UpsertGroupForId(groupName)

	// Synchronize to populate newGroups.
	require.NoError(t, svc.synchronize(ctx))
	// Drain events.
	collectAllEvents(t, emitter, new([]apievents.AuditEvent))

	assignment1, err := ap.CreateOktaAssignment(ctx, assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_GROUP, groupName),
	))
	require.NoError(t, err)

	// When any other error than NotFound is encountered while checking AccessList membership,
	// it's a failure.
	processor.accessPoint.accessListService = &failingAccessListService{
		err: trace.Errorf("test access list service error"),
	}
	_ = processor.processAssignment(ctx, processor.logger, assignment1)

	assignment1, err = ap.GetOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	require.Equal(t, constants.OktaAssignmentStatusFailed, assignment1.GetStatus())

	event := requireAuditEvent[*apievents.OktaAssignmentResult](t, emitter)
	require.Equal(t, events.OktaAssignmentProcessEvent, event.GetType())
	require.Equal(t, events.OktaAssignmentProcessFailureCode, event.GetCode())
	require.Equal(t, constants.OktaAssignmentStatusPending, event.StartingStatus)
	require.Equal(t, constants.OktaAssignmentStatusFailed, event.EndingStatus)
	require.Contains(t, event.Error, `failed to provision target "group:group1": failed to check user's AccessList membership: test access list service error`)

	// But not found is OK.
	processor.accessPoint.accessListService = &failingAccessListService{
		err: trace.NotFound("NotFound should be handled as a happy path"),
	}
	// We need to advance clock so the failed assignment is processed, otherwise
	// assignmentProcessor.shouldProcess() will return false.
	clock.Advance(oktaplugin.DefaultTimeBetweenAssignmentProcessLoops)
	_ = processor.processAssignment(ctx, processor.logger, assignment1)

	assignment1, err = ap.GetOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	require.Equal(t, constants.OktaAssignmentStatusSuccessful, assignment1.GetStatus())

	event = requireAuditEvent[*apievents.OktaAssignmentResult](t, emitter)
	require.Equal(t, events.OktaAssignmentProcessEvent, event.GetType())
	require.Equal(t, events.OktaAssignmentProcessSuccessCode, event.GetCode())
	require.Equal(t, constants.OktaAssignmentStatusFailed, event.StartingStatus)
	require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.EndingStatus)
	require.Empty(t, event.Error)
}

// This test checks if assignments are processed in order from the highest to lowest priority.
func Test_assignmentProcessor_processAssignments_priority(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewRealClock()
	ap := newTestAccessPoint(t, clock)
	oktaClient, oktaData := oktaapitest.NewLocalDataClient(t)
	oktaClient.OrgURLFunc = func(t *testing.T) string { return oktaapitest.TestOrgURL }
	svc, _ := newTestService(t, ap, oktaClient, withClock(clock))

	const user1, uid1, user2, uid2 = "user1", "user_id_1", "user2", "user_id_2"
	const application1, group1 = "application_id_1", "group_id_1"
	oktaData.UpsertUserForId(user1, uid1)
	oktaData.UpsertUserForId(user2, uid2)
	oktaData.UpsertAppForId(application1)
	oktaData.UpsertGroupForId(group1)

	now := clock.Now()

	cleanupTimeZero := time.Time{}
	cleanupTimePast1 := now.Add(-2 * time.Hour)
	cleanupTimePast2 := now.Add(-1 * time.Hour)
	cleanupTimeFuture1 := now.Add(1 * time.Hour)
	cleanupTimeFuture2 := now.Add(2 * time.Hour)
	lastProcessedT1 := now.Add(-4*time.Minute - oktaplugin.DefaultTimeBetweenAssignmentProcessLoops)
	lastProcessedT2 := now.Add(-3*time.Minute - oktaplugin.DefaultTimeBetweenAssignmentProcessLoops)
	lastProcessedT3 := now.Add(-2*time.Minute - oktaplugin.DefaultTimeBetweenAssignmentProcessLoops)
	lastProcessedT4 := now.Add(-1*time.Minute - oktaplugin.DefaultTimeBetweenAssignmentProcessLoops)

	assignmentDescs := []struct {
		cleanupTime   time.Time
		status        string
		lastProcessed time.Time
	}{
		//  1. Assignments to clean up, then by CleanupTime, then by LastTransitionTime.
		{cleanupTimePast1, constants.OktaAssignmentStatusSuccessful, lastProcessedT2}, // 01
		{cleanupTimePast1, constants.OktaAssignmentStatusSuccessful, lastProcessedT4}, // 02
		{cleanupTimePast2, constants.OktaAssignmentStatusFailed, lastProcessedT1},     // 03
		{cleanupTimePast2, constants.OktaAssignmentStatusPending, lastProcessedT3},    // 04
		//  2. Pending assignments, then by LastTransitionTime.
		{cleanupTimeZero, constants.OktaAssignmentStatusPending, lastProcessedT1},    // 05
		{cleanupTimeFuture2, constants.OktaAssignmentStatusPending, lastProcessedT2}, // 06
		{cleanupTimeFuture1, constants.OktaAssignmentStatusPending, lastProcessedT3}, // 07
		{cleanupTimeZero, constants.OktaAssignmentStatusPending, lastProcessedT4},    // 08
		//  3. Assignments stuck in processing, then by LastTransitionTime.
		{cleanupTimeFuture1, constants.OktaAssignmentStatusProcessing, lastProcessedT1}, // 09
		{cleanupTimeFuture2, constants.OktaAssignmentStatusProcessing, lastProcessedT2}, // 10
		{cleanupTimeZero, constants.OktaAssignmentStatusProcessing, lastProcessedT3},    // 11
		{cleanupTimeFuture1, constants.OktaAssignmentStatusProcessing, lastProcessedT4}, // 12
		//  4. Failed assignments, then by LastTransitionTime.
		{cleanupTimeFuture1, constants.OktaAssignmentStatusFailed, lastProcessedT1}, // 13
		{cleanupTimeFuture1, constants.OktaAssignmentStatusFailed, lastProcessedT2}, // 14
		{cleanupTimeZero, constants.OktaAssignmentStatusFailed, lastProcessedT3},    // 15
		{cleanupTimeZero, constants.OktaAssignmentStatusFailed, lastProcessedT4},    // 16
		//  5. LastTransitionTime.
		{cleanupTimeZero, constants.OktaAssignmentStatusSuccessful, lastProcessedT1},    // 17
		{cleanupTimeFuture2, constants.OktaAssignmentStatusSuccessful, lastProcessedT2}, // 18
		{cleanupTimeFuture1, constants.OktaAssignmentStatusSuccessful, lastProcessedT3}, // 19
		{cleanupTimeFuture2, constants.OktaAssignmentStatusSuccessful, lastProcessedT4}, // 20
	}

	var expectedAssignments []types.OktaAssignment
	for i, desc := range assignmentDescs {
		name := fmt.Sprintf("assignment%02d-%s", i+1, uuid.NewString())
		const notFinalized = false
		a, err := ap.CreateOktaAssignment(ctx, assignment(t, name, user1, desc.cleanupTime, desc.status, desc.lastProcessed, notFinalized,
			target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, application1, oktaapitest.TestLink1Name)),
			target(types.OktaAssignmentTargetV1_GROUP, group1),
		))
		require.NoError(t, err)
		expectedAssignments = append(expectedAssignments, a)
	}

	// First let's see if sortAssignmentsByProcessingPriority works as expected.
	// Iterate 10 times to increase the chances to find an edge case.
	for range 10 {
		shuffled := slices.Clone(expectedAssignments)
		rand.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})

		sortAssignmentsByProcessingPriority(now, shuffled)
		requireEqualOktaAssignments(t, expectedAssignments, shuffled)
	}

	// Now let's check if sortAssignmentsByProcessingPriority is used during processing.
	svc.assignmentReconciler.assignmentProcessor.accessPoint.oktaAssignmentService = &shufflingOktaAssignmentService{svc.assignmentReconciler.assignmentProcessor.accessPoint.oktaAssignmentService}
	svc.assignmentReconciler.assignmentProcessor.emitter = dummyEmitter{}
	svc.assignmentReconciler.assignmentProcessor.processTimerEvent(ctx)
	sortedProcessedAssignments := mustGetAllOktaAssignments(t, ap)
	sortByLastTransition(sortedProcessedAssignments) // oldest LastTransition was processed first
	require.Equal(t, getOktaAssignmentNames(expectedAssignments), getOktaAssignmentNames(sortedProcessedAssignments))
}

type shufflingOktaAssignmentService struct {
	oktaAssignmentService
}

func (s *shufflingOktaAssignmentService) ListOktaAssignments(ctx context.Context, limit int, pageToken string) ([]types.OktaAssignment, string, error) {
	page, nextPageToken, err := s.oktaAssignmentService.ListOktaAssignments(ctx, limit, pageToken)
	rand.Shuffle(len(page), func(i, j int) { page[i], page[j] = page[j], page[i] })
	return page, nextPageToken, err
}

func Test_assignmentNeedsUrgentProcessing(t *testing.T) {
	t.Parallel()

	const notFinalized = false
	now := time.Date(2026, time.April, 17, 12, 0, 0, 0, time.UTC)
	cleanupTimePast := now.Add(-time.Minute)
	cleanupTimeFuture := now.Add(time.Minute)
	testUser := userName("test-user")

	tests := []struct {
		name       string
		assignment types.OktaAssignment
		want       bool
	}{
		{
			name:       "pending assignment is urgent",
			assignment: assignment(t, "pending", testUser, time.Time{}, constants.OktaAssignmentStatusPending, now, notFinalized),
			want:       true,
		},
		{
			name:       "CleanupTime in the past and LastTransition before the CleanupTime is urgent",
			assignment: assignment(t, "cleanup-overdue", testUser, cleanupTimePast, constants.OktaAssignmentStatusSuccessful, cleanupTimePast.Add(-time.Second), notFinalized),
			want:       true,
		},
		{
			name:       "CleanupTime in the past and LastTransition before the CleanupTime is urgent even if processing",
			assignment: assignment(t, "cleanup-overdue", testUser, cleanupTimePast, constants.OktaAssignmentStatusProcessing, cleanupTimePast.Add(-time.Second), notFinalized),
			want:       true,
		},
		{
			name:       "CleanupTime in the past and LastTransition exactly at CleanupTime time is not urgent",
			assignment: assignment(t, "cleanup-at-boundary", testUser, cleanupTimePast, constants.OktaAssignmentStatusSuccessful, cleanupTimePast, notFinalized),
			want:       false,
		},
		{
			name:       "CleanupTime in the past and LastTransition after the CleanupTime time is not urgent",
			assignment: assignment(t, "cleanup-after", testUser, cleanupTimePast, constants.OktaAssignmentStatusFailed, cleanupTimePast.Add(time.Second), notFinalized),
			want:       false,
		},
		{
			name:       "CleanupTime in the Future is not urgent",
			assignment: assignment(t, "cleanup-future", testUser, cleanupTimeFuture, constants.OktaAssignmentStatusFailed, now.Add(-100*time.Hour), notFinalized),
			want:       false,
		},
		{
			name:       "Failed assignment without CleanupTime set is not urgent",
			assignment: assignment(t, "failed", testUser, time.Time{}, constants.OktaAssignmentStatusFailed, now.Add(-100*time.Hour), notFinalized),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := assignmentNeedsUrgentProcessing(tt.assignment, now)
			require.Equal(t, tt.want, got)
		})
	}
}

// This test makes sure we don't skip Okta-side cleanup of okta_assignment resources that expired
// during plugin restart. This may happen due to race conditions and/or proper reconcilers seeding.
func Test_assignmentProcessor_cleanup_after_start(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewRealClock()
	ap := newTestAccessPoint(t, clock)
	oktaClient, oktaData := oktaapitest.NewLocalDataClient(t)
	oktaClient.OrgURLFunc = func(t *testing.T) string { return oktaapitest.TestOrgURL }
	svc, _ := newTestService(t, ap, oktaClient, withClock(clock))

	const user1, uid1, user2, uid2 = "user1", "user_id_1", "user2", "user_id_2"
	const application1, group1 = "application_id_1", "group_id_1"
	oktaData.UpsertUserForId(user1, uid1)
	oktaData.UpsertUserForId(user2, uid2)
	oktaData.UpsertAppForId(application1)
	oktaData.UpsertGroupForId(group1)

	err := svc.Start(ctx)
	require.NoError(t, err)

	// Wait for sync.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		requireUsersExist(t, ap, user1, user2)
		requireOktaAppServers(t, ap, []string{mustAppName(t, application1, oktaapitest.TestLink1Name)})
		requireUserGroups(t, ap, []string{group1})
	}, time.Second*10, time.Millisecond*50)

	// Assert there are no Okta-side assignments before creating okta_assignment.
	requireOktaSideApplicationAssignments(t, oktaClient, application1, nil)
	requireOktaSideGroupAssignments(t, oktaClient, group1, nil)

	cleanupTime := time.Time{} // never
	finalized := false
	lastTransitionTime := time.Time{}
	assignment, err := ap.CreateOktaAssignment(ctx, assignment(t, "assignment1", user1, cleanupTime, constants.OktaAssignmentStatusPending, lastTransitionTime, finalized,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, application1, oktaapitest.TestLink1Name)),
		target(types.OktaAssignmentTargetV1_GROUP, group1),
	))
	require.NoError(t, err)

	// Assert the okta_assignment was reconciled and Okta-side assignments are created.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		requireOktaSideApplicationAssignments(t, oktaClient, application1, []string{uid1})
		requireOktaSideGroupAssignments(t, oktaClient, group1, []string{uid1})
	}, time.Second*10, time.Millisecond*50)

	err = svc.Shutdown()
	require.NoError(t, err)

	// Let's now mark the assignment for cleanup before starting the Okta service again.
	assignment, err = ap.GetOktaAssignment(ctx, assignment.GetName())
	require.NoError(t, err)
	// Also make sure it's successful when we are at it.
	require.Equal(t, constants.OktaAssignmentStatusSuccessful, assignment.GetStatus())
	assignment.SetCleanupTime(clock.Now().Add(-1))
	_, err = ap.UpdateOktaAssignment(ctx, assignment)
	require.NoError(t, err)

	// Assert assignments before starting the service.
	requireOktaSideApplicationAssignments(t, oktaClient, application1, []string{uid1})
	requireOktaSideGroupAssignments(t, oktaClient, group1, []string{uid1})

	assignment, err = ap.GetOktaAssignment(ctx, assignment.GetName())
	require.NoError(t, err)
	require.False(t, assignment.GetCleanupTime().IsZero())
	require.False(t, assignment.IsFinalized())

	svc, _ = newTestService(t, ap, oktaClient, withClock(clock))
	svc.assignmentReconciler.assignmentProcessor.timeBetweenAssignmentProcessLoops = 1 * time.Second
	err = svc.Start(ctx)
	require.NoError(t, err)

	// Wait for okta_assignment to be cleaned up.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := ap.GetOktaAssignment(ctx, assignment.GetName())
		require.True(t, trace.IsNotFound(err), "unexpected error = %v", err)
	}, time.Second*10, time.Millisecond*500)

	// After okta_assignment is cleaned up, assert targets are also cleaned up on the
	// Okta-side.
	requireOktaSideApplicationAssignments(t, oktaClient, application1, nil)
	requireOktaSideGroupAssignments(t, oktaClient, group1, nil)
}

func Test_assignmentProcessor_sync_back_filters(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewRealClock()
	ap := newTestAccessPoint(t, clock)

	oktaClient, oktaData := oktaapitest.NewLocalDataClient(t)
	oktaClient.OrgURLFunc = func(t *testing.T) string { return oktaapitest.TestOrgURL }

	svc, emitter := newTestService(t, ap, oktaClient, withClock(clock), func(cfg *Config) {
		cfg.SyncSettings.AppFilters = []string{"^.*dev-app$"}
		cfg.SyncSettings.GroupFilters = []string{"^.*dev-group$"}
	})

	oktaData.UpsertUserForId("test-user", "test-user")
	oktaData.UpsertAppForId("dev-app")
	oktaData.UpsertAppForId("admin-app")
	oktaData.UpsertGroupForId("dev-group")
	oktaData.UpsertGroupForId("product-group")

	require.NoError(t, svc.synchronize(ctx))
	collectAllEvents(t, emitter, new([]apievents.AuditEvent))

	processor := svc.assignmentReconciler.assignmentProcessor

	assignment1 := assignment(t, "test_assignment_1", "test-user", time.Time{}, constants.OktaAssignmentStatusPending, time.Now(), false,
		target(types.OktaAssignmentTargetV1_GROUP, "dev-group"),
	)
	assignment2 := assignment(t, "test_assignment_2", "test-user", time.Time{}, constants.OktaAssignmentStatusPending, time.Now(), false,
		target(types.OktaAssignmentTargetV1_GROUP, "product-group"),
	)
	assignment3 := assignment(t, "test_assignment_3", "test-user", time.Time{}, constants.OktaAssignmentStatusPending, time.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, "dev-app", oktaapitest.TestLink1Name)),
	)
	assignment4 := assignment(t, "test_assignment_4", "test-user", time.Time{}, constants.OktaAssignmentStatusPending, time.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, "admin-app", oktaapitest.TestLink1Name)),
	)

	assignment1, err := ap.CreateOktaAssignment(ctx, assignment1)
	require.NoError(t, err)
	assignment2, err = ap.CreateOktaAssignment(ctx, assignment2)
	require.NoError(t, err)
	assignment3, err = ap.CreateOktaAssignment(ctx, assignment3)
	require.NoError(t, err)
	assignment4, err = ap.CreateOktaAssignment(ctx, assignment4)
	require.NoError(t, err)

	result1 := processor.processAssignment(ctx, processor.logger, assignment1)
	require.Equal(t, processAssignmentProcessed, result1)
	result2 := processor.processAssignment(ctx, processor.logger, assignment2)
	require.Equal(t, processAssignmentProcessed, result2)
	result3 := processor.processAssignment(ctx, processor.logger, assignment3)
	require.Equal(t, processAssignmentProcessed, result3)
	result4 := processor.processAssignment(ctx, processor.logger, assignment4)
	require.Equal(t, processAssignmentProcessed, result4)

	requireOktaSideGroupAssignments(t, oktaClient, "dev-group", []string{"test-user"})
	requireOktaSideGroupAssignments(t, oktaClient, "product-group", []string{})
	requireOktaSideApplicationAssignments(t, oktaClient, "dev-app", []string{"test-user"})
	requireOktaSideApplicationAssignments(t, oktaClient, "admin-app", []string{})
}

func getOktaAssignmentNames(assignments []types.OktaAssignment) []string {
	var res []string
	for _, r := range assignments {
		res = append(res, r.GetName())
	}
	return res
}

func mustGetAllOktaAssignments(t *testing.T, ap *testAccessPoint) []types.OktaAssignment {
	t.Helper()
	ctx := t.Context()

	var res []types.OktaAssignment
	for oa, err := range clientutils.Resources(ctx, ap.ListOktaAssignments) {
		require.NoError(t, err)
		res = append(res, oa)
	}
	return res
}

func sortByLastTransition(oktaAssignments []types.OktaAssignment) {
	slices.SortFunc(oktaAssignments, func(a, b types.OktaAssignment) int {
		return a.GetLastTransition().Compare(b.GetLastTransition())
	})
}

func requireEqualOktaAssignments(t *testing.T, expected, actual []types.OktaAssignment) {
	t.Helper()
	require.Len(t, actual, len(expected))
	for i := range len(expected) {
		require.Empty(t, cmp.Diff(expected[i], actual[i]), "element[%d]", i)
	}
}

type dummyEmitter struct{}

func (dummyEmitter) EmitAuditEvent(context.Context, apievents.AuditEvent) error { return nil }

type failingAccessListService struct {
	err error
}

func (s failingAccessListService) GetAccessListMember(context.Context, string, string) (*accesslist.AccessListMember, error) {
	return nil, s.err
}
