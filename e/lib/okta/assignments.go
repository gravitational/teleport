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
	"github.com/gravitational/teleport/lib/utils"
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
	clusterName         string
	accessPoint         AssignmentReconcilerAccessPoint
	watcher             *services.OktaAssignmentWatcher
	assignmentProcessor *assignmentProcessor

	// assignmentProcessorID is not thread safe and it's supposed to be used by
	// onCreate/onUpdate/onDelete methods for the underlying generic reconciler.
	assignmentProcessorID string

	reconcileCh chan struct{}

	startedCh chan struct{}
	stopCh    chan struct{}
	stopOnce  sync.Once

	assignmentsMu sync.RWMutex
	assignments   map[string]types.OktaAssignment

	newAssignmentsMu sync.RWMutex
	newAssignments   map[string]types.OktaAssignment

	// These are used for testing.
	onReconcile               func(types.OktaAssignments)
	onReconcileCh             chan struct{}
	noAssignmentProcessorLoop bool
}

// newAssignmentReconciler creates a new AssignmentReconciler.
func newAssignmentReconciler(clusterName string, svc *Service) *assignmentReconciler {
	a := &assignmentReconciler{
		logger:                slog.With(teleport.ComponentKey, eteleport.ComponentOktaAssignmentReconciler),
		clock:                 svc.clock,
		clusterName:           clusterName,
		accessPoint:           svc.accessPoint,
		reconcileCh:           make(chan struct{}),
		assignmentProcessorID: "unset_id",
		startedCh:             make(chan struct{}),
		stopCh:                make(chan struct{}),
		assignments:           make(map[string]types.OktaAssignment),
		newAssignments:        make(map[string]types.OktaAssignment),
	}

	a.assignmentProcessor = newAssignmentProcessor(svc, a.getAssignments)

	return a
}

