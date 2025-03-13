/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package daemon

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys/piv"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/cmd"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/teleterm/gatewaytest"
	"github.com/gravitational/teleport/lib/tlsca"
)

type mockGatewayCreator struct {
	t                *testing.T
	callCount        int
	tcpPortAllocator gateway.TCPPortAllocator
}

func (m *mockGatewayCreator) CreateGateway(ctx context.Context, params clusters.CreateGatewayParams) (gateway.Gateway, error) {
	m.callCount++

	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	m.t.Cleanup(func() {
		hs.Close()
	})

	ca := gatewaytest.MustGenCACert(m.t)
	identity := tlsca.Identity{
		Username: "user",
		Groups:   []string{"test-group"},
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: params.TargetURI.GetDbName(),
			Protocol:    defaults.ProtocolPostgres,
			Username:    params.TargetUser,
		},
		KubernetesCluster: params.TargetURI.GetKubeName(),
	}

	targetURI := params.TargetURI

	config := gateway.Config{
		LocalPort:             params.LocalPort,
		TargetURI:             params.TargetURI,
		TargetUser:            params.TargetUser,
		TargetName:            cmp.Or(targetURI.GetDbName(), targetURI.GetKubeName(), targetURI.GetAppName()),
		TargetSubresourceName: params.TargetSubresourceName,
		Protocol:              defaults.ProtocolPostgres,
		Insecure:              true,
		WebProxyAddr:          hs.Listener.Addr().String(),
		TCPPortAllocator:      m.tcpPortAllocator,
		KubeconfigsDir:        m.t.TempDir(),
		Cert:                  gatewaytest.MustGenCertSignedWithCA(m.t, ca, identity),
	}

	gateway, err := gateway.New(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.t.Cleanup(func() {
		if err := gateway.Close(); err != nil {
			m.t.Logf("Ignoring error from gateway.Close() during cleanup, it appears the gateway was already closed. The error was: %s", err)
		}
	})

	return gateway, nil
}

type gatewayCRUDTestContext struct {
	nameToGateway        map[string]gateway.Gateway
	mockGatewayCreator   *mockGatewayCreator
	mockTCPPortAllocator *gatewaytest.MockTCPPortAllocator
}

