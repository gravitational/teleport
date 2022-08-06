//TODO(gavin)

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// AzureMySQLClient implements AzureMySQLClient.
var _ AzureClient = (*azureMySQLClient)(nil)

// azureMySQLClient wraps armmysql.ServersClient so we can implement the AzureMySQLClient interface.
type azureMySQLClient struct {
	client       *armmysql.ServersClient
	clientType   string
	subscription string
}

func NewAzureMySQLClient(subscription string, cred azcore.TokenCredential) (AzureClient, error) {
	// TODO(gavin): if/when we support AzureChina/AzureGovernment, we will need to specify the cloud in these options
	options := &arm.ClientOptions{}
	client, err := armmysql.NewServersClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &azureMySQLClient{
		client:       client,
		clientType:   "mysql",
		subscription: subscription,
	}, nil
}

// ListServers lists all Azure MySQL servers within an Azure subscription, using a configured armmysql client.
func (c *azureMySQLClient) ListServers(ctx context.Context, group string, maxPages int) ([]AzureDBServer, error) {
	var servers []*armmysql.Server
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
		result = append(result, AzureDBServerFromMySQL(server))
	}
	return result, nil
}

// TODO(gavin)
func (c *azureMySQLClient) Kind() string {
	return c.clientType
}

// TODO(gavin)
func (c *azureMySQLClient) Subscription() string {
	return c.subscription
}

func (c *azureMySQLClient) listAll(ctx context.Context, maxPages int) ([]*armmysql.Server, error) {
	var servers []*armmysql.Server
	options := &armmysql.ServersClientListOptions{}
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

func (c *azureMySQLClient) listByGroup(ctx context.Context, group string, maxPages int) ([]*armmysql.Server, error) {
	var servers []*armmysql.Server
	options := &armmysql.ServersClientListByResourceGroupOptions{}
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

var _ AzureDBServer = (*azureMySQLServer)(nil)

type azureMySQLServer struct {
	server *armmysql.Server
	tags   map[string]string
}

// TODO(gavin)
func AzureDBServerFromMySQL(server *armmysql.Server) AzureDBServer {
	return &azureMySQLServer{server: server, tags: convertTags(server.Tags)}
}

// IsVersionSupported returns true if database supports AAD authentication.
// Only available for 5.7 and newer.
func (s *azureMySQLServer) IsVersionSupported() bool {
	switch armmysql.ServerVersion(s.GetVersion()) {
	case armmysql.ServerVersionEight0, armmysql.ServerVersionFive7:
		return true
	case armmysql.ServerVersionFive6:
		return false
	default:
		return false
	}
}

// TODO(gavin)
func (s *azureMySQLServer) IsAvailable() bool {
	switch armmysql.ServerState(s.GetState()) {
	case armmysql.ServerStateReady:
		return true
	case armmysql.ServerStateInaccessible,
		armmysql.ServerStateDropping,
		armmysql.ServerStateDisabled:
		return false
	default:
		return false
	}
}

func (s *azureMySQLServer) GetRegion() string {
	return stringVal(s.server.Location)
}

func (s *azureMySQLServer) GetVersion() string {
	if s.server.Properties != nil && s.server.Properties.Version != nil {
		return string(*s.server.Properties.Version)
	}
	return ""
}

func (s *azureMySQLServer) GetName() string {
	return stringVal(s.server.Name)
}

func (s *azureMySQLServer) GetEndpoint() string {
	if s.server.Properties != nil && s.server.Properties.FullyQualifiedDomainName != nil {
		return *s.server.Properties.FullyQualifiedDomainName + ":" + AzureMySQLPort
	}
	return ""
}

func (s *azureMySQLServer) GetID() string {
	return stringVal(s.server.ID)
}

func (s *azureMySQLServer) GetProtocol() string {
	return defaults.ProtocolMySQL
}

func (s *azureMySQLServer) GetState() string {
	if s.server.Properties != nil && s.server.Properties.UserVisibleState != nil {
		return string(*s.server.Properties.UserVisibleState)
	}
	return ""
}

func (s *azureMySQLServer) GetTags() map[string]string {
	return s.tags
}

func stringVal(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func convertTags(azureTags map[string]*string) map[string]string {
	tags := make(map[string]string, len(azureTags))
	for k, v := range azureTags {
		if v != nil {
			tags[k] = *v
		}
	}
	return tags
}

// TODO(gavin): make the type private and provide a constructor
// the constructor should make the type and check various properties for nil.
// it should also initialize the tags to a map[string]string
