package okta

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"

	ossteleport "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
)

type isLeaderGetter interface {
	IsLeader() bool
}

var (
	// TimeBetweenAssignmentProcessLoops is the amount of time that will pass between running the assignment process loop.
	TimeBetweenAssignmentProcessLoops time.Duration = 5 * time.Minute
)

const (
	// The amount of time that must pass before a failed assignment can be retried.
	timeBeforeFailedRetry time.Duration = 5 * time.Minute

	// Any assignment left in timeout for this amount of time will be marked as failed.
	// TODO measure and document max number of group/app
	processingTimeout time.Duration = 15 * time.Minute

	// processAssignmentTimeout is the amount of time before canceling the context of a process assignment call
	// in the loop.
	processAssignmentTimeout time.Duration = 5 * time.Minute

	// maxNumWorkers is the maximum number of works that can concurrently use the
	// Okta client.
	maxNumWorkers = 5
)

type assignmentProcessorAccessPoint interface {
	// UpdateOktaAssignment updates an existing Okta assignment resource.
	UpdateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
	// since the last transition.
	UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error
	// GetUserGroup returns the specified user group resources.
	GetUserGroup(ctx context.Context, name string) (types.UserGroup, error)
	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	// DeleteOktaAssignment removes the specified Okta assignment resource.
	DeleteOktaAssignment(ctx context.Context, name string) error
}

// assignmentProcessor will process an Okta assignment, updating its status along the way.
type assignmentProcessor struct {
	leader             isLeaderGetter
	logger             *slog.Logger
	clock              clockwork.Clock
	oktaOrgURL         string
	hostID             string
	emitter            apievents.Emitter
	accessPoint        assignmentProcessorAccessPoint
	assignmentGetter   func() types.OktaAssignments
	rateLimiter        *rate.Limiter
	oktaClient         api.Client
	assignmentClientMu sync.RWMutex
	assignmentClient   *assignmentClient
	stopCh             chan struct{}

	userTargetCounterMu sync.Mutex
	// In the event of multiple assignments targeting the same user and group/application, we'll maintain
	// a counter of active assignments. When this counter reaches 0 during a cleanup, the Okta API will
	// be called. Otherwise, the assignment will be marked cleaned up but the Okta API will not be called
	// until the counter reaches 0.
	userTargetCounter map[string]map[string]struct{}
}

func newAssignmentProcessor(svc *Service, assignmentGetter func() types.OktaAssignments) *assignmentProcessor {
	return &assignmentProcessor{
		leader:            svc.leader,
		logger:            svc.logger,
		clock:             svc.clock,
		oktaOrgURL:        svc.orgURL,
		hostID:            svc.hostID,
		emitter:           svc.emitter,
		accessPoint:       svc.accessPoint,
		assignmentGetter:  assignmentGetter,
		oktaClient:        svc.client,
		assignmentClient:  newAssignmentClient(svc.logger, svc.client),
		stopCh:            make(chan struct{}, 1),
		userTargetCounter: map[string]map[string]struct{}{},
	}
}

// start will start the processor loop, which is used for retrying assignment processing.
func (a *assignmentProcessor) start(ctx context.Context, oktaClient api.Client) {
	go a.loop(ctx, oktaClient)
}

// loop runs the main body of the processing loop.
func (a *assignmentProcessor) loop(ctx context.Context, oktaClient api.Client) {
	ticker := a.clock.NewTicker(TimeBetweenAssignmentProcessLoops)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}

		// If the parent Okta service is not the leader, skip processing.
		if !a.leader.IsLeader() {
			continue
		}

		// Refresh the assignment client every loop.
		a.assignmentClientMu.Lock()
		a.assignmentClient = newAssignmentClient(a.logger, oktaClient)
		a.assignmentClientMu.Unlock()

		if err := a.processAssignments(ctx, true); err != nil {
			a.logger.ErrorContext(ctx, "Error while processing assignments", "error", err)
		}
	}
}

// stop will stop the assignment processor loop.
func (a *assignmentProcessor) stop() {
	close(a.stopCh)
}

