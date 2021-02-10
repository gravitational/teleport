package server

import (
	"net"
	"sync/atomic"

	. "github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/packet"
	"github.com/siddontang/go/sync2"
)

/*
   Conn acts like a MySQL server connection, you can use MySQL client to communicate with it.
*/
type Conn struct {
	*packet.Conn

	serverConf     *Server
	capability     uint32
	authPluginName string
	connectionID   uint32
	status         uint16
	salt           []byte // should be 8 + 12 for auth-plugin-data-part-1 and auth-plugin-data-part-2

	credentialProvider  CredentialProvider
	user                string
	password            string
	db                  string
	cachingSha2FullAuth bool

	h Handler

	stmts  map[uint32]*Stmt
	stmtID uint32

	closed sync2.AtomicBool
}

var baseConnID uint32 = 10000

// NewConn: create connection with default server settings
func NewConn(conn net.Conn, user string, password string, h Handler) (*Conn, error) {
	p := NewInMemoryProvider()
	p.AddUser(user, password)
	salt, _ := RandomBuf(20)

	var packetConn *packet.Conn
	if defaultServer.tlsConfig != nil {
		packetConn = packet.NewTLSConn(conn)
	} else {
		packetConn = packet.NewConn(conn)
	}

	c := &Conn{
		Conn:               packetConn,
		serverConf:         defaultServer,
		credentialProvider: p,
		h:                  h,
		connectionID:       atomic.AddUint32(&baseConnID, 1),
		stmts:              make(map[uint32]*Stmt),
		salt:               salt,
	}
	c.closed.Set(false)

	if err := c.handshake(); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

// MakeConn creates a new server side connection without performing the handshake.
func MakeConn(conn net.Conn, serverConf *Server, p CredentialProvider, h Handler) *Conn {
	var packetConn *packet.Conn
	if serverConf.tlsConfig != nil {
		packetConn = packet.NewTLSConn(conn)
	} else {
		packetConn = packet.NewConn(conn)
	}

	salt, _ := RandomBuf(20)
	c := &Conn{
		Conn:               packetConn,
		serverConf:         serverConf,
		credentialProvider: p,
		h:                  h,
		connectionID:       atomic.AddUint32(&baseConnID, 1),
		stmts:              make(map[uint32]*Stmt),
		salt:               salt,
	}
	c.closed.Set(false)

	return c
}

// NewCustomizedConn: create connection with customized server settings
func NewCustomizedConn(conn net.Conn, serverConf *Server, p CredentialProvider, h Handler) (*Conn, error) {
	c := MakeConn(conn, serverConf, p, h)

	if err := c.handshake(); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

func (c *Conn) handshake() error {
	if err := c.WriteInitialHandshake(); err != nil {
		return err
	}

	if err := c.ReadHandshakeResponse(); err != nil {
		if err == ErrAccessDenied {
			err = NewDefaultError(ER_ACCESS_DENIED_ERROR, c.user, c.LocalAddr().String(), "Yes")
		}
		c.WriteError(err)
		return err
	}

	if err := c.WriteOK(nil); err != nil {
		return err
	}

	c.ResetSequence()

	return nil
}

func (c *Conn) Close() {
	c.closed.Set(true)
	c.Conn.Close()
}

func (c *Conn) Closed() bool {
	return c.closed.Get()
}

func (c *Conn) GetUser() string {
	return c.user
}

func (c *Conn) GetDatabase() string {
	return c.db
}

func (c *Conn) ConnectionID() uint32 {
	return c.connectionID
}

func (c *Conn) IsAutoCommit() bool {
	return c.status&SERVER_STATUS_AUTOCOMMIT > 0
}

func (c *Conn) IsInTransaction() bool {
	return c.status&SERVER_STATUS_IN_TRANS > 0
}

func (c *Conn) SetInTransaction() {
	c.status |= SERVER_STATUS_IN_TRANS
}

func (c *Conn) ClearInTransaction() {
	c.status &= ^SERVER_STATUS_IN_TRANS
}
