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

package aggregating

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/testutils/synctest"
)

// TestSubmitOnce tests the behavior of [submitOnce]; the [RunSubmitter]
// function is just a jittered periodic call to submitOnce, so testing it has
// very little use.
func TestSubmitOnce(t *testing.T) {
	ctx := context.Background()
	clk := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock:     clk,
		EventsOff: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	svc := reportService{bk}

	var submitted []*prehogv1.UserActivityReport
	var submittedPresence []*prehogv1.ResourcePresenceReport
	var submittedBotInstanceActivity []*prehogv1.BotInstanceActivityReport
	var submittedIdentitySecuritySummaries []*prehogv1.IdentitySecuritySummariesGeneratedReport
	submitOk := func(ctx context.Context, req *prehogv1.SubmitUsageReportsRequest) (uuid.UUID, error) {
		if l := len(req.UserActivity) + len(req.ResourcePresence) + len(req.BotInstanceActivity) + len(req.IdentitySecuritySummariesReport); l > submitBatchSize {
			return uuid.Nil, trace.LimitExceeded("got %v reports, expected at most %v", l, submitBatchSize)
		}
		submitted = append(submitted, req.UserActivity...)
		submittedPresence = append(submittedPresence, req.ResourcePresence...)
		submittedBotInstanceActivity = append(submittedBotInstanceActivity, req.BotInstanceActivity...)
		submittedIdentitySecuritySummaries = append(submittedIdentitySecuritySummaries, req.IdentitySecuritySummariesReport...)
		return uuid.New(), nil
	}
	submitErr := func(ctx context.Context, req *prehogv1.SubmitUsageReportsRequest) (uuid.UUID, error) {
		submitted = append(submitted, req.UserActivity...)
		return uuid.Nil, errors.New("")
	}

	scfg := SubmitterConfig{
		Backend:   bk,
		Status:    local.NewStatusService(bk),
		Submitter: submitOk,
	}
	require.NoError(t, scfg.CheckAndSetDefaults())

	reportFresh := newReport(time.Now().UTC())
	require.NoError(t, svc.upsertUserActivityReport(ctx, reportFresh, reportTTL))

	resCountReport := newResourcePresenceReport(time.Now().UTC())
	require.NoError(t, svc.upsertResourcePresenceReport(ctx, resCountReport, reportTTL))

	// successful submit, no alerts, no leftover reports
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 1)
	require.True(t, proto.Equal(reportFresh, submitted[0]))
	reports, err := svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, reports)
	rReports, err := svc.listResourcePresenceReports(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, rReports)

	submitted = nil

	alerts, err := scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Empty(t, alerts)

	require.NoError(t, svc.upsertUserActivityReport(ctx, reportFresh, reportTTL))
	// failed submit, report stays but it's not old enough, so no alert
	scfg.Submitter = submitErr
	clk.Advance(submitLockDuration)
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 1)
	require.True(t, proto.Equal(reportFresh, submitted[0]))
	reports, err = svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.True(t, proto.Equal(reportFresh, reports[0]))
	submitted = nil

	alerts, err = scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Empty(t, alerts)

	// overdue report
	require.NoError(t, svc.deleteUserActivityReport(ctx, reportFresh))
	reportOld := newReport(time.Now().UTC().Add(-2 * alertGraceDuration))
	require.NoError(t, svc.upsertUserActivityReport(ctx, reportOld, reportTTL))

	// failed submit, report stays and it's old enough for an alert
	scfg.Submitter = submitErr
	clk.Advance(submitLockDuration)
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 1)
	require.True(t, proto.Equal(reportOld, submitted[0]))
	reports, err = svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.True(t, proto.Equal(reportOld, reports[0]))
	submitted = nil

	alerts, err = scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, alertName, alerts[0].GetName())

	scfg.Submitter = submitOk
	// the lock is still held, nothing happens
	submitOnce(ctx, scfg)
	require.Empty(t, submitted)

	clk.Advance(submitLockDuration)
	// successful submission, no remaining events but the alert stays for one more cycle
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 1)
	require.Len(t, submittedPresence, 1)
	submitted = nil
	submittedPresence = nil

	alerts, err = scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, alertName, alerts[0].GetName())

	// nothing to submit, alert gone
	submitOnce(ctx, scfg)
	require.Empty(t, submitted)
	alerts, err = scfg.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	})
	require.NoError(t, err)
	require.Empty(t, alerts)

	for i := 0; i < 20; i++ {
		require.NoError(t, svc.upsertUserActivityReport(ctx, newReport(time.Now().UTC().Add(time.Duration(i)*time.Second)), reportTTL))
	}
	for i := 0; i < 15; i++ {
		require.NoError(t, svc.upsertResourcePresenceReport(ctx, newResourcePresenceReport(time.Now().UTC().Add(time.Duration(i)*time.Second)), reportTTL))
	}
	clk.Advance(submitLockDuration)
	submitOnce(ctx, scfg)
	clk.Advance(submitLockDuration)
	submitOnce(ctx, scfg)
	clk.Advance(submitLockDuration)
	submitOnce(ctx, scfg)
	clk.Advance(submitLockDuration)
	submitOnce(ctx, scfg)
	require.Len(t, submitted, 20)
	require.Len(t, submittedPresence, 15)
}

func TestSubmitOnceIdentitySecuritySummaries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		testSubmitOnceIdentitySecuritySummaries(t)
	})
}

