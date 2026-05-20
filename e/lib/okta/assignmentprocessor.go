package okta

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"

	ossteleport "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	ossaccesslist "github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/accesslist"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// processAssignmentTargetsTimeout is the maximum amount of time can be spent on processing
	// okta_assignment targets.  It is long because short times don't make sense when the cache
	// is cold. Listing a page of 500 users using the regular API (not Skinny Users Endpoints)
	// may take minutes. For a group/application that has many users assigned we'll repeat the
	// same listing for every user so if the time is shorter it may lead to significantly
	// longer overall processing time.
	//
	// At the same time this timeout means the watcher goroutine may be blocked on reconciling
	// a single assignment blocking other okta_assignment events being processed by the
	// watcher, so it shouldn't be unbound either.
	processAssignmentTargetsTimeout time.Duration = 10 * time.Minute
)

type assignmentProcessorAccessPoint struct {
	oktaAssignmentService
	accessListService
}

type oktaAssignmentService interface {
	// ListOktaAssignments returns a paginated list of all Okta assignment resources.
	ListOktaAssignments(context.Context, int, string) ([]types.OktaAssignment, string, error)
	// ConditionalUpdateOktaAssignment updates an existing Okta assignment resource, protected by optimistic locking.
	ConditionalUpdateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error)
	// DeleteOktaAssignment removes the specified Okta assignment resource.
	DeleteOktaAssignment(ctx context.Context, name string) error
}

type accessListService interface {
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*ossaccesslist.AccessListMember, error)
}

// assignmentProcessor will process an Okta assignment, updating its status along the way.
type assignmentProcessor struct {
	logger      *slog.Logger
	clock       clockwork.Clock
	oktaOrgURL  string
	hostID      string
	emitter     apievents.Emitter
	accessPoint assignmentProcessorAccessPoint
	// syncedAppServers are shared with [Service]. They should not be modified.
	syncedAppServers *utils.SyncMap[string, types.AppServer]
	// syncedUserGroups are shared with [Service]. They should not be modified.
	syncedUserGroups   *utils.SyncMap[string, types.UserGroup]
	rateLimiter        *rate.Limiter
	oktaClient         oktaapi.Interface
	assignmentClientMu sync.RWMutex
	assignmentClient   *assignmentClient
	stopCh             chan struct{}

	// timeBetweenAssignmentProcessLoops is the amount of time that has to pass between running
	// the assignments process loop. It also determines how often the cached Okta assignments
	// client is invalidated. This setting can be configured with the
	// okta.sync_settings.time_between_assignment_process_loops Okta plugin value.
	timeBetweenAssignmentProcessLoops time.Duration
	userTargetCounterMu               sync.Mutex
	// In the event of multiple assignments targeting the same user and group/application, we'll maintain
	// a counter of active assignments. When this counter reaches 0 during a cleanup, the Okta API will
	// be called. Otherwise, the assignment will be marked cleaned up but the Okta API will not be called
	// until the counter reaches 0.
	userTargetCounter map[string]map[string]struct{}

	processingAssignmentLock utils.KeyLock[string]

	// appFilters are used to determine which Okta app assignments to process.
	appFilters []*regexp.Regexp
	// groupFilters are used to determine which Okta groups assignments to process.
	groupFilters []*regexp.Regexp
}

func newAssignmentProcessor(svc *Service) *assignmentProcessor {
	return &assignmentProcessor{
		logger:     svc.logger,
		clock:      svc.clock,
		oktaOrgURL: svc.orgURL,
		hostID:     svc.hostID,
		emitter:    svc.emitter,
		accessPoint: assignmentProcessorAccessPoint{
			oktaAssignmentService: svc.accessPoint,
			accessListService:     svc.accessLists,
		},
		syncedAppServers:                  &svc.appServers,
		syncedUserGroups:                  &svc.groups,
		appFilters:                        svc.accessListSyncAppFilters,
		groupFilters:                      svc.accessListSyncGroupFilters,
		oktaClient:                        svc.client,
		assignmentClient:                  newAssignmentClient(svc.logger, svc.client),
		stopCh:                            make(chan struct{}, 1),
		userTargetCounter:                 map[string]map[string]struct{}{},
		timeBetweenAssignmentProcessLoops: svc.timeBetweenAssignmentProcessLoops,
	}
}

// start will start the processor loop, which is used for retrying assignment processing.
func (a *assignmentProcessor) start(ctx context.Context) {
	go a.loop(ctx)
}

