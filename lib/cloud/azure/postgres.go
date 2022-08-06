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

// AzurePostgresClient implements AzurePostgresClient.
var _ AzureClient = (*azurePostgresClient)(nil)

// azurePostgresClient wraps armpostgresql.ServersClient so we can implement the AzurePostgresClient interface.
type azurePostgresClient struct {
	client       *armpostgresql.ServersClient
	kind         string
	subscription string
}

func NewAzurePostgresClient(subscription string, cred azcore.TokenCredential) (AzureClient, error) {
	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	client, err := armpostgresql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &azurePostgresClient{
		client:       client,
		kind:         "postgres",
		subscription: subscription,
	}, nil
}

// ListServers lists all Azure Postgres servers within an Azure subscription, using a configured armpostgresql client.
func (c *azurePostgresClient) ListServers(ctx context.Context, group string, maxPages int) ([]AzureDBServer, error) {
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

	result := make([]AzureDBServer, 0, len(servers))
	for _, server := range servers {
		result = append(result, AzureDBServerFromPostgres(server))
	}
	return result, nil
}

// TODO(gavin)
func (c *azurePostgresClient) Kind() string {
	return c.kind
}

// TODO(gavin)
func (c *azurePostgresClient) Subscription() string {
	return c.subscription
}

func (c *azurePostgresClient) listAll(ctx context.Context, maxPages int) ([]*armpostgresql.Server, error) {
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

func (c *azurePostgresClient) listByGroup(ctx context.Context, group string, maxPages int) ([]*armpostgresql.Server, error) {
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

var _ AzureDBServer = (*azurePostgresServer)(nil)

func AzureDBServerFromPostgres(server *armpostgresql.Server) AzureDBServer {
	return &azurePostgresServer{server: server, tags: convertTags(server.Tags)}
}

type azurePostgresServer struct {
	server *armpostgresql.Server
	tags   map[string]string
}

// IsVersionSupported returns true if database supports AAD authentication.
// Only available for 5.7 and newer.
func (s *azurePostgresServer) IsVersionSupported() bool {
	switch armpostgresql.ServerVersion(s.GetVersion()) {
	case armpostgresql.ServerVersionNine5, armpostgresql.ServerVersionNine6, armpostgresql.ServerVersionTen,
		armpostgresql.ServerVersionTen0, armpostgresql.ServerVersionTen2, armpostgresql.ServerVersionEleven:
		return true
	default:
		return false
	}
}

func (s *azurePostgresServer) IsAvailable() bool {
	switch armpostgresql.ServerState(s.GetState()) {
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

func (s *azurePostgresServer) GetRegion() string {
	return stringVal(s.server.Location)
}

func (s *azurePostgresServer) GetVersion() string {
	if s.server.Properties != nil && s.server.Properties.Version != nil {
		return string(*s.server.Properties.Version)
	}
	return ""
}

func (s *azurePostgresServer) GetName() string {
	return stringVal(s.server.Name)
}

func (s *azurePostgresServer) GetEndpoint() string {
	if s.server.Properties != nil && s.server.Properties.FullyQualifiedDomainName != nil {
		return *s.server.Properties.FullyQualifiedDomainName + ":" + AzurePostgresPort
	}
	return ""
}

func (s *azurePostgresServer) GetID() string {
	return stringVal(s.server.ID)
}

func (s *azurePostgresServer) GetProtocol() string {
	return defaults.ProtocolPostgres
}

func (s *azurePostgresServer) GetState() string {
	if s.server.Properties != nil && s.server.Properties.UserVisibleState != nil {
		return string(*s.server.Properties.UserVisibleState)
	}
	return ""
}

func (s *azurePostgresServer) GetTags() map[string]string {
	return s.tags
}