// Start will start the reconciler.
func (a *assignmentReconciler) start(ctx context.Context) error {
	defer close(a.startedCh)

	reconciler, err := services.NewReconciler(services.ReconcilerConfig[types.OktaAssignment]{
		Matcher: func(assignment types.OktaAssignment) bool {
			return a.matcher(ctx, assignment)
		},
		GetCurrentResources: toResourcesLabelMap(a.getAssignments),
		GetNewResources:     toResourcesLabelMap(a.getNewAssignments),
		OnCreate:            a.onCreate,
		OnUpdate:            a.onUpdate,
		OnDelete:            a.onDelete,
		Logger:              a.logger.With("kind", types.KindOktaAssignment),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	watcher, err := a.startResourceWatcher(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	a.watcher = watcher

	go a.reconcile(ctx, reconciler)

	if !a.noAssignmentProcessorLoop {
		// Start the assignment processor. This will run periodically to retry calls
		// to Okta.
		a.assignmentProcessor.start(ctx)
	}

	return nil
}

// reconcile will perform the actual assignment reconciliation.
func (a *assignmentReconciler) reconcile(ctx context.Context, reconciler *services.Reconciler[types.OktaAssignment]) {
	for {
		select {
		case _, ok := <-a.reconcileCh:
			if !ok {
				return
			}
			a.assignmentProcessorID = newLoopID(sourceWatcher, a.clock.Now())
			if err := reconciler.Reconcile(ctx); err != nil {
				a.logger.ErrorContext(ctx, "Failed to reconcile", "error", err)
			} else if a.onReconcile != nil {
				a.assignmentsMu.RLock()
				a.onReconcile(copyAssignmentsMapToOktaAssignments(a.assignments))
				a.assignmentsMu.RUnlock()
			}
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

// Wait will wait for the reconciler to complete.
func (a *assignmentReconciler) wait(ctx context.Context) {
	select {
	case <-a.stopCh:
	case <-ctx.Done():
	}
}

// Stop will stop and close any lingering resources in the assignmentReconciler.
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

// getAssignments returns the list of assignments currently known to the reconciler.
func (a *assignmentReconciler) getAssignments() types.OktaAssignments {
	a.assignmentsMu.RLock()
	defer a.assignmentsMu.RUnlock()

	return copyAssignmentsMapToOktaAssignments(a.assignments)
}

// getNewAssignments returns the list of new assignments that the reconciler has yet to act on.
func (a *assignmentReconciler) getNewAssignments() types.OktaAssignments {
	a.newAssignmentsMu.RLock()
	defer a.newAssignmentsMu.RUnlock()

	return copyAssignmentsMapToOktaAssignments(a.newAssignments)
}

// startResourceWatcher starts watching changes to assignment resources.
func (a *assignmentReconciler) startResourceWatcher(ctx context.Context) (*services.OktaAssignmentWatcher, error) {
	a.logger.DebugContext(ctx, "Initializing assignment resource watcher")
	watcher, err := services.NewOktaAssignmentWatcher(ctx, services.OktaAssignmentWatcherConfig{
		RWCfg: services.ResourceWatcherConfig{
			Component: eteleport.ComponentOktaAssignmentReconciler,
			Logger:    a.logger,
			Client:    a.accessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer func() {
			a.logger.DebugContext(ctx, "Access request resource watcher finished")
		}()

		for {
			select {
			case newAssignments := <-watcher.CollectorChan():
				a.newAssignmentsMu.Lock()
				a.newAssignments = map[string]types.OktaAssignment{}
				for _, newAssignment := range newAssignments {
					a.newAssignments[newAssignment.GetName()] = newAssignment
				}
				a.newAssignmentsMu.Unlock()

				select {
				case a.reconcileCh <- struct{}{}:
				case <-a.stopCh:
					return
				case <-ctx.Done():
					return
				}
			case <-a.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return watcher, nil
}

// onCreate will update the Okta API based on newly created Okta assignments.
func (a *assignmentReconciler) onCreate(ctx context.Context, newAssignment types.OktaAssignment) error {
	if r := a.assignmentProcessor.processAssignment(ctx, a.assignmentProcessorID, newAssignment.Copy(), sourceWatcher); r != processAssignmentFailed {
		a.assignmentsMu.Lock()
		a.assignments[newAssignment.GetName()] = newAssignment
		a.assignmentsMu.Unlock()
	}
	return nil
}

// onUpdate will perform necessary Okta assignment operations based on updated Okta assignments.
func (a *assignmentReconciler) onUpdate(ctx context.Context, updatedAssignment, _ types.OktaAssignment) error {
	if r := a.assignmentProcessor.processAssignment(ctx, a.assignmentProcessorID, updatedAssignment.Copy(), sourceWatcher); r != processAssignmentFailed {
		a.assignmentsMu.Lock()
		a.assignments[updatedAssignment.GetName()] = updatedAssignment
		a.assignmentsMu.Unlock()
	}
	return nil
}

// onDelete will perform necessary Okta assignment operations based on deleted Okta assignments.
// NOTE: This should never actually be run as users shouldn't be deleting OktaAssignment objects.
func (a *assignmentReconciler) onDelete(ctx context.Context, deletedAssignment types.OktaAssignment) error {
	a.assignmentsMu.Lock()
	defer a.assignmentsMu.Unlock()
	// No call a.assignmentProcessor.processAssignment here, because the assignment is already
	// deleted form the backend. Assignments are deleted by setting spec.cleanup_time.  When
	// cleanup_time is in the past then the assignmentProcessor will clean the assignment and
	// delete it from the backend.
	// On top of that, when the assignment is not in the backend anymore then it can't be
	// processed because the assignmentProcessor will fail when trying to set its status to
	// "processing".
	delete(a.assignments, deletedAssignment.GetName())
	return nil
}

// matcher will match all Okta assignments.
func (a *assignmentReconciler) matcher(_ context.Context, _ types.ResourceWithLabels) bool {
	return true
}

// toResourcesLabelMap is used by the reconciler. It will call a function that returns OktaAssignments
// and then convert those into a ResourcesWithLabelMap.
func toResourcesLabelMap(fn func() types.OktaAssignments) func() map[string]types.OktaAssignment {
	return func() map[string]types.OktaAssignment {
		return utils.FromSlice(fn(), types.OktaAssignment.GetName)
	}
}

func copyAssignmentsMapToOktaAssignments(assignments map[string]types.OktaAssignment) types.OktaAssignments {
	assignmentsCopy := types.OktaAssignments(make([]types.OktaAssignment, 0, len(assignments)))
	for _, assignment := range assignments {
		assignmentsCopy = append(assignmentsCopy, assignment.Copy())
	}

	return assignmentsCopy
}
