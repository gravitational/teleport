package provisioning

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/services"
)

// UsersService defines the subset of services.UsersService that the
// provisioning system actually uses.
type UsersService interface {
	ListUsers(ctx context.Context, req *usersv1.ListUsersRequest) (*usersv1.ListUsersResponse, error)
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

// AccessListsService defines the subset of services.AccessListsService that the
// provisioning system actually uses.
type AccessListsService interface {
	ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error)
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}

// AccessListPredicate is a filter function for identifying Access Lists to
// provision downstream
type AccessListPredicate func(context.Context, *accesslist.AccessList) (bool, error)

// EventHandler defines a function signature for handling provisioning event
// notifications.
//
// Events delivered via an EventHandler are considered to be notifications only,
// and any errors are expected to be handled by the caller and not propagated
// back to the provisioning system.
type EventHandler func(context.Context, *provisioningv1.PrincipalState)

// nullEventHandler is the default, do-nothing event handler
func nullEventHandler(context.Context, *provisioningv1.PrincipalState) {}

// EventHandler defines a function signature for handling provisioning event
// notifications that may return an error to the provisioning system.
type EventHandlerWithError func(context.Context, *provisioningv1.PrincipalState) error

// nullEventHandlerWithError is the default, do-nothing event handler
func nullEventHandlerWithError(context.Context, *provisioningv1.PrincipalState) error {
	return nil
}

// UserProvisioningMode indicates how the SCIM provisioner should handle
// provisioning users
type UserProvisioningMode int

const (
	// UserProvisioningModeInternal indicates that the SCIM provisioner is
	// responsible for provisioning users into the downstream system.
	UserProvisioningModeInternal UserProvisioningMode = 1

	// UserProvisioningModeExternal indicates that the SCIM provisioner should
	// delegate user provisioning to an external IdP. The SCIM provisioner will
	// still manage Group membership
	UserProvisioningModeExternal UserProvisioningMode = 2
)

type ServiceConfig struct {
	// SCIMClient is the SCIM client implementation the provisioning system will
	// use to interact with the downstream server.
	SCIMClient scimsdk.Client

	// HealthCheckSCIMClient is the SCIM client used for explicit health checks.
	// If unset, it defaults to SCIMClient.
	HealthCheckSCIMClient scimsdk.Client

	// UsersCache is the users service used by the provisioning service. The
	// provisioning service only reads from this service, so a cached service
	// is appropriate.
	UsersCache UsersService

	// AccessListsCache is the AccessLists service read by the provisioning
	// service. The provisioning service only reads from this service, so a
	// cached service is appropriate.
	AccessListsCache AccessListsService

	// Locks provides read-only access to the system locks service. Used to
	// verify users are in good standing before provisioning them downstream.
	Locks services.LockGetter

	// DownstreamID selects which Provisioning Principal State records to read.
	DownstreamID services.DownstreamID

	// StateSvc is a reference to the Provisioning Principal State CRUD service
	// used by the provisioning service. The provisioning service needs read/write
	// access, so this should be a primary CRUD service
	StateSvc services.DownstreamProvisioningStates

	// StateSvcCache is a reference to a Provisioning Principal State CRUD service
	// that can be used as like a cache, when quick reads are important
	StateSvcCache services.DownstreamProvisioningStateGetter

	// UserPredicate is a function used to select which users are provisioned
	// downstream. Returns `true` if the user should be provisioned downstream.
	// Defaults to including ALL non-system Users.
	UserPredicate identitycentercommon.UserFilterFunc

	// AccessListPredicate is a function used to select which access lists are
	// provisioned downstream. Returns `true` if the given access list should be
	// provisioned downstream.
	AccessListPredicate AccessListPredicate

	// EventsClient is used to hook into the eventing system to create resource
	// watchers.
	EventsClient types.Events

	// Logger is the slog logger instance to receive log output from the
	// provisioning service
	Logger *slog.Logger

	// Clock is the service time source. Defaults to the system clock if not
	// specified.
	Clock clockwork.Clock

	// ProvisioningConcurrency sets the upper bound on how many principals can
	// be provisioned at once. Defaults to `defaultProvisioningConcurrency` if
	// not set
	ProvisioningConcurrency int

	// StateRefreshInterval sets the interval between full state refreshes, which
	// scans the User and AccessList services for changes that require
	// provisioning. Defaults to [DefaultStateRefreshInterval] if not set.
	StateRefreshInterval time.Duration

	// EventBufferSize is the number of provisioning events to buffer between
	// the resource monitors and the provisioner. Defaults to
	// `defaultEventBufferSize` if unset.
	EventBufferSize int

	// OnExternalIDUpdated is an optional callback invoked whenever the provisioner
	// detects a change in a principal's ExternalID. The new ExternalID may be
	// empty if the downstream principal has been deleted outside of Teleport's
	// knowledge or control.
	// Defaults to a no-op implementation.
	OnExternalIDUpdated EventHandler

	// OnPrincipalProvisioning is an optional callback invoked just before a
	// principal is provisioned into the downstream system. Implementations may
	// return [ErrDoNotProvision] to indicate that the downstream principal
	// should not be provisioned downstream. All other errors are ignored.
	// Defaults to a no-op implementation.
	OnPrincipalProvisioning EventHandlerWithError

	// OnPrincipalProvisioned is an optional callback to be invoked whenever a
	// principal is successfully provisioned. Defaults to an no-op
	// implementation. Errors returned by the event handler are ignored.
	OnPrincipalProvisioned EventHandler

	// OnPrincipalDeprovisioning is an optional callback invoked just before a
	// principal is de-provisioned in the downstream system. Implementations may
	// return [ErrDoNotProvision] to indicate that the downstream principal
	// should not be de-provisioned. All other errors are ignored.
	// Defaults to a no-op implementation.
	OnPrincipalDeprovisioning EventHandlerWithError

	// UserProvisioningMode indicates how the SCIM provisioner should handle
	// provisioning users into the downstream system. Legal values are
	// [UserProvisioningModeInternal] and [UserProvisioningModeExternal].
	UserProvisioningMode UserProvisioningMode
}