// processAssignments will iterate through all of the assignments, spawning a goroutine to
// process each one.
func (a *assignmentProcessor) processAssignments(ctx context.Context, reconcile bool) error {
	var wg sync.WaitGroup
	assignments := a.assignmentGetter()
	numAssignments := len(assignments)
	errs := make(chan error, numAssignments)

	// Rebuild the target counter in a fresh loop.
	a.rebuildTargetCounter(assignments)

	// Use up to max num workers. If we have fewer assignments than workers,
	// just use a worker per assignment.
	numWorkers := maxNumWorkers
	if numWorkers > numAssignments {
		numWorkers = numAssignments
	}
	assignmentsCh := make(chan types.OktaAssignment, numWorkers)

	// Use a fixed number of workers along with a rate limiter to ensure we don't smack into
	// any rate limits. Enterprise rate limits for the API endpoints we use is 6000 per minute,
	// which is 100 per second:
	// https://developer.okta.com/docs/reference/rl-global-other-endpoints/
	//
	// Each Okta API interaction we do as part of processing a target consists of roughly 2 API calls.
	// By limiting our max workers to 5 and our rate limiting to 5 per second, this means that
	// generally we expect to issue 10 Okta API calls per second (or less) when running through
	// these assignments worst case. The assignment client will cache Okta state per run, so API
	// calls will be minimized.
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				assignment, ok := <-assignmentsCh
				if !ok {
					return
				}
				errs <- a.processAssignment(ctx, assignment, reconcile)
			}
		}()
	}

	for _, assignment := range assignments {
		assignmentsCh <- assignment
	}

	close(assignmentsCh)
	wg.Wait()

	close(errs)
	return trace.NewAggregateFromChannel(errs, ctx)
}

// processAssignment will apply the proper actions dictated by the OktaAssignment. The function will
// update the assignment with the results of the action application. An okta state is optionally suppliable
// for caching in bulk runs. If reconcile is set, the function will attempt to find differences from the Okta
// state and reconcile them. Otherwise, they will not be processed.
func (a *assignmentProcessor) processAssignment(ctx context.Context, assignment types.OktaAssignment, reconcile bool) error {
	// Skip processing if the leadership has not been acquired.
	if !a.leader.IsLeader() {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, processAssignmentTimeout)
	defer cancel()

	cleanupTime := assignment.GetCleanupTime()
	needsCleanup := !cleanupTime.IsZero() && !a.clock.Now().Before(cleanupTime)
	needsReprovision := assignment.IsFinalized() && !needsCleanup

	if assignment.IsFinalized() && needsCleanup {
		// If the assigment was Finalized (Successfully processed in needCleanupState)
		// delete Okta assignment from backend.
		if err := a.deleteFinalizedAssignment(ctx, assignment); err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return nil
	}

	// We only process non-pending assignments if reconcile is set or if the assignment needs to be cleaned up.
	if !needsCleanup && !needsReprovision && !reconcile && assignment.GetStatus() != constants.OktaAssignmentStatusPending {
		return nil
	}

	sinceTransition := a.clock.Since(assignment.GetLastTransition())

	startStatus := assignment.GetStatus()

	// See if we should process this assignment.
	shouldProcess, err := a.shouldProcess(ctx, assignment, needsCleanup)
	if err != nil {
		return trace.Wrap(err)
	}
	if !shouldProcess {
		return nil
	}

	// Before we process, set finalized to false if we're re-processing.
	if assignment.IsFinalized() && !needsCleanup {
		var updateErr error
		assignment.SetFinalized(false)
		assignment, updateErr = a.accessPoint.UpdateOktaAssignment(ctx, assignment)
		if updateErr != nil {
			return trace.Wrap(updateErr)
		}
	}

	if err := assignment.SetStatus(constants.OktaAssignmentStatusProcessing); err != nil {
		if !trace.IsCompareFailed(err) {
			a.logger.DebugContext(ctx, "Skipping assignment claimed by another service", "assignment", assignment.GetName())
			return nil
		}
		return trace.Wrap(err)
	}

	// Update the status to processing, which will lock other services from operating on this.
	err = a.accessPoint.UpdateOktaAssignmentStatus(ctx, assignment.GetName(), constants.OktaAssignmentStatusProcessing, sinceTransition)
	if err != nil {
		return trace.Wrap(err)
	}

	if needsCleanup {
		err = a.cleanupTargets(ctx, assignment)
	} else {
		err = a.processTargets(ctx, assignment)
	}

	// Set to success or failure depending on the errors from the targets.
	nextStatus := constants.OktaAssignmentStatusSuccessful
	finalized := false
	if err != nil {
		nextStatus = constants.OktaAssignmentStatusFailed
	} else if needsCleanup {
		// If we successfully cleaned up the assignment, we'll need to note it.
		finalized = true
	}

	// If we successfully finalized the assignment, we'll update the finalized flag here.
	if finalized {
		if err := assignment.SetStatus(nextStatus); err != nil {
			return trace.Wrap(err)
		}
		assignment.SetLastTransition(a.clock.Now())
		assignment.SetFinalized(true)
		_, updateErr := a.accessPoint.UpdateOktaAssignment(ctx, assignment)
		if updateErr != nil {
			return trace.Wrap(updateErr)
		}
	} else {
		updateErr := a.accessPoint.UpdateOktaAssignmentStatus(ctx, assignment.GetName(), nextStatus, 0)
		if updateErr != nil {
			return trace.NewAggregate(trace.Wrap(updateErr), err)
		}
	}

	// If the starting status and ending status are both successful and this doesn't need a cleanup, we won't emit anything.
	if startStatus == nextStatus && startStatus == constants.OktaAssignmentStatusSuccessful && !needsCleanup {
		return nil
	}

	a.emitAuditEvent(ctx, assignment, startStatus, nextStatus, needsCleanup, err)

	return trace.Wrap(err)
}

