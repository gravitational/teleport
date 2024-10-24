package usagereporter

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
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
	// Logger emits log messages
	Logger *slog.Logger
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

	if c.Logger == nil {
		c.Logger = slog.Default()
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
	// Get returns a single item or not found error
	Get(ctx context.Context, key backend.Key) (*backend.Item, error)
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
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)
	// GetKubernetesServers retrieves kubernetes servers
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)
	// GetApplicationServers retrieves application servers.
	GetApplicationServers(context.Context, string) ([]types.AppServer, error)
	// GetRoles retrieves roles
	GetRoles(context.Context) ([]types.Role, error)
	// SearchEvents allows searching for events with a full pagination support.
	SearchEvents(context.Context, events.SearchEventsRequest) ([]apievents.AuditEvent, string, error)

	// GetClusterAlerts loads matching cluster alerts.
	GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error)
	// UpsertClusterAlert creates the specified alert, overwriting any preexisting alert with the same ID.
	UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error
	// DeleteClusterAlert deletes the cluster alert with the specified ID.
	DeleteClusterAlert(ctx context.Context, alertID string) error

	// GetGithubConnectors retrieves GitHub connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	// GetSAMLConnectors retrieves SAML connectors
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]types.SAMLConnector, error)
	// GetOIDCConnectors retrieves OIDC connectors
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]types.OIDCConnector, error)
}