func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if cfg.SCIMClient == nil {
		return trace.BadParameter("must supply a configured SCIM client")
	}

	if cfg.HealthCheckSCIMClient == nil {
		cfg.HealthCheckSCIMClient = cfg.SCIMClient
	}

	if cfg.DownstreamID == services.DownstreamID("") {
		return trace.BadParameter("must supply downstream state service")
	}

	if cfg.StateSvc == nil {
		return trace.BadParameter("must supply provisioning state service")
	}

	if cfg.StateSvcCache == nil {
		return trace.BadParameter("must supply provisioning state cache service")
	}

	if cfg.UsersCache == nil {
		return trace.BadParameter("must supply user listing service")
	}

	if cfg.AccessListsCache == nil {
		return trace.BadParameter("must supply access lists service")
	}

	if cfg.Locks == nil {
		return trace.BadParameter("must supply locks service")
	}

	if cfg.EventsClient == nil {
		return trace.BadParameter("must supply events")
	}

	if cfg.UserPredicate == nil {
		return trace.BadParameter("must supply user predicate")
	}

	if cfg.AccessListPredicate == nil {
		return trace.BadParameter("must supply access list predicate")
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, provisioningComponent)
	}

	if cfg.EventBufferSize == 0 {
		cfg.EventBufferSize = defaultEventBufferSize
	}

	if cfg.StateRefreshInterval == 0 {
		cfg.StateRefreshInterval = DefaultStateRefreshInterval
	}

	if cfg.ProvisioningConcurrency == 0 {
		cfg.ProvisioningConcurrency = defaultProvisioningConcurrency
	}

	if cfg.OnExternalIDUpdated == nil {
		cfg.OnExternalIDUpdated = nullEventHandler
	}

	if cfg.OnPrincipalProvisioning == nil {
		cfg.OnPrincipalProvisioning = nullEventHandlerWithError
	}

	if cfg.OnPrincipalProvisioned == nil {
		cfg.OnPrincipalProvisioned = nullEventHandler
	}

	if cfg.OnPrincipalDeprovisioning == nil {
		cfg.OnPrincipalDeprovisioning = nullEventHandlerWithError
	}

	return nil
}