func TestGatewayCRUD(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		gatewayNamesToCreate   []string
		appendGatewayTargetURI func(name string) uri.ResourceURI
		// tcpPortAllocator is an optional field which lets us provide a custom
		// gatewaytest.MockTCPPortAllocator with some ports already in use.
		tcpPortAllocator *gatewaytest.MockTCPPortAllocator
		testFunc         func(*testing.T, *gatewayCRUDTestContext, *Service)
	}{
		{
			name:                   "create then find",
			gatewayNamesToCreate:   []string{"gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				createdGateway := c.nameToGateway["gateway"]
				foundGateway, err := daemon.findGateway(createdGateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, createdGateway, foundGateway)
			},
		},
		{
			name:                   "ListGateways",
			gatewayNamesToCreate:   []string{"gateway1", "gateway2"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gateways := daemon.ListGateways()
				gatewayURIs := map[uri.ResourceURI]struct{}{}

				for _, gateway := range gateways {
					gatewayURIs[gateway.URI()] = struct{}{}
				}

				require.Len(t, gateways, 2)
				require.Contains(t, gatewayURIs, c.nameToGateway["gateway1"].URI())
				require.Contains(t, gatewayURIs, c.nameToGateway["gateway2"].URI())
			},
		},
		{
			name:                   "RemoveGateway",
			gatewayNamesToCreate:   []string{"gatewayToRemove", "gatewayToKeep"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gatewayToRemove := c.nameToGateway["gatewayToRemove"]
				gatewayToKeep := c.nameToGateway["gatewayToKeep"]
				err := daemon.RemoveGateway(gatewayToRemove.URI().String())
				require.NoError(t, err)

				_, err = daemon.findGateway(gatewayToRemove.URI().String())
				require.True(t, trace.IsNotFound(err), "gatewayToRemove wasn't removed")

				_, err = daemon.findGateway(gatewayToKeep.URI().String())
				require.NoError(t, err)
			},
		},
		{
			name:                   "SetGatewayLocalPort closes previous gateway if new port is free",
			gatewayNamesToCreate:   []string{"gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				oldGateway := c.nameToGateway["gateway"]
				oldListener := c.mockTCPPortAllocator.RecentListener()

				require.Equal(t, 0, oldListener.CloseCallCount)

				updatedGateway, err := daemon.SetGatewayLocalPort(oldGateway.URI().String(), "12345")
				require.NoError(t, err)
				require.Equal(t, "12345", updatedGateway.LocalPort())
				updatedGatewayAddress := c.mockTCPPortAllocator.RecentListener().RealAddr().String()

				// Check if the restarted gateway is still available under the same URI.
				foundGateway, err := daemon.findGateway(oldGateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, oldGateway.URI(), foundGateway.URI())

				// Verify that the gateway accepts connections on the new address.
				gatewaytest.BlockUntilGatewayAcceptsConnections(t, updatedGatewayAddress)

				// Verify that the old listener was closed.
				require.Equal(t, 1, oldListener.CloseCallCount)
			},
		},
		{
			name:                   "SetGatewayLocalPort doesn't close or modify previous gateway if new port is occupied",
			gatewayNamesToCreate:   []string{"gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			tcpPortAllocator:       &gatewaytest.MockTCPPortAllocator{PortsInUse: []string{"12345"}},
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gateway := c.nameToGateway["gateway"]
				gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())
				listener := c.mockTCPPortAllocator.RecentListener()

				require.Equal(t, 0, listener.CloseCallCount)

				_, err := daemon.SetGatewayLocalPort(gateway.URI().String(), "12345")
				require.ErrorContains(t, err, "address already in use")

				// Verify that the gateway still accepts connections on the old address.
				require.Equal(t, 0, listener.CloseCallCount)
				gatewaytest.BlockUntilGatewayAcceptsConnections(t, gatewayAddress)
			},
		},
		{
			name:                   "SetGatewayLocalPort is a noop if new port is equal to old port",
			gatewayNamesToCreate:   []string{"gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				gateway := c.nameToGateway["gateway"]
				localPort := gateway.LocalPort()
				require.Equal(t, 1, c.mockTCPPortAllocator.CallCount)

				_, err := daemon.SetGatewayLocalPort(gateway.URI().String(), localPort)
				require.NoError(t, err)

				require.Equal(t, 1, c.mockTCPPortAllocator.CallCount)
			},
		},
		{
			name:                   "CreateGateway returns existing kube gateway if targetURI is the same",
			gatewayNamesToCreate:   []string{"kube-gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendKube,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				wantGateway := c.nameToGateway["kube-gateway"]
				actualGateway, err := daemon.CreateGateway(context.Background(), CreateGatewayParams{
					TargetURI: wantGateway.TargetURI().String(),
				})
				require.NoError(t, err)
				require.Equal(t, wantGateway, actualGateway)
			},
		},
		{
			name:                   "CreateGateway returns error if db gateway already exists",
			gatewayNamesToCreate:   []string{"gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				createdGateway := c.nameToGateway["gateway"]
				_, err := daemon.CreateGateway(context.Background(), CreateGatewayParams{
					TargetURI:  createdGateway.TargetURI().String(),
					TargetUser: createdGateway.TargetUser(),
				})
				require.Error(t, err)
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
		{
			name:                   "CreateGateway returns error if app gateway already exists",
			gatewayNamesToCreate:   []string{"gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendApp,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				createdGateway := c.nameToGateway["gateway"]
				_, err := daemon.CreateGateway(context.Background(), CreateGatewayParams{
					TargetURI:             createdGateway.TargetURI().String(),
					TargetSubresourceName: createdGateway.TargetSubresourceName(),
				})
				require.Error(t, err)
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
		{
			name:                   "SetTargetSubresourceName returns error if db gateway already exists",
			gatewayNamesToCreate:   []string{"gateway"},
			appendGatewayTargetURI: uri.NewClusterURI("foo").AppendDB,
			testFunc: func(t *testing.T, c *gatewayCRUDTestContext, daemon *Service) {
				createdGateway := c.nameToGateway["gateway"]
				_, err := daemon.CreateGateway(context.Background(), CreateGatewayParams{
					TargetURI:             createdGateway.TargetURI().String(),
					TargetSubresourceName: "4242",
				})
				require.NoError(t, err)

				_, err = daemon.SetGatewayTargetSubresourceName(context.Background(),
					createdGateway.URI().String(), "4242")
				require.Error(t, err)
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.tcpPortAllocator == nil {
				tt.tcpPortAllocator = &gatewaytest.MockTCPPortAllocator{}
			}

			mockGatewayCreator := &mockGatewayCreator{
				t:                t,
				tcpPortAllocator: tt.tcpPortAllocator,
			}

			daemon, err := New(Config{
				Storage:        fakeStorage{},
				GatewayCreator: mockGatewayCreator,
				KubeconfigsDir: t.TempDir(),
				AgentsDir:      t.TempDir(),
				CreateClientCacheFunc: func(newClientFunc clientcache.NewClientFunc) (ClientCache, error) {
					return fakeClientCache{}, nil
				},
			})
			require.NoError(t, err)

			nameToGateway := make(map[string]gateway.Gateway, len(tt.gatewayNamesToCreate))

			for _, gatewayName := range tt.gatewayNamesToCreate {
				gatewayName := gatewayName
				gateway, err := daemon.CreateGateway(context.Background(), CreateGatewayParams{
					TargetURI:             tt.appendGatewayTargetURI(gatewayName).String(),
					TargetUser:            "alice",
					TargetSubresourceName: "",
					LocalPort:             "",
				})
				require.NoError(t, err)

				nameToGateway[gatewayName] = gateway
			}

			tt.testFunc(t, &gatewayCRUDTestContext{
				nameToGateway:        nameToGateway,
				mockGatewayCreator:   mockGatewayCreator,
				mockTCPPortAllocator: tt.tcpPortAllocator,
			}, daemon)
		})
	}
}

func TestUpdateTshdEventsServerAddress(t *testing.T) {
	homeDir := t.TempDir()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                homeDir,
		InsecureSkipVerify: true,
		HardwareKeyService: piv.NewYubiKeyService(context.TODO(), nil /*prompt*/),
	})
	require.NoError(t, err)

	createTshdEventsClientCredsFuncCallCount := 0
	createTshdEventsClientCredsFunc := func() (grpc.DialOption, error) {
		createTshdEventsClientCredsFuncCallCount++
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	}

	daemon, err := New(Config{
		Storage:                         storage,
		CreateTshdEventsClientCredsFunc: createTshdEventsClientCredsFunc,
		KubeconfigsDir:                  t.TempDir(),
		AgentsDir:                       t.TempDir(),
	})
	require.NoError(t, err)

	ls, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { ls.Close() })

	err = daemon.UpdateAndDialTshdEventsServerAddress(ls.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, daemon.tshdEventsClient)
	require.Equal(t, 1, createTshdEventsClientCredsFuncCallCount,
		"Expected createTshdEventsClientCredsFunc to be called exactly once")
}

