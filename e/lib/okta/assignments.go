/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
)

// AssignmentReconcilerClient is a client that consists of only the interfaces
// needed for the AssignmentsReconciler.
type AssignmentReconcilerClient interface {
	types.Events

	// ListOktaAssignments returns a paginated list of all Okta assignment resources.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
	// GetOktaAssignment returns the specified Okta assignment resources.
	GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error)
	// CreateOktaAssignment creates a new Okta assignment resource.
	CreateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	// UpdateOktaAssignment updates an existing Okta assignment resource.
	UpdateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	// UpdateOktaAssignmentActionStatuses will update the statuses for all actions in an Okta assignment if the
	// status is a valid transition. If a transition is invalid, it will be logged and the rest of the action statuses
	// will be updated if possible.
	UpdateOktaAssignmentActionStatuses(ctx context.Context, name, status string) (types.OktaAssignment, error)
	// DeleteOktaAssignment removes the specified Okta assignment resource.
	DeleteOktaAssignment(ctx context.Context, name string) error
}

// assignmentReconciler is a process that monitors Okta assignments, performs Okta API
// operations, and updates the Okta assignment status afterwards.
type assignmentReconciler struct {
	log   logrus.FieldLogger
	clock clockwork.Clock

	client  AssignmentReconcilerClient
	watcher *services.OktaAssignmentWatcher

	reconcileCh chan struct{}

	stopCh chan struct{}

	assignmentsMu sync.RWMutex
	assignments   map[string]types.OktaAssignment

	newAssignmentsMu sync.RWMutex
	newAssignments   map[string]types.OktaAssignment

	// These are used for testing.
	onReconcile   func(types.OktaAssignments)
	onReconcileCh chan struct{}
}

// newAssignmentReconciler creates a new AssignmentReconciler.
func newAssignmentReconciler(ctx context.Context, svc *Service) *assignmentReconciler {
	log := logrus.WithField(trace.Component, teleport.ComponentOktaAssignmentReconciler)
	a := &assignmentReconciler{
		log:            log,
		clock:          svc.clock,
		client:         svc.accessPoint,
		reconcileCh:    make(chan struct{}),
		stopCh:         make(chan struct{}, 1),
		assignments:    make(map[string]types.OktaAssignment),
		newAssignments: make(map[string]types.OktaAssignment),
	}

	return a
}

// Start will start the reconciler.
func (a *assignmentReconciler) start(ctx context.Context) error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig{
		Matcher: func(resource types.ResourceWithLabels) bool {
			return a.matcher(ctx, resource)
		},
		GetCurrentResources: a.getAssignments,
		GetNewResources:     a.getNewAssignments,
		OnCreate:            a.onCreate,
		OnUpdate:            a.onUpdate,
		OnDelete:            a.onDelete,
		Log:                 a.log,
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

	return nil
}

// reconcile will perform the actual assignment reconciliation.
func (a *assignmentReconciler) reconcile(ctx context.Context, reconciler *services.Reconciler) {
	for {
		select {
		case _, ok := <-a.reconcileCh:
			if !ok {
				return
			}
			if err := reconciler.Reconcile(ctx); err != nil {
				a.log.WithError(err).Error("Failed to reconcile.")
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
	close(a.stopCh)
	close(a.reconcileCh)
	a.watcher.Close()
}

// getAssignments returns the list of assignments currently known to the reconciler.
func (a *assignmentReconciler) getAssignments() types.ResourcesWithLabelsMap {
	a.assignmentsMu.RLock()
	defer a.assignmentsMu.RUnlock()

	return copyAssignmentsMapToOktaAssignments(a.assignments).AsResources().ToMap()
}

// getNewAssignments returns the list of new assignments that the reconciler has yet to act on.
func (a *assignmentReconciler) getNewAssignments() types.ResourcesWithLabelsMap {
	a.newAssignmentsMu.RLock()
	defer a.newAssignmentsMu.RUnlock()

	return copyAssignmentsMapToOktaAssignments(a.newAssignments).AsResources().ToMap()
}

// startResourceWatcher starts watching changes to assignment resources.
func (a *assignmentReconciler) startResourceWatcher(ctx context.Context) (*services.OktaAssignmentWatcher, error) {
	a.log.Debug("Initializing assignment resource watcher.")
	watcher, err := services.NewOktaAssignmentWatcher(ctx, services.OktaAssignmentWatcherConfig{
		RWCfg: services.ResourceWatcherConfig{
			Component: teleport.ComponentOktaAssignmentReconciler,
			Log:       a.log,
			Client:    a.client,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer func() {
			a.log.Debug("Access request resource watcher finished.")
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
func (a *assignmentReconciler) onCreate(ctx context.Context, resource types.ResourceWithLabels) error {
	newAssignment, ok := resource.(types.OktaAssignment)
	if !ok {
		return trace.BadParameter("expected types.OktaAssignment, got %T", resource)
	}

	// TODO(mdwn): Implement onCreate

	a.assignmentsMu.Lock()
	a.assignments[newAssignment.GetName()] = newAssignment
	a.assignmentsMu.Unlock()

	return nil
}

// onUpdate will perform necessary Okta assignment operations based on updated Okta assignments.
func (a *assignmentReconciler) onUpdate(ctx context.Context, resource types.ResourceWithLabels) error {
	updatedAssignment, ok := resource.(types.OktaAssignment)
	if !ok {
		return trace.BadParameter("expected types.AccessRequest, got %T", resource)
	}

	// TODO(mdwn): Implement onUpdate

	a.assignmentsMu.Lock()
	a.assignments[updatedAssignment.GetName()] = updatedAssignment
	a.assignmentsMu.Unlock()

	return nil
}

// onDelete will perform necessary Okta assignment operations based on deleted Okta assignments.
func (a *assignmentReconciler) onDelete(ctx context.Context, resource types.ResourceWithLabels) error {
	deletedAssignment, ok := resource.(types.OktaAssignment)
	if !ok {
		return trace.BadParameter("expected types.OktaAssignment, got %T", resource)
	}

	// TODO(mdwn): Implement onDelete

	a.assignmentsMu.Lock()
	delete(a.assignments, deletedAssignment.GetName())
	a.assignmentsMu.Unlock()

	return nil
}

// matcher will match all Okta assignments.
func (a *assignmentReconciler) matcher(ctx context.Context, resource types.ResourceWithLabels) bool {
	return true
}

func copyAssignmentsMapToOktaAssignments(assignments map[string]types.OktaAssignment) types.OktaAssignments {
	assignmentsCopy := types.OktaAssignments(make([]types.OktaAssignment, 0, len(assignments)))
	for _, assignment := range assignments {
		assignmentsCopy = append(assignmentsCopy, assignment.Copy())
	}

	return assignmentsCopy
}
