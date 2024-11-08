package identitycenter

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	icIter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/monitor"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
)

// resourceEventLoop handles events from the resource monitor.
func (svc *Service) resourceEventLoop(ctx context.Context) error {
	for {
		// TODO: investigate if pulling resource updates in batches and skipping
		//       over multiple updates to the same principal is worthwhile
		select {
		case <-ctx.Done():
			return nil

		case e, ok := <-svc.principalEventCh:
			if !ok {
				return nil
			}

			if err := svc.handleResourceEvent(ctx, e); err != nil {
				svc.log.ErrorContext(ctx, "failed handling resource event",
					"error", err)
			}
		}
	}
}

func (svc *Service) handleResourceEvent(ctx context.Context, event *monitor.PrincipalEvent) error {
	if event.Verb == monitor.VerbCalculateAll {
		return svc.refreshAllPrincipalAssignments(ctx)
	}

	principalID, err := principal.GetIDForPrincipalResource(event.Principal)
	if err != nil {
		return trace.Wrap(err)
	}

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
		if !svc.isTargetedResource(event.Principal) {
			return nil
		}

		principalState, err := svc.icSvc.GetPrincipalAssignment(ctx, principalID)
		switch {
		case err == nil:
			// found it!
			break

		case trace.IsNotFound(err):
			principalState, err = principal.CreateFor(ctx, event.Principal, svc.icSvc)
			if err != nil {
				return trace.Wrap(err, "failed creating principal assignment state for %s", principalID)
			}

		default:
			return trace.Wrap(err, "failed loading principal assignment state %s", principalID)
		}

		if err := svc.refreshPrincipalAssignment(ctx, event.Principal, principalState); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
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

		if !svc.userPredicate(user) {
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
	for acl, err := range icIter.AllAccessLists(ctx, svc.accessListSvc) {
		if err != nil {
			return trace.Wrap(err)
		}

		if !svc.accessListPredicate(acl) {
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
	assignment *identitycenterv1.PrincipalAssignment,
) error {

	_, err := svc.assignmentCalculator.CalcAssignments(ctx, principal, assignment)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(tcsc): invoke provisioner here
	return nil
}
