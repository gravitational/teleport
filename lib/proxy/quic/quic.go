package quic

import (
	"context"
	"net"

	clientapi "github.com/gravitational/teleport/api/client/proto"
)

type Client interface {
	Dial(context.Context, *clientapi.DialRequest) (net.Conn, error)
}

type Server interface {
	Accept(context.Context) (*clientapi.DialRequest, PendingConn, error)
}

type PendingConn interface {
	Accept() (net.Conn, error)
	Reject(msg string) error
}
