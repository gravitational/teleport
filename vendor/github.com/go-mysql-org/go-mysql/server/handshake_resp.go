package server

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"

	. "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/pingcap/errors"
)

func (c *Conn) readHandshakeResponse() error {
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

	cont, err := c.handleAuthMatch()
	if err != nil {
		return err
	}
	if !cont {
		return nil
	}

	// read connection attributes
	if c.capability&CLIENT_CONNECT_ATTRS > 0 {
		// readAttributes returns new position for further processing of data
		_, err = c.readAttributes(data, pos)
		if err != nil {
			return err
		}
	}

	// try to authenticate the client
	return c.compareAuthData(c.authPluginName, authData)
}

func (c *Conn) readFirstPart() ([]byte, int, error) {
	data, err := c.ReadPacket()
	if err != nil {
		return nil, 0, err
	}

	return c.decodeFirstPart(data)
}

func (c *Conn) decodeFirstPart(data []byte) ([]byte, int, error) {
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

	// connection's default character set as defined
	c.charset = data[pos]
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
		pos += len(c.authPluginName) + 1
	} else {
		// The method used is Native Authentication if both CLIENT_PROTOCOL_41 and CLIENT_SECURE_CONNECTION are set,
		// but CLIENT_PLUGIN_AUTH is not set, so we fallback to 'mysql_native_password'
		c.authPluginName = AUTH_NATIVE_PASSWORD
	}
	return pos
}

func (c *Conn) readAuthData(data []byte, pos int) (auth []byte, authLen int, newPos int, err error) {
	// prevent 'panic: runtime error: index out of range' error
	defer func() {
		if recover() != nil {
			err = NewDefaultError(ER_HANDSHAKE_ERROR)
		}
	}()

	// length encoded data
	if c.capability&CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA != 0 {
		authData, isNULL, readBytes, err := LengthEncodedString(data[pos:])
		if err != nil {
			return nil, 0, 0, err
		}
		if isNULL {
			// no auth length and no auth data, just \NUL, considered invalid auth data, and reject connection as MySQL does
			return nil, 0, 0, NewDefaultError(ER_ACCESS_DENIED_ERROR, c.user, c.RemoteAddr().String(), MySQLErrName[ER_NO])
		}
		auth = authData
		authLen = readBytes
	} else if c.capability&CLIENT_SECURE_CONNECTION != 0 {
		// auth length and auth
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

func (c *Conn) handleAuthMatch() (bool, error) {
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

func (c *Conn) readAttributes(data []byte, pos int) (int, error) {
	// read length of attribute data
	attrLen, isNull, skip := LengthEncodedInt(data[pos:])
	pos += skip
	if isNull {
		return pos, nil
	}

	if len(data) < pos+int(attrLen) {
		return pos, errors.New("corrupt attributes data")
	}

	i := 0
	attrs := make(map[string]string)
	var key string

	// read until end of data or NUL for atrribute key/values
	for {
		str, isNull, strLen, err := LengthEncodedString(data[pos:])
		if err != nil {
			return -1, err
		}

		// end of data
		if isNull {
			break
		}

		// reading keys or values
		if i%2 == 0 {
			key = string(str)
		} else {
			attrs[key] = string(str)
		}

		pos += strLen
		i++
	}

	c.attributes = attrs

	return pos, nil
}
