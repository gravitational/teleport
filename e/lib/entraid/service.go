package entraid

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/entraid/accessgraph"
	"github.com/gravitational/teleport/e/lib/entraid/directory"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// DefaultFullSyncInterval is the interval at which periodic synchronizations of Entra ID data happen.
	DefaultFullSyncInterval = 5 * time.Minute

	// semaphoreName is the name of the semaphore used by the Entra ID sync service.
	semaphoreName = "entra_id_sync"

	// semaphoreExpiration is the expiration duration of the semaphore.
	semaphoreExpiration = time.Minute
)

// SyncIntervals configures Entra ID service sync intervals.
type SyncIntervals struct {
	// Delta is the delta sync interval.
	Delta time.Duration
	// Full is the full sync interval.
	Full time.Duration
}

type directoryReconciler interface {
	Reconcile(ctx context.Context, syncMode mdmsync.SyncMode) (directory.Result, error)
}

type accessGraphSynchronizer interface {
	Run(ctx context.Context) error
}

// Config is a Entra ID service config.
type Config struct {
	Clock            clockwork.Clock
	PluginStatusSink common.StatusSink
	SemaphoreSvc     types.Semaphores
	HostID           string
	Logger           *slog.Logger
	SyncIntervals    *SyncIntervals

	// Sub-components

	DirectoryReconciler     *directory.Reconciler
	AccessGraphSynchronizer *accessgraph.Synchronizer
}

// Validate validates configuration options
func (cfg *Config) Validate() error {
	if cfg.PluginStatusSink == nil {
		return trace.BadParameter("PluginStatusSink must be specified")
	}
	if cfg.SemaphoreSvc == nil {
		return trace.BadParameter("SemaphoreSvc must be specified")
	}
	if cfg.HostID == "" {
		return trace.BadParameter("HostID must be specified")
	}
	if cfg.Logger == nil {
		return trace.BadParameter("Logger must be specified")
	}

	if cfg.DirectoryReconciler == nil {
		return trace.BadParameter("DirectoryReconciler must be specified")
	}

	return nil
}

// SetDefaults sets the default values for the options.
func (cfg *Config) SetDefaults() {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	if cfg.SyncIntervals == nil {
		cfg.SyncIntervals = &SyncIntervals{
			// backfill existing default.
			Full:  DefaultFullSyncInterval,
			Delta: 0,
		}
	}
}

// Service is the implementation of Entra ID service.
type Service struct {
	clock            clockwork.Clock
	log              *slog.Logger
	pluginStatusSink common.StatusSink
	semaphoreSvc     types.Semaphores
	hostID           string

	directoryReconciler     directoryReconciler
	accessGraphSynchronizer accessGraphSynchronizer

	// syncIntervals configures Entra ID sync intervals.
	syncIntervals *mdmsync.Scheduler[*SyncIntervals]
	// deltaSyncEnabled indicates if delta sync is enabled.
	deltaSyncEnabled bool
	// firstSyncCompleted indicates whether the first full sync was
	// completed successfully.
	firstSyncCompleted bool
}

// New returns a new Entra ID service.
func New(cfg Config) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SetDefaults()

	scheduler, err := newScheduler(*cfg.SyncIntervals)
	if err != nil {
		return nil, trace.Wrap(err, "init entra id sync schedule")
	}

	svc := &Service{
		clock:               cfg.Clock,
		log:                 cfg.Logger,
		pluginStatusSink:    cfg.PluginStatusSink,
		semaphoreSvc:        cfg.SemaphoreSvc,
		hostID:              cfg.HostID,
		directoryReconciler: cfg.DirectoryReconciler,
		syncIntervals:       scheduler,
		deltaSyncEnabled:    cfg.SyncIntervals.Delta > 0,
	}

	// Be explicit and assign `accessGraphSynchronizer` only if the config field is non-nil
	// to avoid assigning {*AccessGraphSynchronizer, nil} to the interface type.
	if cfg.AccessGraphSynchronizer != nil {
		svc.accessGraphSynchronizer = cfg.AccessGraphSynchronizer
	}
	return svc, nil
}

// Run runs the service indefinitely (until the context is canceled), retrying if needed.
func (s *Service) Run(ctx context.Context) error {
	for {
		if err := s.runWithLock(ctx); err != nil {
			s.log.ErrorContext(ctx, "Entra ID service failed", "error", err)
		}

		select {
		case <-ctx.Done():
			// If context was canceled, we should stop.
			return trace.Wrap(ctx.Err())
		case <-s.clock.After(semaphoreExpiration):
			// Otherwise, semaphore is likely not available.
		}
	}
}

