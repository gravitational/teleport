/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package joinserver

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/utils"
)

type mockJoinServiceClient struct {
	sendChallenge             string
	returnCerts               *proto.Certs
	returnError               error
	gotIAMChallengeResponse   *proto.RegisterUsingIAMMethodRequest
	gotAzureChallengeResponse *proto.RegisterUsingAzureMethodRequest
}

func (c *mockJoinServiceClient) RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterIAMChallengeResponseFunc) (*proto.Certs, error) {
	resp, err := challengeResponse(c.sendChallenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gotIAMChallengeResponse = resp
	return c.returnCerts, c.returnError
}

func (c *mockJoinServiceClient) RegisterUsingAzureMethod(ctx context.Context, challengeResponse client.RegisterAzureChallengeResponseFunc) (*proto.Certs, error) {
	resp, err := challengeResponse(c.sendChallenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gotAzureChallengeResponse = resp
	return c.returnCerts, c.returnError
}

func ConnectionCountingStreamInterceptor(count *atomic.Int32) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		count.Add(1)
		defer count.Add(-1)
		return handler(srv, ss)
	}
}

func newGRPCServer(t *testing.T, opts ...grpc.ServerOption) (*grpc.Server, *bufconn.Listener) {
	lis := bufconn.Listen(1024)
	opts = append(opts,
		grpc.ChainUnaryInterceptor(utils.GRPCServerUnaryErrorInterceptor),
		grpc.ChainStreamInterceptor(utils.GRPCServerStreamErrorInterceptor),
	)
	s := grpc.NewServer(opts...)
	return s, lis
}

func newGRPCConn(t *testing.T, l *bufconn.Listener) *grpc.ClientConn {
	conn, err := grpc.DialContext(
		context.Background(),
		"bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return l.DialContext(ctx)
		}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return conn
}

type testPack struct {
	authClient, proxyClient         *client.JoinServiceClient
	authGRPCClient, proxyGRPCClient proto.JoinServiceClient
	authServer, proxyServer         *JoinServiceGRPCServer
	streamConnectionCount           *atomic.Int32
	mockAuthServer                  *mockJoinServiceClient
}

func newTestPack(t *testing.T) *testPack {
	// create a mock auth server which implements RegisterUsingIAMMethod
	mockAuthServer := &mockJoinServiceClient{}

	streamConnectionCount := &atomic.Int32{}

	// create the first instance of JoinServiceGRPCServer wrapping the mock auth
	// server, to imitate the JoinServiceGRPCServer which runs on Auth
	authGRPCServer, authGRPCListener := newGRPCServer(t, grpc.ChainStreamInterceptor(ConnectionCountingStreamInterceptor(streamConnectionCount)))
	authServer := NewJoinServiceGRPCServer(mockAuthServer)
	proto.RegisterJoinServiceServer(authGRPCServer, authServer)

	// create a client to the "auth" gRPC service
	authConn := newGRPCConn(t, authGRPCListener)
	authGRPCClient := proto.NewJoinServiceClient(authConn)
	authJoinServiceClient := client.NewJoinServiceClient(authGRPCClient)

	// create a second instance of JoinServiceGRPCServer wrapping the "auth"
	// gRPC client, to imitate the JoinServiceGRPCServer which runs on Proxy
	proxyGRPCServer, proxyGRPCListener := newGRPCServer(t, grpc.ChainStreamInterceptor(ConnectionCountingStreamInterceptor(streamConnectionCount)))
	proxyServer := NewJoinServiceGRPCServer(authJoinServiceClient)
	proto.RegisterJoinServiceServer(proxyGRPCServer, proxyServer)

	// create a client to the "proxy" gRPC service
	proxyConn := newGRPCConn(t, proxyGRPCListener)
	proxyGRPCClient := proto.NewJoinServiceClient(proxyConn)
	proxyJoinServiceClient := client.NewJoinServiceClient(proxyGRPCClient)

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		authGRPCServer.Serve(authGRPCListener)
	}()
	go func() {
		defer wg.Done()
		proxyGRPCServer.Serve(proxyGRPCListener)
	}()
	t.Cleanup(func() {
		// stop gRPC servers and make sure goroutines exit
		authGRPCServer.Stop()
		proxyGRPCServer.Stop()
		wg.Wait()
	})

	return &testPack{
		authServer:            authServer,
		proxyServer:           proxyServer,
		authGRPCClient:        authGRPCClient,
		authClient:            authJoinServiceClient,
		proxyGRPCClient:       proxyGRPCClient,
		proxyClient:           proxyJoinServiceClient,
		streamConnectionCount: streamConnectionCount,
		mockAuthServer:        mockAuthServer,
	}
}

