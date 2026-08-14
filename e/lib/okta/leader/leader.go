package leader

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
)

// errStopLeadership will be returned by the leadership acquisition functions when
// the Okta service has been stopped.
var errStopLeadership = errors.New("stop leadership acquisition")

// Leader is the leader service that will acquire leadership and periodically refresh it.
type Leader struct {
	Config
	leadershipAcquired     atomic.Bool
	slog                   *slog.Logger
	leadershipRenewRetries atomic.Int32
	stopCh                 chan struct{}
}

// New creates a new Okta leader service.
func New(cfg Config) (*Leader, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Leader{
		Config: cfg,
		slog:   slog.With(teleport.ComponentKey, "okta:leader"),
		stopCh: make(chan struct{}),
	}, nil
}

// Start will start the leader service.
func (s *Leader) Start(ctx context.Context) {
	go s.becomeLeader(ctx)
}

// Close will stop the leader service.
func (s *Leader) Close() error {
	close(s.stopCh)
	return nil
}

// IsLeader will return true if the Okta service has acquired leadership.
func (s *Leader) IsLeader() bool {
	return s.leadershipAcquired.Load()
}

// BecomeLeader will repeatedly try to acquire and renew a semaphore scoped to the Okta organization
// managed by this service. Once this semaphore is acquired, this Okta service will be allowed to issue
// API calls and process assignments. This will prevent multiple Okta services from managing the same Okta
// organization, as the Okta API rate limits are pretty severe.
func (s *Leader) becomeLeader(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		lease, err := s.acquireSemaphore(ctx)
		if errors.Is(err, errStopLeadership) {
			return
		} else if err != nil {
			s.slog.DebugContext(ctx, "Error acquiring semaphore", "error", err)
			continue
		}

		s.leadershipAcquired.Store(true)

		s.slog.DebugContext(ctx, "Semaphore acquired, going into renew loop")

		err = s.renewSemaphoreLease(ctx, lease)
		s.leadershipAcquired.Store(false)

		if errors.Is(err, errStopLeadership) {
			return
		} else if !trace.IsLimitExceeded(err) {
			s.slog.DebugContext(ctx, "Error renewing lease", "error", err)
		}
	}
}

// acquireSemaphore will acquire the semaphore for this Okta service and org.
func (s *Leader) acquireSemaphore(ctx context.Context) (*types.SemaphoreLease, error) {
	ticker := s.Clock.NewTicker(semaphoreRenewal)
	defer ticker.Stop()
	for {
		s.slog.DebugContext(ctx, "Attempting to acquire semaphore before starting.")
		lease, err := s.Semaphores.AcquireSemaphore(ctx, types.AcquireSemaphoreRequest{
			SemaphoreKind: s.SemaphoreKind,
			SemaphoreName: s.SemaphoreName,
			MaxLeases:     1,
			Expires:       s.Clock.Now().Add(semaphoreExpiration),
			Holder:        s.HostIDHolder,
		})
		if err == nil {
			return lease, nil
		}

		s.slog.DebugContext(ctx, "Unable to get semaphore (%s), seeing if this host already has a lease", "error", err.Error())
		semaphores, err := s.Semaphores.GetSemaphores(ctx, types.SemaphoreFilter{
			SemaphoreKind: s.SemaphoreKind,
			SemaphoreName: s.SemaphoreName,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Look through the existing leases to see if the holder for the existing lease is the same as
		// the current host. We're expected to only have 1 lease per semaphore, so if the holder of that
		// lease is the same as this host, we can be certain that the lease actually belongs to this host.
		for _, semaphore := range semaphores {
			leases := semaphore.LeaseRefs()
			for _, lease := range leases {
				if lease.Holder != s.HostIDHolder {
					continue
				}

				s.slog.DebugContext(ctx, "Lease found for this host")
				// This lease belongs to this host, so we'll return this lease.
				return &types.SemaphoreLease{
					SemaphoreKind: s.SemaphoreKind,
					SemaphoreName: s.SemaphoreName,
					LeaseID:       lease.LeaseID,
					Expires:       lease.Expires,
				}, nil
			}
		}

		s.slog.DebugContext(ctx, "Unable to acquire semaphore, retrying", "semaphore_renewal", semaphoreRenewal.String())

		select {
		case <-s.stopCh:
			return nil, errStopLeadership
		case <-ctx.Done():
			return nil, errStopLeadership
		case <-ticker.Chan():
		}
	}
}

// renewSemaphoreLease will repeatedly attempt to renew the semaphore lease.
func (s *Leader) renewSemaphoreLease(ctx context.Context, lease *types.SemaphoreLease) error {
	// Set up a function to renew the lease regularly.
	ticker := s.Clock.NewTicker(semaphoreRenewal)
	defer ticker.Stop()

	// Reset renew retries.
	s.leadershipRenewRetries.Store(0)

	for {
		select {
		case <-s.stopCh:
			return errStopLeadership
		case <-ctx.Done():
			return errStopLeadership
		case <-ticker.Chan():
		}

		lease.Expires = s.Clock.Now().Add(semaphoreExpiration)
		if err := s.Semaphores.KeepAliveSemaphoreLease(ctx, *lease); err != nil {
			retryCount := s.leadershipRenewRetries.Add(1)
			if retryCount >= semaphoreRenewalMaxRetries {
				return trace.LimitExceeded("max semaphore renew attempts reached, service will stop processing")
			} else {
				s.slog.WarnContext(ctx, "Error renewing semaphore lease, will retry ",
					"semaphore_renewal", semaphoreRenewal.String(),
					"retry_count", retryCount,
					"err", err,
				)
			}
		} else {
			s.leadershipRenewRetries.Store(0)
		}
	}
}
