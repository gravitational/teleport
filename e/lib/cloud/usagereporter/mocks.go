package usagereporter

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

type MockedBackendGetter struct {
}

func (m *MockedBackendGetter) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	return nil, nil
}

func (m *MockedBackendGetter) Clock() clockwork.Clock {
	return clockwork.NewFakeClock()
}

type MockedResourceGetter struct {
	MockedGetNodes            func() ([]types.Server, error)
	MockedGetDatabaseServers  func() ([]types.DatabaseServer, error)
	MockedGetUsers            func() ([]types.User, error)
	MockedGetKubeServices     func() ([]types.Server, error)
	MockedGetAppServers       func() ([]types.Server, error)
	MockedGetRoles            func() ([]types.Role, error)
	MockedGetGithubConnectors func() ([]types.GithubConnector, error)
	MockedGetSAMLConnectors   func() ([]types.SAMLConnector, error)
	MockedGetOIDCConnectors   func() ([]types.OIDCConnector, error)
}

func (g *MockedResourceGetter) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
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

func (g *MockedResourceGetter) GetAppServers(context.Context, string, ...services.MarshalOption) ([]types.Server, error) {
	if g.MockedGetAppServers != nil {
		return g.MockedGetAppServers()
	}

	return nil, trace.NotImplemented("GetAppServers is not implemented")
}

func (g *MockedResourceGetter) GetRoles(context.Context) ([]types.Role, error) {
	if g.MockedGetRoles != nil {
		return g.MockedGetRoles()
	}

	return nil, trace.NotImplemented("GetRoles is not implemented")
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
