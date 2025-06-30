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

package relaytunnel

import (
	"cmp"
	"context"
	"io"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc/status"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tlsca"
)

const maxMessageSize = 128 * 1024

type Server struct {
	mu sync.Mutex

	// conns holds client connections.
	conns map[connKey][]serverConn
}

func (s *Server) Dial(ctx context.Context, hostID string, tunnelType types.TunnelType, src, dst net.Addr) (io.ReadWriteCloser, error) {
	var sc serverConn
	s.mu.Lock()
	scs := s.conns[connKey{
		hostID:     hostID,
		tunnelType: tunnelType,
	}]
	if len(scs) > 0 {
		sc = scs[len(scs)-1]
	}
	s.mu.Unlock()

	if sc == nil {
		return nil, trace.NotFound("dial target not found among connected tunnels")
	}

	rwc, err := sc.dial(ctx, src, dst)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rwc, nil
}

type connKey struct {
	hostID     string
	tunnelType types.TunnelType
}

func (s *Server) addConn(hostID string, tunnelType types.TunnelType, conn serverConn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conns == nil {
		s.conns = make(map[connKey][]serverConn)
	}

	ck := connKey{
		hostID:     hostID,
		tunnelType: tunnelType,
	}
	s.conns[ck] = append(s.conns[ck], conn)
}

func (s *Server) removeConn(hostID string, tunnelType types.TunnelType, conn serverConn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ck := connKey{
		hostID:     hostID,
		tunnelType: tunnelType,
	}
	conns := s.conns[ck]
	idx := slices.Index(conns, conn)
	if idx < 0 {
		return
	}
	s.conns[ck] = slices.Delete(conns, idx, idx+1)
}

type connDial struct {
	dialRequest reversetunnelclient.DialParams
}

type serverConn interface {
	dial(ctx context.Context, src, dst net.Addr) (io.ReadWriteCloser, error)
}

type yamuxServerConn struct {
	session    *yamux.Session
	relaysChan chan []*presencev1.RelayServer
}

var _ serverConn = (*yamuxServerConn)(nil)

// dial implements [serverConn].
func (c *yamuxServerConn) dial(ctx context.Context, src net.Addr, dst net.Addr) (io.ReadWriteCloser, error) {
	stream, err := c.session.OpenStream()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// non-destructively stop the dial request-response when the dial context is
	// canceled
	explode := make(chan struct{})
	defuse := context.AfterFunc(ctx, func() {
		defer close(explode)
		stream.SetDeadline(time.Unix(1, 0))
	})
	defer defuse()

	req := &relaytunnelv1alpha.DialRequest{
		Source:      addrToProto(src),
		Destination: addrToProto(dst),
	}
	if err := writeProto(stream, req); err != nil {
		defuse()
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	resp := new(relaytunnelv1alpha.DialResponse)
	if err := readProto(stream, resp); err != nil {
		defuse()
		_ = stream.Close()
		return nil, trace.Wrap(err)
	}

	if defuse() {
		close(explode)
	}

	if err := status.FromProto(resp.GetStatus()).Err(); err != nil {
		_ = stream.Close()
		return nil, trail.FromGRPC(err)
	}

	<-explode
	stream.SetDeadline(time.Time{})

	return stream, nil
}

func (s *Server) HandleYamuxTunnel(c io.ReadWriteCloser, peerID *tlsca.Identity) error {
	cfg := &yamux.Config{
		AcceptBacklog: 128,

		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,

		MaxStreamWindowSize: 256 * 1024,

		StreamCloseTimeout: time.Minute,
		StreamOpenTimeout:  30 * time.Second,

		LogOutput: nil,
		Logger:    (*yamuxLogger)(slog.Default()),
	}

	session, err := yamux.Server(c, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	defer session.Close()

	const helloTimeout = 30 * time.Second
	helloDeadline := time.Now().Add(helloTimeout)
	helloCtx, cancel := context.WithDeadline(context.Background(), helloDeadline)
	defer cancel()

	controlStream, err := session.AcceptStreamWithContext(helloCtx)
	if err != nil {
		return err
	}
	defer controlStream.Close()

	controlStream.SetDeadline(helloDeadline)

	clientHello := new(relaytunnelv1alpha.ClientHello)
	if err := readProto(controlStream, clientHello); err != nil {
		return trace.Wrap(err)
	}

	tunnelType := types.TunnelType(clientHello.GetTunnelType())
	roleErr := func() error {
		var requiredRole types.SystemRole
		switch tunnelType {
		case types.NodeTunnel:
			requiredRole = types.RoleNode
		default:
			return trace.BadParameter("unsupported tunnel type %q", tunnelType)
		}
		if !slices.Contains(peerID.Groups, string(requiredRole)) && !slices.Contains(peerID.SystemRoles, string(requiredRole)) {
			return trace.AccessDenied("required role %q not in client identity", requiredRole)
		}
		return nil
	}()

	if err := writeProto(controlStream, &relaytunnelv1alpha.ServerHello{
		Status: status.Convert(trail.ToGRPC(roleErr)).Proto(),
	}); err != nil {
		return trace.Wrap(cmp.Or(roleErr, err))
	}
	if roleErr != nil {
		return trace.Wrap(err)
	}

	controlStream.SetDeadline(time.Time{})

	sc := &yamuxServerConn{
		session: session,
	}

	s.addConn(peerID.Username, tunnelType, sc)
	defer s.removeConn(peerID.Username, tunnelType, sc)

	controlMsg := new(relaytunnelv1alpha.ClientStream)
	_ = readProto(controlStream, controlMsg)

	return nil
}
