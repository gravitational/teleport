/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package machineidv1

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
)

// AutoUpdateVersionReporter aggregates bot instance version numbers into an
// `autoupdate_bot_instancenreport` resource periodically. We run a single
// leader-elected instance of the reporter per cluster.
type AutoUpdateVersionReporter struct {
	clock      clockwork.Clock
	logger     *slog.Logger
	store      AutoUpdateReportStore
	cache      BotInstancesCache
	semaphores types.Semaphores
	hostUUID   string

	mu       sync.Mutex
	leaderCh chan struct{}
}

// AutoUpdateReportStore is used to write the report.
type AutoUpdateReportStore interface {
	UpsertAutoUpdateBotInstanceReport(ctx context.Context, report *autoupdate.AutoUpdateBotInstanceReport) (*autoupdate.AutoUpdateBotInstanceReport, error)
}

// AutoUpdateVersionReporterConfig holds configuration for a version reporter.
type AutoUpdateVersionReporterConfig struct {
	// Clock used to mock time in tests.
	Clock clockwork.Clock

	// Logger to which errors and messages will be written.
	Logger *slog.Logger

	// Store is used to write the report.
	Store AutoUpdateReportStore

	// Cache will be used to list and count the bot instances.
	Cache BotInstancesCache

	// Semaphores interface used to implement leader-election.
	Semaphores types.Semaphores

	// HostUUID is the identity of the host running the reporter.
	HostUUID string
}

// NewAutoUpdateVersionReporter creates an AutoUpdateVersionReporter with the
// given configuration. You must call Run to start the leader-election process
// and Trigger whenever you want to generate a new report.
func NewAutoUpdateVersionReporter(cfg AutoUpdateVersionReporterConfig) (*AutoUpdateVersionReporter, error) {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Semaphores == nil {
		return nil, trace.BadParameter("Semaphores is required")
	}
	if cfg.HostUUID == "" {
		return nil, trace.BadParameter("HostUUID is required")
	}
	if cfg.Store == nil {
		return nil, trace.BadParameter("Store is required")
	}
	if cfg.Cache == nil {
		return nil, trace.BadParameter("Cache is required")
	}
	return &AutoUpdateVersionReporter{
		clock:      cfg.Clock,
		logger:     cfg.Logger,
		cache:      cfg.Cache,
		store:      cfg.Store,
		semaphores: cfg.Semaphores,
		hostUUID:   cfg.HostUUID,
		leaderCh:   make(chan struct{}),
	}, nil
}

