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
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/yamux"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	relaytunnelv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaytunnel/v1alpha"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

const maxMessageSize = 128 * 1024

type Server struct {
	mu sync.Mutex

	// conns holds client connections.
	conns map[connKey][]serverConn
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
	dial(src, dst net.Addr) (io.ReadWriteCloser, error)
}

type yamuxServerConn struct {
	session *yamux.Session
}

// dial implements [serverConn].
func (c *yamuxServerConn) dial(src net.Addr, dst net.Addr) (io.ReadWriteCloser, error) {
	panic("unimplemented")
}

var _ serverConn = (*yamuxServerConn)(nil)

func (s *Server) HandleTunnelConnection(c io.ReadWriteCloser, peerRole authz.BuiltinRole) error {
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

	controlStream, err := session.OpenStream()
	if err != nil {
		return trace.Wrap(err)
	}
	defer controlStream.Close()

	const helloTimeout = 30 * time.Second
	controlStream.SetDeadline(time.Now().Add(helloTimeout))

	clientHello := new(relaytunnelv1alpha.ClientHello)
	if err = readProto(controlStream, clientHello); err != nil {
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
		if peerRole.Role != requiredRole && !slices.Contains(peerRole.AdditionalSystemRoles, requiredRole) {
			return trace.AccessDenied("required role %q not in client identity", requiredRole)
		}
		return nil
	}()

	if err := writeProto(controlStream, &relaytunnelv1alpha.ServerHello{
		Status: status.Convert(trail.ToGRPC(roleErr)).Proto(),
	}); err != nil {
		return trace.Wrap(err)
	}
	if roleErr != nil {
		return trace.Wrap(err)
	}

	controlStream.SetDeadline(time.Time{})

	sc := &yamuxServerConn{
		session: session,
	}
	s.addConn(peerRole.GetServerID(), tunnelType, sc)
	defer s.removeConn(peerRole.GetServerID(), tunnelType, sc)

	for {
		select {
		case <-session.CloseChan():

		}
	}

	return nil
}

func readProto(r io.Reader, m proto.Message) error {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return trace.Wrap(err)
	}
	size := binary.LittleEndian.Uint32(sizeBuf[:])
	if size > maxMessageSize {
		return trace.LimitExceeded("bad size")
	}

	msgBuf := make([]byte, size)
	if _, err := io.ReadFull(r, msgBuf); err != nil {
		return trace.Wrap(err)
	}

	if err := proto.Unmarshal(msgBuf, m); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeProto(w io.Writer, m proto.Message) error {
	msgBuf, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	if len(msgBuf) > maxMessageSize {
		return trace.LimitExceeded("bad size")
	}
	var sizeBuf [4]byte
	binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(msgBuf)))
	if _, err := w.Write(sizeBuf[:]); err != nil {
		return trace.Wrap(err)
	}
	if _, err := w.Write(msgBuf); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type yamuxLogger slog.Logger

var _ yamux.Logger = (*yamuxLogger)(nil)

// Print implements [yamux.Logger].
func (l *yamuxLogger) Print(v ...any) {
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintln(v...))
}

// Printf implements [yamux.Logger].
func (l *yamuxLogger) Printf(format string, args ...any) {
	if f, ok := strings.CutPrefix(format, "[ERR] "); ok {
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(f, args...))
	} else if f, ok := strings.CutPrefix(format, "[WARN] "); ok {
		(*slog.Logger)(l).WarnContext(context.Background(), fmt.Sprintf(f, args...))
	} else {
		(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintf(format, args...))
	}
}

// Println implements [yamux.Logger].
func (l *yamuxLogger) Println(args ...any) {
	(*slog.Logger)(l).ErrorContext(context.Background(), fmt.Sprintln(args...))
}
