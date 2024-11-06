package identitycenter

import (
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	icSDK "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimSDK "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/services"
)

// ProvisioningConfig defines the provisioning-specific options for the Identity
// Center service.
type ProvisioningConfig struct {
	// SCIMClient is the SCIM client used to interact with the downstream SCIM
	SCIMClient scimSDK.Client
	// UsersSvcCache is the cache of users to be used by the provisioning service
	UsersSvcCache provisioning.UsersService
	// AccessListsSvcCache is the cache of access lists to be used by the provisioning service
	AccessListsSvcCache provisioning.AccessListsService
	// StateSvc is the service used to manage provisioning states
	StateSvc services.ProvisioningStates
	// StateSvc is the service used to manage provisioning states
	StateSvcCache services.DownstreamProvisioningStateGetter
	// LocksSvc is the service used to manage locks
	LocksSvc services.LockGetter
}

func (cfg *ProvisioningConfig) CheckAndSetDefaults() error {
	if cfg.SCIMClient == nil {
		return trace.BadParameter("missing configured SCIM client")
	}
	if cfg.UsersSvcCache == nil {
		return trace.BadParameter("missing users service cache")
	}
	if cfg.AccessListsSvcCache == nil {
		return trace.BadParameter("missing access lists service cache")
	}
	if cfg.StateSvc == nil {
		return trace.BadParameter("missing provisioning state service")
	}
	if cfg.StateSvcCache == nil {
		return trace.BadParameter("missing provisioning state service cache")
	}
	if cfg.LocksSvc == nil {
		return trace.BadParameter("missing locks service")
	}
	return nil
}

// ServiceConfig provides configuration for an Identity Center service
type ServiceConfig struct {
	Provisioning          ProvisioningConfig
	IdentityCenterClient  icSDK.Client
	Clock                 clockwork.Clock
	EventsClient          types.Events
	IdentityCenterDataSvc services.IdentityCenter
	Log                   *slog.Logger
	RolesSvc              RolesService
	UsersSvc              UsersService
	AccessListsSvc        services.AccessLists
	AccessRequestsSvc     services.AccessRequestGetter

	// SyncInterval defines the interval between synchronization with AWS.
	// Defaults to defaultResourceSyncInterval if not set
	SyncInterval time.Duration

	// UserPredicate is a function used to select which users are provisioned
	// into AWS and have their permission assignments managed by Teleport.
	// Returns `true` if the user should be provisioned and managed by Teleport.
	// Defaults to including ALL non-system Users.
	UserPredicate func(types.User) bool

	// AccessListPredicate is a function used to select which access lists are
	// provisioned into AWS and managed by Teleport. Returns `true` if the given
	// access list should be provisioned and managed by Teleport. Defaults to
	// including ALL access lists.
	AccessListPredicate func(*accesslist.AccessList) bool
}

func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if err := cfg.Provisioning.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating provisioning config")
	}
	if cfg.IdentityCenterDataSvc == nil {
		return trace.BadParameter("missing identity center data service")
	}
	if cfg.UsersSvc == nil {
		return trace.BadParameter("missing users service")
	}
	if cfg.UserPredicate == nil {
		cfg.UserPredicate = func(u types.User) bool {
			return !types.IsSystemResource(u)
		}
	}
	if cfg.AccessListsSvc == nil {
		return trace.BadParameter("missing access lists service")
	}
	if cfg.AccessListPredicate == nil {
		cfg.AccessListPredicate = func(_ *accesslist.AccessList) bool {
			return true
		}
	}
	if cfg.AccessRequestsSvc == nil {
		return trace.BadParameter("missing access request service")
	}
	if cfg.RolesSvc == nil {
		return trace.BadParameter("missing roles service")
	}
	if cfg.EventsClient == nil {
		return trace.BadParameter("missing events client")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default().With(teleport.ComponentKey, Component)
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = defaultResourceSyncInterval
	}

	return nil
}
