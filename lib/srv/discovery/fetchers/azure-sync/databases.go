package azure_sync

import (
	"context"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (a *azureFetcher) fetchDatabases(ctx context.Context) ([]*accessgraphv1alpha.AzureManagedDatabase, error) {
	// Get the clients
	pgCli, err := a.CloudClients.GetAzurePostgresClient(a.GetSubscriptionID())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	mysqlCli, err := a.CloudClients.GetAzureMySQLClient(a.GetSubscriptionID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the DB servers
	servers := make([]*azure.DBServer, 0)
	newServers, err := pgCli.ListAll(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers = append(servers, newServers...)
	newServers, err = mysqlCli.ListAll(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers = append(servers, newServers...)

	// Convert to protobuf
	pbDbs := make([]*accessgraphv1alpha.AzureManagedDatabase, 0, len(servers))
	for _, server := range servers {
		pbDb := accessgraphv1alpha.AzureManagedDatabase{
			Id:             server.ID,
			SubscriptionId: a.GetSubscriptionID(),
			LastSyncTime:   timestamppb.Now(),
			Name:           server.Name,
		}
		pbDbs = append(pbDbs, &pbDb)
	}
	return pbDbs, nil
}