// runWithLock attempts to acquire a semaphore. If successful, it runs the sub-components.
// Sub-components are expected to have their internal periodic polling / retry logic.
// runWithLock is only expected to return on:
//   - Failing to acquire the semaphore;
//   - Fatal errors;
//   - Or when the context is canceled.
func (s *Service) runWithLock(ctx context.Context) error {
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.semaphoreSvc,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindPlugin,
					SemaphoreName: semaphoreName,
					MaxLeases:     1,
					Holder:        s.hostID,
				},
				Expiry: semaphoreExpiration,
				Clock:  s.clock,
			},
			Retry: retryutils.LinearConfig{
				Clock:  s.clock,
				First:  time.Second,
				Step:   semaphoreExpiration / 2,
				Max:    semaphoreExpiration,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.log.WarnContext(ctx, "Error cleaning up Entra ID service semaphore", "error", err)
		}
	}()

	g, ctx := errgroup.WithContext(lease)
	g.Go(func() error {
		return trace.Wrap(s.runScheduled(ctx))
	})
	g.Go(func() error {
		return trace.Wrap(s.runAccessGraphSync(ctx))
	})
	return trace.Wrap(g.Wait())
}

func newScheduler(syncIntervals SyncIntervals) (*mdmsync.Scheduler[*SyncIntervals], error) {
	// TODO(sshah): add a default delay and let the test customize the duration.
	delayFn := func() time.Duration { return 0 /* start immediately */ }
	return mdmsync.New(
		[]*SyncIntervals{&syncIntervals}, delayFn, func(e *SyncIntervals) mdmsync.EntryInfo {
			return mdmsync.EntryInfo{
				SyncPeriodPartial: syncIntervals.Delta,
				SyncPeriodFull:    syncIntervals.Full,
			}
		})
}

func (s *Service) runScheduled(ctx context.Context) error {
	for {
		offset := s.syncIntervals.NextOffset()
		select {
		case <-s.clock.After(offset):
			start := s.clock.Now()
			e := s.syncIntervals.Next()
			if e.Mode == mdmsync.SyncModePartial && !s.firstSyncCompleted {
				// Force the first sync to be a full sync, regardless of the
				// sync interval settings. This is needed because the delta
				// sync depends on the baseline snapshot of the Entra ID
				// directory created by a successful full sync.
				e.Mode = mdmsync.SyncModeFull
			}
			s.log.InfoContext(ctx, "Starting Entra ID directory sync", "sync_mode", directory.FriendlySyncMode(e.Mode))
			result, err := s.directoryReconciler.Reconcile(ctx, e.Mode)
			if e.Mode == mdmsync.SyncModeFull && !s.deltaSyncEnabled {
				// TODO(sshah): Stop joining sync warnigns and error
				// once the plugin status and UI supports diffrentiating
				// between hard reconciler errors and skipped warnings.
				err = errors.Join(err, result.ErrSkippedResources)
			}
			took := s.clock.Since(start)
			// If delta sync is enabled, but error occurred on first full sync,
			// (e.g. failure to set up latest delta token, transient Teleport backend
			// read/write issues etc) the service will never proceed to delta sync
			// and full sync will run in the delta sync interval.
			// TODO(sshah): maybe fallback to running service on DefaultFullSyncInterval
			// in such case?
			if err != nil {
				s.handleReconcilerError(ctx, err, e.Mode, took, result)
			} else {
				if e.Mode == mdmsync.SyncModeFull && !s.firstSyncCompleted {
					s.firstSyncCompleted = true
				}

				s.log.InfoContext(ctx, "Entra ID directory sync completed",
					"sync_mode", directory.FriendlySyncMode(e.Mode),
					"took", took.String(),
					"imported_users", result.ImportedUsers,
					"imported_groups", result.ImportedGroups,
					"warnings", errString(result.ErrSkippedResources),
				)
			}

			s.emitStatus(ctx, err, result)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Service) handleReconcilerError(ctx context.Context, err error, syncMode mdmsync.SyncMode, took time.Duration, result directory.Result) {
	s.maybeResetSyncSchedule(ctx, err, syncMode)

	if result.ImportedUsers > 0 || result.ImportedGroups > 0 {
		s.log.ErrorContext(
			ctx,
			"Entra ID directory sync completed with partial success",
			"sync_mode", directory.FriendlySyncMode(syncMode),
			"took", took.String(),
			"imported_users", result.ImportedUsers,
			"imported_groups", result.ImportedGroups,
			"error", err,
		)
		return
	}

	s.log.ErrorContext(ctx, "Entra ID directory sync failed", "sync_mode", directory.FriendlySyncMode(syncMode), "took", took.String(), "error", err)
}

func (s *Service) emitStatus(ctx context.Context, err error, result directory.Result) {
	if s.pluginStatusSink == nil {
		s.log.DebugContext(ctx, "Failed to emit Entra ID plugin status, status sink is not available")
		return
	}

	code := types.PluginStatusCode_RUNNING
	friendlyErrMsg := ""
	rawErr := ""
	if err != nil {
		friendlyErrMsg = "Entra directory sync failed"
		if result.ImportedUsers > 0 || result.ImportedGroups > 0 {
			friendlyErrMsg = "Entra directory sync completed with partial success"
		}
		code, rawErr = getErrorDetails(err)
	}

	if err := s.pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
		Code:         code,
		LastSyncTime: s.clock.Now(),
		ErrorMessage: friendlyErrMsg,
		LastRawError: rawErr,
		Details: &types.PluginStatusV1_EntraId{
			EntraId: &types.PluginEntraIDStatusV1{
				ImportedUsers:  uint32(result.ImportedUsers),
				ImportedGroups: uint32(result.ImportedGroups),
			},
		},
	}); err != nil {
		s.log.ErrorContext(ctx, "Failed to emit Entra ID plugin status", "error", err)
		return
	}
}

