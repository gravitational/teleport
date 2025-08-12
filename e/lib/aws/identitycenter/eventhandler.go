package identitycenter

import (
	"context"
	"errors"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	icIter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/monitor"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	"github.com/gravitational/teleport/lib/services"
)

// EventBatchDuration specifies how long the event aggregator will collect and
// aggregate resource events before acting on the them. Shorter durations make
// the service more responsive, but longer durations are  able to discard more
// work and are thus more efficient.
//
// There is no particular data behind the initial 30s value, other than being
// short enough to make the service still feel responsive while being long
// enough to reliably aggregate work from sync events and access list updates.
var EventBatchDuration = 30 * time.Second

// resourceEventLoop handles events from the resource monitor.
func (svc *Service) resourceEventLoop(ctx context.Context) error {
	fullRefreshTimer := svc.clock.NewTimer(retryutils.SeventhJitter(svc.assignmentSyncInterval))

	// Start a process to periodically inject a full recalculation event into
	// the resource event channel to ensure that any bad provisions are
	// automatically fixed
	go func() {
		defer fullRefreshTimer.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-fullRefreshTimer.Chan():
				svc.queueResourceEvent(ctx, &monitor.PrincipalEvent{Verb: monitor.VerbCalculateAll})
			}
		}
	}()

	for {
		pendingEvents, doFullRefresh, readOK := svc.batchReadEvents(ctx, EventBatchDuration)
		if !readOK {
			return nil
		}

		if doFullRefresh {
			// No sense in doing a periodic full refresh update if we have just done
			// one in response to an event, so reset the periodic refresh timer back
			// to the start of its interval
			fullRefreshTimer.Reset(retryutils.SeventhJitter(svc.assignmentSyncInterval))

			//  Do the refresh, logging any errors
			if err := svc.refreshAllPrincipalAssignments(ctx); err != nil {
				svc.log.ErrorContext(ctx, "failed handling full refresh",
					"error", err)
			}
			continue
		}

		for id, event := range pendingEvents {
			if err := svc.handleResourceEvent(ctx, id, event); err != nil {
				svc.log.ErrorContext(ctx, "failed handling resource event",
					"error", err)
			}
		}
	}
}

type principalEventMap map[services.PrincipalAssignmentID]*monitor.PrincipalEvent

// batchReadEvents reads a batch of values from the resource event queue over a
// given interval and discards duplicate or obsolete events. Where multiple
// events for a given principal are detected, the latest event wins. In the event
// of a full refresh, throw away all of the pending messages and just do the
// full refresh.
func (svc *Service) batchReadEvents(ctx context.Context, duration time.Duration) (pendingEvents principalEventMap, doFullRefresh bool, ok bool) {
	deadline := svc.clock.After(duration)
	pendingEvents = make(principalEventMap)
	doFullRefresh = false

	keepReading := true
	for keepReading {
		var event *monitor.PrincipalEvent
		select {
		case <-ctx.Done():
			svc.log.DebugContext(ctx, "Resource event handling loop context expired")
			return nil, false /* do not do full recalc */, false /* do not continue reading events*/

		case <-deadline:
			keepReading = false
			continue

		case event, ok = <-svc.principalEventCh:
			if !ok {
				svc.log.DebugContext(ctx, "Resource event channel closed")
				return nil, false, ok
			}
		}

		// If we're already doing a full refresh in this batch, there's no
		// point in recording anything else. Abandon this iteration, but
		// continue looping to drain the channel over the desired intervals.
		if doFullRefresh {
			continue
		}

		if event.Verb == monitor.VerbCalculateAll {
			doFullRefresh = true
			// Keeping track of individual principals to refresh is now pointless
			// because *everything* is getting refreshed; let's free the
			// memory used by the tracking info.
			clear(pendingEvents)
			continue
		}

		principalID, err := principal.GetIDForPrincipalResource(event.Principal)
		if err != nil {
			svc.log.ErrorContext(ctx, "Failed getting Principal ID for resource", "error", err)
			continue
		}

		// Keep track of the *last* event for any given principal, reducing
		// the amount of re-work we do if there are multiple events on the
		// same principal (e.g. while editing an access list)
		pendingEvents[principalID] = event
	}

	return pendingEvents, doFullRefresh, true /* continue reading events */
}

// errResourceExcluded isn an error indicating that a resource has failed its
// inclusion predicate and should not be provisioned by the event handler.
var errResourceExcluded = errors.New("resource excluded")

func (svc *Service) handleResourceEvent(ctx context.Context, principalID services.PrincipalAssignmentID, event *monitor.PrincipalEvent) error {
	switch event.Verb {
	case monitor.VerbDelete:
		// Downstream user and group deletes are handled by the SCIM
		// provisioning system. All we have to do is delete our Principal
		// Assignment record.
		if err := svc.icSvc.DeletePrincipalAssignment(ctx, principalID); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err, "deleting principal assignment state for %s", principalID)
			}
		}
		return nil

	case monitor.VerbCalculate:
		principalState, err := svc.ensurePrincipalAssignment(ctx, principalID, event.Principal)
		if err != nil {
			if errors.Is(err, errResourceExcluded) {
				return nil
			}
			return trace.Wrap(err)
		}

		if err := svc.refreshPrincipalAssignment(ctx, event.Principal, principalState); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ensurePrincipalAssignment checks that the resource that triggered the event
