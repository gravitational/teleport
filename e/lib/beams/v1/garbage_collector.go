package beamsv1

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	beamsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	gcSemaphoreName             = "auth.beams.gc"
	gcSemaphoreTTL              = 5 * time.Minute
	gcDefaultCollectionInterval = 1 * time.Minute
	gcTimeout                   = 10 * time.Second

	// These limits are modest because we don't expect there to be more than a
	// couple of thousand beams per-tenant at the moment, and we want to limit
	// impact on the storage backend and k8s.
	gcDefaultParallelism               = 2
	gcDefaultRateLimitBurst            = 20 // per second
	gcDefaultRateLimit      rate.Limit = 10 // per second
)

// GarbageCollector cleans up expired beams periodically. We do this manually
// rather than relying on the usual resource `metadata.expiry` mechanism because
// we're also responsible for tearing down the beam's provisioned compute.
//
// It's also responsible for cleaning up orphaned beam records when CreateBeam
// partially-fails.
//
// We use a semaphore to ensure there is only a single instance of the garbage
// collector running per-cluster at any time.
type GarbageCollector struct {
	cfg         GarbageCollectorConfig
	rateLimiter *rate.Limiter
}

// NewGarbageCollector creates a new beam garbage collector.
func NewGarbageCollector(cfg GarbageCollectorConfig) (*GarbageCollector, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &GarbageCollector{
		cfg:         cfg,
		rateLimiter: rate.NewLimiter(cfg.RateLimit, cfg.RateLimitBurst),
	}, nil
}

// GarbageCollectorConfig configures the beam garbage collector.
type GarbageCollectorConfig struct {
	// Cache iterates beams from the auth cache.
	Cache services.BeamReader

	// Backend bypasses the cache and reads the current beam state directly from
	// the backend.
	Backend services.BeamReader

	// BeamService performs compute teardown and local resource deletion.
	BeamService *BeamsService

	// Semaphores provides leadership locking across auth servers.
	Semaphores types.Semaphores

	// HostID uniquely identifies this auth server.
	HostID string

	// ScanInterval controls how often expired beams are processed.
	ScanInterval time.Duration

	// Parallelism is the maximum number of cleanups that may happen in-parallel.
	Parallelism int

	// RateLimit is the rate at which beams will be collected.
	RateLimit rate.Limit

	// RateLimitBurst is the maximum burst of collections that may be processed
	// at once.
	RateLimitBurst int

	// UsageReporter is used to emit usage events.
	UsageReporter usagereporter.UsageReporter

	// Logger emits collector logs.
	Logger *slog.Logger
}

// CheckAndSetDefaults validates the config and fills optional values.
func (c *GarbageCollectorConfig) CheckAndSetDefaults() error {
	switch {
	case c.Cache == nil:
		return trace.BadParameter("Cache is required")
	case c.Backend == nil:
		return trace.BadParameter("Backend is required")
	case c.BeamService == nil:
		return trace.BadParameter("BeamService is required")
	case c.Semaphores == nil:
		return trace.BadParameter("Semaphores is required")
	case c.HostID == "":
		return trace.BadParameter("HostID is required")
	case c.UsageReporter == nil:
		return trace.BadParameter("UsageReporter is required")
	case c.Logger == nil:
		return trace.BadParameter("Logger is required")
	}

	if c.ScanInterval <= 0 {
		c.ScanInterval = gcDefaultCollectionInterval
	}
	if c.Parallelism == 0 {
		c.Parallelism = gcDefaultParallelism
	}
	if c.RateLimit == 0 {
		c.RateLimit = rate.Limit(gcDefaultRateLimit)
	}
	if c.RateLimitBurst == 0 {
		c.RateLimitBurst = gcDefaultRateLimitBurst
	}
	if rate.Limit(c.RateLimitBurst) < c.RateLimit {
		return trace.BadParameter("RateLimitBurst cannot be smaller than RateLimit")
	}

	return nil
}

