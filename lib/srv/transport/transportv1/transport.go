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

package transportv1

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	transportv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Dialer is the interface that groups basic dialing methods.
type Dialer interface {
	DialSite(ctx context.Context, cluster string, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error)
	DialHost(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, host, port, cluster string, identity *sshca.Identity, checker services.AccessChecker, agentGetter teleagent.Getter, singer agentless.SignerCreator) (net.Conn, error)
}

// ConnectionMonitor monitors authorized connections and terminates them when
// session controls dictate so.
type ConnectionMonitor interface {
	MonitorConn(ctx context.Context, authCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error)
}

// ServerConfig holds creation parameters for Service.
type ServerConfig struct {
	// FIPS indicates whether the cluster if configured
	// to run in FIPS mode.
	FIPS bool
	// Logger provides a mechanism to log output.
	Logger *slog.Logger
	// Dialer is used to establish remote connections.
	Dialer Dialer
	// SignerFn is used to create an [ssh.Signer] for an authenticated connection.
	SignerFn func(authzCtx *authz.Context, clusterName string) agentless.SignerCreator
	// ConnectionMonitor is used to monitor the connection for activity and terminate it
	// when conditions are met.
	ConnectionMonitor ConnectionMonitor
	// LocalAddr is the local address of the service.
	LocalAddr net.Addr

	// agentGetterFn used by tests to serve the agent directly
	agentGetterFn func(rw io.ReadWriter) teleagent.Getter

	// authzContextFn used by tests to inject an access checker
	authzContextFn func(info credentials.AuthInfo) (*authz.Context, error)
}

// CheckAndSetDefaults ensures required parameters are set
// and applies default values for missing optional parameters.
func (c *ServerConfig) CheckAndSetDefaults() error {
	if c.Dialer == nil {
		return trace.BadParameter("parameter Dialer required")
	}

	if c.LocalAddr == nil {
		return trace.BadParameter("parameter LocalAddr required")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "transport")
	}

	if c.agentGetterFn == nil {
		c.agentGetterFn = func(rw io.ReadWriter) teleagent.Getter {
			return func() (teleagent.Agent, error) {
				return teleagent.NopCloser(agent.NewClient(rw)), nil
			}
		}
	}

	if c.authzContextFn == nil {
		c.authzContextFn = func(info credentials.AuthInfo) (*authz.Context, error) {
			identityInfo, ok := info.(auth.IdentityInfo)
			if !ok {
				return nil, trace.AccessDenied("client is not authenticated")
			}

			return identityInfo.AuthContext, nil
		}
	}

	return nil
}

// Service implements the teleport.transport.v1.TransportService RPC
// service.
type Service struct {
	transportv1pb.UnimplementedTransportServiceServer

	cfg ServerConfig
}

// NewService constructs a new Service from the provided ServerConfig.
func NewService(cfg ServerConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{cfg: cfg}, nil
}

// GetClusterDetails returns the cluster details as seen by this service to the client.
func (s *Service) GetClusterDetails(context.Context, *transportv1pb.GetClusterDetailsRequest) (*transportv1pb.GetClusterDetailsResponse, error) {
	return &transportv1pb.GetClusterDetailsResponse{Details: &transportv1pb.ClusterDetails{FipsEnabled: s.cfg.FIPS}}, nil
}

// ProxyCluster establishes a connection to a cluster and proxies the connection
// over the stream. The client must send the first request with the cluster name
// before the connection is established.
func (s *Service) ProxyCluster(stream transportv1pb.TransportService_ProxyClusterServer) error {
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, "failed receiving first frame")
	}

	ctx := stream.Context()
	p, ok := peer.FromContext(ctx)
	if !ok {
		return trace.BadParameter("unable to find peer")
	}

	clientDst, err := getDestinationAddress(p.Addr, s.cfg.LocalAddr)
	if err != nil {
		return trace.Wrap(err, "could get not client destination address; listener address %q, client source address %q", s.cfg.LocalAddr.String(), p.Addr.String())
	}

	conn, err := s.cfg.Dialer.DialSite(ctx, req.Cluster, p.Addr, clientDst)
	if err != nil {
		return trace.Wrap(err, "failed dialing cluster %q", req.Cluster)
	}

	// A client may provide a frame with the first message. Since the message is
	// already consumed it won't be copying during the proxying below. In order for
	// the contents to be copied to the connection it needs to be manually written.
	if req.Frame != nil {
		if _, err := conn.Write(req.Frame.Payload); err != nil {
			return trace.Wrap(err, "failed writing payload from first frame")
		}
	}

	streamRW, err := streamutils.NewReadWriter(clusterStream{stream: stream})
	if err != nil {
		return trace.Wrap(err, "failed constructing streamer")
	}

	return trace.Wrap(utils.ProxyConn(ctx, conn, streamRW))
}

