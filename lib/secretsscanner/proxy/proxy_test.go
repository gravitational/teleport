/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/defaults"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/fixtures"
	secretscannerclient "github.com/gravitational/teleport/lib/secretsscanner/client"
)

func TestProxy(t *testing.T) {
	// Disable the TLS routing connection upgrade
	t.Setenv(defaults.TLSRoutingConnUpgradeEnvVar, "false")

	authClient := newFakefakeSecretsScannerSvc(t)

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, authClient)
	ctx := context.Background()

	client, err := secretscannerclient.NewSecretsScannerServiceClient(ctx, secretscannerclient.ClientConfig{
		ProxyServer: lis.Addr().String(),
		Insecure:    true,
	})
	require.NoError(t, err)

	stream, err := client.ReportSecrets(ctx)
	require.NoError(t, err)

	// Send the device assertion init message
	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_Init{
					Init: &devicepb.AssertDeviceInit{},
				},
			},
		},
	})
	require.NoError(t, err)

	// Receive the device assertion challenge message
	msg, err := stream.Recv()
	require.NoError(t, err)
	assert.NotNil(t, msg.GetDeviceAssertion().GetChallenge())

	// Send the device assertion challenge response message
	err = stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
		Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceRequest{
				Payload: &devicepb.AssertDeviceRequest_ChallengeResponse{
					ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{Signature: []byte("response")},
				},
			},
		},
	})
	require.NoError(t, err)

	// Receive the device assertion response message
	msg, err = stream.Recv()
	require.NoError(t, err)
	assert.NotNil(t, msg.GetDeviceAssertion().GetDeviceAsserted())

	// Send close message
	err = stream.CloseSend()
	require.NoError(t, err)

	// Receive the termination message
	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)

}

func newFakefakeSecretsScannerSvc(t *testing.T) *fakeSecretsClient {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(server, &fakeSecretsScannerSvc{})
	go func() {
		err := server.Serve(lis)
		assert.NoError(t, err)
	}()
	t.Cleanup(server.GracefulStop)

	client, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return &fakeSecretsClient{
		SecretsScannerServiceClient: accessgraphsecretsv1pb.NewSecretsScannerServiceClient(client),
	}

}

type fakeSecretsClient struct {
	accessgraphsecretsv1pb.SecretsScannerServiceClient
}

func (s *fakeSecretsClient) AccessGraphSecretsScannerClient() accessgraphsecretsv1pb.SecretsScannerServiceClient {
	return s
}

type fakeSecretsScannerSvc struct {
	accessgraphsecretsv1pb.UnimplementedSecretsScannerServiceServer
}

func (f *fakeSecretsScannerSvc) ReportSecrets(in accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer) error {
	msg, err := in.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	if msg.GetDeviceAssertion().GetInit() == nil {
		return trace.BadParameter("missing device init")
	}

	err = in.Send(&accessgraphsecretsv1pb.ReportSecretsResponse{
		Payload: &accessgraphsecretsv1pb.ReportSecretsResponse_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceResponse{
				Payload: &devicepb.AssertDeviceResponse_Challenge{
					Challenge: &devicepb.AuthenticateDeviceChallenge{Challenge: []byte("challenge")},
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	msg, err = in.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	if msg.GetDeviceAssertion().GetChallengeResponse() == nil {
		return trace.BadParameter("missing device challenge")
	}

	err = in.Send(&accessgraphsecretsv1pb.ReportSecretsResponse{
		Payload: &accessgraphsecretsv1pb.ReportSecretsResponse_DeviceAssertion{
			DeviceAssertion: &devicepb.AssertDeviceResponse{
				Payload: &devicepb.AssertDeviceResponse_DeviceAsserted{
					DeviceAsserted: &devicepb.DeviceAsserted{},
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = in.Recv()
	if errors.Is(err, io.EOF) {
		return nil
	}
	return trace.BadParameter("unexpected message")
}

func newProxyService(t *testing.T, lis net.Listener, authClient AuthClient) {
	localTLSConfig, err := fixtures.LocalTLSConfig()
	require.NoError(t, err)

	tlsConfig := localTLSConfig.TLS.Clone()
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.ClientAuth = tls.RequestClientCert
	tlsConfig.RootCAs = nil

	s := grpc.NewServer(
		grpc.Creds(
			credentials.NewTLS(tlsConfig),
		),
	)
	t.Cleanup(s.GracefulStop)

	proxy, err := New(ServiceConfig{
		AuthClient: authClient,
	},
	)
	require.NoError(t, err)

	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(s, proxy)

	go func() {
		err := s.Serve(lis)
		assert.NoError(t, err)
	}()

}
