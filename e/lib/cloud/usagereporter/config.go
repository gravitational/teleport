package usagereporter

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Config is the usage reporter config
type Config struct {
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// CloudClient is a client of the cloud API server
	CloudClient cloudapi.TenantsServiceClient
	// Backend is the configured backend
	BackendGetter BackendAPIGetter
	// APIGetters
	ResourceGetter ResourceAPIGetter
	// Log is the logger
	Log *logrus.Entry
	// Interval is an internal of usage reports
	Interval time.Duration
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.CloudClient == nil {
		return trace.BadParameter("missing Cloud Client")
	}

	if c.ResourceGetter == nil {
		return trace.BadParameter("missing ResourceGetter")
	}

	if c.BackendGetter == nil {
		return trace.BadParameter("missing BackendGetter")
	}

	if c.Log == nil {
		c.Log = logrus.NewEntry(logrus.StandardLogger())
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Interval <= 0 {
		return trace.BadParameter("Interval value should be greater 0")
	}

	return nil
}

// BackendAPIGetter describes required Backend API getters
type BackendAPIGetter interface {
	// Create creates item if it does not exist
	Create(ctx context.Context, i backend.Item) (*backend.Lease, error)
	// Clock returns clock used by this backend
	Clock() clockwork.Clock
}

// ResourceAPIGetter describes required resource API getters
type ResourceAPIGetter interface {
	// GetNodes retrieves servers
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)
	// GetDatabaseServers retrieves database servers
	GetDatabaseServers(context.Context, string, ...services.MarshalOption) ([]types.DatabaseServer, error)
	// GetUsers retrieves users
	GetUsers(withSecrets bool) ([]types.User, error)
	// GetKubeServices retrieves kubernetes servers
	GetKubeServices(context.Context) ([]types.Server, error)
	// GetAppServers retrieves application servers
	GetAppServers(context.Context, string, ...services.MarshalOption) ([]types.Server, error)
	// GetRoles retrieves roles
	GetRoles(context.Context) ([]types.Role, error)

	// GetGithubConnectors retrieves Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	// GetSAMLConnector retrieves SAML connectors
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	// GetOIDCConnector retrieves OIDC connectors
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)
}
