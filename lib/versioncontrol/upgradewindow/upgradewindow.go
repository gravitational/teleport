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

package upgradewindow

import (
	"context"
	"os"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Driver represents a type capable of exporting the maintenance window schedule to an external
// upgrader, such as the teleport-upgrade systemd timer or the kube-updater controller.
type Driver interface {
	// Kind gets the upgrader kind associated with this export driver.
	Kind() string

	// SyncSchedule exports the appropriate maintenance window schedule if one is present, or
	// resets/clears the maintenance window if the schedule response returns no viable scheduling
	// info.
	SyncSchedule(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error

	// ResetSchedule forcibly clears any previously exported maintenance window values. This should be
	// called if teleport experiences prolonged loss of auth connectivity, which may be an indicator
	// that the control plane has been upgraded s.t. this agent is no longer compatible.
	ResetSchedule(ctx context.Context) error
}

// ExportFunc represents the ExportUpgradeWindows rpc exposed by auth servers.
type ExportFunc func(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (proto.ExportUpgradeWindowsResponse, error)

// contextLike lets us abstract over the difference between basic contexts and context-like values such
// as control stream senders or resource watchers. the exporter uses a contextLike value to decide wether
// or not auth connectivity appears healthy. during normal runtime, we end up using the inventory control
// stream send handle.
type contextLike interface {
	Done() <-chan struct{}
}

type testEvent string

const (
	resetFromExport  testEvent = "reset-from-export"
	resetFromRun     testEvent = "reset-from-run"
	exportAttempt    testEvent = "export-attempt"
	exportSuccess    testEvent = "export-success"
	exportFailure    testEvent = "export-failure"
	getExportErr     testEvent = "get-export-err"
	syncExportErr    testEvent = "sync-export-err"
	sentinelAcquired testEvent = "sentinel-acquired"
	sentinelLost     testEvent = "sentinel-lost"
)

// ExporterConfig configures a maintenance window exporter.
type ExporterConfig[C contextLike] struct {
	// Driver is the underlying export driver.
	Driver Driver

	// ExportFunc gets the current maintenance window.
	ExportFunc ExportFunc

	// AuthConnectivitySentinel is a channel that yields context-like values indicating the current health of
	// auth connectivity. When connectivity to auth is established, a context-like value should be sent over
	// the channel. If auth connectivity is subsequently lost, that context-like value must be canceled.
	// During normal runtime, this should use DownstreamInventoryHandle.Sender(). During tests
	// it can just be a stream of context.Context values.
	AuthConnectivitySentinel <-chan C

	// --- below fields are all optional

	// UnhealthyThreshold is the threshold after which failure to export
	// is treated as an unhealthy event.
	UnhealthyThreshold time.Duration

	// ExportInterval is the interval at which exports are attempted
	ExportInterval time.Duration

	// FirstExport is a custom duration used for firt export attempt.
	FirstExport time.Duration

	testEvents chan testEvent
}

func (c *ExporterConfig[C]) CheckAndSetDefaults() error {
	if c.Driver == nil {
		return trace.BadParameter("exporter config missing required parameter 'Driver'")
	}

	if c.ExportFunc == nil {
		return trace.BadParameter("exporter config missing required parameter 'ExportFunc'")
	}

	if c.AuthConnectivitySentinel == nil {
		return trace.BadParameter("exporter config missing required parameter 'AuthConnectivitySentinel'")
	}

	// allow switching to faster default duration values for testing purposes
	fastExport := os.Getenv("TELEPORT_UNSTABLE_FAST_MW_EXPORT") == "yes"

	if c.UnhealthyThreshold == 0 {
		// 9m is fairly arbitrary, but was picked based on the the idea that a good unhealthy threshold aught to be
		// long enough to minimize sensitivity to control plane restarts, but short enough that by the time an instance
		// appears offline in the teleport UI, we aught to be able to assume that its unhealthy status has been propagated
		// to its upgrader.
		c.UnhealthyThreshold = 9 * time.Minute

		if fastExport {
			c.UnhealthyThreshold = 9 * time.Second
		}
	}

	if c.ExportInterval == 0 {
		// 40m is fairly arbitrary, but was picked on the basis that we want to keep exports pretty infrequent, but still frequent
		// *enough* that one should be able to be fairly confident of full propagation to all agents within an hour, even assuming
		// some amount of errors/retries.
		c.ExportInterval = 40 * time.Minute

		if fastExport {
			c.ExportInterval = 40 * time.Second
		}
	}

	if c.FirstExport == 0 {
		// note: we add an extra millisecond since FullJitter can sometimes return 0, but our interval helpers interpret FirstDuration=0
		// as meaning that we don't want a custom first duration. this is usually fine, but since the actual export interval is so long,
		// it is important that a shorter first duration is always observed.
		c.FirstExport = time.Millisecond + utils.FullJitter(c.UnhealthyThreshold/2)
	}

	return nil
}

// Exporter is a helper used to export maintenance window schedule values to external upgraders.
type Exporter[C contextLike] struct {
	cfg          ExporterConfig[C]
	closeContext context.Context
	cancel       context.CancelFunc
	retry        retryutils.Retry
}

// NewExporter builds an exporter. Start must be called in order to begin export operations.
func NewExporter[C contextLike](cfg ExporterConfig[C]) (*Exporter[C], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(cfg.UnhealthyThreshold / 16),
		Max:    cfg.UnhealthyThreshold,
		Jitter: retryutils.NewHalfJitter(),
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Exporter[C]{
		cfg:          cfg,
		retry:        retry,
		closeContext: ctx,
		cancel:       cancel,
	}, nil
}

func (e *Exporter[C]) Run() error {
	e.run(e.closeContext)
	return nil
}

func (e *Exporter[C]) Close() error {
	e.cancel()
	return nil
}

func (e *Exporter[C]) event(event testEvent) {
	if e.cfg.testEvents == nil {
		return
	}
	e.cfg.testEvents <- event
}

func (e *Exporter[C]) run(ctx context.Context) {
	exportInterval := interval.New(interval.Config{
		FirstDuration: utils.FullJitter(e.cfg.FirstExport),
		Duration:      e.cfg.ExportInterval,
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer exportInterval.Stop()

Outer:
	for {
		select {
		case sentinel := <-e.cfg.AuthConnectivitySentinel:
			e.event(sentinelAcquired)
			// auth connectivity is healthy, we can now perform
			// periodic export operations.
			for {
				select {
				case <-exportInterval.Next():
					e.exportWithRetry(ctx) // errors handled internally
				case <-sentinel.Done():
					e.event(sentinelLost)
					// auth connectivity has been lost, resume outer loop
					continue Outer
				case <-ctx.Done():
					return
				}
			}
		case <-time.After(e.cfg.UnhealthyThreshold):
			// if we lose connectivity with auth for too long, forcibly reset any existing schedule.
			// this frees up the upgrader to attempt an upgrade at its discretion.
			if err := e.cfg.Driver.ResetSchedule(ctx); err != nil {
				log.Warnf("Failed to perform %q maintenance window reset: %v", e.cfg.Driver.Kind(), err)
			}
			e.event(resetFromRun)
		case <-ctx.Done():
			return
		}
	}
}

// exportWithRetry attempts to get an exported schedule value and sync it with the appropriate upgrader, retrying
// until UnhealthyThreshold is exceeded.
func (e *Exporter[C]) exportWithRetry(ctx context.Context) {
	start := time.Now()
	defer e.retry.Reset()
	for {

		if time.Now().After(start.Add(e.cfg.UnhealthyThreshold)) {
			// failure state appears persistent. reset and yield back
			// to outer loop to wait for our next scheduled attempt.
			if err := e.cfg.Driver.ResetSchedule(ctx); err != nil {
				log.Warnf("Failed to perform %q maintenance window reset: %v", e.cfg.Driver.Kind(), err)
			}
			e.event(resetFromExport)
			e.event(exportFailure)
			return
		}

		// note that we don't bother tracking the state of the auth connectivity sentinel here. while doing
		// so would theoretically be more optimal, doing so in a way that doesn't cause odd behaviors under
		// highly intermittent auth connectivity is *tricky*. the state-machine is much simpler and has a lot
		// less edge-cases if we only care about the sentinel when deciding when to *start* our export attempt.
		select {
		case <-e.retry.After():
		case <-ctx.Done():
			return
		}

		e.event(exportAttempt)

		// ask auth server to export upcoming windows
		rsp, err := e.cfg.ExportFunc(ctx, proto.ExportUpgradeWindowsRequest{
			TeleportVersion: teleport.Version,
			UpgraderKind:    e.cfg.Driver.Kind(),
		})

		if err != nil {
			log.Warnf("Failed to import %q maintenance window from auth: %v", e.cfg.Driver.Kind(), err)
			e.retry.Inc()
			e.event(getExportErr)
			continue
		}

		// sync exported windows out to our upgrader
		if err := e.cfg.Driver.SyncSchedule(ctx, rsp); err != nil {
			log.Warnf("Failed to sync %q maintenance window: %v", e.cfg.Driver.Kind(), err)
			e.retry.Inc()
			e.event(syncExportErr)
			continue
		}

		log.Infof("Successfully synced %q upgrader maintenance window value.", e.cfg.Driver.Kind())
		e.event(exportSuccess)
		return
	}
}
