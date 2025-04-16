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
	botInstanceActivityReportsPrefix = "botInstanceActivityReports"
	userActivityReportsPrefix        = "userActivityReports"
	// usageReportingLock is a lock that should be held when submitting usage
	// reports to the upstream service. Whilst the underlying key refers
	// specifically to "userActivityReports", this is inaccurate.
	usageReportingLock            = "userActivityReportsLock"
	ResourcePresenceReportsPrefix = "resourcePresenceReports"
)

// userActivityReportKey returns the backend key for a user activity report with
// a given UUID and start time, such that reports with an earlier start time
// will appear earlier in lexicographic ordering.
func userActivityReportKey(reportUUID uuid.UUID, startTime time.Time) backend.Key {
	return backend.NewKey(userActivityReportsPrefix, startTime.Format(time.RFC3339), reportUUID.String())
}

func prepareUserActivityReports(
	clusterName, reporterHostID []byte,
	startTime time.Time, records []*prehogv1.UserActivityRecord,
) (reports []*prehogv1.UserActivityReport, err error) {
	for len(records) > 0 {
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

		records = records[len(report.Records):]
		reports = append(reports, report)
	}

	return reports, nil
}

// resourcePresenceReportKey returns the backend key for a resource presence report with
// a given UUID and start time, such that reports with an earlier start time
// will appear earlier in lexicographic ordering.
func resourcePresenceReportKey(reportUUID uuid.UUID, startTime time.Time) backend.Key {
	return backend.NewKey(ResourcePresenceReportsPrefix, startTime.Format(time.RFC3339), reportUUID.String())
}

// prepareResourcePresenceReport prepares a resource presence report for storage.
// It returns a slice of reports for case when single report is too large to fit in a single item.
// As even a single resource kind report can be too large to fit in a single item, it may return
// multiple reports with single resource kind report split into multiple fragments.
func prepareResourcePresenceReports(
	clusterName, reporterHostID []byte,
	startTime time.Time, kindReports []*prehogv1.ResourceKindPresenceReport,
) ([]*prehogv1.ResourcePresenceReport, error) {
	reports := make([]*prehogv1.ResourcePresenceReport, 0, 1) // at least one report

	for len(kindReports) > 0 {
		// Optimistic case: try to put all records into a single report in hope that they will fit.
		reportUUID := uuid.New()
		report := &prehogv1.ResourcePresenceReport{
			ReportUuid:          reportUUID[:],
			ClusterName:         clusterName,
			ReporterHostid:      reporterHostID,
			StartTime:           timestamppb.New(startTime),
			ResourceKindReports: kindReports,
		}

		if proto.Size(report) <= maxItemSize {
			// The last report fits, so we're done.
			reports = append(reports, report)
			return reports, nil
		}

		// We're over the size limit, so we need to split the report and try again.
		report.ResourceKindReports = make([]*prehogv1.ResourceKindPresenceReport, 0, len(kindReports))

		// Try to fit as many resource kind reports as possible, skipping big ones.
		unfitRecords := make([]*prehogv1.ResourceKindPresenceReport, 0, len(kindReports))
		for _, kindReport := range kindReports {
			report.ResourceKindReports = append(report.ResourceKindReports, kindReport)
			if proto.Size(report) > maxItemSize {
				unfitRecords = append(unfitRecords, kindReport)
				report.ResourceKindReports = report.ResourceKindReports[:len(report.ResourceKindReports)-1]
			}
		}
		kindReports = unfitRecords

		// To reduce kind reports fragmentation between resource reports
		// don't try to split unfit kind reports if some already fit into report as a whole
		if len(report.ResourceKindReports) > 0 {
			reports = append(reports, report)
			continue
		}

		// Try to split the last oversized resource by two until it fits
		resourceIds := kindReports[0].GetResourceIds()
		kindReportHead := &prehogv1.ResourceKindPresenceReport{
			ResourceKind: kindReports[0].GetResourceKind(),
			ResourceIds:  resourceIds[:len(resourceIds)/2],
		}
		kindReportTail := &prehogv1.ResourceKindPresenceReport{
			ResourceKind: kindReports[0].GetResourceKind(),
			ResourceIds:  resourceIds[len(resourceIds)/2:],
		}

		report.ResourceKindReports = []*prehogv1.ResourceKindPresenceReport{kindReportHead}

		for proto.Size(report) > maxItemSize {
			if len(kindReportHead.ResourceIds) < 1 {
				return nil, trace.LimitExceeded("failed to marshal resource presence report within size limit (this is a bug)")
			}
			resourceIds = kindReportHead.GetResourceIds()
			kindReportHead.ResourceIds = resourceIds[:len(resourceIds)/2]
			kindReportTail.ResourceIds = append(resourceIds[len(resourceIds)/2:], kindReportTail.ResourceIds...)
		}

		kindReports[0] = kindReportTail
		reports = append(reports, report)
	}

	return reports, nil
}

