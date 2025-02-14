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
	"encoding/binary"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/backend"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	userActivityReportGranularity        = 15 * time.Minute
	resourceReportGranularity            = time.Hour
	botInstanceActivityReportGranularity = 15 * time.Minute
	rollbackGrace                        = time.Minute
	reportTTL                            = 60 * 24 * time.Hour

	checkInterval = time.Minute
)

// ReporterConfig contains the configuration for a [Reporter].
type ReporterConfig struct {
	// Backend is the backend used to store reports. Required
	Backend backend.Backend
	// Logger is the used for emitting log messages.
	Logger *slog.Logger
	// Clock is the clock used for timestamping reports and deciding when to
	// persist them to the backend. Optional, defaults to the real clock.
	Clock clockwork.Clock
	// ClusterName is the ClusterName resource for the current cluster, used for
	// anonymization and to report the cluster name itself. Required.
	ClusterName types.ClusterName
	// HostID is the host ID of the current Teleport instance, added to reports
	// for auditing purposes. Required.
	HostID string
	// Anonymizer is used to anonymize data user or resource names. Required.
	Anonymizer utils.Anonymizer
}

// CheckAndSetDefaults checks the [ReporterConfig] for validity, returning nil
// if there's no error. Idempotent but not concurrent safe, as it might need to
// write to the config to apply defaults.
func (cfg *ReporterConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.ClusterName == nil {
		return trace.BadParameter("missing ClusterName")
	}
	if cfg.HostID == "" {
		return trace.BadParameter("missing HostID")
	}
	if cfg.Anonymizer == nil {
		return trace.BadParameter("missing Anonymizer")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
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

	baseCtx, baseCancel := context.WithCancel(ctx)

	r := &Reporter{
		anonymizer: cfg.Anonymizer,
		svc:        reportService{cfg.Backend},
		logger:     cfg.Logger,
		clock:      cfg.Clock,

		ingest:  make(chan usagereporter.Anonymizable),
		closing: make(chan struct{}),
		done:    make(chan struct{}),

		clusterName: cfg.ClusterName.GetClusterName(),
		hostID:      cfg.HostID,

		baseCancel: baseCancel,
	}

	go r.run(baseCtx)

	return r, nil
}

// Reporter aggregates and persists usage events to the backend.
type Reporter struct {
	anonymizer utils.Anonymizer
	svc        reportService
	logger     *slog.Logger
	clock      clockwork.Clock

	// ingest collects events from calls to [AnonymizeAndSubmit] to the
	// background goroutine.
	ingest chan usagereporter.Anonymizable
	// closing is closed when we're not interested in collecting events anymore.
	closing chan struct{}
	// closingOnce closes [closing] once.
	closingOnce sync.Once
	// done is closed at the end of the background goroutine.
	done chan struct{}

	// clusterName is the un-anonymized cluster name.
	clusterName string
	// hostID is the un-anonymized host ID of the reporter (this instance).
	hostID string

	// baseCancel cancels the context used by the background goroutine.
	baseCancel context.CancelFunc

	// ingested, if non-nil, received events after being added to the aggregated
	// data. Used in tests.
	ingested chan usagereporter.Anonymizable
}

var _ usagereporter.GracefulStopper = (*Reporter)(nil)

// GracefulStop implements [usagereporter.GracefulStopper].
func (r *Reporter) GracefulStop(ctx context.Context) error {
	r.closingOnce.Do(func() { close(r.closing) })

	select {
	case <-r.done:
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
		// note: make sure this matches the set of all events handled in [*Reporter.run]
		switch event.(type) {
		case *usagereporter.UserLoginEvent,
			*usagereporter.SessionStartEvent,
			*usagereporter.KubeRequestEvent,
			*usagereporter.SFTPEvent,
			*usagereporter.ResourceHeartbeatEvent,
			*usagereporter.UserCertificateIssuedEvent,
			*usagereporter.BotJoinEvent,
			*usagereporter.SPIFFESVIDIssuedEvent,
			*usagereporter.AccessListReviewEvent:
			filtered = append(filtered, event)
		}
	}
	if len(filtered) == 0 {
		return
	}

	// this function must not block
	go r.anonymizeAndSubmit(filtered)
}

// convertUserKind converts a v1alpha UserKind to a v1 UserKind.
func convertUserKind(v1AlphaUserKind prehogv1alpha.UserKind) prehogv1.UserKind {
	switch v1AlphaUserKind {
	case prehogv1alpha.UserKind_USER_KIND_BOT:
		return prehogv1.UserKind_USER_KIND_BOT
	case prehogv1alpha.UserKind_USER_KIND_HUMAN:
		return prehogv1.UserKind_USER_KIND_HUMAN
	default:
		return prehogv1.UserKind_USER_KIND_UNSPECIFIED
	}
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

// run processes events incoming from r.ingest, keeping statistics of
// users activity and resource usage per users, also collects cluster resource counts.
// Every granularity time period, it sends accumulated stats to the prehog.
// Runs perpetually in a goroutine until the context is canceled or reporter is closed.
func (r *Reporter) run(ctx context.Context) {
	defer r.baseCancel()
	defer close(r.done)

	ticker := r.clock.NewTicker(checkInterval)
	defer ticker.Stop()

	userActivityStartTime := r.clock.Now().UTC().Truncate(userActivityReportGranularity)
	userActivityWindowStart := userActivityStartTime.Add(-rollbackGrace)
	userActivityWindowEnd := userActivityStartTime.Add(userActivityReportGranularity)

	userActivity := make(map[string]*prehogv1.UserActivityRecord)
	userRecord := func(userName string, v1AlphaUserKind prehogv1alpha.UserKind) *prehogv1.UserActivityRecord {
		v1UserKind := convertUserKind(v1AlphaUserKind)

		record := userActivity[userName]
		if record == nil {
			record = &prehogv1.UserActivityRecord{
				UserKind: v1UserKind,
			}
			userActivity[userName] = record
		}

		// Attempt to sanely handle any changes to the record's UserKind that
		// might occur after it's original creation.
		if record.UserKind != v1UserKind {
			recordUnspecified := record.UserKind == prehogv1.UserKind_USER_KIND_UNSPECIFIED
			incomingUnspecified := v1UserKind == prehogv1.UserKind_USER_KIND_UNSPECIFIED

			switch {
			case incomingUnspecified:
				// Ignore any incoming unspecified events.
			case recordUnspecified && !incomingUnspecified:
				// It's okay to discover the kind of a user later. This may
				// indicate the first event that established a record came from
				// an outdated node.
				record.UserKind = v1UserKind
			default:
				// Otherwise, update and log a warning. Flipping between
				// bot/human is a programming error.
				r.logger.WarnContext(ctx, "Record user_kind has changed unexpectedly", "from", record.UserKind, "to", v1UserKind)
				record.UserKind = v1UserKind
			}
		}

		return record
	}
	// userSPIFFEIDActivity is map[username]map[spiffeid]count
	userSPIFFEIDActivity := make(map[string]map[string]uint32)
	incrementUserSPIFFEIDActivity := func(userName string, spiffeID string) {
		user := userSPIFFEIDActivity[userName]
		if user == nil {
			user = make(map[string]uint32)
			userSPIFFEIDActivity[userName] = user
		}
		user[spiffeID]++
	}

	botInstanceActivityStartTime := r.clock.Now().UTC().Truncate(botInstanceActivityReportGranularity)
	botInstanceActivityWindowStart := botInstanceActivityStartTime.Add(-rollbackGrace)
	botInstanceActivityWindowEnd := botInstanceActivityStartTime.Add(botInstanceActivityReportGranularity)
	botInstanceActivity := make(map[botInstanceActivityKey]*prehogv1.BotInstanceActivityRecord)
	botInstanceRecord := func(botUserName string, botInstanceID string) *prehogv1.BotInstanceActivityRecord {
		key := botInstanceActivityKey{
			botUserName:   botUserName,
			botInstanceID: botInstanceID,
		}
		record := botInstanceActivity[key]
		if record == nil {
			record = &prehogv1.BotInstanceActivityRecord{}
			botInstanceActivity[key] = record
		}
		return record
	}

	resourceUsageStartTime := r.clock.Now().UTC().Truncate(resourceReportGranularity)
	resourceUsageWindowStart := resourceUsageStartTime.Add(-rollbackGrace)
	resourceUsageWindowEnd := resourceUsageStartTime.Add(resourceReportGranularity)

	// ResourcePresences is a map of resource kinds to sets of resource names.
	// As there may be multiple heartbeats for the same resource, we use a set to count each once.
	resourcePresences := make(map[prehogv1.ResourceKind]map[string]struct{})
	resourcePresence := func(kind prehogv1.ResourceKind) map[string]struct{} {
		record := resourcePresences[kind]
		if record == nil {
			resourcePresences[kind] = make(map[string]struct{})
		}
		return resourcePresences[kind]
	}

	var wg sync.WaitGroup

Ingest:
	for {
		var ae usagereporter.Anonymizable
		select {
		case <-ticker.Chan():
		case ae = <-r.ingest:
		case <-ctx.Done():
			r.closingOnce.Do(func() { close(r.closing) })
			break Ingest
		case <-r.closing:
			break Ingest
		}

		if now := r.clock.Now().UTC(); now.Before(userActivityWindowStart) || !now.Before(userActivityWindowEnd) {
			if len(userActivity) > 0 {
				wg.Add(1)
				go func(
					ctx context.Context,
					startTime time.Time,
					userActivity map[string]*prehogv1.UserActivityRecord,
					userSPIFFEIDActivity map[string]map[string]uint32,
				) {
					defer wg.Done()
					r.persistUserActivity(ctx, startTime, userActivity, userSPIFFEIDActivity)
				}(ctx, userActivityStartTime, userActivity, userSPIFFEIDActivity)
			}

			userActivityStartTime = now.Truncate(userActivityReportGranularity)
			userActivityWindowStart = userActivityStartTime.Add(-rollbackGrace)
			userActivityWindowEnd = userActivityStartTime.Add(userActivityReportGranularity)
			userActivity = make(map[string]*prehogv1.UserActivityRecord, len(userActivity))
			userSPIFFEIDActivity = make(map[string]map[string]uint32, len(userSPIFFEIDActivity))
		}

		if now := r.clock.Now().UTC(); now.Before(botInstanceActivityWindowStart) || !now.Before(botInstanceActivityWindowEnd) {
			if len(botInstanceActivity) > 0 {
				wg.Add(1)
				go func(
					ctx context.Context,
					startTime time.Time,
					botInstanceActivity map[botInstanceActivityKey]*prehogv1.BotInstanceActivityRecord,
				) {
					defer wg.Done()
					r.persistBotInstanceActivity(ctx, startTime, botInstanceActivity)
				}(ctx, botInstanceActivityStartTime, botInstanceActivity)
			}

			botInstanceActivityStartTime = now.Truncate(botInstanceActivityReportGranularity)
			botInstanceActivityWindowStart = botInstanceActivityStartTime.Add(-rollbackGrace)
			botInstanceActivityWindowEnd = botInstanceActivityStartTime.Add(botInstanceActivityReportGranularity)
			botInstanceActivity = make(map[botInstanceActivityKey]*prehogv1.BotInstanceActivityRecord, len(botInstanceActivity))
		}

		if now := r.clock.Now().UTC(); now.Before(resourceUsageWindowStart) || !now.Before(resourceUsageWindowEnd) {
			if len(resourcePresences) > 0 {
				wg.Add(1)
				go func(ctx context.Context, startTime time.Time, resourcePresences map[prehogv1.ResourceKind]map[string]struct{}) {
					defer wg.Done()
					r.persistResourcePresence(ctx, startTime, resourcePresences)
				}(ctx, resourceUsageStartTime, resourcePresences)
			}

			resourceUsageStartTime = now.Truncate(resourceReportGranularity)
			resourceUsageWindowStart = resourceUsageStartTime.Add(-rollbackGrace)
			resourceUsageWindowEnd = resourceUsageStartTime.Add(resourceReportGranularity)
			resourcePresences = make(map[prehogv1.ResourceKind]map[string]struct{}, len(resourcePresences))
		}

		switch te := ae.(type) {
		case *usagereporter.UserLoginEvent:
			// Bots never generate tp.user.login events.
			userRecord(te.UserName, prehogv1alpha.UserKind_USER_KIND_HUMAN).Logins++
		case *usagereporter.SessionStartEvent:
			switch te.SessionType {
			case string(types.SSHSessionKind):
				userRecord(te.UserName, te.UserKind).SshSessions++
			case string(types.AppSessionKind):
				userRecord(te.UserName, te.UserKind).AppSessions++
			case string(types.KubernetesSessionKind):
				userRecord(te.UserName, te.UserKind).KubeSessions++
			case string(types.DatabaseSessionKind):
				userRecord(te.UserName, te.UserKind).DbSessions++
			case string(types.WindowsDesktopSessionKind):
				userRecord(te.UserName, te.UserKind).DesktopSessions++
			case usagereporter.PortSSHSessionType:
				userRecord(te.UserName, te.UserKind).SshPortV2Sessions++
			case usagereporter.PortKubeSessionType:
				userRecord(te.UserName, te.UserKind).KubePortSessions++
			case usagereporter.TCPSessionType:
				userRecord(te.UserName, te.UserKind).AppTcpSessions++
			}
		case *usagereporter.KubeRequestEvent:
			userRecord(te.UserName, te.UserKind).KubeRequests++
		case *usagereporter.SFTPEvent:
			userRecord(te.UserName, te.UserKind).SftpEvents++
		case *usagereporter.ResourceHeartbeatEvent:
			// ResourceKind is the same int32 in both prehogv1 and prehogv1alpha1.
			resourcePresence(prehogv1.ResourceKind(te.Kind))[te.Name] = struct{}{}
		case *usagereporter.SPIFFESVIDIssuedEvent:
			userRecord(te.UserName, te.UserKind).SpiffeSvidsIssued++
			incrementUserSPIFFEIDActivity(te.UserName, te.SpiffeId)
			if te.BotInstanceId != "" {
				botInstanceRecord(te.UserName, te.BotInstanceId).SpiffeSvidsIssued++
			}
		case *usagereporter.BotJoinEvent:
			botUserName := machineidv1.BotResourceName(te.BotName)
			userRecord(botUserName, prehogv1alpha.UserKind_USER_KIND_BOT).BotJoins++
			if te.BotInstanceId != "" {
				botInstanceRecord(botUserName, te.BotInstanceId).BotJoins++
			}
		case *usagereporter.UserCertificateIssuedEvent:
			// Note: kind is poorly defined for this event type, so we'll assume
			// unspecified even though non-bot users are almost certainly human.
			kind := prehogv1alpha.UserKind_USER_KIND_UNSPECIFIED
			if te.IsBot {
				kind = prehogv1alpha.UserKind_USER_KIND_BOT
			}

			userRecord(te.UserName, kind).CertificatesIssued++
			if te.BotInstanceId != "" {
				botInstanceRecord(te.UserName, te.BotInstanceId).CertificatesIssued++
			}
		case *usagereporter.AccessListReviewEvent:
			userRecord(te.UserName, prehogv1alpha.UserKind_USER_KIND_HUMAN).AccessListsReviewed++
		}

		if ae != nil && r.ingested != nil {
			r.ingested <- ae
		}
	}

	if len(userActivity) > 0 {
		r.persistUserActivity(ctx, userActivityStartTime, userActivity, userSPIFFEIDActivity)
	}

	if len(botInstanceActivity) > 0 {
		r.persistBotInstanceActivity(ctx, botInstanceActivityStartTime, botInstanceActivity)
	}

	if len(resourcePresences) > 0 {
		r.persistResourcePresence(ctx, resourceUsageStartTime, resourcePresences)
	}

	wg.Wait()
}

type botInstanceActivityKey struct {
	botUserName   string
	botInstanceID string
}

func (r *Reporter) persistBotInstanceActivity(
	ctx context.Context,
	startTime time.Time,
	botInstanceActivity map[botInstanceActivityKey]*prehogv1.BotInstanceActivityRecord,
) {
	records := make([]*prehogv1.BotInstanceActivityRecord, 0, len(botInstanceActivity))
	for key, record := range botInstanceActivity {
		record.BotUserName = r.anonymizer.AnonymizeNonEmpty(key.botUserName)
		record.BotInstanceId = r.anonymizer.AnonymizeNonEmpty(key.botInstanceID)
		records = append(records, record)
	}

	anonymizedClusterName := r.anonymizer.AnonymizeNonEmpty(r.clusterName)
	anonymizedHostID := r.anonymizer.AnonymizeNonEmpty(r.hostID)

	reports, err := prepareBotInstanceActivityReports(
		anonymizedClusterName, anonymizedHostID, startTime, records,
	)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to prepare bot instance activity report, dropping data.",
			"start_time", startTime,
			"lost_records", len(records),
			"error", err,
		)
		return
	}

	for _, report := range reports {
		if err := r.svc.upsertBotInstanceActivityReport(ctx, report, reportTTL); err != nil {
			r.logger.ErrorContext(ctx, "Failed to persist bot instance activity report, dropping data.",
				"start_time", startTime,
				"lost_records", len(report.Records),
				"error", err,
			)
			continue
		}

		reportUUID, _ := uuid.FromBytes(report.ReportUuid)
		r.logger.DebugContext(ctx, "Persisted bot instance activity report.",
			"report_uuid", reportUUID,
			"start_time", startTime,
			"records", len(report.Records),
		)
	}
}

