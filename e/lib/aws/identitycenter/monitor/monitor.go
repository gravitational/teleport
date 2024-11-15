package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

// Verb describes the type of action that needs to happen in response to a
// PrincipalEvent
type Verb int

const (
	// VerbCalculate indicates that permissions on the supplied principal
	// need to be recalculated
	VerbCalculate Verb = 1
	// VerbDelete indicates that the supplied principal has to be deleted
	VerbDelete Verb = 2
	// VerbCalculateAll indicates that the system should do a full
	// recalculation of the assignments for all principal. No resource will be
	// provided
	VerbCalculateAll Verb = 3
)

// PrincipalEvent encapsulates an event from the resource watcher, telling the
// wider identity center system what it needs to do. The PrincipalEvent contains
// a Verb (essentially an event code), and a optionally a Principal (either a
// Teleport User or Access List resource) that the verb applies to.
type PrincipalEvent struct {
	// EventType indicates the action that needs to be taken by the identity
	// center system
	Verb Verb

	// Principal holds the Teleport User or AccessList resource indicating
	// the principal that the Verb applies to. Only valid for VerbCalculate
	// and VerbDelete, as VerbCalculateAll does not target a specific principal
	// resource
	Principal types.Resource
}

// EventHandler describes a function that can handle a ResourceEvent
type EventHandler func(context.Context, *PrincipalEvent)

// nilEventHandler defies a no-op event handler
func nilEventHandler(context.Context, *PrincipalEvent) {}

// AccessListsGetter defines the subset of Access List operations that the
// Resource Monitor actually uses.
type AccessListsGetter interface {
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
}

// UsersGetter defines an interface for fetching users from a data store
type UsersGetter interface {
	// GetUser fetches a specific user from the backend data store
	GetUser(context.Context, string, bool) (types.User, error)
}

// Config defines the configuration options for a new resource monitor
type Config struct {
	// Events is used to create watchers. Required.
	Events types.Events
	// AccessListsSvcCache is used to map events on Access List Members back to
	// the owning Access List. Required.
	AccessListsSvcCache AccessListsGetter
	// UsersS UsersSvcCache used to map access requests back to the requesting
	// Teleport user. Required.
	UsersSvcCache UsersGetter
	// OnEvent is the event handler function that will be called when a resource
	// event is detected. Optional. Defaults to a no-op handler.
	OnEvent EventHandler
	// Logger is the logger used to write output. Optional. Will be set to the
	// system default logger
	Logger *slog.Logger
	// Clock is the clock to use when timing watcher restarts, etc. Optional.
	// Defaults to the system clock
	Clock clockwork.Clock
}

// CheckAndSetDefaults tests that the config is valid and supplies defaults for
// missing values where appropriate.
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.AccessListsSvcCache == nil {
		return trace.BadParameter("must supply access lists service")
	}

	if cfg.UsersSvcCache == nil {
		return trace.BadParameter("must supply users service")
	}

	if cfg.Events == nil {
		return trace.BadParameter("must supply events")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.OnEvent == nil {
		cfg.OnEvent = nilEventHandler
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return nil
}

// ResourceMonitor defines a monitor for watching resources that the Identity
// Center is interested in, and translates it into an Identity Center specific
// event.
type ResourceMonitor struct {
	Config
}

// New creates a new resource monitor that watches for changes in Users, Access
// Lists, Roles and Access Requests in order to triggers permission recalculations
// and ensure AWS permissions are up-to-date
func New(cfg Config) (*ResourceMonitor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ResourceMonitor{Config: cfg}, nil
}

// SetEventHandler sets the event handler to call when the monitor encounters an
// event of interest. As per the config, setting the handler to `nil` sets a
// no-op handler. Calling SetEventHandler on a running resource monitor is
// undefined behavior.
func (rm *ResourceMonitor) SetEventHandler(h EventHandler) {
	if h == nil {
		rm.OnEvent = nilEventHandler
		return
	}
	rm.OnEvent = h
}

// Watch starts the resource monitor watching for resource events. Will
// automatically recreate the underlying watcher if it fails. Blocks until
// the supplied context is canceled.
func (m *ResourceMonitor) Watch(ctx context.Context) {
	const userMonitorRetryPeriod = 5 * time.Second

	for {
		err := m.watchEvents(ctx)
		if ctx.Err() != nil {
			return
		}

		m.Logger.ErrorContext(ctx, "Watcher closed",
			"error", err,
			"retry_in", userMonitorRetryPeriod)

		select {
		case <-m.Clock.After(userMonitorRetryPeriod):
			continue
		case <-ctx.Done():
			return
		}
	}
}

func (m *ResourceMonitor) newWatcher(ctx context.Context) (types.Watcher, error) {
	watcher, err := m.Events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindUser},
			{Kind: types.KindAccessListMember},
			{Kind: types.KindAccessList},
			{Kind: types.KindRole},
			{Kind: types.KindAccessRequest},
		},
	})
	return watcher, trace.Wrap(err, "creating identity center resource watcher")
}