func TestUpdateTshdEventsServerAddress_CredsErr(t *testing.T) {
	homeDir := t.TempDir()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                homeDir,
		InsecureSkipVerify: true,
		HardwareKeyService: piv.NewYubiKeyService(context.TODO(), nil /*prompt*/),
	})
	require.NoError(t, err)

	createTshdEventsClientCredsFunc := func() (grpc.DialOption, error) {
		return nil, trace.Errorf("Error while creating creds")
	}

	daemon, err := New(Config{
		Storage:                         storage,
		CreateTshdEventsClientCredsFunc: createTshdEventsClientCredsFunc,
		KubeconfigsDir:                  t.TempDir(),
		AgentsDir:                       t.TempDir(),
	})
	require.NoError(t, err)

	err = daemon.UpdateAndDialTshdEventsServerAddress("foo")
	require.ErrorContains(t, err, "Error while creating creds")
}

func TestRetryWithRelogin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	resolvableErr := trace.Errorf("ssh: cert has expired")
	unresolvableErr := trace.AccessDenied("")
	concurrentCallErr := trace.AlreadyExists("")
	reloginTimeoutErr := context.DeadlineExceeded
	unknownErr := trace.Errorf("foo")
	tests := []struct {
		name             string
		fnErrs           []error
		reloginErr       error
		serviceOpt       func(t *testing.T, service *Service)
		wantFnCalls      int
		wantReloginCalls int
		wantErr          error
		wantAddedMessage string
	}{
		{
			name:        "calls the function once if it returns successfully",
			wantFnCalls: 1,
		},
		{
			name:             "calls the function once if it returns error unresolvable with relogin",
			fnErrs:           []error{unresolvableErr},
			wantFnCalls:      1,
			wantReloginCalls: 0,
			wantErr:          unresolvableErr,
		},
		{
			name:             "resolves error with relogin and calls the function twice",
			fnErrs:           []error{resolvableErr},
			wantFnCalls:      2,
			wantReloginCalls: 1,
		},
		{
			name:   "fails on concurrent relogin calls",
			fnErrs: []error{concurrentCallErr},
			serviceOpt: func(t *testing.T, service *Service) {
				t.Helper()
				require.True(t, service.reloginMu.TryLock(), "Couldn't lock reloginClient")
			},
			wantFnCalls:      1,
			wantReloginCalls: 0,
			wantErr:          concurrentCallErr,
		},
		{
			name:             "fails with additional message to error on timeout during relogin",
			fnErrs:           []error{resolvableErr},
			reloginErr:       reloginTimeoutErr,
			wantFnCalls:      1,
			wantReloginCalls: 1,
			wantErr:          status.Error(codes.DeadlineExceeded, reloginTimeoutErr.Error()),
			wantAddedMessage: "the user did not refresh the session within",
		},
		{
			name:             "fails with additional message to error on unexpected error during relogin",
			fnErrs:           []error{resolvableErr},
			reloginErr:       unknownErr,
			wantFnCalls:      1,
			wantReloginCalls: 1,
			wantErr:          status.Error(codes.Unknown, unknownErr.Error()),
			wantAddedMessage: "could not refresh the session",
		},
		{
			name:             "fails if the second call to the function fails",
			fnErrs:           []error{resolvableErr, unresolvableErr},
			wantFnCalls:      2,
			wantReloginCalls: 1,
			wantErr:          unresolvableErr,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storage, err := clusters.NewStorage(clusters.Config{
				Dir:                t.TempDir(),
				InsecureSkipVerify: true,
				HardwareKeyService: piv.NewYubiKeyService(ctx, nil /*prompt*/),
			})
			require.NoError(t, err)

			daemon, err := New(Config{
				Storage: storage,
				CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
					return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
				},
				KubeconfigsDir: t.TempDir(),
				AgentsDir:      t.TempDir(),
				CreateClientCacheFunc: func(newClientFunc clientcache.NewClientFunc) (ClientCache, error) {
					return fakeClientCache{}, nil
				},
			})
			require.NoError(t, err)

			service, addr := newMockTSHDEventsServiceServer(t)
			service.reloginErr = tt.reloginErr

			err = daemon.UpdateAndDialTshdEventsServerAddress(addr)
			require.NoError(t, err)

			var fnCallCount int
			fn := func() error {
				fnCallCount++
				if fnCallCount > len(tt.fnErrs) {
					return nil
				}
				return tt.fnErrs[fnCallCount-1]
			}

			err = daemon.RetryWithRelogin(ctx, &api.ReloginRequest{}, fn)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.ErrorContains(t, err, tt.wantAddedMessage)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantFnCalls, fnCallCount,
				"Unexpected number of calls to fn")
			require.EqualValues(t, tt.wantReloginCalls, service.reloginCount.Load(),
				"Unexpected number of calls to service.Relogin")
		})
	}
}

