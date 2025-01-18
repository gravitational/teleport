package vnet

import (
	"context"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// clientApplicationServiceClient is a client for the client application
// service. This client is used in the Windows admin service to make requests to
// the VNet client application.
type clientApplicationServiceClient struct {
	clt  vnetv1.ClientApplicationServiceClient
	conn *grpc.ClientConn
}

func newClientApplicationServiceClient(ctx context.Context, addr string) (*clientApplicationServiceClient, error) {
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

func (c *clientApplicationServiceClient) Close() error {
	return trace.Wrap(c.conn.Close())
}

func (c *clientApplicationServiceClient) Ping(ctx context.Context) error {
	if _, err := c.clt.Ping(ctx, &vnetv1.PingRequest{}); err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

func (c *clientApplicationServiceClient) AuthenticateProcess(ctx context.Context, pipePath string) error {
	resp, err := c.clt.AuthenticateProcess(ctx, &vnetv1.AuthenticateProcessRequest{
		Version:  api.Version,
		PipePath: pipePath,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	if resp.Version != api.Version {
		return trace.BadParameter("version mismatch, user process version is %s, admin process version is %s",
			resp.Version, api.Version)
	}
	return nil
}

func (c *clientApplicationServiceClient) ResolveAppInfo(ctx context.Context, fqdn string) (*vnetv1.AppInfo, error) {
	resp, err := c.clt.ResolveAppInfo(ctx, &vnetv1.ResolveAppInfoRequest{
		Fqdn: fqdn,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.GetAppInfo(), nil
}

func (c *clientApplicationServiceClient) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) ([]byte, error) {
	resp, err := c.clt.ReissueAppCert(ctx, &vnetv1.ReissueAppCertRequest{
		AppInfo:    appInfo,
		TargetPort: uint32(targetPort),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.GetCert(), nil
}

func (c *clientApplicationServiceClient) SignForApp(ctx context.Context, req *vnetv1.SignForAppRequest) ([]byte, error) {
	resp, err := c.clt.SignForApp(ctx, req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.GetSignature(), nil
}

func (c *clientApplicationServiceClient) OnNewConnection(ctx context.Context, appKey *vnetv1.AppKey) error {
	_, err := c.clt.OnNewConnection(ctx, &vnetv1.OnNewConnectionRequest{
		AppKey: appKey,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}

func (c *clientApplicationServiceClient) OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) error {
	_, err := c.clt.OnInvalidLocalPort(ctx, &vnetv1.OnInvalidLocalPortRequest{
		AppInfo:    appInfo,
		TargetPort: uint32(targetPort),
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	return nil
}