// is managed by the Identity Center integration, and has all of the appropriate
// state records it needs in order to be provisioned. Include special handling to
// ensure that resources which transition out of Identity Center control are
// cleaned up correctly.
func (svc *Service) ensurePrincipalAssignment(
	ctx context.Context,
	principalID services.PrincipalAssignmentID,
	principalResource types.Resource,
) (*identitycenterv1.PrincipalAssignment, error) {

	needsAssignment, err := svc.isTargetedResource(ctx, principalResource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	principalAssignment, err := svc.icSvc.GetPrincipalAssignment(ctx, principalID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	hasAssignment := (principalAssignment != nil)

	if needsAssignment && hasAssignment {
		// The principal passes the predicate and we already have an assignment
		// record for it; everything is as it should be.
		return principalAssignment, nil
	}

	if needsAssignment && !hasAssignment {
		// The principal should be provisioned, but has no assignment state
		// record. Just create one for them and carry on.
		principalAssignment, err = principal.CreateFor(ctx, principalResource, svc.icSvc)
		if err != nil {
			return nil, trace.Wrap(err, "failed creating principal assignment state for %s", principalID)
		}
		return principalAssignment, nil
	}

	if !needsAssignment && hasAssignment {
		// The principal should NOT be provisioned downstream but DOES have
		// an existing assignment state record. This can happen when a
		// principal is updated and transitions from matching the predicate
		// to NOT matching it. As above, the actual principal deletion will
		// be handled by the SCIM Provisioning system, we just have to
		// delete the assignment state in Teleport.
		if err := svc.icSvc.DeletePrincipalAssignment(ctx, principalID); err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err, "deleting principal assignment state for %s", principalID)
			}
			svc.log.WarnContext(ctx,
				"Principal assignment state unexpectedly deleted",
				"principal_id", principalID)
		}
	}

	return nil, errResourceExcluded
}

// refreshAllPrincipalAssignments reconciles the list of PrincipalAssignments with
// all Teleport users and Access Lists, recalculating their assignment sets and re-
// provisioning their assignments as necessary.
func (svc *Service) refreshAllPrincipalAssignments(ctx context.Context) error {
	// provisioningConcurrency value is completely arbitrary.
	// TODO: Update when we have more data, expose via config.
	const provisioningConcurrency = 8

	svc.log.DebugContext(ctx, "Performing full principal refresh")

	// Build a map of all the know principal assignment states in the backend
	principals, err := principal.Load(ctx, svc.icSvc)
	if err != nil {
		return trace.Wrap(err)
	}

	// There may be a *lot* of principals to re-provision, especially on startup
	// so we will try to run a few at a time with an error group
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(provisioningConcurrency)

	// Walk the list of users, and recalculate their assignment set and
	// re-provision their assignments if necessary
	for user, err := range icIter.AllUsers(ctx, svc.usersSvc) {
		if err != nil {
			return trace.Wrap(err)
		}

		if !svc.userMatchesPredicate(user) {
			continue
		}

		principalID := principal.GetIDForUser(user)
		pa, ok := principals[principalID]
		if ok {
			// Mark the principal as reachable by removing it from the map
			delete(principals, principalID)
		} else {
			pa, err = principal.CreateFor(ctx, user, svc.icSvc)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		group.Go(func() error {
			if err := svc.refreshPrincipalAssignment(groupCtx, user, pa); err != nil {
				svc.log.ErrorContext(groupCtx, "failed refreshing user",
					"error", err)
			}
			return nil
		})
	}

	// walk the list of access lists, also recalculating their permission sets
	// and re-provisioning their assignments as necessary
	for acl, err := range icIter.AllAccessLists(ctx, svc.accessListSvcCache) {
		if err != nil {
			return trace.Wrap(err)
		}

		includeACL, err := svc.accessListMatchesPredicate(ctx, acl)
		if err != nil {
			svc.log.ErrorContext(ctx,
				"Access List predicate returned an error. Excluding Access List.",
				"error", err,
				"access_list", acl.GetName(),
				"access_list_title", acl.Spec.Title)
			continue
		}
		if !includeACL {
			continue
		}

		principalID := principal.GetIDForAccessList(acl)
		pa, ok := principals[principalID]
		if ok {
			// Mark the principal as reachable by removing it from the map
			delete(principals, principalID)
		} else {
			pa, err = principal.CreateFor(ctx, acl, svc.icSvc)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		group.Go(func() error {
			if err := svc.refreshPrincipalAssignment(groupCtx, acl, pa); err != nil {
				svc.log.ErrorContext(groupCtx, "failed refreshing access list",
					"error", err)
			}
			return nil
		})
	}

	// Whatever principals left in the map are not related to a live Teleport
	// resource. They need to go.
	for id := range principals {
		// The actual user and/or group deletions will be taken care of by the
		// provisioning service. All we have to do is delete the principal
		// assignment record.
		if err := svc.icSvc.DeletePrincipalAssignment(ctx, id); err != nil {
			svc.log.ErrorContext(ctx, "failed deleting principal assignments",
				"error", err)
		}
	}

	// Wait for the error group to complete all of its outstanding tasks before
	// returning
	return trace.Wrap(group.Wait())
}

// refreshPrincipalAssignment recalculates a principal's permission assignments
// and (if necessary) invokes the assignment provisioner to sync any changes
// with AWS.
func (svc *Service) refreshPrincipalAssignment(
	ctx context.Context,
	principal types.Resource,
	pa *identitycenterv1.PrincipalAssignment,
) error {

	pa, err := svc.assignmentCalculator.CalcAssignments(ctx, principal, pa)
	if err != nil {
		return trace.Wrap(err)
	}

	if pa.GetStatus().GetProvisioningState() != identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE {
		return nil
	}

	_, err = svc.assignmentProvisioner.Provision(ctx, pa)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
