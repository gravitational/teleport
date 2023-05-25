package reversetunnel

import (
	"context"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

func (m mockAuthClient) Ping(ctx context.Context) (proto.PingResponse, error) {
	return proto.PingResponse{}, nil
}

func (m mockAuthClient) Close() error {
	return nil
}

func (m mockAccessPoint) Close() error {
	return nil
}

// TestRemoteClientManagerRace tests a RemoteClientManager for races.
func TestRemoteClientManagerRace(t *testing.T) {
	ctx := context.Background()
	cm, err := newRemoteClientManager(ctx, remoteClientManagerConfig{
		newClientFunc: func(ctx context.Context) (auth.ClientI, error) {
			return &mockAuthClient{}, nil
		},
		newAccessPointFunc: func(ctx context.Context, client auth.ClientI, version string) (auth.RemoteProxyAccessPoint, error) {
			return &mockAccessPoint{}, nil
		},
		newNodeWatcherFunc: func(ctx context.Context, rpap auth.RemoteProxyAccessPoint) (*services.NodeWatcher, error) {
			return nil, nil
		},
		newCAWatcher: func(ctx context.Context, rpap auth.RemoteProxyAccessPoint) (*services.CertAuthorityWatcher, error) {
			return nil, nil
		},
		log: logrus.New(),
	})
	require.NoError(t, err)

	wg := &sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Ignore errors, we expect some when the context gets canceled before connect finishes.
			_ = cm.Connect()
		}()
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cm.Connect()
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cm.Close()
		}()
	}
	wg.Wait()
}
