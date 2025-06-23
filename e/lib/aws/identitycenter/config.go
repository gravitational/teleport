package identitycenter

import (
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	"github.com/gravitational/teleport/lib/services"
)

// ProvisioningConfig defines the provisioning-specific options for the Identity
// Center service.
type ProvisioningConfig struct {
	// SCIMClient is the SCIM client used to interact with the downstream SCIM
	SCIMClient scimsdk.Client
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

// ImportConfig defines configuration parameters for Identity Center resource
// import services.
type ImportConfig struct {
	// AccessListDefaultOwners is a list of Teleport users name that will be used
	// as default owners of Access List created for Identity Center groups.
	AccessListDefaultOwners []string

	// GroupSyncFilter is a filter that is used to filter groups that are
	// synchronized from Identity Center to Teleport.
	GroupSyncFilter icfilters.Filters

	// AccountFilters is an optional collection of filters used to create an
	// allow-list when importing AWS accounts. An empty collection implies
	// "import everything"
	AccountFilters icfilters.Filters
}

func (cfg *ImportConfig) CheckAndSetDefaults() error {
	if len(cfg.AccessListDefaultOwners) == 0 {
		return trace.BadParameter("missing Access List default owners")
	}

	return nil
}

// RolesSyncMode values describe the possible ways an Identity Center service
// should create and maintain Teleport roles for AWS Account Assignments.
type RolesSyncMode int

const (
	// RolesSyncModeAll indicates that the AWS Identity Center integration
	// should create and maintain roles for all possible Account Assignments.
	RolesSyncModeAll RolesSyncMode = 1

	// RolesSyncModeNone indicates that the AWS Identity Center integration
	// should *not* create any roles representing potential account Account
	// Assignments.
	RolesSyncModeNone RolesSyncMode = 2
)

// ServiceConfig provides configuration for an Identity Center service
type ServiceConfig struct {
	Provisioning ProvisioningConfig
	// ICClient is Identity Center SDK client
	ICClient                   icsdk.Client
	Clock                      clockwork.Clock
	EventsClient               types.Events
	IdentityCenterDataSvc      services.IdentityCenter
	IdentityCenterDataSvcCache services.IdentityCenterAccountAssignmentGetter
	Log                        *slog.Logger
	RolesSvc                   RolesService
	UsersSvc                   UsersService
	AccessListsSvc             services.AccessLists
	AccessRequestsSvc          services.AccessRequestGetter
	ImportConfig               ImportConfig

	// AWSSyncInterval defines the interval between synchronization with AWS.
	// Defaults to defaultResourceSyncInterval if not set
	AWSSyncInterval time.Duration

	// AssignmentSyncInterval defines the interval between performing full
	// assignment refreshes.
	AssignmentSyncInterval time.Duration

	// UserPredicate is a function used to select which users are provisioned
	// into AWS and have their permission assignments managed by Teleport.
	// Returns `true` if the user should be provisioned and managed by Teleport.
	// Defaults to including ALL non-system Users.
	UserPredicate identitycentercommon.UserFilterFunc

	// PluginStatusSink is used to emit plugin status. It can be used to report the main
	// plugin runtime status or emit internal sub-process status such as group import
	// operation status.
	PluginStatusSink common.StatusSink

	// PluginsService is used to interface with Plugins service.
	PluginsService pluginsService

	// EventBufferSize is the number of resource events to buffer between
	// the resource monitors and the provisioner. Defaults to
	// `defaultEventBufferSize` if unset.
	EventBufferSize int

	// Emitter is an audit event emitter.
	Emitter apievents.Emitter

	// RolesSyncMode indicates how the integration will create and manage Teleport
	// Roles representing possible Account Assignments
	RolesSyncMode RolesSyncMode
}

func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if err := cfg.Provisioning.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating provisioning config")
	}
	if cfg.IdentityCenterDataSvc == nil {
		return trace.BadParameter("missing Identity Center data service")
	}
	if cfg.IdentityCenterDataSvcCache == nil {
		return trace.BadParameter("missing Identity Center data service cache")
	}
	if cfg.UsersSvc == nil {
		return trace.BadParameter("missing users service")
	}
	if cfg.UserPredicate == nil {
		return trace.BadParameter("missing user predicate")
	}
	if cfg.AccessListsSvc == nil {
		return trace.BadParameter("missing access lists service")
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
		cfg.Log = slog.Default().With(teleport.ComponentKey, eteleport.ComponentAWSIC)
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if err := cfg.ImportConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating import config")
	}
	if cfg.AWSSyncInterval == 0 {
		cfg.AWSSyncInterval = defaultResourceSyncInterval
	}
	if cfg.AssignmentSyncInterval == 0 {
		cfg.AssignmentSyncInterval = defaultAssignmentSyncInterval
	}
	if cfg.PluginStatusSink == nil {
		return trace.BadParameter("missing plugin status sink")
	}
	if cfg.PluginsService == nil {
		return trace.BadParameter("missing plugins service")
	}
	if cfg.EventBufferSize == 0 {
		cfg.EventBufferSize = defaultEventEventBufferSize
	}
	if cfg.Emitter == nil {
		return trace.BadParameter("missing event emitter")
	}

	switch cfg.RolesSyncMode {
	case RolesSyncModeAll, RolesSyncModeNone:
	default:
		return trace.BadParameter("invalid role sync mode: %d", int(cfg.RolesSyncMode))
	}

	return nil
}
