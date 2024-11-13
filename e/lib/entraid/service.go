package entraid

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// syncInterval is the interval at which periodic synchronizations of Entra ID data happen.
	syncInterval = 5 * time.Minute

	// semaphoreName is the name of the semaphore used by the Entra ID sync service.
	semaphoreName = "entra_id_sync"

	// semaphoreExpiration is the expiration duration of the semaphore.
	semaphoreExpiration = time.Minute
)

type directoryReconciler interface {
	Reconcile(ctx context.Context) error
	ImportedUsers() int
	ImportedGroups() int
}

type accessGraphSynchronizer interface {
	Run(ctx context.Context) error
}

type ServiceConfig struct {
	Clock            clockwork.Clock
	PluginStatusSink common.StatusSink
	SemaphoreSvc     types.Semaphores
	HostID           string
	Logger           *slog.Logger

	// Sub-components

	DirectoryReconciler     *DirectoryReconciler
	AccessGraphSynchronizer *AccessGraphSynchronizer
}

// SetDefaults validates configuration options
func (cfg *ServiceConfig) Validate() error {
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
func (cfg *ServiceConfig) SetDefaults() {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
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
}

func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.SetDefaults()

	svc := &Service{
		clock:               cfg.Clock,
		log:                 cfg.Logger,
		pluginStatusSink:    cfg.PluginStatusSink,
		semaphoreSvc:        cfg.SemaphoreSvc,
		hostID:              cfg.HostID,
		directoryReconciler: cfg.DirectoryReconciler,
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
		err := s.runWithLock(ctx)
		s.log.ErrorContext(ctx, "Entra ID service failed", "error", err)

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
			s.log.WarnContext(ctx, "error cleaning up semaphore", "error", err)
		}
	}()

	g, ctx := errgroup.WithContext(lease)
	g.Go(func() error {
		return trace.Wrap(s.runDirectoryReconciler(ctx))
	})
	g.Go(func() error {
		return trace.Wrap(s.runAccessGraphSync(ctx))
	})
	return trace.Wrap(g.Wait())
}

// runDirectoryReconciler periodically runs the directory reconciler until the context is canceled.
func (s *Service) runDirectoryReconciler(ctx context.Context) error {
	ticker := s.clock.NewTicker(syncInterval)
	defer ticker.Stop()
	for {
		err := s.directoryReconciler.Reconcile(ctx)
		if err != nil {
			s.log.ErrorContext(ctx, "Entra directory reconciler failed.", "error", err)
		}

		code, msg := getErrorDetails(err)
		if s.pluginStatusSink != nil {
			s.pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
				Code:         code,
				LastSyncTime: s.clock.Now(),
				ErrorMessage: msg,
				Details: &types.PluginStatusV1_EntraId{
					EntraId: &types.PluginEntraIDStatusV1{
						ImportedUsers:  uint32(s.directoryReconciler.ImportedUsers()),
						ImportedGroups: uint32(s.directoryReconciler.ImportedGroups()),
					},
				},
			})
		}
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.Chan():
		}
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