// loop runs the main body of the processing loop.
func (a *assignmentProcessor) loop(ctx context.Context) {
	a.logger.DebugContext(ctx, "Starting timer-based Okta assignments reconciler")
	defer a.logger.DebugContext(ctx, "Stopped timer-based Okta assignments reconciler")

	timer := a.clock.NewTimer(a.timeBetweenAssignmentProcessLoops)
	defer timer.Stop()

	for {
		select {
		case <-timer.Chan():
		case <-a.stopCh:
			return
		case <-ctx.Done():
			return
		}

		a.processTimerEvent(ctx)

		timer.Reset(a.timeBetweenAssignmentProcessLoops)
	}
}

// stop will stop the assignment processor loop.
func (a *assignmentProcessor) stop() {
	close(a.stopCh)
}

// processWatcherEvent processes all Okta assignments coming from a watcher event. It will only
// process assignments which are pending or require cleanup.
func (a *assignmentProcessor) processWatcherEvent(ctx context.Context, assignment types.OktaAssignment) {
	logger := a.logger.With("loop_id", newLoopID(sourceWatcher))

	start := time.Now()
	logger.DebugContext(ctx, "Started processing Okta assignments")
	defer func() {
		took := time.Since(start)
		logger.DebugContext(ctx, "Finished processing Okta assignments", "took", logutils.StringerAttr(took))
	}()

	// Note that this loop is using a.assignmentClient which is reset only during
	// [processTimerEvent]. There is also NO target counter re-building here.  Watcher-based
	// loops are only processing assignments that are pending or scheduled for cleanup. Those
	// are excluded during target counter building.

	// TODO(kopiczko): Get rid of the processAssignment return value.
	_ = a.processAssignment(ctx, logger, assignment)
}

// processTimerEvent is supposed to be called in a periodic loop and re-processes all assignments
// stored in Teleport cache. It also resets the cached assignment client (which is also used for
// watcher events).
func (a *assignmentProcessor) processTimerEvent(ctx context.Context) {
	logger := a.logger.With("loop_id", newLoopID(sourceTimer))

	start := time.Now()
	logger.DebugContext(ctx, "Started processing Okta assignments")
	defer func() {
		took := time.Since(start)
		logger.DebugContext(ctx, "Finished processing Okta assignments", "took", logutils.StringerAttr(took))
	}()

	// Cached assignments client is reset periodically at the beginning of every timer-based
	// loop.
	a.resetAssignmentClient()

	var assignments []types.OktaAssignment
	for assignment, err := range clientutils.Resources(ctx, a.accessPoint.ListOktaAssignments) {
		if err != nil {
			logger.ErrorContext(ctx, "Failed to list Okta assignments", "error", err)
			return
		}
		assignments = append(assignments, assignment)
	}
	sortAssignmentsByProcessingPriority(a.clock.Now(), assignments)

	// Target counter are re-build only during full re-processing timer-based loops.
	// Watcher-based loops are only processing assignments that are pending or scheduled for
	// cleanup. Those are excluded during target counter building.
	a.rebuildTargetCounter(assignments)

	for _, assignment := range assignments {
		// TODO(kopiczko): Get rid of the processAssignment return value.
		_ = a.processAssignment(ctx, logger, assignment)
	}
}

type processAssignmentResult int

const (
	_ processAssignmentResult = iota
	processAssignmentProcessed
	processAssignmentSkipped
	processAssignmentFailed
)

