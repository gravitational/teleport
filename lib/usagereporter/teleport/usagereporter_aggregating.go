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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const aggregateTimeGranularity = 15 * time.Minute

const (
	userActivityReportsPrefix = "userActivityReports"
	userActivityReportsLock   = "userActivityReportsLock"

	// userActivityReportMaxSize is the maximum size for a backend value;
	// dynamodb has a limit of 400KiB per item, we store values in base64,
	// there's some framing, and a healthy 50% margin gets us to about 128KiB.
	userActivityReportMaxSize = 131072

	userActivityReportTTL = 60 * 24 * time.Hour
)

// userActivityReportKey returns the backend key for a user activity report with
// a given UUID and start time, such that reports with an earlier start time
// will appear earlier in lexicographic ordering.
func userActivityReportKey(reportUUID uuid.UUID, startTime time.Time) []byte {
	return backend.Key(userActivityReportsPrefix, startTime.Format(time.RFC3339), reportUUID.String())
}

func NewAggregatingUsageReporter(
	ctx context.Context,
	anonymizer utils.Anonymizer,
	backend backend.Backend,
	submitter UsageReportsSubmitter,
	status services.StatusInternal,
	clusterName string,
	reporterHostID string,
) (*AggregatingUsageReporter, error) {
	baseCtx, baseCancel := context.WithCancel(ctx)
	periodicCtx, periodicCancel := context.WithCancel(baseCtx)

	r := &AggregatingUsageReporter{
		anonymizer: anonymizer,
		backend:    backend,
		submitter:  submitter,
		status:     status,

		clusterName:    anonymizer.AnonymizeNonEmpty(clusterName),
		reporterHostID: anonymizer.AnonymizeNonEmpty(reporterHostID),

		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		periodicCancel: periodicCancel,
	}

	r.wg.Add(2)
	go r.runPeriodicSubmit(periodicCtx)
	go r.runPeriodicFinalize(periodicCtx)

	return r, nil
}

type UsageReportsSubmitter interface {
	SubmitUsageReports(context.Context, *connect.Request[prehogv1.SubmitUsageReportsRequest]) (*connect.Response[prehogv1.SubmitUsageReportsResponse], error)
}

// AggregatingUsageReporter aggregates and persists all user activity events to
// the backend, then periodically submits them.
type AggregatingUsageReporter struct {
	anonymizer utils.Anonymizer
	backend    backend.Backend
	submitter  UsageReportsSubmitter
	status     services.StatusInternal

	clusterName    []byte
	reporterHostID []byte

	wg             sync.WaitGroup
	baseCtx        context.Context
	baseCancel     context.CancelFunc
	periodicCancel context.CancelFunc

	mu        sync.Mutex
	stopping  bool
	startTime time.Time
	records   map[string]*prehogv1.UserActivityRecord
}

var _ GracefulStopper = (*AggregatingUsageReporter)(nil)

// GracefulStop implements [GracefulStopper]
func (r *AggregatingUsageReporter) GracefulStop(ctx context.Context) error {
	defer r.baseCancel()
	r.periodicCancel()

	r.mu.Lock()

	r.stopping = true

	startTime := r.startTime
	records := r.records
	// don't hold up memory
	r.records = nil

	r.mu.Unlock()

	if len(records) > 0 {
		r.wg.Add(1)
		go r.persistReport(startTime, records)
	}

	go func() {
		select {
		case <-ctx.Done():
			r.baseCancel()
		case <-r.baseCtx.Done():
		}
	}()

	r.wg.Wait()

	return r.baseCtx.Err()
}

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

	if r.stopping {
		return
	}

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
		r.wg.Add(1)
		go r.persistReport(startTime, records)
	}
}

func (r *AggregatingUsageReporter) persistReport(startTime time.Time, records map[string]*prehogv1.UserActivityRecord) {
	defer r.wg.Done()

	pbRecords := make([]*prehogv1.UserActivityRecord, 0, len(records))
	for userName, record := range records {
		record.UserName = r.anonymizer.AnonymizeNonEmpty(userName)
		pbRecords = append(pbRecords, record)
	}

	pbStartTime := timestamppb.New(startTime)
	expiry := startTime.Add(userActivityReportTTL)

	for len(pbRecords) > 0 {
		reportUUID, wire, rem, err := prepareReport(r.clusterName, r.reporterHostID, pbStartTime, pbRecords)
		if err != nil {
			// TODO
			return
		}
		pbRecords = rem

		if _, err := r.backend.Put(r.baseCtx, backend.Item{
			Key:     userActivityReportKey(reportUUID, startTime),
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
	startTime *timestamppb.Timestamp,
	records []*prehogv1.UserActivityRecord,
) (uuid.UUID, []byte, []*prehogv1.UserActivityRecord, error) {
	reportUUID := uuid.New()
	report := &prehogv1.UserActivityReport{
		ReportUuid:     reportUUID[:],
		ClusterName:    clusterName,
		ReporterHostid: reporterHostID,
		StartTime:      startTime,
		Records:        records,
	}

	for proto.Size(report) > userActivityReportMaxSize {
		if len(report.Records) <= 1 {
			return uuid.Nil, nil, records, trace.LimitExceeded("failed to marshal user activity report within size limit")
		}

		report.Records = report.Records[:len(report.Records)/2]
	}

	wire, err := proto.Marshal(report)
	if err != nil {
		// TODO: sad marshal noises
		return uuid.Nil, nil, records, trace.Wrap(err)
	}

	return reportUUID, wire, records[len(report.Records):], nil
}

func (r *AggregatingUsageReporter) runPeriodicFinalize(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// if r.mu is locked then something has just called
		// maybeFinalizeReportLocked anyway
		if r.mu.TryLock() {
			if !r.stopping {
				r.maybeFinalizeReportLocked()
			}
			r.mu.Unlock()
		}
	}
}

func (r *AggregatingUsageReporter) runPeriodicSubmit(ctx context.Context) {
	defer r.wg.Done()

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
		_ = r.status.DeleteClusterAlert(ctx, "reporting-failed")
		return
	}

	reports := make([]*prehogv1.UserActivityReport, 0, len(getResult.Items))
	for _, item := range getResult.Items {
		report := new(prehogv1.UserActivityReport)
		if err := proto.Unmarshal(item.Value, report); err != nil {
			return
		}
	}

	if _, err := r.backend.Create(ctx, backend.Item{
		Key:     backend.Key(userActivityReportsLock),
		Value:   []byte("null"),
		Expires: r.backend.Clock().Now().UTC().Add(75 * time.Second),
	}); err != nil {
		if trace.IsAlreadyExists(err) {
			// TODO: log that some other auth is already submitting
			_ = 0
		}
		return
	}

	lockCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if _, err = r.submitter.SubmitUsageReports(lockCtx,
		connect.NewRequest(&prehogv1.SubmitUsageReportsRequest{
			UserActivity: reports,
		}),
	); err != nil {
		if time.Since(reports[0].StartTime.AsTime()) > time.Hour*24 {
			alert, err := types.NewClusterAlert(
				"reporting-failed",
				"Failed to sync usage data for over 24 hours",
				types.WithAlertLabel(types.AlertOnLogin, "yes"),
				types.WithAlertLabel(types.AlertPermitAll, "yes"),
			)
			if err == nil {
				_ = r.status.UpsertClusterAlert(ctx, alert)
			}
		}
		// TODO: log
		return
	}

	for _, item := range getResult.Items {
		if err := r.backend.Delete(ctx, item.Key); err != nil {
			// TODO: log and try to delete the other items, might as well
			_ = 0
		}
	}
}
