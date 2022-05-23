package server

import (
	"errors"
	"net"
	"sync/atomic"

	. "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/packet"
	"github.com/siddontang/go/sync2"
)

/*
   Conn acts like a MySQL server connection, you can use MySQL client to communicate with it.
*/
type Conn struct {
	*packet.Conn

	serverConf     *Server
	capability     uint32
	charset        uint8
	authPluginName string
	attributes     map[string]string
	connectionID   uint32
	status         uint16
	warnings       uint16
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

	defaultServer := NewDefaultServer()

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
	if err := c.writeInitialHandshake(); err != nil {
		return err
	}

	if err := c.readHandshakeResponse(); err != nil {
		if errors.Is(err, ErrAccessDenied) {
			var usingPasswd uint16 = ER_YES
			if errors.Is(err, ErrAccessDeniedNoPassword) {
				usingPasswd = ER_NO
			}
			err = NewDefaultError(ER_ACCESS_DENIED_ERROR, c.user, c.RemoteAddr().String(), MySQLErrName[usingPasswd])
		}
		_ = c.writeError(err)
		return err
	}

	if err := c.writeOK(nil); err != nil {
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

func (c *Conn) Capability() uint32 {
	return c.capability
}

func (c *Conn) SetCapability(cap uint32) {
	c.capability |= cap
}

func (c *Conn) UnsetCapability(cap uint32) {
	c.capability &= ^cap
}

func (c *Conn) HasCapability(cap uint32) bool {
	return c.capability&cap > 0
}

func (c *Conn) Charset() uint8 {
	return c.charset
}

func (c *Conn) Attributes() map[string]string {
	return c.attributes
}

func (c *Conn) ConnectionID() uint32 {
	return c.connectionID
}

func (c *Conn) IsAutoCommit() bool {
	return c.HasStatus(SERVER_STATUS_AUTOCOMMIT)
}

func (c *Conn) IsInTransaction() bool {
	return c.HasStatus(SERVER_STATUS_IN_TRANS)
}

func (c *Conn) SetInTransaction() {
	c.SetStatus(SERVER_STATUS_IN_TRANS)
}

func (c *Conn) ClearInTransaction() {
	c.UnsetStatus(SERVER_STATUS_IN_TRANS)
}

func (c *Conn) SetStatus(status uint16) {
	c.status |= status
}

func (c *Conn) UnsetStatus(status uint16) {
	c.status &= ^status
}

func (c *Conn) HasStatus(status uint16) bool {
	return c.status&status > 0
}

func (c *Conn) SetWarnings(warnings uint16) {
	c.warnings = warnings
}