// processAssignment processes the assignment creating Okta-side assignments according to the spec.
// If the assignment has a cleanup time set in the past it will be scheduled for cleanup and
// eventually removed from the backend.
// NOTE: This should not be used directly. [processTimerEvent] or [processWatcherEvent] should be used instead.
func (a *assignmentProcessor) processAssignment(ctx context.Context, logger *slog.Logger, assignment types.OktaAssignment) processAssignmentResult {
	logger = logger.With(
		"assignment", assignment.GetName(),
		"user", assignment.GetUser(),
	)

	startTime := a.clock.Now()
	needsCleanup := assignmentNeedsCleanup(assignment, startTime)
	needsReprovision := assignment.IsFinalized() && !needsCleanup

	// If the assignment was Finalized (Successfully processed in needCleanupState) delete Okta
	// assignment from backend.
	if assignment.IsFinalized() && needsCleanup {
		if err := a.accessPoint.DeleteOktaAssignment(ctx, assignment.GetName()); err != nil && !trace.IsNotFound(err) {
			logger.ErrorContext(ctx, "Error deleting finalized assignment. Will retry on the next loop", "error", err)
			return processAssignmentFailed
		}
		logger.DebugContext(ctx, "Deleted finalized and cleaned up assignment")
		return processAssignmentProcessed
	}

	startStatus := assignment.GetStatus()

	if !a.shouldProcess(ctx, logger, assignment, startTime) {
		return processAssignmentSkipped
	}

	logger.DebugContext(ctx, "Processing assignment", assignmentDetailsSlogGroup(assignment))

	// Make sure we process only one event for each assignment at a time. We can receive
	// processing event for the same assignment the watcher or the timer-based re-processing
	// loop.
	timeBeforeLocking := a.clock.Now()
	a.processingAssignmentLock.Lock(assignment.GetName())
	defer a.processingAssignmentLock.Unlock(assignment.GetName())
	if timeToAcquire := a.clock.Since(timeBeforeLocking); timeToAcquire > 30*time.Second {
		logger.DebugContext(ctx, "Took more than 30s to acquire assignment processing lock. Probably the same assignment (possibly different revision) has been processed in parallel", "time_to_acquire_lock", logutils.StringerAttr(timeToAcquire))
	}

	// Before we process, set finalized to false if we're re-processing.
	// TODO(kopiczko): Verify the `needsReprovision` case is really needed and delete if not.
	if needsReprovision {
		var err error
		assignment.SetFinalized(false)
		assignment, err = a.accessPoint.ConditionalUpdateOktaAssignment(ctx, assignment)
		if err != nil {
			logger.ErrorContext(ctx, "Error unsetting finalized on assignment that doesn't need cleanup. Will retry on the next loop", "error", err)
			return processAssignmentFailed
		}
	}

	// Set status to processing to indicate this assignment is being processed and verify we
	// are dealing with latest version of the resource.
	if err := assignment.SetStatus(constants.OktaAssignmentStatusProcessing); err != nil {
		logger.ErrorContext(ctx, "Illegal assignment status transition (this is a bug)", "error", err)
		return processAssignmentFailed
	}
	assignment.SetLastTransition(a.clock.Now())
	assignment, err := a.accessPoint.ConditionalUpdateOktaAssignment(ctx, assignment)
	if err != nil {
		if trace.IsCompareFailed(err) {
			logger.DebugContext(ctx, "Assignment is stale. Skipping", "error", err.Error())
		} else {
			logger.ErrorContext(ctx, "Error updating assignment status to processing", "error", err)
		}
		return processAssignmentFailed
	}

	// TODO(kopiczko) pass the logger with extra attributes to assignmentClient.
	assignmentClient := a.getAssignmentClient()
	op := opProvision
	if needsCleanup {
		op = opCleanup
	}
	processErrs := a.processTargets(ctx, logger, assignmentClient, assignment, op)

	if len(processErrs) == 0 {
		err = assignment.SetStatus(constants.OktaAssignmentStatusSuccessful)
	} else {
		err = assignment.SetStatus(constants.OktaAssignmentStatusFailed)
	}
	if err != nil {
		logger.ErrorContext(ctx, "Illegal assignment status transition after processing the assignment (this is a bug)", "error", err)
		return processAssignmentFailed
	}
	assignment.SetLastTransition(a.clock.Now())
	assignment.SetFinalized(len(processErrs) == 0 && needsCleanup)
	if _, err := a.accessPoint.ConditionalUpdateOktaAssignment(ctx, assignment); err != nil {
		if trace.IsCompareFailed(err) {
			logger.DebugContext(ctx, "Assignment was updated while processing. Will try again during next re-process loop", "error", err.Error())
		} else {
			logger.ErrorContext(ctx, "Error updating assignment status after finished processing. Will try again during next re-process loop.", "error", err)
		}
		return processAssignmentFailed
	}

	// Errors are logged while processing the assignment, so log success only when there are no
	// processing errors.
	if len(processErrs) == 0 {
		logger.DebugContext(ctx, "Successfully processed assignment", assignmentDetailsSlogGroup(assignment))
	}

	// Emit the event if there was a processing error or cleanup was needed, or the starting and ending status aren't
	// both successful.
	emitEvent := len(processErrs) != 0 ||
		needsCleanup ||
		startStatus != constants.OktaAssignmentStatusSuccessful ||
		assignment.GetStatus() != constants.OktaAssignmentStatusSuccessful
	if emitEvent {
		a.emitAuditEvent(ctx, assignment, startStatus, assignment.GetStatus(), needsCleanup, trace.NewAggregate(processErrs...))
	}

	return processAssignmentProcessed
}

