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

package aggregating

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/backend"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const aggregateTimeGranularity = 15 * time.Minute

const (
	userActivityReportsPrefix = "userActivityReports"
	userActivityReportsLock   = "userActivityReportsLock"

	// maxSize is the maximum size for a backend value; dynamodb has a limit of
	// 400KiB per item, we store values in base64, there's some framing, and a
	// healthy 50% margin gets us to about 128KiB.
	maxSize = 131072

	reportTTL = 60 * 24 * time.Hour
)

// userActivityReportKey returns the backend key for a user activity report with
// a given UUID and start time, such that reports with an earlier start time
// will appear earlier in lexicographic ordering.
func userActivityReportKey(reportUUID uuid.UUID, startTime time.Time) []byte {
	return backend.Key(userActivityReportsPrefix, startTime.Format(time.RFC3339), reportUUID.String())
}

type ReporterConfig struct {
	Backend backend.Backend
	Log     logrus.FieldLogger

	ClusterName    types.ClusterName
	ReporterHostID string
}

func (cfg *ReporterConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if cfg.Log == nil {
		return trace.BadParameter("missing Log")
	}
	if cfg.ClusterName == nil {
		return trace.BadParameter("missing ClusterName")
	}
	if cfg.ReporterHostID == "" {
		return trace.BadParameter("missing ReporterHostID")
	}
	return nil
}

// NewReporter returns a new running [Reporter]. To avoid resource leaks,
// GracefulStop must be called or the base context must be closed. The context
// will be used for all backend operations.
func NewReporter(
	ctx context.Context,
	cfg ReporterConfig,
) (*Reporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	anonymizer, err := utils.NewHMACAnonymizer(cfg.ClusterName.GetClusterID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	baseCtx, baseCancel := context.WithCancel(ctx)
	periodicCtx, periodicCancel := context.WithCancel(baseCtx)

	r := &Reporter{
		anonymizer: anonymizer,
		backend:    cfg.Backend,
		log:        cfg.Log,

		clusterName:    anonymizer.AnonymizeNonEmpty(cfg.ClusterName.GetClusterName()),
		reporterHostID: anonymizer.AnonymizeNonEmpty(cfg.ReporterHostID),

		baseCtx:        baseCtx,
		baseCancel:     baseCancel,
		periodicCancel: periodicCancel,
	}

	r.wg.Add(1)
	go r.runPeriodicFinalize(periodicCtx)

	return r, nil
}

// Reporter aggregates and persists usage events to the backend.
type Reporter struct {
	anonymizer utils.Anonymizer
	backend    backend.Backend
	log        logrus.FieldLogger

	clusterName    []byte
	reporterHostID []byte

	wg             sync.WaitGroup
	baseCtx        context.Context
	baseCancel     context.CancelFunc
	periodicCancel context.CancelFunc

	mu           sync.Mutex
	stopped      bool
	startTime    time.Time
	userActivity map[string]*prehogv1.UserActivityRecord
}

var _ usagereporter.GracefulStopper = (*Reporter)(nil)

// GracefulStop implements [usagereporter.GracefulStopper]. Can only be called
// at most once.
func (r *Reporter) GracefulStop(ctx context.Context) error {
	r.periodicCancel()

	r.mu.Lock()

	r.stopped = true

	startTime := r.startTime
	userActivity := r.userActivity
	// release memory
	r.userActivity = nil

	r.mu.Unlock()

	if len(userActivity) > 0 {
		// we're allowed to do this because we know that we'll only Wait on r.wg
		// later on in this function's body; anywhere else we're not allowed to
		// Add while r.stopped is true, as Wait can't be called concurrently
		// with an Add from 0, and if we began shutting down we can't assume
		// that the count is above 0
		r.wg.Add(1)
		go r.persistUserActivity(startTime, userActivity)
	}

	go func() {
		select {
		case <-ctx.Done():
			r.baseCancel()
		case <-r.baseCtx.Done():
		}
	}()

	r.wg.Wait()
	r.baseCancel()
	return r.baseCtx.Err()
}

// AnonymizeAndSubmit implements [usagereporter.UsageReporter].
func (r *Reporter) AnonymizeAndSubmit(events ...usagereporter.Anonymizable) {
	filtered := events[:0]
	for _, event := range events {
		// this should drop all events that we don't care about
		switch event.(type) {
		case *usagereporter.UserLoginEvent,
			*usagereporter.SessionStartEvent,
			*usagereporter.KubeRequestEvent,
			*usagereporter.SFTPEvent:
			filtered = append(filtered, event)
		}
	}
	if len(filtered) == 0 {
		return
	}

	// this function must not block
	go r.anonymizeAndSubmit(filtered)
}

func (r *Reporter) anonymizeAndSubmit(events []usagereporter.Anonymizable) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopped {
		return
	}

	r.maybeFinalizeReportsLocked()

	userRecord := func(userName string) *prehogv1.UserActivityRecord {
		record := r.userActivity[userName]
		if record == nil {
			record = &prehogv1.UserActivityRecord{}
			r.userActivity[userName] = record
		}
		return record
	}

	for _, ae := range events {
		switch te := ae.(type) {
		case *usagereporter.UserLoginEvent:
			userRecord(te.UserName).Logins++
		case *usagereporter.SessionStartEvent:
			switch te.SessionType {
			case string(types.SSHSessionKind):
				userRecord(te.UserName).SshSessions++
			case string(types.AppSessionKind):
				userRecord(te.UserName).AppSessions++
			case string(types.KubernetesSessionKind):
				userRecord(te.UserName).KubeSessions++
			case string(types.DatabaseSessionKind):
				userRecord(te.UserName).DbSessions++
			case string(types.WindowsDesktopSessionKind):
				userRecord(te.UserName).DesktopSessions++
			case usagereporter.PortSessionType:
				userRecord(te.UserName).SshPortSessions++
			case usagereporter.TCPSessionType:
				userRecord(te.UserName).AppTcpSessions++
			}
		case *usagereporter.KubeRequestEvent:
			userRecord(te.UserName).KubeRequests++
		case *usagereporter.SFTPEvent:
			userRecord(te.UserName).SftpEvents++
		}
	}
}

