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

package client

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
)

// Client is a client for the SecretsScannerService.
type Client interface {
	// ReportSecrets is used by trusted devices to report secrets found on the host that could be used to bypass Teleport.
	// The client (device) should first authenticate using the [ReportSecretsRequest.device_assertion] flow. Please refer to
	// the [teleport.devicetrust.v1.AssertDeviceRequest] and [teleport.devicetrust.v1.AssertDeviceResponse] messages for more details.
	//
	// Once the device is asserted, the client can send the secrets using the [ReportSecretsRequest.private_keys] field
	// and then close the client side of the stream.
	//
	// -> ReportSecrets (client) [1 or more]
	// -> CloseStream (client)
	// <- TerminateStream (server)
	//
	// Any failure in the assertion ceremony will result in the stream being terminated by the server. All secrets
	// reported by the client before the assertion terminates will be ignored and result in the stream being terminated.
	ReportSecrets(ctx context.Context, opts ...grpc.CallOption) (accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient, error)
	// Close closes the client connection.
	Close() error
}

// ClientConfig specifies parameters for the client to dial credentialless via the proxy.
type ClientConfig struct {
	// ProxyServer is the address of the proxy server
	ProxyServer string
	// CipherSuites is a list of cipher suites to use for TLS client connection
	CipherSuites []uint16
	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification.
	// Defaults to real clock if unspecified
	Clock clockwork.Clock
	// Insecure trusts the certificates from the Auth Server or Proxy during registration without verification.
	Insecure bool
	// Log is the logger.
	Log *slog.Logger
}

// NewSecretsScannerServiceClient creates a new SecretsScannerServiceClient that connects to the proxy
// gRPC server that does not require authentication (credentialless) to report secrets found during scanning.
func NewSecretsScannerServiceClient(ctx context.Context, cfg ClientConfig) (Client, error) {
	if cfg.ProxyServer == "" {
		return nil, trace.BadParameter("missing ProxyServer")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	grpcConn, err := proxyinsecureclient.NewConnection(
		ctx,
		proxyinsecureclient.ConnectionConfig{
			ProxyServer:  cfg.ProxyServer,
			CipherSuites: cfg.CipherSuites,
			Clock:        cfg.Clock,
			Insecure:     cfg.Insecure,
			Log:          cfg.Log,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to the proxy")
	}

	return &secretsSvcClient{
		SecretsScannerServiceClient: accessgraphsecretsv1pb.NewSecretsScannerServiceClient(grpcConn),
		conn:                        grpcConn,
	}, nil
}

type secretsSvcClient struct {
	accessgraphsecretsv1pb.SecretsScannerServiceClient
	conn *grpc.ClientConn
}

func (c *secretsSvcClient) Close() error {
	return c.conn.Close()
}
