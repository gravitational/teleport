package identitycenter

import (
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/provisioning"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
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
	// DownstreamID is the ID to be used when storing provisioning records
	DownstreamID services.DownstreamID
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
	if cfg.LocksSvc == nil {
		return trace.BadParameter("missing locks service")
	}
	return nil
}

// AWSConfig defines the AWS-specific options for the Identity Center service.
type AWSConfig struct {
	// InstanceARN is the ARN of the Identity Center instance.
	InstanceARN arn.ARN
	// Region is the AWS region in which the Identity Center instance is running.
	Region string
	// IntegrationName is the name of the Teleport OIDC integration to use.
	IntegrationName string
	// IntegrationsService is the service used to list integrations.
	IntegrationsService IntegrationsLister
	// TokenFactoryFn is the function used to generate OIDC tokens.
	TokenFactoryFn credprovider.GenerateOIDCTokenFn
}

func (cfg *AWSConfig) CheckAndSetDefaults() error {
	if (cfg.InstanceARN == arn.ARN{}) {
		return trace.BadParameter("missing Identity Center instance ARN")
	}
	if cfg.Region == "" {
		return trace.BadParameter("missing AWS region")
	}
	if cfg.IntegrationName == "" {
		return trace.BadParameter("missing Teleport OIDC integration name")
	}
	if cfg.IntegrationsService == nil {
		return trace.BadParameter("missing integrations data service")
	}
	if cfg.TokenFactoryFn == nil {
		return trace.BadParameter("missing OIDC token generator")
	}
	return nil
}

// ServiceConfig provides configuration for an Identity Center service
type ServiceConfig struct {
	Provisioning          ProvisioningConfig
	AWS                   AWSConfig
	Clock                 clockwork.Clock
	EventsClient          types.Events
	IdentityCenterDataSvc services.IdentityCenter
	Log                   *slog.Logger
	RolesSvc              RolesService
	UsersSvc              UsersService
	AccessListsSvc        services.AccessLists
	AccessRequestsSvc     services.AccessRequestGetter
}

func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if err := cfg.Provisioning.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating provisioning config")
	}
	if err := cfg.AWS.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "validating AWS config")
	}
	if cfg.IdentityCenterDataSvc == nil {
		return trace.BadParameter("missing identity center data service")
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
		cfg.Log = slog.Default().With(teleport.ComponentKey, Component)
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}
