package okta

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
)

// AssignmentReconcilerAccessPoint is a client that consists of only the interfaces
// needed for the AssignmentsReconciler.
type AssignmentReconcilerAccessPoint interface {
	types.Events

	// ListOktaAssignments returns a paginated list of all Okta assignment resources.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
	// GetOktaAssignment returns the specified Okta assignment resources.
	GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error)
	// CreateOktaAssignment creates a new Okta assignment resource.
	CreateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	UpdateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
	// since the last transition.
	UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error
	// DeleteOktaAssignment removes the specified Okta assignment resource.
	DeleteOktaAssignment(ctx context.Context, name string) error
}

// operations, and updates the Okta assignment status afterwards.
type assignmentReconciler struct {
	logger              *slog.Logger
	clock               clockwork.Clock
	accessPoint         AssignmentReconcilerAccessPoint
	watcher             *services.OktaAssignmentWatcher
	assignmentProcessor *assignmentProcessor

	startedCh chan struct{}
	stopCh    chan struct{}
	stopOnce  sync.Once

	// These are used for testing.
	onReconcileCh             chan struct{}
	noAssignmentProcessorLoop bool
}

// newAssignmentReconciler creates a new AssignmentReconciler.
func newAssignmentReconciler(svc *Service) *assignmentReconciler {
	return &assignmentReconciler{
		logger:              slog.With(teleport.ComponentKey, eteleport.ComponentOktaAssignmentReconciler),
		clock:               svc.clock,
		accessPoint:         svc.accessPoint,
		assignmentProcessor: newAssignmentProcessor(svc),
		startedCh:           make(chan struct{}),
		stopCh:              make(chan struct{}),
	}
}

// Start will start the reconciler.
func (a *assignmentReconciler) start(ctx context.Context) error {
	defer close(a.startedCh)
	var err error
	a.watcher, err = services.NewOktaAssignmentWatcher(ctx, services.OktaAssignmentWatcherConfig{
		RWCfg: services.ResourceWatcherConfig{
			Component: eteleport.ComponentOktaAssignmentReconciler,
			Logger:    a.logger,
			Client:    a.accessPoint,
		},
	})
	if err != nil {
		return trace.Wrap(err, "creating Okta assignments watcher")
	}

	go a.reconcileLoop(ctx)

	if !a.noAssignmentProcessorLoop {
		// Start the assignment processor. This will run periodically to retry calls
		// to Okta.
		a.assignmentProcessor.start(ctx)
	}

	return nil
}

func (a *assignmentReconciler) reconcileLoop(ctx context.Context) {
	a.logger.DebugContext(ctx, "Starting watcher-based Okta assignments reconciler")
	defer a.logger.DebugContext(ctx, "Stopped watcher-based Okta assignments reconciler")

	for {
		select {
		case newAssignments := <-a.watcher.CollectorChan():
			a.assignmentProcessor.processWatcherEvent(ctx, newAssignments)
			if a.onReconcileCh != nil {
				a.onReconcileCh <- struct{}{}
			}
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// wait will wait for the reconciler to complete.
func (a *assignmentReconciler) wait(ctx context.Context) {
	select {
	case <-a.stopCh:
	case <-ctx.Done():
	}
}

// stop will stop and close any lingering resources in the assignmentReconciler.
func (a *assignmentReconciler) stop() {
	// Make sure start() returned in case Okta plugin was stopped right after start to avoid
	// races in accessing assignmentReconciler fields.
	select {
	case <-a.startedCh:
	case <-time.After(30 * time.Second):
		slog.ErrorContext(context.Background(), "Timed out waiting for the assignmentReconciler to start. Returning early. Was stop called without calling start? (this is a bug)")
		return
	}

	a.stopOnce.Do(func() {
		close(a.stopCh)
		if a.watcher != nil {
			a.watcher.Close()
		}
		if !a.noAssignmentProcessorLoop && a.assignmentProcessor != nil {
			a.assignmentProcessor.stop()
		}
	})
}