// clusterStream implements the [streamutils.Source] interface
// for a [transportv1pb.TransportService_ProxyClusterServer].
type clusterStream struct {
	stream transportv1pb.TransportService_ProxyClusterServer
}

func (c clusterStream) Recv() ([]byte, error) {
	req, err := c.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Frame == nil {
		return nil, trace.BadParameter("received invalid frame")
	}

	return req.Frame.Payload, nil
}

func (c clusterStream) Send(frame []byte) error {
	return trace.Wrap(c.stream.Send(&transportv1pb.ProxyClusterResponse{Frame: &transportv1pb.Frame{Payload: frame}}))
}

// ProxySSH establishes a connection to a host and proxies both the SSH and SSH
// Agent protocol over the stream. The first request from the client must contain
// a valid dial target before the connection can be established.
func (s *Service) ProxySSH(stream transportv1pb.TransportService_ProxySSHServer) (err error) {
	ctx := stream.Context()

	p, ok := peer.FromContext(ctx)
	if !ok {
		return trace.BadParameter("unable to find peer")
	}

	authzContext, err := s.cfg.authzContextFn(p.AuthInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	// wait for the first request to arrive with the dial request
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, "failed receiving first frame")
	}

	// validate the target
	if req.DialTarget == nil {
		return trace.BadParameter("first frame must contain a dial target")
	}

	host, port, err := net.SplitHostPort(req.DialTarget.HostPort)
	if err != nil {
		return trace.BadParameter("dial target contains an invalid hostport")
	}

	// create streams for SSH and Agent protocols
	sshStream, agentStream := newSSHStreams(stream)

	// multiplex incoming frames to the appropriate protocol
	// handlers for the duration of the stream
	go func() {
		for {
			req, err := stream.Recv()
			if err != nil {
				if !utils.IsOKNetworkError(err) && !errors.Is(err, context.Canceled) && status.Code(err) != codes.Canceled {
					s.cfg.Logger.ErrorContext(ctx, "ssh stream terminated unexpectedly", "error", err)
				}

				return
			}

			// The writes to the channels are intentionally not selecting
			// on `ctx.Done()` to ensure that all data is flushed to the
			// clients.
			switch frame := req.Frame.(type) {
			case *transportv1pb.ProxySSHRequest_Ssh:
				sshStream.incomingC <- frame.Ssh.Payload
			case *transportv1pb.ProxySSHRequest_Agent:
				agentStream.incomingC <- frame.Agent.Payload
			default:
				s.cfg.Logger.ErrorContext(ctx, "received unexpected ssh frame", "frame", logutils.TypeAttr(frame))
				continue
			}
		}
	}()

	// create a reader/writer for SSH Agent protocol
	agentStreamRW, err := streamutils.NewReadWriter(agentStream)
	if err != nil {
		return trace.Wrap(err, "failed constructing ssh agent streamer")
	}
	defer agentStreamRW.Close()

	// create a reader/writer for SSH protocol
	sshStreamRW, err := streamutils.NewReadWriter(sshStream)
	if err != nil {
		return trace.Wrap(err, "failed constructing ssh streamer")
	}

	clientDst, err := getDestinationAddress(p.Addr, s.cfg.LocalAddr)
	if err != nil {
		return trace.Wrap(err, "could get not client destination address; listener address %q, client source address %q", s.cfg.LocalAddr.String(), p.Addr.String())
	}

	ident := authzContext.Identity.GetIdentity()

	sshIdent := &sshca.Identity{
		Username:           ident.Username,
		Roles:              ident.Groups,
		Traits:             ident.Traits,
		AllowedResourceIDs: ident.AllowedResourceIDs,
		CertType:           ssh.UserCert,
	}

	signer := s.cfg.SignerFn(authzContext, req.DialTarget.Cluster)
	hostConn, err := s.cfg.Dialer.DialHost(ctx, p.Addr, clientDst, host, port, req.DialTarget.Cluster, sshIdent, authzContext.Checker, s.cfg.agentGetterFn(agentStreamRW), signer)
	if err != nil {
		// Return ambiguous errors unadorned so that clients can detect them easily.
		if errors.Is(err, teleport.ErrNodeIsAmbiguous) {
			return trace.Wrap(err)
		}
		return trace.Wrap(err, "failed to dial target host")
	}

	// ensure the connection to the target host
	// gets closed when exiting
	defer func() {
		hostConn.Close()
	}()

	targetAddr, err := utils.ParseAddr(req.DialTarget.HostPort)
	if err != nil {
		return trace.Wrap(err)
	}

	// monitor the user connection
	conn := streamutils.NewConn(sshStreamRW, p.Addr, targetAddr)
	monitorCtx, userConn, err := s.cfg.ConnectionMonitor.MonitorConn(ctx, authzContext, conn)
	if err != nil {
		return trace.Wrap(err)
	}

	// send back the cluster details to alert the other side that
	// the connection has been established
	if err := stream.Send(&transportv1pb.ProxySSHResponse{
		Details: &transportv1pb.ClusterDetails{FipsEnabled: s.cfg.FIPS},
	}); err != nil {
		return trace.Wrap(err, "failed sending cluster details ")
	}

	// copy data to/from the host/user
	err = utils.ProxyConn(monitorCtx, hostConn, userConn)
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		err = nil
	}
	return trace.Wrap(err)
}

