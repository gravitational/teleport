package quic

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/quicvarint"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/protoadapt"

	clientapi "github.com/gravitational/teleport/api/client/proto"
)

type Client interface {
	Dial(context.Context, *clientapi.DialRequest) (net.Conn, error)
}

type Server interface {
	Accept(context.Context) (PendingConn, error)
}

type PendingConn interface {
	DialRequest() *clientapi.DialRequest

	Accept() (net.Conn, error)
	Reject(msg string) error
}

func NewServer(conn quic.Connection) Server {
	return &server{conn}
}

type server struct {
	conn quic.Connection
}

var _ Server = (*server)(nil)

// Accept implements [Server].
func (s *server) Accept(ctx context.Context) (PendingConn, error) {
	stream, err := s.conn.AcceptStream(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dr := &clientapi.DialRequest{}
	if err := protodelim.UnmarshalFrom(quicvarint.NewReader(stream), protoadapt.MessageV2Of(dr)); err != nil {
		stream.CancelWrite(quic.StreamErrorCode(1))
		stream.CancelRead(quic.StreamErrorCode(1))
		return nil, trace.Wrap(err)
	}

	return &pendingConn{
		stream: stream,
		dr:     dr,
	}, nil
}

type pendingConn struct {
	stream quic.Stream
	dr     *clientapi.DialRequest
}

var _ PendingConn = (*pendingConn)(nil)

// DialRequest implements [PendingConn].
func (p *pendingConn) DialRequest() *clientapi.DialRequest {
	return p.dr
}

// Accept implements [PendingConn].
func (p *pendingConn) Accept() (net.Conn, error) {
	if _, err := protodelim.MarshalTo(p.stream, &status.Status{
		Code: int32(code.Code_OK),
	}); err != nil {
		p.stream.CancelRead(quic.StreamErrorCode(2))
		p.stream.CancelWrite(quic.StreamErrorCode(2))
		return nil, trace.Wrap(err)
	}

	return newStreamConn(p.stream, p.dr.GetDestination(), p.dr.GetSource()), nil
}

// Reject implements [PendingConn].
func (p *pendingConn) Reject(msg string) error {
	p.stream.CancelRead(quic.StreamErrorCode(2))

	if _, err := protodelim.MarshalTo(p.stream, &status.Status{
		Code:    int32(code.Code_UNKNOWN),
		Message: msg,
	}); err != nil {
		p.stream.CancelWrite(quic.StreamErrorCode(2))
		return trace.Wrap(err)
	}

	return trace.Wrap(p.stream.Close())
}

func NewClient(conn quic.Connection) Client {
	return &client{conn}
}

type client struct {
	conn quic.Connection
}

var _ Client = (*client)(nil)

// Dial implements [Client].
func (c *client) Dial(ctx context.Context, dr *clientapi.DialRequest) (net.Conn, error) {
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := protodelim.MarshalTo(stream, protoadapt.MessageV2Of(dr)); err != nil {
		stream.CancelRead(quic.StreamErrorCode(3))
		stream.CancelWrite(quic.StreamErrorCode(3))
		return nil, trace.Wrap(err)
	}

	st := &status.Status{}
	if err := protodelim.UnmarshalFrom(quicvarint.NewReader(stream), st); err != nil {
		stream.CancelWrite(quic.StreamErrorCode(4))
		stream.CancelRead(quic.StreamErrorCode(4))
		return nil, trace.Wrap(err)
	}

	if st.GetCode() != int32(code.Code_OK) {
		stream.CancelWrite(quic.StreamErrorCode(5))
		stream.CancelRead(quic.StreamErrorCode(5))
		return nil, trace.Errorf("%s", st)
	}

	return newStreamConn(stream, dr.GetSource(), dr.GetDestination()), nil
}
