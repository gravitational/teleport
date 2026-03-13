// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package debugv1

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

// ClusterDialer returns a cluster for tunnel dialing.
type ClusterDialer interface {
	Cluster(ctx context.Context, clusterName string) (reversetunnelclient.Cluster, error)
}

// LazyClusterDialer wraps a ClusterDialer that may not be available at creation
// time. Used when auth and proxy run in the same process but are initialized
// at different times.
type LazyClusterDialer struct {
	mu     sync.RWMutex
	dialer ClusterDialer
}

// Set sets the underlying cluster dialer.
func (d *LazyClusterDialer) Set(cd ClusterDialer) {
	d.mu.Lock()
	d.dialer = cd
	d.mu.Unlock()
}

// Cluster returns the cluster matching the provided name. Returns a NotFound
// error if the dialer has not been set yet.
func (d *LazyClusterDialer) Cluster(ctx context.Context, name string) (reversetunnelclient.Cluster, error) {
	d.mu.RLock()
	cd := d.dialer
	d.mu.RUnlock()
	if cd == nil {
		return nil, trace.NotFound("cluster dialer not yet initialized")
	}
	return cd.Cluster(ctx, name)
}

// LazyLocalDebugDialer provides a local connection path to a node's debug
// HTTP service for combined processes where no reverse tunnel is available.
type LazyLocalDebugDialer struct {
	mu       sync.RWMutex
	listener localDebugListener
	hostID   string
}

// localDebugListener is the interface needed to send a connection to the
// node's debug HTTP server.
type localDebugListener interface {
	HandleConnection(conn net.Conn)
}

// Set configures the local debug listener and host ID.
func (d *LazyLocalDebugDialer) Set(listener localDebugListener, hostID string) {
	d.mu.Lock()
	d.listener = listener
	d.hostID = hostID
	d.mu.Unlock()
}

// Dial creates an in-process connection to the local debug HTTP server
// if any of the candidate IDs match the local host ID. Returns nil, false
// if not a local target.
func (d *LazyLocalDebugDialer) Dial(candidateIDs ...string) (net.Conn, bool) {
	d.mu.RLock()
	lis := d.listener
	hid := d.hostID
	d.mu.RUnlock()
	if lis == nil {
		return nil, false
	}
	for _, id := range candidateIDs {
		if id == hid {
			client, server := net.Pipe()
			lis.HandleConnection(server)
			return client, true
		}
	}
	return nil, false
}

// ServiceConfig holds configuration options for the debug gRPC service.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	// ClusterDialer provides access to the reverse tunnel for routing
	// debug connections to target nodes.
	ClusterDialer ClusterDialer
	// ClusterName is the local cluster name for tunnel routing.
	ClusterName string
	// LocalDebugDialer provides local connections for combined processes.
	LocalDebugDialer *LazyLocalDebugDialer
	// Forwarder forwards debug connections to remote servers that are not
	// reachable via the local or cluster dialers. Used in split deployments
	// where the auth server needs to forward to other auth servers.
	Forwarder func(ctx context.Context, serverID string) (net.Conn, error)
}

// Service implements the teleport.debug.v1.DebugService RPC service.
// It tunnels HTTP traffic to the target node's debug HTTP service.
type Service struct {
	debugpb.UnimplementedDebugServiceServer

	authorizer       authz.Authorizer
	clusterDialer    ClusterDialer
	clusterName      string
	localDebugDialer *LazyLocalDebugDialer
	forwarder        func(ctx context.Context, serverID string) (net.Conn, error)
}

// NewService returns a new debug gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}

	return &Service{
		authorizer:       cfg.Authorizer,
		clusterDialer:    cfg.ClusterDialer,
		clusterName:      cfg.ClusterName,
		localDebugDialer: cfg.LocalDebugDialer,
		forwarder:        cfg.Forwarder,
	}, nil
}

