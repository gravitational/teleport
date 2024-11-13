package provisioning

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/accesslists"
)

type resourceMonitor struct {
	svc *Service

	// notifyCh is an un-buffered channel used as a signal from the resource
	// monitor to an external client that there are pending resource changes
	// which should be up and processed
	notifyCh chan struct{}
}

func newResourceMonitor(svc *Service) (*resourceMonitor, error) {
	return &resourceMonitor{
		svc:      svc,
		notifyCh: make(chan struct{}),
	}, nil
}

func (rm *resourceMonitor) watch(ctx context.Context) {
	const userMonitorRetryPeriod = 5 * time.Second
	defer close(rm.notifyCh)

	for {
		err := rm.watchEvents(ctx)
		if ctx.Err() != nil {
			return
		}

		rm.svc.log.ErrorContext(ctx, "Watcher closed",
			"error", err,
			"retry_in", userMonitorRetryPeriod)

		select {
		case <-rm.svc.clock.After(userMonitorRetryPeriod):
		case <-ctx.Done():
			close(rm.svc.fullRefreshSignal)
			return
		}
	}
}

func (rm *resourceMonitor) newWatcher(ctx context.Context) (types.Watcher, error) {
	watcher, err := rm.svc.eventsSvc.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
			{Kind: types.KindLock},
			{Kind: types.KindUser},
		},
	})
	return watcher, trace.Wrap(err)
}

func (rm *resourceMonitor) watchEvents(ctx context.Context) error {
	notifyTicker := time.NewTicker(10 * time.Second)
	defer notifyTicker.Stop()

	rm.svc.log.DebugContext(ctx, "Initializing watcher")
	watcher, err := rm.newWatcher(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init operation on start, got %s", event.Type.String())
		}

	case <-watcher.Done():
		return watcher.Error()
	}

	// we may have missed events while the resource monitor was down, so force
	// the provisioning system to do a full refresh.
	rm.svc.signalFullRefresh()

	rm.svc.log.DebugContext(ctx, "Entering watch loop")
	for {
		select {
		case event := <-watcher.Events():
			log := rm.svc.log.With(
				"event", event.Type,
				"resource_type", event.Resource.GetKind(),
				"resource", event.Resource.GetName())
			if err := rm.processEvent(ctx, event.Resource, event.Type); err != nil {
				log.ErrorContext(ctx, "failed handling resource event",
					"error", err)
			}

		case <-watcher.Done():
			rm.svc.log.DebugContext(ctx, "Watcher has signaled exit")
			return watcher.Error()
		}
	}
}

func (rm *resourceMonitor) processEvent(ctx context.Context, resource types.Resource, op types.OpType) error {
	switch resource.GetKind() {
	case types.KindUser:
		name := resource.GetName()
		switch op {
		case types.OpPut:
			err := rm.svc.enqueuePrincipalEvent(ctx,
				provisioningOpStale,
				provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
				name)
			return trace.Wrap(err)
		case types.OpDelete:
			err := rm.svc.enqueuePrincipalEvent(ctx,
				provisioningOpDelete,
				provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER,
				name)
			return trace.Wrap(err)
		}

	case types.KindAccessList:
		switch op {
		case types.OpPut:
			acl, ok := resource.(*accesslist.AccessList)
			if !ok {
				return trace.BadParameter("unexpected Access List resource type %T", resource)
			}
			return trace.Wrap(rm.handleAccessListUpdate(ctx, acl))

		case types.OpDelete:
			err := rm.svc.enqueuePrincipalEvent(ctx,
				provisioningOpDelete,
				provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST,
				resource.GetName())
			return trace.Wrap(err)
		}

	case types.KindAccessListMember:
		switch op {
		case types.OpPut:
			aclMember, ok := resource.(*accesslist.AccessListMember)
			if !ok {
				return trace.BadParameter("Expected AccessListMember resource, got %T", resource)
			}
			acl, err := rm.svc.accessListsSvcCache.GetAccessList(ctx, aclMember.Spec.AccessList)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(rm.handleAccessListUpdate(ctx, acl))

		case types.OpDelete:
			// the AccessListMember parser smuggles the name of the access list
			// in the resource header's Description field so we can track back
			// to the owning access list
			aclName := resource.GetMetadata().Description
			if aclName == "" {
				return trace.BadParameter("AccessListMember missing Access List Name in Description")
			}
			acl, err := rm.svc.accessListsSvcCache.GetAccessList(ctx, aclName)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(rm.handleAccessListUpdate(ctx, acl))
		}

	case types.KindLock:
		switch op {
		case types.OpPut:
			lock, ok := resource.(types.Lock)
			if !ok {
				return trace.BadParameter("Expected Lock resource, got %T", resource)
			}

			if err := rm.svc.handleLockCreation(ctx, lock); err != nil {
				return trace.Wrap(err)
			}
			return nil

		case types.OpDelete:
			if err := rm.svc.handleLockDeletion(ctx, resource.GetName()); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
	}

	return nil
}

// handleAccessListUpdate issues re-provisioning requests for the target
// AccessList and all of the Access Lists that it is a member of.
//
// The Provisioner handles nested Access Lists by expanding them in to a single
// Group containing all users reachable from a given Access List, either directly
// or transitively via nested Access Lists. This means that any change to an
// Access List *also* needs to re-provision all of its ancestor Access Lists as
// well, otherwise the provisioned Groups will have inconsistent member lists.
func (rm *resourceMonitor) handleAccessListUpdate(ctx context.Context, acl *accesslist.AccessList) error {
	targetACLs := []*accesslist.AccessList{acl}

	ancestors, err := accesslists.GetAncestorsFor(ctx, acl, accesslists.RelationshipKindMember, rm.svc.accessListsSvcCache)
	if err != nil {
		return trace.Wrap(err)
	}
	targetACLs = append(targetACLs, ancestors...)

	for _, a := range targetACLs {
		err := rm.svc.enqueuePrincipalEvent(ctx,
			provisioningOpStale,
			provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST,
			a.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
