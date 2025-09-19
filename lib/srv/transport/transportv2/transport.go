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

package transportv2

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	transportv2pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v2"
	"github.com/gravitational/teleport/api/types"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/sshagent"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Dialer is the interface that groups basic dialing methods.
type Dialer interface {
	DialSite(ctx context.Context, cluster string, clientSrcAddr, clientDstAddr net.Addr) (net.Conn, error)
	DialHost(ctx context.Context, clientSrcAddr, clientDstAddr net.Addr, host, port, cluster string, clusterAccessChecker func(types.RemoteCluster) error, agentGetter sshagent.ClientGetter, singer agentless.SignerCreator) (net.Conn, error)
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
	agentGetterFn func(rw io.ReadWriter) sshagent.ClientGetter

	// authzContextFn used by tests to inject an access checker
	authzContextFn func(info credentials.AuthInfo) (*authz.Context, error)

	// authClient is used to interact with the auth service.
	AuthClient *authclient.Client
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
		c.agentGetterFn = func(rw io.ReadWriter) sshagent.ClientGetter {
			return sshagent.NewStaticClientGetter(agent.NewClient(rw))
		}
	}

	if c.authzContextFn == nil {
		c.authzContextFn = func(info credentials.AuthInfo) (*authz.Context, error) {
			identityInfo, ok := info.(interface{ AuthzContext() *authz.Context })
			if !ok {
				return nil, trace.AccessDenied("client is not authenticated")
			}

			return identityInfo.AuthzContext(), nil
		}
	}

	return nil
}

// Service implements the teleport.transport.v1.TransportService RPC
// service.
type Service struct {
	transportv2pb.UnimplementedTransportServiceServer

	cfg ServerConfig
}

// NewService constructs a new Service from the provided ServerConfig.
func NewService(cfg ServerConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{cfg: cfg}, nil
}

// ProxySSH establishes a connection to a host and proxies both the SSH and SSH
// Agent protocol over the stream. The first request from the client must contain
// a valid dial target before the connection can be established.
func (s *Service) ProxySSH(stream transportv2pb.TransportService_ProxySSHServer) (err error) {
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
	if req.GetDialTarget() == nil {
		return trace.BadParameter("first frame must contain a dial target")
	}

	host, port, err := net.SplitHostPort(req.GetDialTarget().GetHostPort())
	if err != nil {
		return trace.BadParameter("dial target contains an invalid hostport")
	}

	// TODO(cthach): Check with Decision API if MFA is required. If it is, send a challenge to the client.
	s.cfg.Logger.InfoContext(ctx, "Checking if MFA is required")

	mfaResp, err := s.cfg.AuthClient.PerformMFACeremony(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
	})
	if err != nil {
		return trace.Wrap(err, "failed creating MFA ceremony")
	}

	// This really only checks if the user has any MFA devices registered.
	if mfaResp.GetWebauthn() == nil && mfaResp.GetSSO() == nil {
		s.cfg.Logger.InfoContext(ctx, "MFA is not required for this connection", "login", authzContext.GetUserMetadata().Login, "node", host)
	}

	// TODO(cthach): Send the challenge to the client and wait for a response.
	s.cfg.Logger.InfoContext(ctx, "MFA is required for this connection", "login", authzContext.GetUserMetadata().Login, "node", host)

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
			switch frame := req.GetPayload().(type) {
			case *transportv2pb.ProxySSHRequest_Ssh:
				sshStream.incomingC <- frame.Ssh.Payload
			case *transportv2pb.ProxySSHRequest_Agent:
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

	signer := s.cfg.SignerFn(authzContext, req.GetDialTarget().GetCluster())
	hostConn, err := s.cfg.Dialer.DialHost(ctx, p.Addr, clientDst, host, port, req.GetDialTarget().GetCluster(), authzContext.Checker.CheckAccessToRemoteCluster, s.cfg.agentGetterFn(agentStreamRW), signer)
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

	targetAddr, err := utils.ParseAddr(req.GetDialTarget().GetHostPort())
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
	if err := stream.Send(&transportv2pb.ProxySSHResponse{
		Payload: &transportv2pb.ProxySSHResponse_Details{
			Details: &transportv2pb.ClusterDetails{FipsEnabled: s.cfg.FIPS},
		},
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
// for a [transportv2pb.TransportService_ProxySSHServer]. Instead of
// reading directly from the stream reads are from an incoming
// channel that is fed by the multiplexer.
type sshStream struct {
	incomingC  chan []byte
	responseFn func(payload []byte) *transportv2pb.ProxySSHResponse
	wLock      *sync.Mutex
	stream     transportv2pb.TransportService_ProxySSHServer
}

func newSSHStreams(stream transportv2pb.TransportService_ProxySSHServer) (ssh *sshStream, agent *sshStream) {
	mu := &sync.Mutex{}

	ssh = &sshStream{
		incomingC: make(chan []byte, 10),
		stream:    stream,
		responseFn: func(payload []byte) *transportv2pb.ProxySSHResponse {
			return &transportv2pb.ProxySSHResponse{
				Payload: &transportv2pb.ProxySSHResponse_Ssh{Ssh: &transportv2pb.Frame{Payload: payload}},
			}
		},
		wLock: mu,
	}

	agent = &sshStream{
		incomingC: make(chan []byte, 10),
		stream:    stream,
		responseFn: func(payload []byte) *transportv2pb.ProxySSHResponse {
			return &transportv2pb.ProxySSHResponse{
				Payload: &transportv2pb.ProxySSHResponse_Agent{Agent: &transportv2pb.Frame{Payload: payload}},
			}
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