func (r *Reporter) maybeFinalizeReportsLocked() {
	now := time.Now().UTC()

	if !r.startTime.Add(aggregateTimeGranularity).Before(now) {
		return
	}

	startTime := r.startTime
	userActivity := r.userActivity
	r.startTime = now.Truncate(aggregateTimeGranularity)
	// we expect the amount of users to not vary a lot between one time window
	// and the next
	r.userActivity = make(map[string]*prehogv1.UserActivityRecord, len(userActivity))

	if len(userActivity) > 0 {
		r.wg.Add(1)
		go r.persistUserActivity(startTime, userActivity)
	}
}

func (r *Reporter) persistUserActivity(startTime time.Time, userActivity map[string]*prehogv1.UserActivityRecord) {
	defer r.wg.Done()

	records := make([]*prehogv1.UserActivityRecord, 0, len(userActivity))
	for userName, record := range userActivity {
		record.UserName = r.anonymizer.AnonymizeNonEmpty(userName)
		records = append(records, record)
	}

	pbStartTime := timestamppb.New(startTime)
	expiry := startTime.Add(reportTTL)

	for len(records) > 0 {
		reportUUID, data, rem, err := prepareUserActivityReport(r.clusterName, r.reporterHostID, pbStartTime, records)
		if err != nil {
			r.log.WithError(err).WithFields(logrus.Fields{
				"start_time":   startTime,
				"lost_records": len(records),
			}).Error("Failed to prepare user activity report, dropping data.")
			return
		}
		reportRecords := len(records) - len(rem)
		records = rem

		if _, err := r.backend.Put(r.baseCtx, backend.Item{
			Key:     userActivityReportKey(reportUUID, startTime),
			Value:   data,
			Expires: expiry,
		}); err != nil {
			r.log.WithError(err).WithFields(logrus.Fields{
				"start_time":   startTime,
				"lost_records": reportRecords,
			}).Error("Failed to persist user activity report, dropping data.")
		}
	}
}

func prepareUserActivityReport(
	clusterName, reporterHostID []byte,
	startTime *timestamppb.Timestamp,
	records []*prehogv1.UserActivityRecord,
) (reportUUID uuid.UUID, data []byte, remaining []*prehogv1.UserActivityRecord, err error) {
	reportUUID = uuid.New()
	report := &prehogv1.UserActivityReport{
		ReportUuid:     reportUUID[:],
		ClusterName:    clusterName,
		ReporterHostid: reporterHostID,
		StartTime:      startTime,
		Records:        records,
	}

	for proto.Size(report) > maxSize {
		if len(report.Records) <= 1 {
			return uuid.Nil, nil, nil, trace.LimitExceeded("failed to marshal user activity report within size limit (this is a bug)")
		}

		report.Records = report.Records[:len(report.Records)/2]
	}

	wire, err := proto.Marshal(report)
	if err != nil {
		return uuid.Nil, nil, nil, trace.Wrap(err)
	}

	return reportUUID, wire, records[len(report.Records):], nil
}

func (r *Reporter) runPeriodicFinalize(ctx context.Context) {
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
			if !r.stopped {
				r.maybeFinalizeReportsLocked()
			}
			r.mu.Unlock()
		}
	}
}
