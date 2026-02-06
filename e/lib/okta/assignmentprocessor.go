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
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	ossaccesslist "github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/accesslist"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
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
)

type assignmentProcessorAccessPoint struct {
	oktaAssignmentService
	accessListService
}

type oktaAssignmentService interface {
	// UpdateOktaAssignment updates an existing Okta assignment resource.
	UpdateOktaAssignment(context.Context, types.OktaAssignment) (types.OktaAssignment, error)
	// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
	// since the last transition.
	UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error
	// DeleteOktaAssignment removes the specified Okta assignment resource.
	DeleteOktaAssignment(ctx context.Context, name string) error
}

type accessListService interface {
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*ossaccesslist.AccessListMember, error)
}

// assignmentProcessor will process an Okta assignment, updating its status along the way.
type assignmentProcessor struct {
	leader      isLeaderGetter
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
	assignmentGetter   func() types.OktaAssignments
	rateLimiter        *rate.Limiter
	oktaClient         oktaapi.Interface
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
		leader:     svc.leader,
		logger:     svc.logger,
		clock:      svc.clock,
		oktaOrgURL: svc.orgURL,
		hostID:     svc.hostID,
		emitter:    svc.emitter,
		accessPoint: assignmentProcessorAccessPoint{
			oktaAssignmentService: svc.accessPoint,
			accessListService:     svc.accessLists,
		},
		syncedAppServers:  &svc.appServers,
		syncedUserGroups:  &svc.groups,
		assignmentGetter:  assignmentGetter,
		oktaClient:        svc.client,
		assignmentClient:  newAssignmentClient(svc.logger, svc.client),
		stopCh:            make(chan struct{}, 1),
		userTargetCounter: map[string]map[string]struct{}{},
	}
}

// start will start the processor loop, which is used for retrying assignment processing.
func (a *assignmentProcessor) start(ctx context.Context) {
	go a.loop(ctx)
}