// botInstanceActivityReportKey returns the backend key for a bot instance
// activity report with  a given UUID and start time, such that reports with
// an earlier start time will appear earlier in lexicographic ordering.
func botInstanceActivityReportKey(reportUUID uuid.UUID, startTime time.Time) backend.Key {
	return backend.NewKey(botInstanceActivityReportsPrefix, startTime.Format(time.RFC3339), reportUUID.String())
}

func prepareBotInstanceActivityReports(
	clusterName, reporterHostID []byte,
	startTime time.Time, records []*prehogv1.BotInstanceActivityRecord,
) (reports []*prehogv1.BotInstanceActivityReport, err error) {
	for len(records) > 0 {
		reportUUID := uuid.New()
		report := &prehogv1.BotInstanceActivityReport{
			ReportUuid:     reportUUID[:],
			ClusterName:    clusterName,
			ReporterHostid: reporterHostID,
			StartTime:      timestamppb.New(startTime),
			Records:        records,
		}

		for proto.Size(report) > maxItemSize {
			if len(report.Records) <= 1 {
				return nil, trace.LimitExceeded("failed to marshal bot instance activity report within size limit (this is a bug)")
			}

			report.Records = report.Records[:len(report.Records)/2]
		}

		records = records[len(report.Records):]
		reports = append(reports, report)
	}

	return reports, nil
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

// createUsageReportingLock creates a lock that should be held when reading
// reports and submitting them to the upstream service.
func (r reportService) createUsageReportingLock(ctx context.Context, ttl time.Duration, payload []byte) error {
	if len(payload) == 0 {
		payload = []byte("null")
	}
	lockKey := backend.NewKey(usageReportingLock)
	// HACK(espadolini): dynamodbbk doesn't let you Create over an expired item
	// but it will explicitly delete expired items on a Get; in addition, reads
	// are cheaper than writes in most backends, so we do a Get here first
	if _, err := r.b.Get(ctx, lockKey); err == nil {
		return trace.AlreadyExists(usageReportingLock + " already exists")
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if _, err := r.b.Create(ctx, backend.Item{
		Key:     lockKey,
		Value:   payload,
		Expires: r.b.Clock().Now().UTC().Add(ttl),
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r reportService) upsertResourcePresenceReport(ctx context.Context, report *prehogv1.ResourcePresenceReport, ttl time.Duration) error {
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
		Key:     resourcePresenceReportKey(reportUUID, startTime),
		Value:   wire,
		Expires: startTime.Add(ttl),
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r reportService) deleteResourcePresenceReport(ctx context.Context, report *prehogv1.ResourcePresenceReport) error {
	reportUUID, err := uuid.FromBytes(report.GetReportUuid())
	if err != nil {
		return trace.Wrap(err)
	}

	startTime := report.GetStartTime().AsTime()
	if startTime.IsZero() {
		return trace.BadParameter("missing start_time")
	}

	if err := r.b.Delete(ctx, resourcePresenceReportKey(reportUUID, startTime)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// listResourcePresenceReports returns the first `count` resource reports
// according to the key order; as we store them with time and UUID in the key,
// this results in returning earlier reports first.
func (r reportService) listResourcePresenceReports(ctx context.Context, count int) ([]*prehogv1.ResourcePresenceReport, error) {
	rangeStart := backend.ExactKey(ResourcePresenceReportsPrefix)
	result, err := r.b.GetRange(ctx, rangeStart, backend.RangeEnd(rangeStart), count)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reports := make([]*prehogv1.ResourcePresenceReport, 0, len(result.Items))
	for _, item := range result.Items {
		report := &prehogv1.ResourcePresenceReport{}
		if err := proto.Unmarshal(item.Value, report); err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}

func (r reportService) upsertBotInstanceActivityReport(
	ctx context.Context, report *prehogv1.BotInstanceActivityReport, ttl time.Duration,
) error {
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
		Key:     botInstanceActivityReportKey(reportUUID, startTime),
		Value:   wire,
		Expires: startTime.Add(ttl),
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r reportService) deleteBotInstanceActivityReport(
	ctx context.Context, report *prehogv1.BotInstanceActivityReport,
) error {
	reportUUID, err := uuid.FromBytes(report.GetReportUuid())
	if err != nil {
		return trace.Wrap(err)
	}

	startTime := report.GetStartTime().AsTime()
	if startTime.IsZero() {
		return trace.BadParameter("missing start_time")
	}

	if err := r.b.Delete(ctx, botInstanceActivityReportKey(reportUUID, startTime)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// listBotInstanceActivityReports returns the first `count` user activity reports
// according to the key order; as we store them with time and uuid in the key,
// this results in returning earlier reports first.
func (r reportService) listBotInstanceActivityReports(
	ctx context.Context, count int,
) ([]*prehogv1.BotInstanceActivityReport, error) {
	rangeStart := backend.ExactKey(botInstanceActivityReportsPrefix)
	result, err := r.b.GetRange(ctx, rangeStart, backend.RangeEnd(rangeStart), count)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reports := make([]*prehogv1.BotInstanceActivityReport, 0, len(result.Items))
	for _, item := range result.Items {
		report := &prehogv1.BotInstanceActivityReport{}
		if err := proto.Unmarshal(item.Value, report); err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, report)
	}

	return reports, nil
}