func (m *ResourceMonitor) watchEvents(ctx context.Context) error {
	m.Logger.DebugContext(ctx, "Initializing Identity Center watcher")
	watcher, err := m.newWatcher(ctx)
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

	// we may have missed events during the watcher restart, so signal to the
	// outside world that it needs to do a full recalc
	m.OnEvent(ctx, &PrincipalEvent{Verb: VerbCalculateAll})

	for {
		select {
		case event := <-watcher.Events():
			if err := m.processEvent(ctx, event.Resource, event.Type); err != nil {
				m.Logger.With(
					slog.Group("event",
						slog.String("type", event.Type.String()),
						slog.Group("resource",
							slog.String("name", event.Resource.GetName()),
							slog.String("kind", event.Resource.GetKind()),
						),
					)).ErrorContext(ctx, "error handling resource event", "error", err)
			}

		case <-watcher.Done():
			return watcher.Error()
		}
	}
}

func (m *ResourceMonitor) processEvent(ctx context.Context, resource types.Resource, op types.OpType) error {
	switch resource.GetKind() {
	case types.KindUser, types.KindAccessList:
		switch op {
		case types.OpPut:
			m.OnEvent(ctx, &PrincipalEvent{Verb: VerbCalculate, Principal: resource})

		case types.OpDelete:
			m.OnEvent(ctx, &PrincipalEvent{Verb: VerbDelete, Principal: resource})
		}

	case types.KindAccessListMember:
		switch op {
		case types.OpPut:
			aclMember, ok := resource.(*accesslist.AccessListMember)
			if !ok {
				return trace.BadParameter("Expected AccessListMember resource, got %T", resource)
			}
			acl, err := m.AccessListsSvcCache.GetAccessList(ctx, aclMember.Spec.AccessList)
			if err != nil {
				return trace.Wrap(err, "access list lookup failed")
			}
			m.OnEvent(ctx, &PrincipalEvent{Verb: VerbCalculate, Principal: acl})

		case types.OpDelete:
			// the AccessListMember event parser smuggles the name of the access
			// list in the resource header's Description field so we can track
			// back to the owning access list
			acl, err := m.AccessListsSvcCache.GetAccessList(ctx, resource.GetMetadata().Description)
			if err != nil {
				return trace.Wrap(err, "access list lookup failed")
			}
			m.OnEvent(ctx, &PrincipalEvent{Verb: VerbCalculate, Principal: acl})
		}

	case types.KindRole:
		// TODO: Work out the affected users and access lists and only recalculate
		// (& re-provision, if necessary) those, rather than trigger a full
		// recalculation
		m.OnEvent(ctx, &PrincipalEvent{Verb: VerbCalculateAll})

	case types.KindAccessRequest:
		if op != types.OpPut {
			return nil
		}

		ar, ok := resource.(types.AccessRequest)
		if !ok {
			return trace.BadParameter("Expected AccessRequest resource, got %T", resource)
		}

		user, err := m.UsersSvcCache.GetUser(ctx, ar.GetUser(), false /* no secrets */)
		if err != nil {
			return trace.Wrap(err)
		}
		m.OnEvent(ctx, &PrincipalEvent{Verb: VerbCalculate, Principal: user})
	}

	return nil
}