func (r *Reporter) persistUserActivity(
	ctx context.Context,
	startTime time.Time,
	userActivity map[string]*prehogv1.UserActivityRecord,
	issuedSPIFFEIDs map[string]map[string]uint32,
) {
	records := make([]*prehogv1.UserActivityRecord, 0, len(userActivity))
	for userName, record := range userActivity {
		record.UserName = r.anonymizer.AnonymizeNonEmpty(userName)

		spiffeIDRecords := make([]*prehogv1.SPIFFEIDRecord, 0, len(issuedSPIFFEIDs[userName]))
		for spiffeID, count := range issuedSPIFFEIDs[userName] {
			spiffeIDRecords = append(spiffeIDRecords, &prehogv1.SPIFFEIDRecord{
				SpiffeId:    r.anonymizer.AnonymizeNonEmpty(spiffeID),
				SvidsIssued: count,
			})
		}
		record.SpiffeIdsIssued = spiffeIDRecords

		records = append(records, record)
	}

	anonymizedClusterName := r.anonymizer.AnonymizeNonEmpty(r.clusterName)
	anonymizedHostID := r.anonymizer.AnonymizeNonEmpty(r.hostID)

	reports, err := prepareUserActivityReports(anonymizedClusterName, anonymizedHostID, startTime, records)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to prepare user activity report, dropping data.",
			"start_time", startTime,
			"lost_records", len(records),
			"error", err,
		)
		return
	}

	for _, report := range reports {
		if err := r.svc.upsertUserActivityReport(ctx, report, reportTTL); err != nil {
			r.logger.ErrorContext(ctx, "Failed to persist user activity report, dropping data.",
				"start_time", startTime,
				"lost_records", len(report.Records),
				"error", err,
			)
			continue
		}

		reportUUID, _ := uuid.FromBytes(report.ReportUuid)
		r.logger.DebugContext(ctx, "Persisted user activity report.",
			"report_uuid", reportUUID,
			"start_time", startTime,
			"records", len(report.Records),
		)
	}
}

