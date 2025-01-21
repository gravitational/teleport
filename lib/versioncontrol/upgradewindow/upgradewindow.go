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
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/kubernetes"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/teleport/lib/versioncontrol"
)

const (
	// kubeSchedKey is the key under which the kube controller schedule is exported
	kubeSchedKey = "agent-maintenance-schedule"

	// unitScheduleFile is the name of the file to which the unit schedule is exported.
	unitScheduleFile = "schedule"

	// scheduleNop is the name of the no-op schedule.
	scheduleNop = "nop"
)

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
		// 9m is fairly arbitrary, but was picked based on the idea that a good unhealthy threshold aught to be
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
		c.FirstExport = time.Millisecond + retryutils.FullJitter(c.UnhealthyThreshold/2)
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
		Jitter: retryutils.HalfJitter,
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
		FirstDuration: retryutils.FullJitter(e.cfg.FirstExport),
		Duration:      e.cfg.ExportInterval,
		Jitter:        retryutils.SeventhJitter,
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
			if err := e.cfg.Driver.Reset(ctx); err != nil {
				slog.WarnContext(ctx, "Failed to perform maintenance window reset", "upgrader_kind", e.cfg.Driver.Kind(), "error", err)
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
			if err := e.cfg.Driver.Reset(ctx); err != nil {
				slog.WarnContext(ctx, "Failed to perform maintenance window reset", "upgrader_kind", e.cfg.Driver.Kind(), "error", err)
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
			slog.WarnContext(ctx, "Failed to import maintenance window from auth", "upgrader_kind", e.cfg.Driver.Kind(), "error", err)
			e.retry.Inc()
			e.event(getExportErr)
			continue
		}

		// sync exported windows out to our upgrader
		if err := e.cfg.Driver.Sync(ctx, rsp); err != nil {
			slog.WarnContext(ctx, "Failed to sync %q maintenance window", "upgrader_kind", e.cfg.Driver.Kind(), "error", err)
			e.retry.Inc()
			e.event(syncExportErr)
			continue
		}

		slog.InfoContext(ctx, "Successfully synced upgrader maintenance window value", "upgrader_kind", e.cfg.Driver.Kind())
		e.event(exportSuccess)
		return
	}
}

