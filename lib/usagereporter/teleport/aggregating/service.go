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
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	"github.com/gravitational/teleport/lib/backend"
)

const (
	// maxItemSize is the maximum size for a backend value; dynamodb has a limit of
	// 400KiB per item, we store values in base64, there's some framing, and a
	// healthy 50% margin gets us to about 128KiB.
	maxItemSize = 128 * 1024
)

const (
	userActivityReportsPrefix = "userActivityReports"
	userActivityReportsLock   = "userActivityReportsLock"
)

// userActivityReportKey returns the backend key for a user activity report with
// a given UUID and start time, such that reports with an earlier start time
// will appear earlier in lexicographic ordering.
func userActivityReportKey(reportUUID uuid.UUID, startTime time.Time) []byte {
	return backend.Key(userActivityReportsPrefix, startTime.Format(time.RFC3339), reportUUID.String())
}

func prepareUserActivityReport(
	clusterName, reporterHostID []byte,
	startTime time.Time, records []*prehogv1.UserActivityRecord,
) (*prehogv1.UserActivityReport, error) {
	reportUUID := uuid.New()
	report := &prehogv1.UserActivityReport{
		ReportUuid:     reportUUID[:],
		ClusterName:    clusterName,
		ReporterHostid: reporterHostID,
		StartTime:      timestamppb.New(startTime),
		Records:        records,
	}

	for proto.Size(report) > maxItemSize {
		if len(report.Records) <= 1 {
			return nil, trace.LimitExceeded("failed to marshal user activity report within size limit (this is a bug)")
		}

		report.Records = report.Records[:len(report.Records)/2]
	}

	return report, nil
}

// reportService is a [backend.Backend] wrapper that handles usage reports.
type reportService struct {
	b backend.Backend
}

func (r reportService) upsertUserActivityReport(ctx context.Context, report *prehogv1.UserActivityReport, ttl time.Duration) error {
	wire, err := proto.Marshal(report)
	if err != nil {
		return trace.Wrap(err)
	}

	reportUUID, err := uuid.FromBytes(report.GetReportUuid())
	if err != nil {
		return trace.Wrap(err)
	}

	startTime := report.GetStartTime().AsTime()
	if startTime.IsZero() {
		return trace.BadParameter("missing start_time")
	}

	if _, err := r.b.Put(ctx, backend.Item{
		Key:     userActivityReportKey(reportUUID, startTime),
		Value:   wire,
		Expires: startTime.Add(ttl),
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r reportService) deleteUserActivityReport(ctx context.Context, report *prehogv1.UserActivityReport) error {
	reportUUID, err := uuid.FromBytes(report.GetReportUuid())
	if err != nil {
		return trace.Wrap(err)
	}

	startTime := report.GetStartTime().AsTime()
	if startTime.IsZero() {
		return trace.BadParameter("missing start_time")
	}

	if err := r.b.Delete(ctx, userActivityReportKey(reportUUID, startTime)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// listUserActivityReports returns the first `count` user activity reports
// according to the key order; as we store them with time and uuid in the key,
// this results in returning earlier reports first.
func (r reportService) listUserActivityReports(ctx context.Context, count int) ([]*prehogv1.UserActivityReport, error) {
	rangeStart := backend.ExactKey(userActivityReportsPrefix)
	result, err := r.b.GetRange(ctx, rangeStart, backend.RangeEnd(rangeStart), count)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reports := make([]*prehogv1.UserActivityReport, 0, len(result.Items))
	for _, item := range result.Items {
		report := &prehogv1.UserActivityReport{}
		if err := proto.Unmarshal(item.Value, report); err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

func (r reportService) createUserActivityReportsLock(ctx context.Context, ttl time.Duration, payload []byte) error {
	if len(payload) == 0 {
		payload = []byte("null")
	}
	if _, err := r.b.Create(ctx, backend.Item{
		Key:     backend.Key(userActivityReportsLock),
		Value:   payload,
		Expires: r.b.Clock().Now().UTC().Add(ttl),
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
