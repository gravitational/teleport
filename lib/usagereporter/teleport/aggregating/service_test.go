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
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func newReport(startTime time.Time) *prehogv1.UserActivityReport {
	u := uuid.New()
	r := &prehogv1.UserActivityReport{
		ReportUuid: u[:],
		StartTime:  timestamppb.New(startTime),
	}
	return r
}

func TestCRUD(t *testing.T) {
	ctx := context.Background()
	clk := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock:     clk,
		EventsOff: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	svc := reportService{bk}

	r0 := newReport(clk.Now().Add(time.Minute))
	r1 := newReport(clk.Now().Add(time.Minute))
	r2 := newReport(clk.Now().Add(2 * time.Minute))

	require.NoError(t, svc.upsertUserActivityReport(ctx, r0, time.Hour))
	require.NoError(t, svc.upsertUserActivityReport(ctx, r1, time.Hour))
	require.NoError(t, svc.upsertUserActivityReport(ctx, r2, time.Hour))

	// we expect r0 and r1 in unspecified order
	reports, err := svc.listUserActivityReports(ctx, 2)
	require.NoError(t, err)
	require.Len(t, reports, 2)
	if proto.Equal(r0, reports[0]) {
		require.True(t, proto.Equal(r1, reports[1]))
	} else {
		require.True(t, proto.Equal(r0, reports[1]))
		require.True(t, proto.Equal(r1, reports[0]))
	}

	require.NoError(t, svc.deleteUserActivityReport(ctx, r1))
	reports, err = svc.listUserActivityReports(ctx, 5)
	require.NoError(t, err)
	require.Len(t, reports, 2)
	// r2 has a later start time, so it must appear later
	require.True(t, proto.Equal(r0, reports[0]))
	require.True(t, proto.Equal(r2, reports[1]))

	clk.Advance(time.Minute + time.Hour)
	reports, err = svc.listUserActivityReports(ctx, 5)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.True(t, proto.Equal(r2, reports[0]))
}

func TestUserActivityReportSplitting(t *testing.T) {
	recordCount := 10000
	records := make([]*prehogv1.UserActivityRecord, 0, recordCount)
	for range recordCount {
		records = append(records, &prehogv1.UserActivityRecord{
			UserName:    []byte("user"),
			Logins:      100500,
			SshSessions: 42,
		})
	}
	reports, err := prepareUserActivityReports([]byte("clusterName"), []byte("reporterHostID"), time.Now(), records)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(reports), 2)            // some reports were split into two
	require.GreaterOrEqual(t, len(reports[0].Records), 2) // first report was able to contain a few user activity records

	// reassemble records and ensure that nothing was lost
	recordsCopy := make([]*prehogv1.UserActivityRecord, 0, recordCount)
	for _, report := range reports {
		recordsCopy = append(recordsCopy, report.Records...)
	}
	require.Equal(t, records, recordsCopy, "some user activity records have been lost during splitting")
}

func TestLock(t *testing.T) {
	ctx := context.Background()
	clk := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock:     clk,
		EventsOff: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	svc := reportService{bk}

	require.NoError(t, svc.createUsageReportingLock(ctx, 2*time.Minute, nil))
	clk.Advance(time.Minute)
	err = svc.createUsageReportingLock(ctx, 2*time.Minute, nil)
	require.Error(t, err)
	require.True(t, trace.IsAlreadyExists(err))
	clk.Advance(time.Minute)
	require.NoError(t, svc.createUsageReportingLock(ctx, 2*time.Minute, nil))
}

func newResourcePresenceReport(startTime time.Time) *prehogv1.ResourcePresenceReport {
	u := uuid.New()
	return &prehogv1.ResourcePresenceReport{
		ReportUuid: u[:],
		StartTime:  timestamppb.New(startTime),
	}
}

