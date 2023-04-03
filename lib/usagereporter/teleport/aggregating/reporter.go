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

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/backend"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	reportGranularity = 15 * time.Minute
	rollbackGrace     = time.Minute
	reportTTL         = 60 * 24 * time.Hour
)

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
func NewReporter(ctx context.Context, cfg ReporterConfig) (*Reporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	anonymizer, err := utils.NewHMACAnonymizer(cfg.ClusterName.GetClusterID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	baseCtx, baseCancel := context.WithCancel(ctx)

	r := &Reporter{
		anonymizer: anonymizer,
		svc:        reportService{cfg.Backend},
		log:        cfg.Log,

		ingest:  make(chan usagereporter.Anonymizable),
		closing: make(chan struct{}),
		done:    make(chan struct{}),

		clusterName:    anonymizer.AnonymizeNonEmpty(cfg.ClusterName.GetClusterName()),
		reporterHostID: anonymizer.AnonymizeNonEmpty(cfg.ReporterHostID),

		baseCancel: baseCancel,
	}

	go r.run(baseCtx)

	return r, nil
}

// Reporter aggregates and persists usage events to the backend.
type Reporter struct {
	anonymizer utils.Anonymizer
	svc        reportService
	log        logrus.FieldLogger

	// ingest collects events from calls to [AnonymizeAndSubmit] to the
	// background goroutine.
	ingest chan usagereporter.Anonymizable
	// closing is closed when we're not interested in collecting events anymore.
	closing chan struct{}
	// closingOnce closes [closing] once.
	closingOnce sync.Once
	// done is closed at the end of the background goroutine.
	done chan struct{}

	// clusterName is the anonymized cluster name.
	clusterName []byte
	// reporterHostID is the anonymized host ID of the reporter (this agent).
	reporterHostID []byte

	// baseCancel cancels the context used by the background goroutine.
	baseCancel context.CancelFunc
}

var _ usagereporter.GracefulStopper = (*Reporter)(nil)

// GracefulStop implements [usagereporter.GracefulStopper].
func (r *Reporter) GracefulStop(ctx context.Context) error {
	r.closingOnce.Do(func() { close(r.closing) })

	select {
	case <-r.done:
		r.baseCancel()
		return nil
	case <-ctx.Done():
	}

	r.baseCancel()
	<-r.done

	return ctx.Err()
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
	for _, e := range events {
		select {
		case r.ingest <- e:
		case <-r.closing:
			return
		}
	}
}

func (r *Reporter) run(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	startTime := time.Now().UTC().Truncate(reportGranularity)
	userActivity := make(map[string]*prehogv1.UserActivityRecord)

	userRecord := func(userName string) *prehogv1.UserActivityRecord {
		record := userActivity[userName]
		if record == nil {
			record = &prehogv1.UserActivityRecord{}
			userActivity[userName] = record
		}
		return record
	}

	var wg sync.WaitGroup

Ingest:
	for {
		var ae usagereporter.Anonymizable
		select {
		case <-ticker.C:
		case ae = <-r.ingest:

		case <-ctx.Done():
			r.closingOnce.Do(func() { close(r.closing) })
			break Ingest
		case <-r.closing:
			break Ingest
		}

		now := time.Now().UTC()
		if !now.Before(startTime.Add(reportGranularity)) || now.Before(startTime.Add(-rollbackGrace)) {
			if len(userActivity) > 0 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					r.persistUserActivity(ctx, startTime, userActivity)
				}()
			}

			startTime = now.Truncate(reportGranularity)
			userActivity = make(map[string]*prehogv1.UserActivityRecord, len(userActivity))
		}

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

	if len(userActivity) > 0 {
		r.persistUserActivity(ctx, startTime, userActivity)
	}

	wg.Wait()
}

func (r *Reporter) persistUserActivity(ctx context.Context, startTime time.Time, userActivity map[string]*prehogv1.UserActivityRecord) {
	records := make([]*prehogv1.UserActivityRecord, 0, len(userActivity))
	for userName, record := range userActivity {
		record.UserName = r.anonymizer.AnonymizeNonEmpty(userName)
		records = append(records, record)
	}

	for len(records) > 0 {
		report, err := prepareUserActivityReport(r.clusterName, r.reporterHostID, startTime, records)
		if err != nil {
			r.log.WithError(err).WithFields(logrus.Fields{
				"start_time":   startTime,
				"lost_records": len(records),
			}).Error("Failed to prepare user activity report, dropping data.")
			return
		}
		records = records[len(report.Records):]

		if err := r.svc.upsertUserActivityReport(ctx, report, reportTTL); err != nil {
			r.log.WithError(err).WithFields(logrus.Fields{
				"start_time":   startTime,
				"lost_records": len(report.Records),
			}).Error("Failed to persist user activity report, dropping data.")
		}
	}
}