func (a *assignmentProcessor) shouldProcess(ctx context.Context, assignment types.OktaAssignment, needsCleanup bool) (bool, error) {
	sinceTransition := a.clock.Since(assignment.GetLastTransition())
	startStatus := assignment.GetStatus()

	// If this assignment doesn't need cleanup or it last transitioned after the cleanup time,
	// check if enough time has passed to process this again.
	if !needsCleanup || assignment.GetLastTransition().After(assignment.GetCleanupTime()) {
		switch startStatus {
		case constants.OktaAssignmentStatusPending:
		case constants.OktaAssignmentStatusSuccessful:
			// If the assignment is marked finalized, it means this assignment was recently unlocked
			// and we need to re-process it.
			if assignment.IsFinalized() {
				return true, nil
			}

			// Otherwise, we should only retry successful objects if the time between loops has passes since
			// it last became successful
			if sinceTransition < TimeBetweenAssignmentProcessLoops {
				return false, nil
			}
		case constants.OktaAssignmentStatusFailed:
			// Only process this if enough time has passed since the failure state.
			if sinceTransition < timeBeforeFailedRetry {
				return false, nil
			}
		case constants.OktaAssignmentStatusProcessing:
			// Only process this if enough time has passed since trying to process this.
			if sinceTransition < processingTimeout {
				return false, nil
			}
			a.logger.DebugContext(ctx, "Restarting processing of stuck assignment", "assignment", assignment.GetName())
		default:
			return false, trace.BadParameter("unknown state %s, unable to process assignment %s", assignment.GetStatus(), assignment.GetName())
		}
	} else {
		a.logger.DebugContext(ctx, "Processing assignment which transitioned into cleanup state immediately", "assignment", assignment.GetName())
	}

	return true, nil
}