// Connect establishes a tunneled connection to a node's debug HTTP service.
// The first frame must contain the target server_id. Subsequent frames carry
// raw HTTP data bidirectionally.
func (s *Service) Connect(stream grpc.BidiStreamingServer[debugpb.Frame, debugpb.Frame]) error {
	ctx := stream.Context()
	if err := s.authorize(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Read the first frame to get the target server ID.
	first, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	serverID := first.GetServerId()
	if serverID == "" {
		return trace.BadParameter("first frame must contain server_id")
	}

	conn, err := s.dialNode(ctx, serverID)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	// Bidirectional copy between gRPC stream and tunnel connection.
	errCh := make(chan error, 2)

	// stream → conn: forward HTTP requests from tctl to the node.
	go func() {
		for {
			frame, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			if _, err := conn.Write(frame.GetData()); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// conn → stream: forward HTTP responses from the node to tctl.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				if sendErr := stream.Send(&debugpb.Frame{
					Payload: &debugpb.Frame_Data{Data: append([]byte(nil), buf[:n]...)},
				}); sendErr != nil {
					errCh <- sendErr
					return
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Wait for either direction to finish. If the context is cancelled
	// (client disconnect), treat it as a clean shutdown.
	err = <-errCh
	if err == io.EOF || ctx.Err() != nil {
		return nil
	}
	return trace.Wrap(err)
}

// authzContextProvider is implemented by gRPC AuthInfo types that carry
// a pre-authorized context from the transport handshake.
type authzContextProvider interface {
	AuthzContext() (*authz.Context, bool)
}

// authorize checks that the caller has permission to use the debug service.
// It supports two authorization paths:
//  1. Standard gRPC servers where the user identity is set in the context
//     via middleware (e.g. auth server's main gRPC).
//  2. The proxy's SSH gRPC server where the identity is in the peer's
//     AuthInfo (set during TLS handshake by TransportCredentials).
func (s *Service) authorize(ctx context.Context) error {
	// Path 1: try the standard authorizer (works when middleware sets the user in context).
	authCtx, err := s.authorizer.Authorize(ctx)
	if err == nil {
		return authCtx.CheckAccessToKind(types.KindDebugService, types.VerbCreate)
	}

	// Path 2: extract auth context from the peer's transport credentials.
	// The proxy's SSH gRPC server uses TransportCredentials that store
	// the pre-authorized context in the peer's AuthInfo.
	p, ok := peer.FromContext(ctx)
	if !ok {
		return trace.Wrap(err)
	}
	provider, ok := p.AuthInfo.(authzContextProvider)
	if !ok {
		return trace.Wrap(err)
	}
	authCtx, ok = provider.AuthzContext()
	if !ok {
		return trace.Wrap(err)
	}
	return authCtx.CheckAccessToKind(types.KindDebugService, types.VerbCreate)
}

// dialNode dials the target node's debug HTTP service through the reverse
// tunnel. For combined processes it uses a local in-process connection.
// In split deployments where neither is available, it falls back to the
// forwarder which routes through other auth servers.
func (s *Service) dialNode(ctx context.Context, serverID string) (net.Conn, error) {
	// For combined processes (auth+node in same process), try a local
	// in-process connection before falling back to the reverse tunnel.
	if s.localDebugDialer != nil {
		qualifiedID := serverID + "." + s.clusterName
		if conn, ok := s.localDebugDialer.Dial(serverID, qualifiedID); ok {
			return conn, nil
		}
	}

	if s.clusterDialer != nil {
		cluster, err := s.clusterDialer.Cluster(ctx, s.clusterName)
		if err == nil {
			conn, err := cluster.DialTCP(reversetunnelclient.DialParams{
				ServerID: serverID + "." + s.clusterName,
				ConnType: types.DebugTunnel,
			})
			if err == nil {
				return conn, nil
			}
		}
	}

	// Fall back to the forwarder for split deployments where the cluster
	// dialer is not available (auth and proxy run in separate processes).
	if s.forwarder != nil {
		conn, err := s.forwarder(ctx, serverID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	}

	return nil, trace.NotImplemented("debug service cannot reach server %s: no reverse tunnel or forwarding path available", serverID)
}