func testSubmitOnceIdentitySecuritySummaries(t *testing.T) {
	ctx := t.Context()
	clk := clockwork.NewRealClock()
	bk, err := memory.New(memory.Config{
		Clock:     clk,
		EventsOff: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	svc := reportService{bk}

	var submittedUserActivity []*prehogv1.UserActivityReport
	var submittedResourcePresence []*prehogv1.ResourcePresenceReport
	var submittedBotInstanceActivity []*prehogv1.BotInstanceActivityReport
	var submittedIdentitySecuritySummaries []*prehogv1.IdentitySecuritySummariesGeneratedReport

	submitOk := func(ctx context.Context, req *prehogv1.SubmitUsageReportsRequest) (uuid.UUID, error) {
		totalReports := len(req.UserActivity) + len(req.ResourcePresence) + len(req.BotInstanceActivity) + len(req.IdentitySecuritySummariesReport)
		if totalReports > submitBatchSize {
			return uuid.Nil, trace.LimitExceeded("got %v reports, expected at most %v", totalReports, submitBatchSize)
		}
		submittedUserActivity = append(submittedUserActivity, req.UserActivity...)
		submittedResourcePresence = append(submittedResourcePresence, req.ResourcePresence...)
		submittedBotInstanceActivity = append(submittedBotInstanceActivity, req.BotInstanceActivity...)
		submittedIdentitySecuritySummaries = append(submittedIdentitySecuritySummaries, req.IdentitySecuritySummariesReport...)
		return uuid.New(), nil
	}

	scfg := SubmitterConfig{
		Backend:   bk,
		Status:    local.NewStatusService(bk),
		Submitter: submitOk,
	}
	require.NoError(t, scfg.CheckAndSetDefaults())

	// Test 1: Submit identity security summaries report alone
	identityReport1 := newIdentitySecuritySummariesGeneratedReport(time.Now().UTC())
	require.NoError(t, svc.upsertIdentitySecuritySummariesGeneratedReport(ctx, identityReport1, reportTTL))

	submitOnce(ctx, scfg)
	require.Len(t, submittedIdentitySecuritySummaries, 1)
	require.True(t, proto.Equal(identityReport1, submittedIdentitySecuritySummaries[0]))

	// Verify report was deleted after submission
	reports, err := svc.listIdentitySecuritySummariesGeneratedReports(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, reports)

	// Reset submitted slice
	submittedUserActivity = nil
	submittedResourcePresence = nil
	submittedBotInstanceActivity = nil
	submittedIdentitySecuritySummaries = nil

	// Test 2: Submit mixed report types respecting batch size
	// Add 5 user activity reports
	for i := range 5 {
		userReport := newReport(time.Now().UTC().Add(time.Duration(i) * time.Second))
		require.NoError(t, svc.upsertUserActivityReport(ctx, userReport, reportTTL))
	}
	// Add 3 resource presence reports
	for i := range 3 {
		resourceReport := newResourcePresenceReport(time.Now().UTC().Add(time.Duration(i) * time.Second))
		require.NoError(t, svc.upsertResourcePresenceReport(ctx, resourceReport, reportTTL))
	}
	// Add 2 identity security summaries reports
	for i := range 2 {
		identityReport := newIdentitySecuritySummariesGeneratedReport(time.Now().UTC().Add(time.Duration(i) * time.Second))
		require.NoError(t, svc.upsertIdentitySecuritySummariesGeneratedReport(ctx, identityReport, reportTTL))
	}

	time.Sleep(submitLockDuration)
	submitOnce(ctx, scfg)

	// Should submit exactly 10 reports (5 user + 3 resource + 2 identity)
	totalSubmitted := len(submittedUserActivity) + len(submittedResourcePresence) + len(submittedIdentitySecuritySummaries)
	require.Equal(t, 10, totalSubmitted)
	require.Len(t, submittedUserActivity, 5)
	require.Len(t, submittedResourcePresence, 3)
	require.Len(t, submittedIdentitySecuritySummaries, 2)

	// All reports should be deleted
	userReports, err := svc.listUserActivityReports(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, userReports)

	resourceReports, err := svc.listResourcePresenceReports(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, resourceReports)

	identityReports, err := svc.listIdentitySecuritySummariesGeneratedReports(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, identityReports)

	// Reset
	submittedUserActivity = nil
	submittedResourcePresence = nil
	submittedBotInstanceActivity = nil
	submittedIdentitySecuritySummaries = nil

	// Test 3: Priority ordering - user activity fills batch first, then resource, then bot, then identity
	// Add enough reports to test multiple batches
	for i := range 12 {
		userReport := newReport(time.Now().UTC().Add(time.Duration(i) * time.Second))
		require.NoError(t, svc.upsertUserActivityReport(ctx, userReport, reportTTL))
	}
	for i := range 8 {
		identityReport := newIdentitySecuritySummariesGeneratedReport(time.Now().UTC().Add(time.Duration(i) * time.Second))
		require.NoError(t, svc.upsertIdentitySecuritySummariesGeneratedReport(ctx, identityReport, reportTTL))
	}

	// First batch: should get 10 user activity reports (prioritized)
	time.Sleep(submitLockDuration)
	submitOnce(ctx, scfg)
	require.Len(t, submittedUserActivity, 10)
	require.Empty(t, submittedIdentitySecuritySummaries)

	// Second batch: remaining 2 user + 8 identity summaries (but only 8 more will fit)
	time.Sleep(submitLockDuration)
	submitOnce(ctx, scfg)
	require.Len(t, submittedUserActivity, 12) // 10 from first + 2 from second
	require.Len(t, submittedIdentitySecuritySummaries, 8)

	// Verify all deleted
	userReports, err = svc.listUserActivityReports(ctx, 20)
	require.NoError(t, err)
	require.Empty(t, userReports)

	identityReports, err = svc.listIdentitySecuritySummariesGeneratedReports(ctx, 20)
	require.NoError(t, err)
	require.Empty(t, identityReports)
}