// Run the garbage collector, at the configured interval, until the given context
// is canceled or reaches its deadline.
func (g *GarbageCollector) Run(ctx context.Context) {
	for {
		if err := g.runWithLock(ctx); err != nil && !errors.Is(err, context.Canceled) {
			if trace.IsLimitExceeded(err) {
				g.cfg.Logger.DebugContext(ctx, "Beam garbage collector is already running", "error", err)
			} else {
				g.cfg.Logger.ErrorContext(ctx, "Beam garbage collector failed", "error", err)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(gcSemaphoreTTL):
		}
	}
}

func (g *GarbageCollector) runWithLock(ctx context.Context) error {
	lease, err := services.AcquireSemaphoreLock(
		ctx,
		services.SemaphoreLockConfig{
			Service: g.cfg.Semaphores,
			Params: types.AcquireSemaphoreRequest{
				SemaphoreKind: types.KindBeam,
				SemaphoreName: gcSemaphoreName,
				MaxLeases:     1,
				Holder:        g.cfg.HostID,
			},
			Expiry: gcSemaphoreTTL,
			Clock:  clockwork.NewRealClock(),
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(lease)
	defer cancel()
	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			g.cfg.Logger.WarnContext(ctx, "Error cleaning up beam GC semaphore", "error", err)
		}
	}()

	ticker := interval.New(interval.Config{
		Clock:         clockwork.NewRealClock(),
		Duration:      g.cfg.ScanInterval,
		FirstDuration: retryutils.FullJitter(g.cfg.ScanInterval),
		Jitter:        retryutils.SeventhJitter,
	})
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.Next():
			if _, err := g.processExpiredBeams(ctx); err != nil {
				g.cfg.Logger.ErrorContext(ctx, "Failed processing expired beams", "error", err)
			}
		}
	}
}

func (g *GarbageCollector) processExpiredBeams(ctx context.Context) (int, error) {
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(g.cfg.Parallelism)

	var deleted atomic.Int32
	for beam, err := range g.cfg.Cache.IterateBeams(groupCtx, "") {
		if err != nil {
			_ = group.Wait()
			return 0, trace.Wrap(err)
		}
		if time.Now().Before(beam.GetSpec().GetExpires().AsTime()) {
			continue
		}
		group.Go(func() error {
			if err := g.rateLimiter.Wait(groupCtx); err != nil {
				return trace.Wrap(err)
			}
			if err := g.deleteExpiredBeam(groupCtx, beam); err == nil {
				_ = deleted.Add(1)
			} else {
				g.cfg.Logger.ErrorContext(ctx, "Failed to delete expired beam", "beam_id", beam.GetMetadata().GetName(), "error", err)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return 0, trace.Wrap(err)
	}

	count := int(deleted.Load())
	if count > 0 {
		g.cfg.Logger.DebugContext(ctx, "Deleted batch of expired beams", "count", count)
	}
	return count, nil
}

func (g *GarbageCollector) deleteExpiredBeam(ctx context.Context, beam *beamsv1pb.Beam) error {
	ctx, cancel := context.WithTimeout(ctx, gcTimeout)
	defer cancel()

	if err := g.cfg.BeamService.destroyBeamCompute(ctx, beam); err != nil {
		return trace.Wrap(err)
	}

	beamID := beam.GetMetadata().GetName()
	err := g.cfg.BeamService.deleteBeam(ctx, beam)
	switch {
	case err == nil:
		g.cfg.UsageReporter.AnonymizeAndSubmit(&usagereporter.BeamsDestroyedEvent{
			BeamId: beamID,
			Reason: prehogv1a.BeamDestroyReason_BEAM_DESTROY_REASON_GC_EXPIRED,
			Region: beam.GetStatus().GetRegion(),
		})
		return nil
	case !errors.Is(err, backend.ErrConditionFailed):
		return trace.Wrap(err)
	}

	// Beam has been updated and the cache is stale. Re-read the beam from the
	// storage backend to get the latest revision, and try again. Do this rather
	// than unconditionally deleting the beam in case the beam has been published
	// and there's a new app resource to delete.
	beam, err = g.cfg.Backend.GetBeam(ctx, beamID)
	switch {
	case trace.IsNotFound(err):
		return nil
	case err != nil:
		return trace.Wrap(err)
	}
	if err := g.cfg.BeamService.deleteBeam(ctx, beam); err != nil {
		return trace.Wrap(err)
	}
	g.cfg.UsageReporter.AnonymizeAndSubmit(&usagereporter.BeamsDestroyedEvent{
		BeamId: beamID,
		Reason: prehogv1a.BeamDestroyReason_BEAM_DESTROY_REASON_GC_EXPIRED,
		Region: beam.GetStatus().GetRegion(),
	})
	return nil
}