func (a *assignmentProcessor) shouldProcess(ctx context.Context, logger *slog.Logger, assignment types.OktaAssignment, now time.Time) bool {
	sinceTransition := a.clock.Since(assignment.GetLastTransition())
	startStatus := assignment.GetStatus()

	if assignmentNeedsUrgentProcessing(assignment, now) {
		return true
	}

	switch startStatus {
	case constants.OktaAssignmentStatusPending:
		// "pending" is caught with assignmentNeedsUrgentProcessing so it's here only for
		// the sake the sake of exhaustively handling all values.
	case constants.OktaAssignmentStatusProcessing:
	case constants.OktaAssignmentStatusSuccessful:
		// If the assignment is marked finalized, it means this assignment was recently unlocked
		// and we need to re-process it.
		if assignment.IsFinalized() {
			return true
		}

		// Otherwise, we should only retry successful objects if the time between loops has
		// passed since it last became successful
		if sinceTransition < a.timeBetweenAssignmentProcessLoops {
			return false
		}
	case constants.OktaAssignmentStatusFailed:
		// Only process this if enough time has passed since the failure state.
		if sinceTransition < a.timeBetweenAssignmentProcessLoops {
			return false
		}
	default:
		logger.ErrorContext(ctx, "Unknown assignment status, unable to process (this is a bug)", "unknown_status", startStatus)
		return false
	}

	return true
}

// processTargets will try to provision/cleanup targets. It returns the updates assignment status
// and errors that should be reported in the audit event if any.
func (a *assignmentProcessor) processTargets(ctx context.Context, logger *slog.Logger, client *assignmentClient, assignment types.OktaAssignment, op opType) []error {
	ctx, cancel := context.WithTimeout(ctx, processAssignmentTargetsTimeout)
	defer cancel()

	// If we can't find the user in Okta, skip trying to process any of the targets.
	if _, err := client.userID(ctx, userName(assignment.GetUser())); err != nil {
		logger.WarnContext(ctx, "Okta user for the assignment not found (was the user deleted/deactivated in Okta?). Skipping")
		return []error{trace.NotFound("Okta user for the assignment not found (was the user deleted/deactivated in Okta?)")}
	}

	var errs []error
	targets := assignment.GetTargets()
	for i, target := range targets {
		logger := logger.With(
			"target_type", target.GetTargetType(),
			"target_id", target.GetID(),
		)

		if err := a.processTarget(ctx, logger, client, assignment, target, op); err != nil {
			errs = append(errs, trace.Wrap(err))
		}

		// If context timed out, break the loop, otherwise we can log a lot of confusing
		// "context deadline exceeded" errors for the remaining targets.
		if ctx.Err() != nil {
			// In case context is canceled just after a.processTarget call, we don't
			// want the okta_assignment status to be marked as "successful".
			logger.ErrorContext(ctx, "Assignment targets processing timed out")
			errs = append(errs, trace.Errorf("assignment targets processing timed out after %s; processed %d of %d targets", processAssignmentTargetsTimeout, i+1, len(targets)))
			return errs
		}
	}
	return errs
}

func (a *assignmentProcessor) processTarget(ctx context.Context, logger *slog.Logger, client *assignmentClient, assignment types.OktaAssignment, target types.OktaAssignmentTarget, op opType) error {
	switch outcome := a.authorizeTarget(target); outcome {
	case targetAuthorized:
		// Carry on with processing.
	case targetNotFound:
		// If we can't find the target, then we'll continue because there's nothing we can do here.
		logger.DebugContext(ctx, "Resource for the target not found, ignoring")
		return nil
	case targetNotIncluded:
		logger.DebugContext(ctx, "Target not included, skipping")
		return nil
	default:
		logger.WarnContext(ctx, "target is not managed by this service", "reason", outcome)
		return nil
	}

	switch op {
	case opProvision:
		return trace.Wrap(a.provisionTarget(ctx, logger, client, assignment, target))
	case opCleanup:
		return trace.Wrap(a.cleanupTarget(ctx, logger, client, assignment, target))
	default:
		logger.ErrorContext(ctx, "Unknown process target operation (this is a bug)", "op", op)
		return nil
	}
}