func TestConcurrentHeadlessAuthPrompts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	storage, err := clusters.NewStorage(clusters.Config{
		Dir:                t.TempDir(),
		InsecureSkipVerify: true,
		HardwareKeyService: piv.NewYubiKeyService(ctx, nil /*prompt*/),
	})
	require.NoError(t, err)

	daemon, err := New(Config{
		Storage: storage,
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
		CreateClientCacheFunc: func(newClientFunc clientcache.NewClientFunc) (ClientCache, error) {
			return fakeClientCache{}, nil
		},
	})
	require.NoError(t, err)

	service, addr := newMockTSHDEventsServiceServer(t)
	err = daemon.UpdateAndDialTshdEventsServerAddress(addr)
	require.NoError(t, err)

	// Claim the important modal semaphore.

	customWaitDuration := 10 * time.Millisecond
	daemon.headlessAuthSemaphore.waitDuration = customWaitDuration
	err = daemon.headlessAuthSemaphore.Acquire(ctx)
	require.NoError(t, err)

	// Pending headless authentications should be blocked.

	headlessPromptErr1 := make(chan error)
	go func() {
		headlessPromptErr1 <- daemon.sendPendingHeadlessAuthentication(ctx, &types.HeadlessAuthentication{}, "")
	}()

	headlessPromptErr2 := make(chan error)
	go func() {
		headlessPromptErr2 <- daemon.sendPendingHeadlessAuthentication(ctx, &types.HeadlessAuthentication{}, "")
	}()

	select {
	case <-headlessPromptErr1:
		t.Error("sendPendingHeadlessAuthentication for the first prompt completed successfully without acquiring the semaphore")
	case <-headlessPromptErr2:
		t.Error("sendPendingHeadlessAuthentication for the second prompt completed successfully without acquiring the semaphore")
	case <-time.After(100 * time.Millisecond):
	}

	// If the request's ctx is canceled, they will unblock and return an error instead.

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	err = daemon.sendPendingHeadlessAuthentication(cancelCtx, &types.HeadlessAuthentication{}, "")
	require.Error(t, err)
	err = daemon.sendPendingHeadlessAuthentication(cancelCtx, &types.HeadlessAuthentication{}, "")
	require.Error(t, err)

	// Release the semaphore. Pending headless authentication should
	// complete successfully after a short delay between each semaphore release.

	releaseTime := time.Now()
	daemon.headlessAuthSemaphore.Release()

	var otherC chan error
	select {
	case err := <-headlessPromptErr1:
		require.NoError(t, err)
		otherC = headlessPromptErr2
	case err := <-headlessPromptErr2:
		require.NoError(t, err)
		otherC = headlessPromptErr1
	case <-time.After(time.Second):
		t.Error("important modal operations failed to acquire unclaimed semaphore")
	}

	if time.Since(releaseTime) < customWaitDuration {
		t.Error("important modal semaphore should not be acquired before waiting the specified duration")
	}

	select {
	case err := <-otherC:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Error("important modal operations failed to acquire unclaimed semaphore")
	}

	if time.Since(releaseTime) < 2*customWaitDuration {
		t.Error("important modal semaphore should not be acquired before waiting the specified duration")
	}

	require.EqualValues(t, 2, service.sendPendingHeadlessAuthenticationCount.Load(), "Unexpected number of calls to service.SendPendingHeadlessAuthentication")
}