// runAccessGraphSync runs the access grap synchronizer.
func (s *Service) runAccessGraphSync(ctx context.Context) error {
	if s.accessGraphSynchronizer == nil {
		// Access graph disabled.
		s.log.InfoContext(ctx, "Access graph sync not enabled. Will only synchronize the directory.")
		return nil
	}

	return trace.Wrap(s.accessGraphSynchronizer.Run(ctx))
}

// maybeResetSyncSchedule processes reconciler error and may reset
// the sync schedule based on the known error types. In case of throttled
// error, it downgrades Graph client's goroutine limit to reduce parallel API
// calls to Graph API.
func (s *Service) maybeResetSyncSchedule(ctx context.Context, err error, syncMode mdmsync.SyncMode) {
	if err == nil {
		return
	}
	// Delay with DefaultFullSyncInterval as a recovery backoff.
	// It also resembles older full sync interval.
	delayInterval := DefaultFullSyncInterval

	retryAfter, isThrottled := directory.IsErrGraphAPIThrottled(err)

	switch {
	case isErrDeltaSetup(err) || isErrDeltaAPI(err):
		s.log.DebugContext(ctx, "Failed to complete Entra ID delta sync, next sync will be a full sync", "sync_mode", directory.FriendlySyncMode(syncMode))
	case isThrottled:
		// If this was a delta sync (highly unlikely) sync, and
		// the next sync is also a delta sync, resetting to force full
		// sync may seem counter productive. But in this case, Teleport
		// may have partial data which is better corrected with a full sync.

		// RetryAfter is not guaranteed to be populated.
		// If present, expect it to be greater than DefaultFullSyncInterval
		// because the graph client already retries five times before
		// returning this error. DefaultFullSyncInterval as minimum ensures
		// extra delay cushion.
		if retryAfter > DefaultFullSyncInterval {
			delayInterval = retryAfter
		}
		s.log.DebugContext(ctx, "Entra ID graph API requests are being throttled, next sync will be a full sync", "sync_mode", directory.FriendlySyncMode(syncMode))
	case s.deltaSyncEnabled && syncMode == mdmsync.SyncModeFull:
		// If delta sync is enabled but the full sync fails, delta
		// sync can either entirely fail or proceed with incomplete
		// snapshot of the Entra ID directory and bear unwanted results.
		s.log.DebugContext(ctx, "Failed to complete Entra ID full sync, another full sync will be retried before delta sync", "sync_mode", directory.FriendlySyncMode(syncMode))
	default:
		return
	}

	delayFn := func() time.Duration { return delayInterval }
	if err := s.syncIntervals.Reset(delayFn); err != nil {
		s.log.ErrorContext(ctx, "Failed to reset Entra ID sync schedule", "sync_mode", directory.FriendlySyncMode(syncMode), "error", err)
	}
}

func errString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
