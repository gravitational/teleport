// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usagereporter

import (
	"context"
	"sync"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	prehogv1c "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha/v1alphaconnect"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const aggregateTimeGranularity = 15 * time.Minute

const userActivityTTL = 60 * 24 * time.Hour

const (
	userActivityReportsPrefix = "userActivityReports"
	userActivityReportsLock   = userActivityReportsPrefix
)

// AggregatingUsageReporter aggregates and persists all user activity events to
// the backend, then periodically submits them.
type AggregatingUsageReporter struct {
	anonymizer utils.Anonymizer
	backend    backend.Backend
	client     prehogv1c.TeleportReportingServiceClient

	clusterName    []byte
	reporterHostID []byte

	mu        sync.Mutex
	startTime time.Time
	records   map[string]*prehogv1.UserActivityRecord
}

func NewAggregatingUsageReporter(
	ctx context.Context,
	anonymizer utils.Anonymizer,
	backend backend.Backend,
	clusterName string,
	reporterHostID string,
) (*AggregatingUsageReporter, error) {
	r := &AggregatingUsageReporter{
		anonymizer:     anonymizer,
		backend:        backend,
		clusterName:    anonymizer.AnonymizeNonEmpty(clusterName),
		reporterHostID: anonymizer.AnonymizeNonEmpty(reporterHostID),
	}

	go r.runPeriodicSubmit(ctx)
	go r.runPeriodicFinalize(ctx)

	return r, nil
}

var _ UsageReporter = (*AggregatingUsageReporter)(nil)

// AnonymizeAndSubmit implements [UsageReporter].
func (r *AggregatingUsageReporter) AnonymizeAndSubmit(events ...Anonymizable) {
	filtered := events[:0]
	for _, event := range events {
		switch event.(type) {
		case *UserLoginEvent, *SessionStartEvent, *KubeRequestEvent, *SFTPEvent:
			filtered = append(filtered, event)
		}
	}
	if len(filtered) == 0 {
		return
	}

	// this function must not block
	go r.anonymizeAndSubmit(filtered)
}

func (r *AggregatingUsageReporter) anonymizeAndSubmit(events []Anonymizable) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.maybeFinalizeReportLocked()

	getRecord := func(userName string) *prehogv1.UserActivityRecord {
		record := r.records[userName]
		if record == nil {
			record = &prehogv1.UserActivityRecord{}
			r.records[userName] = record
		}
		return record
	}

	for _, ae := range events {
		switch te := ae.(type) {
		case *UserLoginEvent:
			getRecord(te.UserName).Logins++
		case *SessionStartEvent:
			switch te.SessionType {
			case string(types.SSHSessionKind):
				getRecord(te.UserName).SshSessions++
			case string(types.AppSessionKind):
				getRecord(te.UserName).AppSessions++
			case string(types.KubernetesSessionKind):
				getRecord(te.UserName).KubeSessions++
			case string(types.DatabaseSessionKind):
				getRecord(te.UserName).DbSessions++
			case string(types.WindowsDesktopSessionKind):
				getRecord(te.UserName).DesktopSessions++
			case portSessionType:
				getRecord(te.UserName).SshPortSessions++
			case tcpSessionType:
				getRecord(te.UserName).AppTcpSessions++
			}
		case *KubeRequestEvent:
			getRecord(te.UserName).KubeRequests++
		case *SFTPEvent:
			getRecord(te.UserName).SftpEvents++
		}
	}
}

func (r *AggregatingUsageReporter) maybeFinalizeReport() {
	// if r.mu is locked then something has just called
	// maybeFinalizeReportLocked anyway
	if !r.mu.TryLock() {
		return
	}
	defer r.mu.Unlock()
	r.maybeFinalizeReportLocked()
}

func (r *AggregatingUsageReporter) maybeFinalizeReportLocked() {
	now := time.Now().UTC()

	if !r.startTime.Add(aggregateTimeGranularity).Before(now) {
		return
	}

	startTime := r.startTime
	records := r.records
	r.startTime = now.Truncate(aggregateTimeGranularity)
	r.records = make(map[string]*prehogv1.UserActivityRecord, len(records))

	if len(records) > 0 {
		go r.persistReport(startTime, records)
	}
}