func (a *assignmentProcessor) provisionTarget(ctx context.Context, logger *slog.Logger, client *assignmentClient, assignment types.OktaAssignment, target types.OktaAssignmentTarget) error {
	m, err := a.accessPoint.GetAccessListMember(ctx, target.GetID(), assignment.GetUser())
	switch {
	case err == nil && m.Spec.AddedBy == accesslist.OktaServiceRoleUsername:
		// If the assignment was added by the "okta-service" role, it means it originated
		// from Okta and was imported into Teleport via Okta Access List Sync.
		//
		// In this case, we treat the assignment as being managed by Okta upstream,
		// so we should not attempt to re-provision the target resource
		return nil
	case trace.IsNotFound(err):
		// User member, nothing to check.
	case err != nil:
		logger.ErrorContext(ctx, "Failed to check user's AccessList membership", "error", err)
		return trace.Wrap(newTargetAuditError(target, opProvision, trace.Errorf("failed to check user's AccessList membership: %s", err)))
	}

	var registerErr error
	switch target.GetTargetType() {
	case constants.OktaAssignmentTargetGroup:
		registerErr = client.registerUserToGroup(ctx, userName(assignment.GetUser()), oktaGroupID(target.GetID()))
	case constants.OktaAssignmentTargetApplication:
		if appID, ok := a.getOktaAppIDFromAppServer(target.GetID()); !ok {
			registerErr = trace.Errorf("app_server %q does not have an Okta App ID", target.GetID())
		} else {
			registerErr = trace.Wrap(client.registerUserToApp(ctx, userName(assignment.GetUser()), appID))
		}
	default:
		logger.ErrorContext(ctx, "Unrecognized target type, skipping (this is a bug)", "target_type", target.GetTargetType())
		return nil
	}
	if registerErr != nil {
		logger.ErrorContext(ctx, "Error provisioning target", "error", registerErr)
		return trace.Wrap(newTargetAuditError(target, opProvision, registerErr))
	}
	a.registerUserTarget(assignment, target)

	return nil
}