func TestJoinServiceGRPCServer_RegisterUsingIAMMethod(t *testing.T) {
	t.Parallel()
	testPack := newTestPack(t)

	testCases := []struct {
		desc                 string
		challenge            string
		challengeResponse    *proto.RegisterUsingIAMMethodRequest
		challengeResponseErr error
		authErr              error
		certs                *proto.Certs
	}{
		{
			desc:              "pass case",
			challenge:         "foo",
			challengeResponse: &proto.RegisterUsingIAMMethodRequest{StsIdentityRequest: []byte("bar")},
			certs:             &proto.Certs{SSH: []byte("baz")},
		},
		{
			desc:              "auth error",
			challenge:         "foo",
			challengeResponse: &proto.RegisterUsingIAMMethodRequest{StsIdentityRequest: []byte("bar")},
			authErr:           trace.AccessDenied("test auth error"),
		},
		{
			desc:                 "challenge response error",
			challenge:            "foo",
			challengeResponseErr: trace.BadParameter("test challenge error"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			testPack.mockAuthServer.sendChallenge = tc.challenge
			testPack.mockAuthServer.returnCerts = tc.certs
			testPack.mockAuthServer.returnError = tc.authErr
			challengeResponder := func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
				// client should get the challenge from auth
				require.Equal(t, tc.challenge, challenge)
				return tc.challengeResponse, tc.challengeResponseErr
			}

			for suffix, clt := range map[string]*client.JoinServiceClient{
				"_auth":  testPack.authClient,
				"_proxy": testPack.proxyClient,
			} {
				t.Run(tc.desc+suffix, func(t *testing.T) {
					certs, err := clt.RegisterUsingIAMMethod(context.Background(), challengeResponder)
					if tc.challengeResponseErr != nil {
						// error returned to the client should equal the error
						// returned from the challenge responder
						require.Equal(t, tc.challengeResponseErr, err)
						return
					}
					if tc.authErr != nil {
						// error returned to the client should contain the error
						// returned from the auth server, wrapped with gRPC
						// errors
						require.Contains(t, err.Error(), tc.authErr.Error())
						return
					}
					require.NoError(t, err)
					// client should get the certs from auth
					require.Equal(t, tc.certs, certs)
					// auth should get the challenge response from client
					require.Equal(t, tc.challengeResponse, testPack.mockAuthServer.gotIAMChallengeResponse)
				})
			}
		})
	}
}

