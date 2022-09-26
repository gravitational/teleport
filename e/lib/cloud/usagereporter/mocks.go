package usagereporter

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

type MockedBackendGetter struct {
	items map[string]backend.Item
}

func NewMockedBackendGetter() *MockedBackendGetter {
	return &MockedBackendGetter{
		items: make(map[string]backend.Item),
	}
}

func (m *MockedBackendGetter) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	if item, ok := m.items[string(key)]; ok {
		return &item, nil
	}
	return nil, trace.NotFound("item %q not found", string(key))
}

func (m *MockedBackendGetter) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	m.items[string(i.Key)] = i
	return &backend.Lease{
		Key: i.Key,
	}, nil
}

func (m *MockedBackendGetter) Clock() clockwork.Clock {
	return clockwork.NewFakeClock()
}

type MockedResourceGetter struct {
	MockedGetNodes              func() ([]types.Server, error)
	MockedGetDatabaseServers    func() ([]types.DatabaseServer, error)
	MockedGetUsers              func() ([]types.User, error)
	MockedGetKubeServices       func() ([]types.Server, error)
	MockedGetApplicationServers func() ([]types.AppServer, error)
	MockedGetRoles              func() ([]types.Role, error)
	MockedSearchEvents          func() ([]events.AuditEvent, string, error)
	MockedGetClusterAlerts      func() ([]types.ClusterAlert, error)
	MockedUpsertClusterAlert    func(ctx context.Context, alert types.ClusterAlert) error
	MockedGetGithubConnectors   func() ([]types.GithubConnector, error)
	MockedGetSAMLConnectors     func() ([]types.SAMLConnector, error)
	MockedGetOIDCConnectors     func() ([]types.OIDCConnector, error)
}

func (g *MockedResourceGetter) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	if g.MockedGetNodes != nil {
		return g.MockedGetNodes()
	}

	return nil, trace.NotImplemented("GetNodes is not implemented")
}

func (g *MockedResourceGetter) GetDatabaseServers(ctx context.Context, namespace string, options ...services.MarshalOption) ([]types.DatabaseServer, error) {
	if g.MockedGetDatabaseServers != nil {
		return g.MockedGetDatabaseServers()
	}

	return nil, trace.NotImplemented("GetDatabaseServers is not implemented")
}

func (g *MockedResourceGetter) GetUsers(withSecrets bool) ([]types.User, error) {
	if g.MockedGetUsers != nil {
		return g.MockedGetUsers()
	}

	return nil, trace.NotImplemented("GetUsers is not implemented")
}

func (g *MockedResourceGetter) GetKubeServices(context.Context) ([]types.Server, error) {
	if g.MockedGetKubeServices != nil {
		return g.MockedGetKubeServices()
	}

	return nil, trace.NotImplemented("GetKubeServices is not implemented")
}

func (g *MockedResourceGetter) GetApplicationServers(context.Context, string) ([]types.AppServer, error) {
	if g.MockedGetApplicationServers != nil {
		return g.MockedGetApplicationServers()
	}

	return nil, trace.NotImplemented("GetAppServers is not implemented")
}

func (g *MockedResourceGetter) GetRoles(context.Context) ([]types.Role, error) {
	if g.MockedGetRoles != nil {
		return g.MockedGetRoles()
	}

	return nil, trace.NotImplemented("GetRoles is not implemented")
}

func (g *MockedResourceGetter) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error) {
	if g.MockedSearchEvents != nil {
		return g.MockedSearchEvents()
	}

	return nil, "", trace.NotImplemented("SearchEvents is not implemented")
}

func (g *MockedResourceGetter) GetClusterAlerts(context.Context, types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	if g.MockedGetClusterAlerts != nil {
		return g.MockedGetClusterAlerts()
	}

	return nil, trace.NotImplemented("GetClusterAlerts is not implemented")
}

func (g *MockedResourceGetter) UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error {
	if g.MockedUpsertClusterAlert != nil {
		return g.MockedUpsertClusterAlert(ctx, alert)
	}

	return trace.NotImplemented("UpsertClusterAlert is not implemented")
}

func (g *MockedResourceGetter) GetGithubConnectors(context.Context, bool) ([]types.GithubConnector, error) {
	if g.MockedGetGithubConnectors != nil {
		return g.MockedGetGithubConnectors()
	}

	return nil, trace.NotImplemented("GetGithubConnectors is not implemented")
}

func (g *MockedResourceGetter) GetOIDCConnectors(context.Context, bool) ([]types.OIDCConnector, error) {
	if g.MockedGetOIDCConnectors != nil {
		return g.MockedGetOIDCConnectors()
	}

	return nil, trace.NotImplemented("GetOIDCConnectors is not implemented")
}

func (g *MockedResourceGetter) GetSAMLConnectors(context.Context, bool) ([]types.SAMLConnector, error) {
	if g.MockedGetSAMLConnectors != nil {
		return g.MockedGetSAMLConnectors()
	}

	return nil, trace.NotImplemented("GetSAMLConnectors is not implemented")
}
