//TODO(gavin)

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// postgresClient implements ServersClient
var _ ServersClient = (*postgresClient)(nil)

// postgresClient wraps armpostgresql.ServersClient so we can implement the ServersClient interface.
type postgresClient struct {
	client       *armpostgresql.ServersClient
	kind         string
	subscription string
}

// TODO(gavin)
func NewPostgresClient(subscription string, cred azcore.TokenCredential) (ServersClient, error) {
	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	client, err := armpostgresql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &postgresClient{
		client:       client,
		kind:         "postgres",
		subscription: subscription,
	}, nil
}

// ListServers lists all database servers within an Azure subscription.
func (c *postgresClient) ListServers(ctx context.Context, group string, maxPages int) ([]Server, error) {
	var servers []*armpostgresql.Server
	var err error
	if group == types.Wildcard {
		servers, err = c.listAll(ctx, maxPages)
	} else {
		servers, err = c.listByGroup(ctx, group, maxPages)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := make([]Server, 0, len(servers))
	for _, s := range servers {
		server, err := ServerFromPostgresServer(s)
		if err != nil {
			continue
		}
		result = append(result, server)
	}
	return result, nil
}

// TODO(gavin)
func (c *postgresClient) Kind() string {
	return c.kind
}

// TODO(gavin)
func (c *postgresClient) Subscription() string {
	return c.subscription
}

func (c *postgresClient) Get(ctx context.Context, group, name string) (Server, error) {
	res, err := c.client.Get(ctx, group, name, nil)
	if err != nil {
		// TODO(gavin) convert at call site?
		return nil, trace.Wrap(err)
	}
	server, err := ServerFromPostgresServer(&res.Server)
	return server, trace.Wrap(err)
}

func (c *postgresClient) listAll(ctx context.Context, maxPages int) ([]*armpostgresql.Server, error) {
	var servers []*armpostgresql.Server
	options := &armpostgresql.ServersClientListOptions{}
	pager := c.client.NewListPager(options)
	for pageNum := 0; pageNum < maxPages && pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}

func (c *postgresClient) listByGroup(ctx context.Context, group string, maxPages int) ([]*armpostgresql.Server, error) {
	var servers []*armpostgresql.Server
	options := &armpostgresql.ServersClientListByResourceGroupOptions{}
	pager := c.client.NewListByResourceGroupPager(group, options)
	for pageNum := 0; pageNum < maxPages && pager.More(); pageNum++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers = append(servers, page.Value...)
	}
	return servers, nil
}

var _ Server = (*postgresServer)(nil)

// TODO(gavin)
func ServerFromPostgresServer(server *armpostgresql.Server) (Server, error) {
	if server == nil {
		return nil, trace.BadParameter("nil server")
	}
	id, err := parseID(server.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &postgresServer{
		server: server,
		tags:   convertTags(server.Tags),
		id:     *id,
	}, nil
}

type postgresServer struct {
	server *armpostgresql.Server
	tags   map[string]string
	id     types.AzureResourceID
}

// IsVersionSupported returns true if database supports AAD authentication.
func (s *postgresServer) IsVersionSupported() bool {
	switch armpostgresql.ServerVersion(s.Version()) {
	case armpostgresql.ServerVersionNine5, armpostgresql.ServerVersionNine6, armpostgresql.ServerVersionTen,
		armpostgresql.ServerVersionTen0, armpostgresql.ServerVersionTen2, armpostgresql.ServerVersionEleven:
		return true
	default:
		return false
	}
}

func (s *postgresServer) IsAvailable() bool {
	switch armpostgresql.ServerState(s.State()) {
	case armpostgresql.ServerStateReady:
		return true
	case armpostgresql.ServerStateInaccessible,
		armpostgresql.ServerStateDropping,
		armpostgresql.ServerStateDisabled:
		return false
	default:
		return false
	}
}

func (s *postgresServer) Region() string {
	return stringVal(s.server.Location)
}

func (s *postgresServer) Version() string {
	if s.server.Properties != nil && s.server.Properties.Version != nil {
		return string(*s.server.Properties.Version)
	}
	return ""
}

func (s *postgresServer) Name() string {
	return stringVal(s.server.Name)
}

func (s *postgresServer) Endpoint() string {
	if s.server.Properties != nil && s.server.Properties.FullyQualifiedDomainName != nil {
		return *s.server.Properties.FullyQualifiedDomainName + ":" + PostgresPort
	}
	return ""
}

func (s *postgresServer) ID() types.AzureResourceID {
	return s.id
}

func (s *postgresServer) Protocol() string {
	return defaults.ProtocolPostgres
}

func (s *postgresServer) State() string {
	if s.server.Properties != nil && s.server.Properties.UserVisibleState != nil {
		return string(*s.server.Properties.UserVisibleState)
	}
	return ""
}

func (s *postgresServer) Tags() map[string]string {
	return s.tags
}
