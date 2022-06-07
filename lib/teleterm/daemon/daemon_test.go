package daemon

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

type mockCluster struct {
	cluster
	servers []clusters.Server
}

func (mc *mockCluster) GetServers(ctx context.Context) ([]clusters.Server, error) {
	return mc.servers, nil
}

type mockStorage struct {
	Storage
	clusterToReturn cluster
}

func (ms *mockStorage) GetByURI(uri string) (cluster, error) {
	return ms.clusterToReturn, nil
}

func TestService_ListServers(t *testing.T) {
	s := &Service{
		Config: Config{
			Storage: &mockStorage{
				clusterToReturn: &mockCluster{
					servers: []clusters.Server{{}},
				},
			},
		},
	}

	ctx := context.Background()
	s.ListServers(ctx, "a uri")
}