// getDestinationAddress is used to get client destination for connection coming from gRPC. We don't have a way to get
// real connection dst address, but we rely on listener address to be that. Returned IP version always have to match
// IP version of src address. If IP versions don't match or if listener is unspecified address we return loopback.
func getDestinationAddress(clientSrc, listenerAddr net.Addr) (net.Addr, error) {
	la, err := netip.ParseAddrPort(listenerAddr.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := netip.ParseAddrPort(clientSrc.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If listener address is specified and matches IP version of source address, we just return it
	if !la.Addr().IsUnspecified() && la.Addr().Is4() == ca.Addr().Is4() {
		return listenerAddr, nil
	}

	// Otherwise we return loopback with matching IP version of source address
	if ca.Addr().Is4() {
		return &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: int(la.Port()),
		}, nil
	}

	return &net.TCPAddr{
		IP:   net.IPv6loopback,
		Port: int(la.Port()),
	}, nil
}

// sshStream implements the [streamutils.Source] interface
// for a [transportv1pb.TransportService_ProxySSHServer]. Instead of
// reading directly from the stream reads are from an incoming
// channel that is fed by the multiplexer.
type sshStream struct {
	incomingC  chan []byte
	responseFn func(payload []byte) *transportv1pb.ProxySSHResponse
	wLock      *sync.Mutex
	stream     transportv1pb.TransportService_ProxySSHServer
}

func newSSHStreams(stream transportv1pb.TransportService_ProxySSHServer) (ssh *sshStream, agent *sshStream) {
	mu := &sync.Mutex{}

	ssh = &sshStream{
		incomingC: make(chan []byte, 10),
		stream:    stream,
		responseFn: func(payload []byte) *transportv1pb.ProxySSHResponse {
			return &transportv1pb.ProxySSHResponse{Frame: &transportv1pb.ProxySSHResponse_Ssh{Ssh: &transportv1pb.Frame{Payload: payload}}}
		},
		wLock: mu,
	}

	agent = &sshStream{
		incomingC: make(chan []byte, 10),
		stream:    stream,
		responseFn: func(payload []byte) *transportv1pb.ProxySSHResponse {
			return &transportv1pb.ProxySSHResponse{Frame: &transportv1pb.ProxySSHResponse_Agent{Agent: &transportv1pb.Frame{Payload: payload}}}
		},
		wLock: mu,
	}

	return ssh, agent
}

// Recv consumes ssh frames from the gRPC stream.
// All data must be consumed by clients to prevent
// leaking the multiplexing goroutine in Service.ProxySSH.
func (s *sshStream) Recv() ([]byte, error) {
	select {
	case <-s.stream.Context().Done():
		return nil, io.EOF
	case frame := <-s.incomingC:
		return frame, nil
	}
}

func (s *sshStream) Send(frame []byte) error {
	s.wLock.Lock()
	defer s.wLock.Unlock()

	return trace.Wrap(s.stream.Send(s.responseFn(frame)))
}