func (a *assignmentProcessor) cleanupTarget(ctx context.Context, logger *slog.Logger, client *assignmentClient, assignment types.OktaAssignment, target types.OktaAssignmentTarget) error {
	// Only cleanup the target if there are no more known assignments that have the given target.
	referencingAssignments := a.unregisterUserTarget(assignment, target)
	if referencingAssignments != 0 {
		logger.InfoContext(ctx, "Skipping target clean up, because other assignments still reference it",
			"referencing_assignments", referencingAssignments)
		return nil
	}

	var unregisterErr error
	switch target.GetTargetType() {
	case constants.OktaAssignmentTargetGroup:
		unregisterErr = client.unregisterUserFromGroup(ctx, userName(assignment.GetUser()), oktaGroupID(target.GetID()))
	case constants.OktaAssignmentTargetApplication:
		if appID, ok := a.getOktaAppIDFromAppServer(target.GetID()); !ok {
			unregisterErr = trace.Errorf("app_server %q does not have an Okta App ID", target.GetID())
		} else {
			unregisterErr = client.unregisterUserFromApp(ctx, userName(assignment.GetUser()), appID)
		}
	default:
		logger.ErrorContext(ctx, "Unrecognized target type, skipping (this is a bug)", "target_type", target.GetTargetType())
		return nil
	}
	if unregisterErr != nil {
		logger.ErrorContext(ctx, "Error cleaning up target", "error", unregisterErr)
		return trace.Wrap(newTargetAuditError(target, opCleanup, unregisterErr))
	}

	return nil
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
		// TODO(kopiczko): Get rid of the OktaAssignment.Finalized filed and create a common method which matches assignments considered during watcher loops and then replace the condition below.
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
func (a *assignmentProcessor) unregisterUserTarget(assignment types.OktaAssignment, target types.OktaAssignmentTarget) int {
	a.userTargetCounterMu.Lock()
	defer a.userTargetCounterMu.Unlock()

	targetName := userTargetName(assignment, target)

	assignments, ok := a.userTargetCounter[targetName]
	if !ok {
		return 0
	}

	delete(assignments, assignment.GetName())

	if len(assignments) == 0 {
		delete(a.userTargetCounter, targetName)
	}

	return len(assignments)
}

// resetAssignmentClient resets the caching assignment client.
func (a *assignmentProcessor) resetAssignmentClient() {
	a.assignmentClientMu.Lock()
	defer a.assignmentClientMu.Unlock()

	a.assignmentClient = newAssignmentClient(a.logger, a.oktaClient)
}

// getAssignmentClient returns the caching assignment client.
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

type opType int

const (
	_ opType = iota // unset
	opProvision
	opCleanup
)

// newTargetAuditError creates an error for an assignment target which denotes the error details
// are already logged and can be found using assignment reference in this error message.
func newTargetAuditError(target types.OktaAssignmentTarget, verb opType, err error) error {
	// The assignment reference should be already present in the audit event, but there is no
	// info about the target so add the target ref to the error message.
	targetRef := target.GetTargetType() + ":" + target.GetID()

	// verb type is intentionally not a string to discourage from using %s formatting which
	// would make it difficult to grep the code for the error message.
	switch verb {
	case opProvision:
		return trace.BadParameter("failed to provision target %q: %s", targetRef, err)
	case opCleanup:
		return trace.BadParameter("failed to cleanup target %q: %s", targetRef, err)
	default:
		return trace.BadParameter("failed to process target (unsupported verb [%d]) %q: %s", verb, targetRef, err)
	}
}

type processorSource string

const (
	sourceWatcher processorSource = "watcher"
	sourceTimer   processorSource = "timer"
)

var newLoopSeq atomic.Uint64

func newLoopID(source processorSource) string {
	seq := strconv.FormatUint(newLoopSeq.Add(1), 36)
	if len(seq) < 8 {
		seq = strings.Repeat("0", 8-len(seq)) + seq
	}
	return string(source) + ":" + seq
}

// sortAssignmentsByProcessingPriority following rules below:
//  1. Assignments to clean up, then by CleanupTime, then by LastTransitionTime.
//  2. Pending assignments, then by LastTransitionTime.
//  3. Assignments stuck in processing, then by LastTransitionTime.
//  4. Failed assignments, then by LastTransitionTime.
//  5. LastTransitionTime.
func sortAssignmentsByProcessingPriority(now time.Time, assignments []types.OktaAssignment) {
	statusPriority := func(assignment types.OktaAssignment) int {
		if assignmentNeedsCleanup(assignment, now) {
			return 0
		}
		switch assignment.GetStatus() {
		case constants.OktaAssignmentStatusPending:
			return 1
		case constants.OktaAssignmentStatusProcessing:
			return 2
		case constants.OktaAssignmentStatusFailed:
			return 3
		default:
			return 4
		}
	}

	slices.SortFunc(assignments, func(a, b types.OktaAssignment) int {
		cmpLastTransition := a.GetLastTransition().Compare(b.GetLastTransition())
		if assignmentNeedsCleanup(a, now) && assignmentNeedsCleanup(b, now) {
			return cmp.Or(a.GetCleanupTime().Compare(b.GetCleanupTime()), cmpLastTransition)
		}
		return cmp.Or(cmp.Compare(statusPriority(a), statusPriority(b)), cmpLastTransition)
	})
}

// assignmentNeedsCleanup returns true if the assignment needs cleanup.
func assignmentNeedsCleanup(a types.OktaAssignment, now time.Time) bool {
	cleanupTime := a.GetCleanupTime()
	return !cleanupTime.IsZero() && !cleanupTime.After(now)
}

// assignmentNeedsUrgentProcessing indicates if an assignment observed by the watcher should be
// scheduled for processing.
func assignmentNeedsUrgentProcessing(assignment types.OktaAssignment, now time.Time) bool {
	// If it's pending it's always urgent. The there is not valid transition to pending so it's
	// always a freshly created assignment.
	if assignment.GetStatus() == constants.OktaAssignmentStatusPending {
		return true
	}
	// If needs cleanup, it's only urgent if it was not yet processed after CleanupTime.
	if assignmentNeedsCleanup(assignment, now) && assignment.GetLastTransition().Before(assignment.GetCleanupTime()) {
		return true
	}
	return false
}

func assignmentDetailsSlogGroup(a types.OktaAssignment) slog.Attr {
	return slog.Group("details",
		"status", a.GetStatus(),
		"finalized", a.IsFinalized(),
		"last_transition", timeAttr(a.GetLastTransition()),
		"cleanup_time", timeAttr(a.GetCleanupTime()),
	)
}

// TODO(kopiczko) Move to OSS lib/utils/log (https://github.com/gravitational/teleport/pull/62057)
func timeAttr(t time.Time) slog.LogValuer {
	return &timeAttrT{t}
}

type timeAttrT struct{ v time.Time }

func (a *timeAttrT) LogValue() slog.Value {
	return slog.StringValue(a.v.UTC().Format(time.RFC3339Nano))
}