func (r *Reporter) persistResourcePresence(ctx context.Context, startTime time.Time, resourcePresences map[prehogv1.ResourceKind]map[string]struct{}) {
	records := make([]*prehogv1.ResourceKindPresenceReport, 0, len(resourcePresences))
	for kind, set := range resourcePresences {
		record := &prehogv1.ResourceKindPresenceReport{
			ResourceKind: kind,
			ResourceIds:  make([]uint64, 0, len(set)),
		}
		for name := range set {
			anonymized := r.anonymizer.AnonymizeNonEmpty(name)
			packed := binary.LittleEndian.Uint64(anonymized[:]) // only the first 8 bytes are used
			record.ResourceIds = append(record.ResourceIds, packed)
		}
		records = append(records, record)
	}

	anonymizedClusterName := r.anonymizer.AnonymizeNonEmpty(r.clusterName)
	anonymizedHostID := r.anonymizer.AnonymizeNonEmpty(r.hostID)

	reports, err := prepareResourcePresenceReports(anonymizedClusterName, anonymizedHostID, startTime, records)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to prepare resource presence report, dropping data.", "start_time", startTime, "error", err)
		return
	}

	for _, report := range reports {
		if err := r.svc.upsertResourcePresenceReport(ctx, report, reportTTL); err != nil {
			r.logger.ErrorContext(ctx, "Failed to persist resource presence report, dropping data.", "start_time", startTime, "error", err)
			continue
		}

		reportUUID, _ := uuid.FromBytes(report.ReportUuid)
		r.logger.DebugContext(ctx, "Persisted resource presence report.", "report_uuid", reportUUID, "start_time", startTime)
	}
}
