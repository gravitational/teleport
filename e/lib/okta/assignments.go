package okta

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
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
	assignmentProcessor *assignmentProcessor

	startedCh chan struct{}
	stopCh    chan struct{}
	stopOnce  sync.Once

	// These are used for testing.
	testOnWatchEventCh            chan struct{}
	testNoAssignmentProcessorLoop bool
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

	go a.runWatcher(ctx)

	if !a.testNoAssignmentProcessorLoop {
		// Start the assignment processor. This will run periodically to retry calls
		// to Okta.
		a.assignmentProcessor.start(ctx)
	}

	return nil
}

// runWatcher makes sure to run OktaAssignment watcher at all times.
func (a *assignmentReconciler) runWatcher(ctx context.Context) {
	a.logger.DebugContext(ctx, "Starting watcher-based Okta assignments reconciler")
	defer a.logger.DebugContext(ctx, "Stopped watcher-based Okta assignments reconciler")

	const cooldownPeriod = 5 * time.Second

	for {
		a.watch(ctx)
		if ctx.Err() != nil {
			return
		}

		select {
		case <-a.clock.After(cooldownPeriod):
			continue
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// watch watches for OktaAssignment events and dispatches them for processing if needed.
func (a *assignmentReconciler) watch(ctx context.Context) {
	watcher, err := a.accessPoint.NewWatcher(ctx, types.Watch{
		Name: "okta-assignment-watcher",
		Kinds: []types.WatchKind{
			{Kind: types.KindOktaAssignment},
		},
	})
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to create Okta assignments watcher. Will retry", "error", err)
		return
	}
	defer watcher.Close()

	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			a.logger.ErrorContext(ctx, "Unexpected watcher event type. Want OpInit. Will retry", "unexpected_event_type", event.Type.String())
			return
		}
		if a.testOnWatchEventCh != nil {
			a.testOnWatchEventCh <- struct{}{}
		}
	case <-a.stopCh:
		return
	case <-watcher.Done():
		if ctx.Err() == nil {
			a.logger.ErrorContext(ctx, "Unexpected Okta assignments watcher error. Will retry", "error", watcher.Error())
		}
		return
	case <-ctx.Done():
		return
	}

	for {
		select {
		case event := <-watcher.Events():
			if event.Type != types.OpPut {
				continue
			}
			assignment, ok := event.Resource.(types.OktaAssignment)
			if !ok {
				a.logger.ErrorContext(ctx, "Unexpected Okta assignments watcher event resource type. Want OktaAssignment. (this is a bug)", "unexpected_event_resource_type", fmt.Sprintf("%T", event.Resource))
				continue
			}

			if assignmentNeedsUrgentProcessing(assignment, a.clock.Now()) {
				a.assignmentProcessor.processWatcherEvent(ctx, assignment)
				if a.testOnWatchEventCh != nil {
					a.testOnWatchEventCh <- struct{}{}
				}
			}
		case <-a.stopCh:
			return
		case <-watcher.Done():
			if ctx.Err() == nil {
				a.logger.ErrorContext(ctx, "Unexpected Okta assignments watcher error. Will retry", "error", watcher.Error())
			}
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
		if !a.testNoAssignmentProcessorLoop && a.assignmentProcessor != nil {
			a.assignmentProcessor.stop()
		}
	})
}