//nolint:unused // will be needed for graceful stopping
func (r *AggregatingUsageReporter) blockingAlwaysFinalizeReport() {
	now := time.Now().UTC()

	startTime := r.startTime
	r.startTime = now.Truncate(aggregateTimeGranularity)
	records := r.records
	r.records = make(map[string]*prehogv1.UserActivityRecord, len(records))

	if len(records) > 0 {
		r.persistReport(startTime, records)
	}
}

func (r *AggregatingUsageReporter) persistReport(startTime time.Time, records map[string]*prehogv1.UserActivityRecord) {
	if len(records) < 1 {
		return
	}

	pbRecords := make([]*prehogv1.UserActivityRecord, 0, len(records))
	for userName, record := range records {
		record.UserName = r.anonymizer.AnonymizeNonEmpty(userName)
		pbRecords = append(pbRecords, record)
	}

	pbStartTime := timestamppb.New(startTime)
	expiry := startTime.Add(userActivityTTL)

	for len(pbRecords) > 0 {
		reportID, wire, rem, err := prepareReport(r.clusterName, r.reporterHostID, pbStartTime, pbRecords)
		if err != nil {
			// TODO
			return
		}
		pbRecords = rem

		if _, err := r.backend.Put(context.TODO(), backend.Item{
			Key:     backend.Key(userActivityReportsPrefix, reportID.String()),
			Value:   wire,
			Expires: expiry,
		}); err != nil {
			// backend error, can't do much about that
			continue
		}
	}
}

func prepareReport(
	clusterName, reporterHostID []byte,
	pbStartTime *timestamppb.Timestamp,
	pbRecords []*prehogv1.UserActivityRecord,
) (uuid.UUID, []byte, []*prehogv1.UserActivityRecord, error) {
	reportID := uuid.New()
	report := &prehogv1.UserActivityReport{
		ReportUuid:     reportID[:],
		ClusterName:    clusterName,
		ReporterHostid: reporterHostID,
		StartTime:      pbStartTime,
		Records:        pbRecords,
	}

	wire, err := proto.Marshal(report)
	if err != nil {
		return uuid.Nil, nil, pbRecords, trace.Wrap(err)
	}

	for len(wire) > 131072 {
		if len(report.Records) <= 32 {
			return uuid.Nil, nil, pbRecords, trace.LimitExceeded("failed to marshal user activity report within size limit")
		}

		report.Records = report.Records[:len(report.Records)/2]

		wire, err = protojson.Marshal(report)
		if err != nil {
			return uuid.Nil, nil, pbRecords, trace.Wrap(err)
		}
	}

	return reportID, wire, pbRecords[len(report.Records):], nil
}

func (r *AggregatingUsageReporter) runPeriodicFinalize(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		r.maybeFinalizeReport()
	}
}

func (r *AggregatingUsageReporter) runPeriodicSubmit(ctx context.Context) {
	iv := interval.New(interval.Config{
		FirstDuration: utils.HalfJitter(10 * time.Minute),
		Duration:      5 * time.Minute,
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer iv.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-iv.Next():
		}

		r.doSubmit(ctx)
	}
}

func (r *AggregatingUsageReporter) doSubmit(ctx context.Context) {
	rangeStart := backend.ExactKey(userActivityReportsPrefix)
	getResult, err := r.backend.GetRange(ctx, rangeStart, backend.RangeEnd(rangeStart), 10)
	if err != nil {
		return
	}
	if len(getResult.Items) < 1 {
		return
	}

	reports := make([]*prehogv1.UserActivityReport, 0, len(getResult.Items))
	for _, item := range getResult.Items {
		report := new(prehogv1.UserActivityReport)
		if err := proto.Unmarshal(item.Value, report); err != nil {
			// TODO: delete broken values? just skip and fetch more? if we only
			// skip we can get in a situation where the first 10 items are
			// broken with low UUIDs, and then we can't submit anything
			return
		}
	}

	if _, err := backend.AcquireLock(ctx, r.backend, userActivityReportsLock, 75*time.Second); err != nil {
		// TODO: log that some other auth is already submitting
		return
	}

	lockCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if _, err = r.client.SubmitUsageReports(lockCtx,
		connect.NewRequest(&prehogv1.SubmitUsageReportsRequest{
			UserActivity: reports,
		}),
	); err != nil {
		// TODO: log
		return
	}

	for _, item := range getResult.Items {
		if err := r.backend.Delete(ctx, item.Key); err != nil {
			// TODO: log and try to delete the other items, might as well
			continue
		}
	}
}