type mockTSHDEventsService struct {
	api.UnimplementedTshdEventsServiceServer
	reloginErr                             error
	reloginCount                           atomic.Uint32
	sendNotificationCount                  atomic.Uint32
	sendPendingHeadlessAuthenticationCount atomic.Uint32
}

func newMockTSHDEventsServiceServer(t *testing.T) (service *mockTSHDEventsService, addr string) {
	t.Helper()

	tshdEventsService := &mockTSHDEventsService{}

	ls, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	api.RegisterTshdEventsServiceServer(grpcServer, tshdEventsService)

	serveErr := make(chan error)
	go func() {
		serveErr <- grpcServer.Serve(ls)
	}()

	t.Cleanup(func() {
		grpcServer.GracefulStop()

		// For test cases that did not send any grpc calls, test may finish
		// before grpcServer.Serve is called and grpcServer.Serve will return
		// grpc.ErrServerStopped.
		err := <-serveErr
		if !errors.Is(err, grpc.ErrServerStopped) {
			assert.NoError(t, err)
		}
	})

	return tshdEventsService, ls.Addr().String()
}

func (c *mockTSHDEventsService) Relogin(context.Context, *api.ReloginRequest) (*api.ReloginResponse, error) {
	c.reloginCount.Add(1)
	if c.reloginErr != nil {
		return nil, c.reloginErr
	}
	return &api.ReloginResponse{}, nil
}

func (c *mockTSHDEventsService) SendNotification(context.Context, *api.SendNotificationRequest) (*api.SendNotificationResponse, error) {
	c.sendNotificationCount.Add(1)
	return &api.SendNotificationResponse{}, nil
}

func (c *mockTSHDEventsService) SendPendingHeadlessAuthentication(context.Context, *api.SendPendingHeadlessAuthenticationRequest) (*api.SendPendingHeadlessAuthenticationResponse, error) {
	c.sendPendingHeadlessAuthenticationCount.Add(1)
	return &api.SendPendingHeadlessAuthenticationResponse{}, nil
}

