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
	Accept(context.Context) (PendingConn, error)
}

type PendingConn interface {
	DialRequest() *clientapi.DialRequest

	Accept() (net.Conn, error)
	Reject(msg string) error
}