// Run begins the leader-election process until the given context is canceled or
// reaches its deadline. It will spawn a new goroutine, you do not need to run it
// with the go keyword.
func (r *AutoUpdateVersionReporter) Run(ctx context.Context) error {
	// The runLeader method will do its own retrying around acquiring the
	// semaphore. This retries the whole operation (e.g. after the lease is
	// lost).
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  30 * time.Second,
		Driver: retryutils.NewExponentialDriver(30 * time.Second),
		Max:    10 * time.Minute,
		Jitter: retryutils.HalfJitter,
		Clock:  r.clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		defer r.logger.DebugContext(ctx, "Shutting down")

		for {
			started := r.clock.Now()
			r.runLeader(ctx)
			leaderFor := r.clock.Now().Sub(started)

			// Context is done, exit immediately.
			if ctx.Err() != nil {
				return
			}

			// If we were leader for a decent amount of time, any previous
			// backoff likely doesn't apply anymore.
			if leaderFor > 5*time.Minute {
				retry.Reset()
			}

			// Wait for the next retry interval.
			retry.Inc()

			select {
			case <-retry.After():
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (r *AutoUpdateVersionReporter) runLeader(ctx context.Context) error {
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: r.semaphores,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindAuthServer,
					SemaphoreName: "auto_update_bot_version_reporter",
					MaxLeases:     1,
					Holder:        r.hostUUID,
				},
				Expiry: 1 * time.Minute,
				Clock:  r.clock,
			},
			Retry: retryutils.LinearConfig{
				Clock:  r.clock,
				First:  time.Second,
				Step:   30 * time.Second,
				Max:    1 * time.Minute,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		r.mu.Lock()
		r.leaderCh = make(chan struct{})
		r.mu.Unlock()

		r.logger.DebugContext(ctx, "No longer the leader")
		lease.Stop()

		if err := lease.Wait(); err != nil {
			r.logger.WarnContext(ctx, "Error cleaning up semaphore", "error", err)
		}
	}()

	r.mu.Lock()
	close(r.leaderCh)
	r.mu.Unlock()

	r.logger.DebugContext(ctx, "Acquired semaphore and became the leader")

	select {
	case <-lease.Done():
		return lease.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Report triggers the generation of a new report, ignored if the reporter is
// not the current elected leader. This method is typically called by the auth
// server's runPeriodicOperations method.
func (r *AutoUpdateVersionReporter) Report(ctx context.Context) error {
	if !r.IsLeader() {
		r.logger.DebugContext(ctx, "Not the leader, ignoring trigger to generate report")
		return nil
	}

	r.logger.DebugContext(ctx, "Generating report")

	groups := make(map[string]*autoupdate.AutoUpdateBotInstanceReportSpecGroup)

	var nextToken string
	for {
		var (
			instances []*machineidv1.BotInstance
			err       error
		)
		instances, nextToken, err = r.cache.ListBotInstances(
			ctx,
			defaults.DefaultChunkSize,
			nextToken,
			&services.ListBotInstancesRequestOptions{},
		)
		if err != nil {
			return trace.Wrap(err, "listing bot instances")
		}

		for _, inst := range instances {
			// Defend against backends not removing expired items in a timely manner.
			expiry := inst.GetMetadata().GetExpires().AsTime()
			if expiry.Before(r.clock.Now()) && !expiry.IsZero() {
				continue
			}

			// Take the version information from the latest heartbeat.
			heartbeats := inst.GetStatus().GetLatestHeartbeats()
			if len(heartbeats) == 0 {
				continue
			}
			latest := heartbeats[len(heartbeats)-1]

			// If the bot did not send an ExternalUpdater, it does not properly
			// support managed updates - so we put it in the no-group ("") group.
			var groupName string
			if ui := latest.GetUpdaterInfo(); latest.ExternalUpdater != "" && ui != nil {
				groupName = ui.UpdateGroup
			}

			group, ok := groups[groupName]
			if !ok {
				group = &autoupdate.AutoUpdateBotInstanceReportSpecGroup{
					Versions: make(map[string]*autoupdate.AutoUpdateBotInstanceReportSpecGroupVersion),
				}
				groups[groupName] = group
			}

			version, ok := group.Versions[latest.GetVersion()]
			if !ok {
				version = &autoupdate.AutoUpdateBotInstanceReportSpecGroupVersion{}
				group.Versions[latest.GetVersion()] = version
			}
			version.Count++
		}

		if nextToken == "" || len(instances) == 0 {
			break
		}
	}

	report, err := update.NewAutoUpdateBotInstanceReport(&autoupdate.AutoUpdateBotInstanceReportSpec{
		Timestamp: timestamppb.New(r.clock.Now()),
		Groups:    groups,
	})
	if err != nil {
		return trace.Wrap(err, "creating report")
	}
	if _, err = r.store.UpsertAutoUpdateBotInstanceReport(ctx, report); err != nil {
		return trace.Wrap(err, "storing report")
	}

	return nil
}

// LeaderCh returns a channel that will be closed when this instance of the
// reporter becomes the leader. It is used to block in tests.
func (r *AutoUpdateVersionReporter) LeaderCh() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.leaderCh
}

// IsLeader returns whether this instance of the reporter is the current leader.
func (r *AutoUpdateVersionReporter) IsLeader() bool {
	select {
	case <-r.LeaderCh():
		return true
	default:
		return false
	}
}

// EmitInstancesMetric updates the given gauge metric based on the instance report.
func EmitInstancesMetric(report *autoupdate.AutoUpdateBotInstanceReport, gauge *prometheus.GaugeVec) {
	gauge.Reset()

	byVersion := make(map[string]int32)

	for group, groupMetrics := range report.GetSpec().GetGroups() {
		// Empty group means the bot isn't using Managed Updates.
		if group == "" {
			for version, versionMetrics := range groupMetrics.GetVersions() {
				gauge.With(prometheus.Labels{
					teleport.TagVersion:          version,
					teleport.TagAutomaticUpdates: "false",
				}).Set(float64(versionMetrics.Count))
			}
			continue
		}

		for version, metrics := range groupMetrics.GetVersions() {
			byVersion[version] += metrics.Count
		}
	}

	for version, count := range byVersion {
		gauge.With(prometheus.Labels{
			teleport.TagVersion:          version,
			teleport.TagAutomaticUpdates: "true",
		}).Set(float64(count))
	}
}