func TestResourcePresenceReporting(t *testing.T) {
	ctx := context.Background()
	clk := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock:     clk,
		EventsOff: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	svc := reportService{bk}

	r0 := newResourcePresenceReport(clk.Now().Add(time.Minute))
	r1 := newResourcePresenceReport(clk.Now().Add(time.Minute))
	r2 := newResourcePresenceReport(clk.Now().Add(2 * time.Minute))

	require.NoError(t, svc.upsertResourcePresenceReport(ctx, r0, time.Hour))
	require.NoError(t, svc.upsertResourcePresenceReport(ctx, r1, time.Hour))
	require.NoError(t, svc.upsertResourcePresenceReport(ctx, r2, time.Hour))

	// we expect r0 and r1 in unspecified order
	reports, err := svc.listResourcePresenceReports(ctx, 2)
	require.NoError(t, err)
	require.Len(t, reports, 2)
	if proto.Equal(r0, reports[0]) {
		require.True(t, proto.Equal(r1, reports[1]))
	} else {
		require.True(t, proto.Equal(r0, reports[1]))
		require.True(t, proto.Equal(r1, reports[0]))
	}
}

func TestResourcePresenceReportSplitting(t *testing.T) {
	resKinds := []prehogv1.ResourceKind{1, 2, 3, 4, 5}
	resKindReports := make([]*prehogv1.ResourceKindPresenceReport, 0, len(resKinds))
	for idx, kind := range resKinds {
		resourceIdsPerReport := int(math.Pow(10, float64(len(resKinds)-idx))) // max 100k per report
		kindReport := prehogv1.ResourceKindPresenceReport{
			ResourceKind: kind,
			ResourceIds:  make([]uint64, 0, resourceIdsPerReport),
		}
		for i := range resourceIdsPerReport {
			kindReport.ResourceIds = append(kindReport.ResourceIds, uint64(i))
		}
		resKindReports = append(resKindReports, &kindReport)
	}

	reports, err := prepareResourcePresenceReports([]byte("clusterName"), []byte("reporterHostID"), time.Now(), resKindReports)
	require.NoError(t, err)
	require.Greater(t, len(reports), len(resKindReports))             // some reports were split into two
	require.GreaterOrEqual(t, len(reports[0].ResourceKindReports), 2) // first report was able to contain a few resource kind reports

	// reassemble resource ids per resource kind and ensure that nothing was lost
	resourceIdsPerKind := make(map[prehogv1.ResourceKind][]uint64)
	for idx, kind := range resKinds {
		resourceIdsPerReport := int(math.Pow(10, float64(len(resKinds)-idx)))
		resourceIdsPerKind[kind] = make([]uint64, 0, resourceIdsPerReport)
	}
	for _, report := range reports {
		for _, kindReport := range report.ResourceKindReports {
			resourceIdsPerKind[kindReport.ResourceKind] = append(resourceIdsPerKind[kindReport.ResourceKind], kindReport.ResourceIds...)
		}
	}
	for _, resKindReport := range resKindReports {
		require.Equal(t, resKindReport.ResourceIds, resourceIdsPerKind[resKindReport.ResourceKind],
			"resource ids for resource kind %v do not match", resKindReport.ResourceKind)
	}
}

func TestBotInstanceActivityReportSplitting(t *testing.T) {
	recordCount := 10000
	records := make([]*prehogv1.BotInstanceActivityRecord, 0, recordCount)
	for range recordCount {
		records = append(records, &prehogv1.BotInstanceActivityRecord{
			BotUserName:   []byte("user"),
			BotInstanceId: []byte("foo"),
			BotJoins:      1000,
		})
	}
	reports, err := prepareBotInstanceActivityReports(
		[]byte("clusterName"), []byte("reporterHostID"), time.Now(), records,
	)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(reports), 2)            // some reports were split into two
	require.GreaterOrEqual(t, len(reports[0].Records), 2) // first report was able to contain a few user activity records

	// reassemble records and ensure that nothing was lost
	recordsCopy := make([]*prehogv1.BotInstanceActivityRecord, 0, recordCount)
	for _, report := range reports {
		recordsCopy = append(recordsCopy, report.Records...)
	}
	require.Equal(t, records, recordsCopy, "some bot instance activity records have been lost during splitting")
}