func TestGetGatewayCLICommand(t *testing.T) {
	t.Parallel()

	daemon, err := New(Config{
		Storage: fakeStorage{},
		CreateTshdEventsClientCredsFunc: func() (grpc.DialOption, error) {
			return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
		},
		KubeconfigsDir: t.TempDir(),
		AgentsDir:      t.TempDir(),
		CreateClientCacheFunc: func(newClientFunc clientcache.NewClientFunc) (ClientCache, error) {
			return fakeClientCache{}, nil
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		inputGateway gateway.Gateway
		checkError   require.ErrorAssertionFunc
		checkCmds    func(*testing.T, cmd.Cmds)
	}{
		{
			name: "unsupported gateway",
			inputGateway: fakeGateway{
				targetURI: uri.NewClusterURI("profile").AppendServer("server"),
			},
			checkError: require.Error,
			checkCmds:  func(*testing.T, cmd.Cmds) {},
		},
		{
			name: "database gateway",
			inputGateway: fakeGateway{
				targetURI:       uri.NewClusterURI("profile").AppendDB("db"),
				subresourceName: "subresource-name",
			},
			checkError: require.NoError,
			checkCmds: func(t *testing.T, cmds cmd.Cmds) {
				t.Helper()
				require.Contains(t, strings.Join(cmds.Exec.Args, " "), "subresource-name")
				require.Contains(t, strings.Join(cmds.Preview.Args, " "), "subresource-name")
			},
		},
		{
			name: "kube gateway",
			inputGateway: fakeKubeGateway{
				targetURI: uri.NewClusterURI("profile").AppendKube("kube"),
			},
			checkError: require.NoError,
			checkCmds: func(t *testing.T, cmds cmd.Cmds) {
				t.Helper()
				require.Equal(t, []string{"KUBECONFIG=test.kubeconfig"}, cmds.Exec.Env)
				require.Equal(t, []string{"KUBECONFIG=test.kubeconfig"}, cmds.Preview.Env)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmds, err := daemon.GetGatewayCLICommand(context.Background(), test.inputGateway)
			test.checkError(t, err)
			test.checkCmds(t, cmds)
		})
	}
}

type fakeGateway struct {
	gateway.Gateway
	targetURI       uri.ResourceURI
	subresourceName string
}

func (m fakeGateway) TargetURI() uri.ResourceURI    { return m.targetURI }
func (m fakeGateway) TargetName() string            { return m.targetURI.GetDbName() }
func (m fakeGateway) TargetUser() string            { return "alice" }
func (m fakeGateway) TargetSubresourceName() string { return m.subresourceName }
func (m fakeGateway) Protocol() string              { return defaults.ProtocolSQLServer }
func (m fakeGateway) Log() *slog.Logger             { return nil }
func (m fakeGateway) LocalAddress() string          { return "localhost" }
func (m fakeGateway) LocalPortInt() int             { return 8888 }
func (m fakeGateway) LocalPort() string             { return "8888" }

type fakeKubeGateway struct {
	gateway.Kube
	targetURI       uri.ResourceURI
	subresourceName string
}

func (m fakeKubeGateway) TargetURI() uri.ResourceURI    { return m.targetURI }
func (m fakeKubeGateway) TargetName() string            { return m.targetURI.GetKubeName() }
func (m fakeKubeGateway) TargetUser() string            { return "alice" }
func (m fakeKubeGateway) TargetSubresourceName() string { return m.subresourceName }
func (m fakeKubeGateway) Protocol() string              { return "" }
func (m fakeKubeGateway) Log() *slog.Logger             { return nil }
func (m fakeKubeGateway) LocalAddress() string          { return "localhost" }
func (m fakeKubeGateway) LocalPortInt() int             { return 8888 }
func (m fakeKubeGateway) LocalPort() string             { return "8888" }
func (m fakeKubeGateway) KubeconfigPath() string        { return "test.kubeconfig" }

type fakeStorage struct {
	Storage
}

func (f fakeStorage) GetByResourceURI(resourceURI uri.ResourceURI) (*clusters.Cluster, *client.TeleportClient, error) {
	return &clusters.Cluster{}, &client.TeleportClient{}, nil
}

type fakeClientCache struct {
	ClientCache
}

func (f fakeClientCache) Get(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	return &client.ClusterClient{}, nil
}
