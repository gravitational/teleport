package client

import (
	"context"
	"net"

	"github.com/pingcap/errors"
	. "github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/packet"
)

// Dialer connects to the address on the named network using the provided context.
type Dialer func(ctx context.Context, network, address string) (net.Conn, error)

// Connect to a MySQL server using the given Dialer.
func ConnectWithDialer(ctx context.Context, network string, addr string, user string, password string, dbName string, dialer Dialer, options ...func(*Conn)) (*Conn, error) {
	c := new(Conn)

	var err error
	conn, err := dialer(ctx, network, addr)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if c.tlsConfig != nil {
		c.Conn = packet.NewTLSConn(conn)
	} else {
		c.Conn = packet.NewConn(conn)
	}

	c.user = user
	c.password = password
	c.db = dbName
	c.proto = network

	// use default charset here, utf-8
	c.charset = DEFAULT_CHARSET

	// Apply configuration functions.
	for i := range options {
		options[i](c)
	}

	if err = c.handshake(); err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}