// Driver represents a type capable of exporting the maintenance window schedule to an external
// upgrader, such as the teleport-upgrade systemd timer or the kube-updater controller.
type Driver interface {
	// Kind gets the upgrader kind associated with this export driver.
	Kind() string

	// Sync exports the appropriate maintenance window schedule if one is present, or
	// resets/clears the maintenance window if the schedule response returns no viable scheduling
	// info.
	Sync(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error

	// Reset forcibly clears any previously exported maintenance window values. This should be
	// called if teleport experiences prolonged loss of auth connectivity, which may be an indicator
	// that the control plane has been upgraded s.t. this agent is no longer compatible.
	Reset(ctx context.Context) error

	// ForceNop sets the NOP schedule, ensuring that updates do not happen.
	// This schedule was originally only used for testing, but now it is also used by the
	// teleport-update binary to protect against package updates that could interfere with
	// the new update system.
	ForceNop(ctx context.Context) error
}

// NewDriver sets up a new export driver corresponding to the specified upgrader kind.
func NewDriver(kind string) (Driver, error) {
	switch kind {
	case types.UpgraderKindKubeController:
		return NewKubeControllerDriver(KubeControllerDriverConfig{})
	case types.UpgraderKindSystemdUnit:
		return NewSystemdUnitDriver(SystemdUnitDriverConfig{})
	default:
		return nil, trace.BadParameter("unsupported upgrader kind: %q", kind)
	}
}

type KubeControllerDriverConfig struct {
	// Backend is an optional backend. Must be an instance of the kuberenets shared-state backend
	// if not nil.
	Backend KubernetesBackend
}

// KubernetesBackend interface for kube shared storage backend.
type KubernetesBackend interface {
	// Put puts value into backend (creates if it does not
	// exists, updates it otherwise)
	Put(ctx context.Context, i backend.Item) (*backend.Lease, error)
}

type kubeDriver struct {
	cfg KubeControllerDriverConfig
}

func NewKubeControllerDriver(cfg KubeControllerDriverConfig) (Driver, error) {
	if cfg.Backend == nil {
		var err error
		cfg.Backend, err = kubernetes.NewShared()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &kubeDriver{cfg: cfg}, nil
}

func (e *kubeDriver) Kind() string {
	return types.UpgraderKindKubeController
}

func (e *kubeDriver) Sync(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error {
	return trace.Wrap(e.setSchedule(ctx, rsp.KubeControllerSchedule))
}

func (e *kubeDriver) ForceNop(ctx context.Context) error {
	return trace.Wrap(e.setSchedule(ctx, scheduleNop))
}

func (e *kubeDriver) setSchedule(ctx context.Context, schedule string) error {
	if schedule == "" {
		return e.Reset(ctx)
	}

	_, err := e.cfg.Backend.Put(ctx, backend.Item{
		// backend.KeyFromString is intentionally used here instead of backend.NewKey
		// because existing backend items were persisted without the leading /.
		Key:   backend.KeyFromString(kubeSchedKey),
		Value: []byte(schedule),
	})

	return trace.Wrap(err)
}

func (e *kubeDriver) Reset(ctx context.Context) error {
	// kube backend doesn't support deletes right now, so just set
	// the key to empty.
	_, err := e.cfg.Backend.Put(ctx, backend.Item{
		// backend.KeyFromString is intentionally used here instead of backend.NewKey
		// because existing backend items were persisted without the leading /.
		Key:   backend.KeyFromString(kubeSchedKey),
		Value: []byte{},
	})

	return trace.Wrap(err)
}

type SystemdUnitDriverConfig struct {
	// ConfigDir is the directory from which the teleport-upgrade periodic loads its
	// configuration parameters. Most notably, the 'schedule' file.
	ConfigDir string
}

type systemdDriver struct {
	cfg SystemdUnitDriverConfig
}

func NewSystemdUnitDriver(cfg SystemdUnitDriverConfig) (Driver, error) {
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = versioncontrol.UnitConfigDir
	}

	return &systemdDriver{cfg: cfg}, nil
}

func (e *systemdDriver) Kind() string {
	return types.UpgraderKindSystemdUnit
}

func (e *systemdDriver) Sync(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error {
	return trace.Wrap(e.setSchedule(ctx, rsp.SystemdUnitSchedule))
}

func (e *systemdDriver) ForceNop(ctx context.Context) error {
	return trace.Wrap(e.setSchedule(ctx, scheduleNop))
}

func (e *systemdDriver) setSchedule(ctx context.Context, schedule string) error {
	if len(schedule) == 0 {
		// treat an empty schedule value as equivalent to a reset
		return e.Reset(ctx)
	}

	// ensure config dir exists. if created it is set to 755, which is reasonably safe and seems to
	// be the standard choice for config dirs like this in /etc/.
	if err := os.MkdirAll(e.cfg.ConfigDir, defaults.DirectoryPermissions); err != nil {
		return trace.Wrap(err)
	}

	// export schedule file. if created it is set to 644, which is reasonable for a sensitive but non-secret config value.
	if err := os.WriteFile(e.scheduleFile(), []byte(schedule), defaults.FilePermissions); err != nil {
		return trace.Errorf("failed to write schedule file: %v", err)
	}

	return nil
}

func (e *systemdDriver) Reset(_ context.Context) error {
	if _, err := os.Stat(e.scheduleFile()); os.IsNotExist(err) {
		return nil
	}

	// note that we blank the file rather than deleting it, this is intended to allow us to
	// preserve custom file permissions, such as those that might be used in a scenario where
	// teleport is operating with limited privileges.
	if err := os.WriteFile(e.scheduleFile(), []byte{}, teleport.FileMaskOwnerOnly); err != nil {
		return trace.Errorf("failed to reset schedule file: %v", err)
	}

	return nil
}

func (e *systemdDriver) scheduleFile() string {
	return filepath.Join(e.cfg.ConfigDir, unitScheduleFile)
}
