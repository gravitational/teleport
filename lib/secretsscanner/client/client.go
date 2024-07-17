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
	"crypto/tls"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
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

	grpcConn, err := proxyConn(ctx, cfg)
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

// proxyConn attempts to connect to the proxy insecure grpc server.
// The Proxy's TLS cert will be verified using the host's root CA pool
// (PKI) unless the --insecure flag was passed.
func proxyConn(
	ctx context.Context, params ClientConfig,
) (*grpc.ClientConn, error) {
	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now
	// set NextProtos for TLS routing, the actual protocol will be h2
	tlsConfig.NextProtos = []string{string(common.ProtocolProxyGRPCInsecure), http2.NextProtoTLS}

	if params.Insecure {
		tlsConfig.InsecureSkipVerify = true
		params.Log.WarnContext(ctx, "Connecting to the cluster without validating the identity of the Proxy Server.")
	}

	// Check if proxy is behind a load balancer. If so, the connection upgrade
	// will verify the load balancer's cert using system cert pool. This
	// provides the same level of security as the client only verifies Proxy's
	// web cert against system cert pool when connection upgrade is not
	// required.
	//
	// With the ALPN connection upgrade, the tunneled TLS Routing request will
	// skip verify as the Proxy server will present its host cert which is not
	// fully verifiable at this point since the client does not have the host
	// CAs yet before completing registration.
	alpnConnUpgrade := client.IsALPNConnUpgradeRequired(ctx, params.ProxyServer, params.Insecure)
	if alpnConnUpgrade && !params.Insecure {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = verifyALPNUpgradedConn(params.Clock)
	}

	dialer := client.NewDialer(
		ctx,
		apidefaults.DefaultIdleTimeout,
		apidefaults.DefaultIOTimeout,
		client.WithInsecureSkipVerify(params.Insecure),
		client.WithALPNConnUpgrade(alpnConnUpgrade),
	)

	conn, err := grpc.NewClient(
		params.ProxyServer,
		grpc.WithContextDialer(client.GRPCContextDialer(dialer)),
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	return conn, trace.Wrap(err)
}

// verifyALPNUpgradedConn is a tls.Config.VerifyConnection callback function
// used by the tunneled TLS Routing request to verify the host cert of a Proxy
// behind a L7 load balancer.
//
// Since the client has not obtained the cluster CAs at this point, the
// presented cert cannot be fully verified yet. For now, this function only
// checks if "teleport.cluster.local" is present as one of the DNS names and
// verifies the cert is not expired.
func verifyALPNUpgradedConn(clock clockwork.Clock) func(tls.ConnectionState) error {
	return func(server tls.ConnectionState) error {
		for _, cert := range server.PeerCertificates {
			if slices.Contains(cert.DNSNames, constants.APIDomain) && clock.Now().Before(cert.NotAfter) {
				return nil
			}
		}
		return trace.AccessDenied("server is not a Teleport proxy or server certificate is expired")
	}
}
