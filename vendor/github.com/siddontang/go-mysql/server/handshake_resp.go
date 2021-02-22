package server

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"

	"github.com/pingcap/errors"
	. "github.com/siddontang/go-mysql/mysql"
)

func (c *Conn) ReadHandshakeResponse() error {
	data, pos, err := c.readFirstPart()
	if err != nil {
		return err
	}
	if pos, err = c.readUserName(data, pos); err != nil {
		return err
	}
	authData, authLen, pos, err := c.readAuthData(data, pos)
	if err != nil {
		return err
	}

	pos += authLen

	if pos, err = c.readDb(data, pos); err != nil {
		return err
	}

	pos = c.readPluginName(data, pos)

	cont, err := c.handleAuthMatch(authData, pos)
	if err != nil {
		return err
	}
	if !cont {
		return nil
	}

	// ignore connect attrs for now, the proxy does not support passing attrs to actual MySQL server

	// try to authenticate the client
	return c.compareAuthData(c.authPluginName, authData)
}

func (c *Conn) readFirstPart() ([]byte, int, error) {
	data, err := c.ReadPacket()
	if err != nil {
		return nil, 0, err
	}

	pos := 0

	// check CLIENT_PROTOCOL_41
	if uint32(binary.LittleEndian.Uint16(data[:2]))&CLIENT_PROTOCOL_41 == 0 {
		return nil, 0, errors.New("CLIENT_PROTOCOL_41 compatible client is required")
	}

	//capability
	c.capability = binary.LittleEndian.Uint32(data[:4])
	if c.capability&CLIENT_SECURE_CONNECTION == 0 {
		return nil, 0, errors.New("CLIENT_SECURE_CONNECTION compatible client is required")
	}
	pos += 4

	//skip max packet size
	pos += 4

	//charset, skip, if you want to use another charset, use set names
	//c.collation = CollationId(data[pos])
	pos++

	//skip reserved 23[00]
	pos += 23

	// is this a SSLRequest packet?
	if len(data) == (4 + 4 + 1 + 23) {
		if c.serverConf.capability&CLIENT_SSL == 0 {
			return nil, 0, errors.Errorf("The host '%s' does not support SSL connections", c.RemoteAddr().String())
		}
		// switch to TLS
		tlsConn := tls.Server(c.Conn.Conn, c.serverConf.tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			return nil, 0, err
		}
		c.Conn.Conn = tlsConn

		// mysql handshake again
		return c.readFirstPart()
	}
	return data, pos, nil
}

func (c *Conn) readUserName(data []byte, pos int) (int, error) {
	//user name
	user := string(data[pos : pos+bytes.IndexByte(data[pos:], 0x00)])
	pos += len(user) + 1
	c.user = user
	return pos, nil
}

func (c *Conn) readDb(data []byte, pos int) (int, error) {
	if c.capability&CLIENT_CONNECT_WITH_DB != 0 {
		if len(data[pos:]) == 0 {
			return pos, nil
		}

		db := string(data[pos : pos+bytes.IndexByte(data[pos:], 0x00)])
		pos += len(db) + 1

		if err := c.h.UseDB(db); err != nil {
			return 0, err
		}
		c.db = db
	}
	return pos, nil
}

func (c *Conn) readPluginName(data []byte, pos int) int {
	if c.capability&CLIENT_PLUGIN_AUTH != 0 {
		c.authPluginName = string(data[pos : pos+bytes.IndexByte(data[pos:], 0x00)])
		pos += len(c.authPluginName)
	} else {
		// The method used is Native Authentication if both CLIENT_PROTOCOL_41 and CLIENT_SECURE_CONNECTION are set,
		// but CLIENT_PLUGIN_AUTH is not set, so we fallback to 'mysql_native_password'
		c.authPluginName = AUTH_NATIVE_PASSWORD
	}
	return pos
}

func (c *Conn) readAuthData(data []byte, pos int) ([]byte, int, int, error) {
	// length encoded data
	var auth []byte
	var authLen int
	if c.capability&CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA != 0 {
		authData, isNULL, readBytes, err := LengthEncodedString(data[pos:])
		if err != nil {
			return nil, 0, 0, err
		}
		if isNULL {
			// no auth length and no auth data, just \NUL, considered invalid auth data, and reject connection as MySQL does
			return nil, 0, 0, NewDefaultError(ER_ACCESS_DENIED_ERROR, c.LocalAddr().String(), c.user, "Yes")
		}
		auth = authData
		authLen = readBytes
	} else if c.capability&CLIENT_SECURE_CONNECTION != 0 {
		//auth length and auth
		authLen = int(data[pos])
		pos++
		auth = data[pos : pos+authLen]
	} else {
		authLen = bytes.IndexByte(data[pos:], 0x00)
		auth = data[pos : pos+authLen]
		// account for last NUL
		authLen++
	}
	return auth, authLen, pos, nil
}

// Public Key Retrieval
// See: https://dev.mysql.com/doc/internals/en/public-key-retrieval.html
func (c *Conn) handlePublicKeyRetrieval(authData []byte) (bool, error) {
	// if the client use 'sha256_password' auth method, and request for a public key
	// we send back a keyfile with Protocol::AuthMoreData
	if c.authPluginName == AUTH_SHA256_PASSWORD && len(authData) == 1 && authData[0] == 0x01 {
		if c.serverConf.capability&CLIENT_SSL == 0 {
			return false, errors.New("server does not support SSL: CLIENT_SSL not enabled")
		}
		if err := c.writeAuthMoreDataPubkey(); err != nil {
			return false, err
		}

		return false, c.handleAuthSwitchResponse()
	}
	return true, nil
}

func (c *Conn) handleAuthMatch(authData []byte, pos int) (bool, error) {
	// if the client responds the handshake with a different auth method, the server will send the AuthSwitchRequest packet
	// to the client to ask the client to switch.

	if c.authPluginName != c.serverConf.defaultAuthMethod {
		if err := c.writeAuthSwitchRequest(c.serverConf.defaultAuthMethod); err != nil {
			return false, err
		}
		c.authPluginName = c.serverConf.defaultAuthMethod
		// handle AuthSwitchResponse
		return false, c.handleAuthSwitchResponse()
	}
	return true, nil
}