func TestJoinServiceGRPCServer_RegisterUsingAzureMethod(t *testing.T) {
	t.Parallel()
	testPack := newTestPack(t)

	testCases := []struct {
		desc                 string
		challenge            string
		challengeResponse    *proto.RegisterUsingAzureMethodRequest
		challengeResponseErr error
		authErr              error
		certs                *proto.Certs
	}{
		{
			desc:              "pass case",
			challenge:         "foo",
			challengeResponse: &proto.RegisterUsingAzureMethodRequest{AttestedData: []byte("bar"), AccessToken: "baz"},
			certs:             &proto.Certs{SSH: []byte("qux")},
		},
		{
			desc:              "auth error",
			challenge:         "foo",
			challengeResponse: &proto.RegisterUsingAzureMethodRequest{AttestedData: []byte("bar"), AccessToken: "baz"},
			authErr:           trace.AccessDenied("test auth error"),
		},
		{
			desc:                 "challenge response error",
			challenge:            "foo",
			challengeResponseErr: trace.BadParameter("test challenge error"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			testPack.mockAuthServer.sendChallenge = tc.challenge
			testPack.mockAuthServer.returnCerts = tc.certs
			testPack.mockAuthServer.returnError = tc.authErr
			challengeResponder := func(challenge string) (*proto.RegisterUsingAzureMethodRequest, error) {
				require.Equal(t, tc.challenge, challenge)
				return tc.challengeResponse, tc.challengeResponseErr
			}

			for suffix, clt := range map[string]*client.JoinServiceClient{
				"_auth":  testPack.authClient,
				"_proxy": testPack.proxyClient,
			} {
				t.Run(tc.desc+suffix, func(t *testing.T) {
					certs, err := clt.RegisterUsingAzureMethod(context.Background(), challengeResponder)
					if tc.challengeResponseErr != nil {
						require.Equal(t, tc.challengeResponseErr, err)
						return
					}
					if tc.authErr != nil {
						require.Contains(t, err.Error(), tc.authErr.Error())
						return
					}
					require.NoError(t, err)
					require.Equal(t, tc.certs, certs)
					require.Equal(t, tc.challengeResponse, testPack.mockAuthServer.gotAzureChallengeResponse)
				})
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	testPack := newTestPack(t)

	fakeClock := clockwork.NewFakeClock()
	testPack.authServer.clock = fakeClock
	testPack.proxyServer.clock = fakeClock

	for _, tc := range []struct {
		desc string
		clt  *client.JoinServiceClient
	}{
		{
			desc: "good auth client",
			clt:  testPack.authClient,
		},
		{
			desc: "good proxy client",
			clt:  testPack.proxyClient,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// When a well-behaved client returns an error responding to the
			// challenge, the client should cancel the context immediately and all
			// open stream connections should quickly be closed, much before the
			// request timeout has to come into effect.
			tc.clt.RegisterUsingIAMMethod(context.Background(), func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
				return nil, trace.BadParameter("")
			})
			require.Eventually(t, func() bool {
				return testPack.streamConnectionCount.Load() == 0
			}, 10*time.Second, 1*time.Millisecond)
			// ^ This timeout is absurdly large but I really don't want this to
			// be flaky in CI. This test is still pretty fast most of the time and
			// still tests what it is meant to - if the connections never close
			// this would still fail.
		})
	}

	for _, tc := range []struct {
		desc string
		clt  proto.JoinServiceClient
	}{
		{
			desc: "bad auth client",
			clt:  testPack.authGRPCClient,
		},
		{
			desc: "bad proxy client",
			clt:  testPack.proxyGRPCClient,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			srv, err := tc.clt.RegisterUsingIAMMethod(context.Background())
			require.NoError(t, err)

			_, err = srv.Recv()
			require.NoError(t, err)

			// Sanity check there are some open connections after the first gRPC
			// Recv
			require.Greater(t, testPack.streamConnectionCount.Load(), int32(0))

			// Instead of sending a challenge response, a poorly behaved client
			// might just hang and never close the connection.
			//
			// Make sure the request is automatically timed out on the server and all
			// connections are closed shortly after the timeout.
			fakeClock.Advance(iamJoinRequestTimeout)
			require.Eventually(t, func() bool {
				return testPack.streamConnectionCount.Load() == 0
			}, 10*time.Second, 1*time.Millisecond)
			// ^ This timeout is absurdly large but I really don't want this to
			// be flaky in CI. This test is still pretty fast most of the time and
			// still tests what it is meant to - if the connections never close
			// this would still fail.
		})
	}
}