// loop runs the main body of the processing loop.
func (a *assignmentProcessor) loop(ctx context.Context) {
	timer := a.clock.NewTimer(TimeBetweenAssignmentProcessLoops)
	defer timer.Stop()

	for {
		select {
		case <-timer.Chan():
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
		a.assignmentClient = newAssignmentClient(a.logger, a.oktaClient)
		a.assignmentClientMu.Unlock()

		a.processAllAssignments(ctx)

		timer.Reset(TimeBetweenAssignmentProcessLoops)
	}
}

// stop will stop the assignment processor loop.
func (a *assignmentProcessor) stop() {
	close(a.stopCh)
}

// processAllAssignments will iterate through and process all of the assignments.
func (a *assignmentProcessor) processAllAssignments(ctx context.Context) {
	id := newAssignmentProcessorIDGen(a.clock.Now())
	assignments := a.assignmentGetter()

	// Rebuild the target counter in a fresh loop.
	a.rebuildTargetCounter(assignments)

	for _, assignment := range assignments {
		// processAssignment does not return error, it only returns a
		// result to signal if the assignment was processed or not so the
		// return value can be ignored here.
		_ = a.processAssignment(ctx, id, assignment, sourceTimer)
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
func (a *assignmentProcessor) processAssignment(ctx context.Context, id string, assignment types.OktaAssignment, source processorSource) processAssignmentResult {
	// Skip processing if the leadership has not been acquired.
	if !a.leader.IsLeader() {
		return processAssignmentSkipped
	}

	ctx, cancel := context.WithTimeout(ctx, processAssignmentTimeout)
	defer cancel()

	logger := a.logger.With(
		"loop_id", string(source)+":"+id,
		"assignment", assignment.GetName(),
		"user", assignment.GetUser(),
	)

	cleanupTime := assignment.GetCleanupTime()
	needsCleanup := !cleanupTime.IsZero() && !a.clock.Now().Before(cleanupTime)
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

	// We only process non-pending assignments if the source is timer (i.e. the assignment
	// comes from the periodic retry mechanism and not from event watcher) or if the assignment
	// needs to be cleaned up.
	if source != sourceTimer {
		force := needsCleanup ||
			needsReprovision ||
			assignment.GetStatus() == constants.OktaAssignmentStatusPending
		if !force {
			return processAssignmentSkipped
		}
	}

	startStatus := assignment.GetStatus()
	sinceTransition := a.clock.Since(assignment.GetLastTransition())

	if shouldProcess := a.shouldProcess(ctx, logger, assignment, needsCleanup); !shouldProcess {
		return processAssignmentSkipped
	}

	logger.DebugContext(ctx, "Processing assignment", slog.Group("details",
		"status", assignment.GetStatus(),
		"finalized", assignment.IsFinalized(),
		"cleanup_time", timeAttr(assignment.GetCleanupTime()),
	))

	// Before we process, set finalized to false if we're re-processing.
	if needsReprovision {
		var err error
		assignment.SetFinalized(false)
		assignment, err = a.accessPoint.UpdateOktaAssignment(ctx, assignment)
		if err != nil {
			logger.ErrorContext(ctx, "Error unsetting finalized on assignment that doesn't need cleanup. Will retry on the next loop", "error", err)
			return processAssignmentFailed
		}
	}

	if err := assignment.SetStatus(constants.OktaAssignmentStatusProcessing); err != nil {
		if !trace.IsCompareFailed(err) {
			a.logger.DebugContext(ctx, "Skipping assignment claimed by another service", "assignment", assignment.GetName())
			return processAssignmentFailed
		}
		logger.ErrorContext(ctx, "Illegal assignment status transition (this is a bug)", "error", err)
		return processAssignmentFailed
	}

	// Update the status to processing, which will lock other processor goroutines from operating on this.
	if err := a.accessPoint.UpdateOktaAssignmentStatus(ctx, assignment.GetName(), assignment.GetStatus(), sinceTransition); err != nil {
		if trace.IsBadParameter(err) {
			// err.Error() to not print the whole stack. This will be the "since last
			// transition" error.
			logger.DebugContext(ctx, "Failed to acquire assignment for processing, probably acquired by another processor", "error", err.Error())
		} else {
			logger.ErrorContext(ctx, "Failed to acquire assignment for processing", "error", err)
		}
		return processAssignmentFailed
	}

	var processErrs []error
	// TODO(kopiczko) pass the logger with extra attributes to assignmentClient.
	assignmentClient := a.getAssignmentClient()
	// If we can't find the user in Okta, skip trying to process any of the targets.
	if _, err := assignmentClient.userID(ctx, userName(assignment.GetUser())); err != nil {
		processErrs = []error{trace.NotFound("Okta user for the assignment not found; it could have been deleted in the meantime")}
		logger.DebugContext(ctx, "Okta user for the assignment not found. It could have been deleted in the meantime. Skipping", "error", err.Error())
	} else if needsCleanup {
		processErrs = a.cleanupTargets(ctx, logger, assignmentClient, assignment)
	} else {
		processErrs = a.processTargets(ctx, logger, assignmentClient, assignment)
	}

	var err error
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
	if _, err := a.accessPoint.UpdateOktaAssignment(ctx, assignment); err != nil {
		logger.ErrorContext(ctx, "Failed to update processed assignment resource", "error", err)
		return processAssignmentFailed
	}

	// Errors are logged while processing the assignment, so log success only when there are no
	// processing errors.
	if len(processErrs) == 0 {
		logger.DebugContext(ctx, "Successfully processed assignment", slog.Group("details",
			"status", assignment.GetStatus(),
			"finalized", assignment.IsFinalized(),
			"cleanup_time", timeAttr(assignment.GetCleanupTime()),
		))

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

func (a *assignmentProcessor) shouldProcess(ctx context.Context, logger *slog.Logger, assignment types.OktaAssignment, needsCleanup bool) bool {
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
				return true
			}

			// Otherwise, we should only retry successful objects if the time between loops has passes since
			// it last became successful
			if sinceTransition < TimeBetweenAssignmentProcessLoops {
				return false
			}
		case constants.OktaAssignmentStatusFailed:
			// Only process this if enough time has passed since the failure state.
			if sinceTransition < timeBeforeFailedRetry {
				return false
			}
		case constants.OktaAssignmentStatusProcessing:
			// Only process this if enough time has passed since trying to process this.
			if sinceTransition < processingTimeout {
				return false
			}
			logger.DebugContext(ctx, "Restarting processing of stuck assignment", "assignment", assignment.GetName())
		default:
			logger.ErrorContext(ctx, "Unknown assignment status, unable to process", "status", startStatus)
			return false
		}
	}

	return true
}

// processTargets will process or retry the targets for an assignment.
func (a *assignmentProcessor) processTargets(ctx context.Context, logger *slog.Logger, client *assignmentClient, assignment types.OktaAssignment) []error {
	var errs []error
	for _, target := range assignment.GetTargets() {
		logger := logger.With(
			"target_type", target.GetTargetType(),
			"target_id", target.GetID(),
		)

		m, err := a.accessPoint.GetAccessListMember(ctx, target.GetID(), assignment.GetUser())
		switch {
		case err == nil && m.Spec.AddedBy == accesslist.OktaServiceRoleUsername:
			// If the assignment was added by the "okta-service" role, it means it originated
			// from Okta and was imported into Teleport via Okta Access List Sync.
			//
			// In this case, we treat the assignment as being managed by Okta upstream,
			// so we should not attempt to re-provision the target resource
			continue
		case trace.IsNotFound(err):
			// User member, nothing to check.
		case err != nil:
			logger.WarnContext(ctx, "Failed to check access list membership", "error", err)
		}

		switch outcome := a.authorizeTarget(target); outcome {
		case targetAuthorized:
			// Carry on with processing.
		case targetNotFound:
			// If we can't find the target, then we'll continue because there's nothing we can do here.
			logger.DebugContext(ctx, "Resource for the target not found, ignoring")
			continue
		default:
			logger.WarnContext(ctx, "target is not managed by this service", "reason", outcome)
			continue
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
		}
		if registerErr != nil {
			logger.ErrorContext(ctx, "Error provisioning target", "error", registerErr)
			errs = append(errs, newTargetAuditError(target, verbProvision, registerErr))
		} else {
			a.registerUserTarget(assignment, target)
		}
	}
	return errs
}

// cleanupTargets will cleanup the targets for an assignment.
func (a *assignmentProcessor) cleanupTargets(ctx context.Context, logger *slog.Logger, client *assignmentClient, assignment types.OktaAssignment) []error {
	var errs []error
	for _, target := range assignment.GetTargets() {
		logger := logger.With(
			"target_type", target.GetTargetType(),
			"target_id", target.GetID(),
		)

		switch outcome := a.authorizeTarget(target); outcome {
		case targetAuthorized:
			// Carry on with processing.
		case targetNotFound:
			// If we can't find the target, then we'll continue because there's nothing we can do here.
			logger.DebugContext(ctx, "Resource for the target not found, ignoring")
			continue
		default:
			logger.WarnContext(ctx, "target is not managed by this service", "reason", outcome)
			continue
		}

		// Only cleanup the target if there are no more known assignments that have the given target.
		remainingAssignments := a.unregisterUserTarget(assignment, target)
		if remainingAssignments != 0 {
			logger.InfoContext(ctx, "Skipping target clean up, because other assignments still reference it",
				"remaining_assignments", remainingAssignments)
			continue
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
		}
		if unregisterErr != nil {
			logger.ErrorContext(ctx, "Error cleaning up target", "error", unregisterErr)
			errs = append(errs, unregisterErr)
		}
	}
	return errs
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

type verb int

const (
	_ verb = iota // unset
	verbProvision
	verbCleanup
)

// newTargetAuditError creates an error for an assignment target which denotes the error details
// are already logged and can be found using assignment reference in this error message.
func newTargetAuditError(target types.OktaAssignmentTarget, verb verb, err error) error {
	// The assignment reference should be already present in the audit event, but there is no
	// info about the target so add the target ref to the error message.
	targetRef := target.GetTargetType() + ":" + target.GetID()

	// verb type is intentionally not a string to discourage from using %s formatting which
	// would make it difficult to grep the code for the error message.
	switch verb {
	case verbProvision:
		return trace.BadParameter("failed to provision target %q: %s", targetRef, err)
	case verbCleanup:
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

func newAssignmentProcessorIDGen(time time.Time) string {
	return time.UTC().Format("02150405") // ddhhmmss
}

// TODO(kopiczko) Move to OSS lib/utils/log (https://github.com/gravitational/teleport/pull/62057)
func timeAttr(t time.Time) slog.LogValuer {
	return &timeAttrT{t}
}

type timeAttrT struct{ v time.Time }

func (a *timeAttrT) LogValue() slog.Value {
	return slog.StringValue(a.v.UTC().Format(time.RFC3339))
}
