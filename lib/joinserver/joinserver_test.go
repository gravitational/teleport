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
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mockJoinServiceClient struct {
	sendChallenge        string
	returnCerts          *proto.Certs
	returnError          error
	gotChallengeResponse *proto.RegisterUsingIAMMethodRequest
}

func (c *mockJoinServiceClient) RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterChallengeResponseFunc) (*proto.Certs, error) {
	resp, err := challengeResponse(c.sendChallenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.gotChallengeResponse = resp
	return c.returnCerts, c.returnError
}

func newGRPCServer(t *testing.T) (*grpc.Server, *bufconn.Listener) {
	lis := bufconn.Listen(1024)
	s := grpc.NewServer(
		grpc.UnaryInterceptor(utils.ErrorConvertUnaryInterceptor),
		grpc.StreamInterceptor(utils.ErrorConvertStreamInterceptor),
	)
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

func TestJoinServiceGRPCServer_RegisterUsingIAMMethod(t *testing.T) {
	// create a mock auth server which implements RegisterUsingIAMMethod
	mockAuthServer := &mockJoinServiceClient{}

	// create the first instance of JoinServiceGRPCServer wrapping the mock auth
	// server, to imitate the JoinServiceGRPCServer which runs on Auth
	authGRPCServer, authGRPCListener := newGRPCServer(t)
	authJoinService := NewJoinServiceGRPCServer(mockAuthServer)
	proto.RegisterJoinServiceServer(authGRPCServer, authJoinService)

	// create a client to the "auth" gRPC service
	authConn := newGRPCConn(t, authGRPCListener)
	authJoinServiceClient := client.NewJoinServiceClient(proto.NewJoinServiceClient(authConn))

	// create a second instance of JoinServiceGRPCServer wrapping the "auth"
	// gRPC client, to imitate the JoinServiceGRPCServer which runs on Proxy
	proxyGRPCServer, proxyGRPCListener := newGRPCServer(t)
	proxyJoinService := NewJoinServiceGRPCServer(authJoinServiceClient)
	proto.RegisterJoinServiceServer(proxyGRPCServer, proxyJoinService)

	// create a client to the "proxy" gRPC service
	proxyConn := newGRPCConn(t, proxyGRPCListener)
	proxyJoinServiceClient := client.NewJoinServiceClient(proto.NewJoinServiceClient(proxyConn))

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
			mockAuthServer.sendChallenge = tc.challenge
			mockAuthServer.returnCerts = tc.certs
			mockAuthServer.returnError = tc.authErr
			challengeResponder := func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
				// client should get the challenge from auth
				require.Equal(t, tc.challenge, challenge)
				return tc.challengeResponse, tc.challengeResponseErr
			}

			for suffix, clt := range map[string]*client.JoinServiceClient{
				"_auth":  authJoinServiceClient,
				"_proxy": proxyJoinServiceClient,
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
					require.Equal(t, tc.challengeResponse, mockAuthServer.gotChallengeResponse)
				})
			}
		})
	}
}
