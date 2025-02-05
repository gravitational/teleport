// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// clientApplicationServiceClient is a gRPC client for the client application
// service. This client is used in the Windows admin service to make requests to
// the VNet client application.
type clientApplicationServiceClient struct {
	clt  vnetv1.ClientApplicationServiceClient
	conn *grpc.ClientConn
}

func newClientApplicationServiceClient(ctx context.Context, addr string) (*clientApplicationServiceClient, error) {
	// TODO(nklaassen): add mTLS credentials for client application service.
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating user process gRPC client")
	}
	return &clientApplicationServiceClient{
		clt:  vnetv1.NewClientApplicationServiceClient(conn),
		conn: conn,
	}, nil
}

func (c *clientApplicationServiceClient) close() error {
	return trace.Wrap(c.conn.Close())
}

// Ping pings the client application.
func (c *clientApplicationServiceClient) Ping(ctx context.Context) error {
	if _, err := c.clt.Ping(ctx, &vnetv1.PingRequest{}); err != nil {
		return trace.Wrap(err, "calling Ping rpc")
	}
	return nil
}

// Authenticate process authenticates the client application process.
func (c *clientApplicationServiceClient) AuthenticateProcess(ctx context.Context, pipePath string) error {
	resp, err := c.clt.AuthenticateProcess(ctx, &vnetv1.AuthenticateProcessRequest{
		Version:  api.Version,
		PipePath: pipePath,
	})
	if err != nil {
		return trace.Wrap(err, "calling AuthenticateProcess rpc")
	}
	if resp.Version != api.Version {
		return trace.BadParameter("version mismatch, user process version is %s, admin process version is %s",
			resp.Version, api.Version)
	}
	return nil
}

// ResolveAppInfo resolves fqdn to a [*vnetv1.AppInfo], or returns an error if
// no matching app is found.
func (c *clientApplicationServiceClient) ResolveAppInfo(ctx context.Context, fqdn string) (*vnetv1.AppInfo, error) {
	resp, err := c.clt.ResolveAppInfo(ctx, &vnetv1.ResolveAppInfoRequest{
		Fqdn: fqdn,
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling ResolveAppInfo rpc")
	}
	return resp.GetAppInfo(), nil
}

// ReissueAppCert issues a new certificate for the requested app.
func (c *clientApplicationServiceClient) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) ([]byte, error) {
	resp, err := c.clt.ReissueAppCert(ctx, &vnetv1.ReissueAppCertRequest{
		AppInfo:    appInfo,
		TargetPort: uint32(targetPort),
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling ReissueAppCert rpc")
	}
	return resp.GetCert(), nil
}

// SignForApp returns a cryptographic signature with the key associated with the
// requested app. The key resides in the client application process.
func (c *clientApplicationServiceClient) SignForApp(ctx context.Context, req *vnetv1.SignForAppRequest) ([]byte, error) {
	resp, err := c.clt.SignForApp(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "calling SignForApp rpc")
	}
	return resp.GetSignature(), nil
}

// OnNewConnection reports a new TCP connection to the target app.
func (c *clientApplicationServiceClient) OnNewConnection(ctx context.Context, appKey *vnetv1.AppKey) error {
	_, err := c.clt.OnNewConnection(ctx, &vnetv1.OnNewConnectionRequest{
		AppKey: appKey,
	})
	if err != nil {
		return trace.Wrap(err, "calling OnNewConnection rpc")
	}
	return nil
}

// OnInvalidLocalPort reports a failed connection to an invalid local port for
// the target app.
func (c *clientApplicationServiceClient) OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) error {
	_, err := c.clt.OnInvalidLocalPort(ctx, &vnetv1.OnInvalidLocalPortRequest{
		AppInfo:    appInfo,
		TargetPort: uint32(targetPort),
	})
	if err != nil {
		return trace.Wrap(err, "calling OnInvalidLocalPort rpc")
	}
	return nil
}

// GetTargetOSConfiguration returns the configuration values that should be
// configured in the OS, including DNS zones that should be handled by the VNet
// DNS nameserver and the IPv4 CIDR ranges that should be routed to the VNet TUN
// interface.
func (c *clientApplicationServiceClient) GetTargetOSConfiguration(ctx context.Context) (*vnetv1.TargetOSConfiguration, error) {
	resp, err := c.clt.GetTargetOSConfiguration(ctx, &vnetv1.GetTargetOSConfigurationRequest{})
	if err != nil {
		return nil, trace.Wrap(err, "calling GetTargetOSConfiguration rpc")
	}
	return resp.GetTargetOsConfiguration(), nil
}