// processTargets will process or retry the targets for an assignment.
func (a *assignmentProcessor) processTargets(ctx context.Context, assignment types.OktaAssignment) error {
	assignmentClient := a.getAssignmentClient()

	a.logger.InfoContext(ctx, "Provisioning assignment", "assignment", assignment.GetName(), "user", assignment.GetUser())

	// If we can't find the user in Okta, skip trying to process any of the targets.
	if _, err := assignmentClient.userID(ctx, userName(assignment.GetUser())); err != nil {
		return trace.Wrap(err)
	}

	var errs []error
	for _, target := range assignment.GetTargets() {
		ok, err := a.authorizeTarget(ctx, target)
		if err != nil {
			// If we can't find the target, then we'll continue because there's nothing we can do here.
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err)
		}

		if !ok {
			a.logger.WarnContext(ctx, "target is not managed by this service",
				"assignment", assignment.GetName(),
				"user", assignment.GetUser(),
				"target_type", target.GetTargetType(),
				"target_id", target.GetID(),
			)
			continue
		}

		switch target.GetTargetType() {
		case constants.OktaAssignmentTargetGroup:
			err = assignmentClient.registerUserToGroup(ctx, userName(assignment.GetUser()), oktaGroupID(target.GetID()))
		case constants.OktaAssignmentTargetApplication:
			var appID oktaAppID
			appID, err = a.getOktaAppIDFromAppServer(ctx, target.GetID())
			if err != nil {
				break
			}

			err = assignmentClient.registerUserToApp(ctx, userName(assignment.GetUser()), appID)
		}

		if err == nil {
			a.logger.InfoContext(ctx, "Successfully provisioned target",
				"assignment", assignment.GetName(),
				"user", assignment.GetUser(),
				"target_type", target.GetTargetType(),
				"target_id", target.GetID(),
			)
			a.registerUserTarget(assignment, target)
		} else {
			a.logger.ErrorContext(ctx, "Error provisioning target",
				"assignment", assignment.GetName(),
				"user", assignment.GetUser(),
				"target_type", target.GetTargetType(),
				"target_id", target.GetID(),
			)
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}

// cleanupTargets will cleanup the targets for an assignment.
func (a *assignmentProcessor) cleanupTargets(ctx context.Context, assignment types.OktaAssignment) error {
	var errs []error
	assignmentClient := a.getAssignmentClient()

	logger := a.logger.With(
		"assignment", assignment.GetName(),
		"user", assignment.GetUser(),
	)

	logger.InfoContext(ctx, "Cleaning up target")

	// If we can't find the user in Okta, skip trying to process any of the targets.
	if _, err := assignmentClient.userID(ctx, userName(assignment.GetUser())); err != nil {
		return trace.Wrap(err)
	}

	for _, target := range assignment.GetTargets() {
		logger := logger.With(
			"target_type", target.GetTargetType(),
			"target_id", target.GetID(),
		)
		ok, err := a.authorizeTarget(ctx, target)
		if err != nil {
			// If we can't find the target, then we'll continue because there's nothing we can do here.
			if trace.IsNotFound(err) {
				continue
			}
			return trace.Wrap(err)
		}

		if !ok {
			logger.WarnContext(ctx, "Target is not managed by this service")
			continue
		}

		// Only cleanup the target if there are no more known assignments that have the given target.
		remainingAssignments := a.unregisterUserTarget(assignment, target)
		if len(remainingAssignments) != 0 {
			logger.InfoContext(ctx, "Target cleaned up, but assignments still reference it, so the target will not be removed",
				"remaining_assignments", len(remainingAssignments),
			)
			continue
		}

		switch target.GetTargetType() {
		case constants.OktaAssignmentTargetGroup:
			err = assignmentClient.unregisterUserFromGroup(ctx, userName(assignment.GetUser()), oktaGroupID(target.GetID()))
		case constants.OktaAssignmentTargetApplication:
			var appID oktaAppID
			appID, err = a.getOktaAppIDFromAppServer(ctx, target.GetID())
			if err != nil {
				break
			}

			err = assignmentClient.unregisterUserFromApp(ctx, userName(assignment.GetUser()), appID)
		}

		if err != nil {
			logger.ErrorContext(ctx, "Error cleaning up target", "error", err)
			errs = append(errs, err)
		} else {
			logger.InfoContext(ctx, "Successfully cleaned up target")
		}
	}

	return trace.NewAggregate(errs...)
}

// rebuildTargetCounter will rebuild the target counter based on the list of assignments.
func (a *assignmentProcessor) rebuildTargetCounter(assignments []types.OktaAssignment) {
	a.userTargetCounterMu.Lock()
	defer a.userTargetCounterMu.Unlock()

	a.userTargetCounter = map[string]map[string]struct{}{}

	for _, assignment := range assignments {
		status := assignment.GetStatus()
		cleanupTime := assignment.GetCleanupTime()
		needsCleanup := !cleanupTime.IsZero() && !a.clock.Now().Before(cleanupTime)

		// Only count targets if they aren't in need of cleanup and if they are
		// something other than pending. The result here is that, if multiple
		// assignments are attempting to clean up the same target, then they'll
		// all be able to issue the cleanup command since there won't be multiple
		// assignments holding onto a target.
		if !needsCleanup && status != constants.OktaAssignmentStatusPending {
			for _, target := range assignment.GetTargets() {
				targetName := userTargetName(assignment, target)
				if _, ok := a.userTargetCounter[targetName]; !ok {
					a.userTargetCounter[targetName] = map[string]struct{}{}
				}
				a.userTargetCounter[targetName][assignment.GetName()] = struct{}{}
			}
		}
	}
}

// registerUserTarget registers a user target with the user target counter. This will be used in the event of
// OktaAssignments with duplicate grants so that we don't clean up an assignment when the assignment is still valid
// in a different assignment.
func (a *assignmentProcessor) registerUserTarget(assignment types.OktaAssignment, target types.OktaAssignmentTarget) {
	a.userTargetCounterMu.Lock()
	defer a.userTargetCounterMu.Unlock()

	targetName := userTargetName(assignment, target)

	_, ok := a.userTargetCounter[targetName]
	if !ok {
		a.userTargetCounter[targetName] = map[string]struct{}{}
	}

	a.userTargetCounter[targetName][assignment.GetName()] = struct{}{}
}

// unregisterUserTarget unregisters a user target with the user target counter and returns the references left to it.
func (a *assignmentProcessor) unregisterUserTarget(assignment types.OktaAssignment, target types.OktaAssignmentTarget) []string {
	a.userTargetCounterMu.Lock()
	defer a.userTargetCounterMu.Unlock()

	targetName := userTargetName(assignment, target)

	assignments, ok := a.userTargetCounter[targetName]
	if !ok {
		return nil
	}

	delete(assignments, assignment.GetName())

	remainingAssignmentsMap := assignments

	if len(remainingAssignmentsMap) == 0 {
		delete(a.userTargetCounter, targetName)
	}

	var remainingAssignmentNames []string
	for assignmentName := range remainingAssignmentsMap {
		remainingAssignmentNames = append(remainingAssignmentNames, assignmentName)
	}

	return remainingAssignmentNames
}

// getAssignmentClient returns the assignment client.
func (a *assignmentProcessor) getAssignmentClient() *assignmentClient {
	a.assignmentClientMu.RLock()
	defer a.assignmentClientMu.RUnlock()

	return a.assignmentClient
}

// emitAuditEvent will emit an audit event after an assignment is processed.
func (a *assignmentProcessor) emitAuditEvent(ctx context.Context, assignment types.OktaAssignment, startStatus, nextStatus string, needsCleanup bool, err error) {
	var eventType string
	var eventCode string
	var success bool
	var errMsg string

	if needsCleanup {
		eventType = events.OktaAssignmentCleanupEvent
		eventCode = events.OktaAssignmentCleanupSuccessCode
		success = true

		if err != nil {
			eventCode = events.OktaAssignmentCleanupFailureCode
			success = false
			errMsg = err.Error()
		}
	} else {
		eventType = events.OktaAssignmentProcessEvent
		eventCode = events.OktaAssignmentProcessSuccessCode
		success = true

		if err != nil {
			eventCode = events.OktaAssignmentProcessFailureCode
			success = false
			errMsg = err.Error()
		}
	}

	// Get the source (i.e. creator) of this Okta Assignment.
	// The source of this Okta assignment is expected to be present, but even if it isn't
	// we'd rather emit the event with the empty source label than skip creating the audit
	// trail for this assignment.
	source, _ := assignment.GetLabel(teleport.OktaAssignmentSourceLabel)

	event := &apievents.OktaAssignmentResult{
		Metadata: apievents.Metadata{
			Type: eventType,
			Code: eventCode,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion: ossteleport.Version,
			ServerID:      a.hostID,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name: assignment.GetName(),
		},
		Status: apievents.Status{
			Success: success,
			Error:   errMsg,
		},
		OktaAssignmentMetadata: apievents.OktaAssignmentMetadata{
			Source:         source,
			User:           assignment.GetUser(),
			StartingStatus: startStatus,
			EndingStatus:   nextStatus,
		},
	}

	if emitErr := a.emitter.EmitAuditEvent(ctx, event); emitErr != nil {
		a.logger.WarnContext(ctx, "Failed to emit Okta assignment result", "event_type", event.GetType(), "error", emitErr)
	}
}

// userTargetName returns a target name for a user target.
func userTargetName(assignment types.OktaAssignment, target types.OktaAssignmentTarget) string {
	return fmt.Sprintf("%x:%x:%x", assignment.GetUser(), target.GetTargetType(), target.GetID())
}

func (a *assignmentProcessor) deleteFinalizedAssignment(ctx context.Context, assignment types.OktaAssignment) error {
	a.logger.DebugContext(ctx, "Pruning cleaned up assignment from backend", "assignment", assignment.GetName())
	if err := a.accessPoint.DeleteOktaAssignment(ctx, assignment.GetName()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
