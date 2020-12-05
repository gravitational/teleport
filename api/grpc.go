package api

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/proto/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// ConnectGRPC establishes a grpc connection for the client, if it hasn't done so
// yet. This can be used to connect for lazy loading the connection.
func (c *Client) ConnectGRPC() error {
	// it's ok to lock here, because Dial below is not locking
	c.Lock()
	defer c.Unlock()

	if c.grpc != nil {
		return nil
	}

	dialer := grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		if c.isClosed() {
			return nil, trace.ConnectionProblem(nil, "client is closed")
		}
		c, err := c.Dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			log.Debugf("Dial to addr %v failed: %v.", addr, err)
		}
		return c, err
	})
	tlsConfig := c.TLS.Clone()
	tlsConfig.NextProtos = []string{http2.NextProtoTLS}
	log.Debugf("GRPC(CLIENT): keep alive %v count: %v.", c.KeepAlivePeriod, c.KeepAliveCount)
	conn, err := grpc.Dial(teleport.APIDomain,
		dialer,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.KeepAlivePeriod,
			Timeout:             c.KeepAlivePeriod * time.Duration(c.KeepAliveCount),
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return trail.FromGRPC(err)
	}
	c.conn = conn
	c.grpc = auth.NewAuthServiceClient(c.conn)
	return nil
}

// GetGRPC is a getter method for the client's AuthServiceClient.
// TODO: Once grpc client is factored out of /lib, this can be removed.
func (c *Client) GetGRPC() (auth.AuthServiceClient, error) {
	if err := c.ConnectGRPC(); err != nil {
		return nil, trace.Wrap(err)
	}

	return c.grpc, nil
}

// GetUsers returns a list of users
func (c *Client) GetUsers(withSecrets bool) ([]services.User, error) {
	if err := c.ConnectGRPC(); err != nil {
		return []services.User{}, err
	}
	stream, err := c.grpc.GetUsers(context.TODO(), &auth.GetUsersRequest{
		WithSecrets: withSecrets,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	var users []services.User
	for {
		user, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, trail.FromGRPC(err)
		}
		users = append(users, user)
	}
	return users, nil
}

// Ping gets basic info about the auth server.
func (c *Client) Ping(ctx context.Context) (auth.PingResponse, error) {
	if err := c.ConnectGRPC(); err != nil {
		return auth.PingResponse{}, err
	}
	rsp, err := c.grpc.Ping(ctx, &auth.PingRequest{})
	if err != nil {
		return auth.PingResponse{}, trail.FromGRPC(err)
	}
	return *rsp, nil
}
