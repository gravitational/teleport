package client

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"

	"github.com/pingcap/errors"
	. "github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/utils"
	"github.com/siddontang/go/hack"
)

func (c *Conn) readUntilEOF() (err error) {
	var data []byte

	for {
		data, err = c.ReadPacket()

		if err != nil {
			return
		}

		// EOF Packet
		if c.isEOFPacket(data) {
			return
		}
	}
	return
}

func (c *Conn) isEOFPacket(data []byte) bool {
	return data[0] == EOF_HEADER && len(data) <= 5
}

func (c *Conn) handleOKPacket(data []byte) (*Result, error) {
	var n int
	var pos = 1

	r := new(Result)

	r.AffectedRows, _, n = LengthEncodedInt(data[pos:])
	pos += n
	r.InsertId, _, n = LengthEncodedInt(data[pos:])
	pos += n

	if c.capability&CLIENT_PROTOCOL_41 > 0 {
		r.Status = binary.LittleEndian.Uint16(data[pos:])
		c.status = r.Status
		pos += 2

		//todo:strict_mode, check warnings as error
		//Warnings := binary.LittleEndian.Uint16(data[pos:])
		//pos += 2
	} else if c.capability&CLIENT_TRANSACTIONS > 0 {
		r.Status = binary.LittleEndian.Uint16(data[pos:])
		c.status = r.Status
		pos += 2
	}

	//new ok package will check CLIENT_SESSION_TRACK too, but I don't support it now.

	//skip info
	return r, nil
}

func (c *Conn) handleErrorPacket(data []byte) error {
	e := new(MyError)

	var pos = 1

	e.Code = binary.LittleEndian.Uint16(data[pos:])
	pos += 2

	if c.capability&CLIENT_PROTOCOL_41 > 0 {
		//skip '#'
		pos++
		e.State = hack.String(data[pos : pos+5])
		pos += 5
	}

	e.Message = hack.String(data[pos:])

	return e
}

func (c *Conn) handleAuthResult() error {
	data, switchToPlugin, err := c.readAuthResult()
	if err != nil {
		return err
	}
	// handle auth switch, only support 'sha256_password', and 'caching_sha2_password'
	if switchToPlugin != "" {
		//fmt.Printf("now switching auth plugin to '%s'\n", switchToPlugin)
		if data == nil {
			data = c.salt
		} else {
			copy(c.salt, data)
		}
		c.authPluginName = switchToPlugin
		auth, addNull, err := c.genAuthResponse(data)
		if err = c.WriteAuthSwitchPacket(auth, addNull); err != nil {
			return err
		}

		// Read Result Packet
		data, switchToPlugin, err = c.readAuthResult()
		if err != nil {
			return err
		}

		// Do not allow to change the auth plugin more than once
		if switchToPlugin != "" {
			return errors.Errorf("can not switch auth plugin more than once")
		}
	}

	// handle caching_sha2_password
	if c.authPluginName == AUTH_CACHING_SHA2_PASSWORD {
		if data == nil {
			return nil // auth already succeeded
		}
		if data[0] == CACHE_SHA2_FAST_AUTH {
			if _, err = c.readOK(); err == nil {
				return nil // auth successful
			}
		} else if data[0] == CACHE_SHA2_FULL_AUTH {
			// need full authentication
			if c.tlsConfig != nil || c.proto == "unix" {
				if err = c.WriteClearAuthPacket(c.password); err != nil {
					return err
				}
			} else {
				if err = c.WritePublicKeyAuthPacket(c.password, c.salt); err != nil {
					return err
				}
			}
		} else {
			errors.Errorf("invalid packet")
		}
	} else if c.authPluginName == AUTH_SHA256_PASSWORD {
		if len(data) == 0 {
			return nil // auth already succeeded
		}
		block, _ := pem.Decode(data)
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return err
		}
		// send encrypted password
		err = c.WriteEncryptedPassword(c.password, c.salt, pub.(*rsa.PublicKey))
		if err != nil {
			return err
		}
		_, err = c.readOK()
		return err
	}
	return nil
}

func (c *Conn) readAuthResult() ([]byte, string, error) {
	data, err := c.ReadPacket()
	if err != nil {
		return nil, "", err
	}

	// see: https://insidemysql.com/preparing-your-community-connector-for-mysql-8-part-2-sha256/
	// packet indicator
	switch data[0] {

	case OK_HEADER:
		_, err := c.handleOKPacket(data)
		return nil, "", err

	case MORE_DATE_HEADER:
		return data[1:], "", err

	case EOF_HEADER:
		// server wants to switch auth
		if len(data) < 1 {
			// https://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::OldAuthSwitchRequest
			return nil, AUTH_MYSQL_OLD_PASSWORD, nil
		}
		pluginEndIndex := bytes.IndexByte(data, 0x00)
		if pluginEndIndex < 0 {
			return nil, "", errors.New("invalid packet")
		}
		plugin := string(data[1:pluginEndIndex])
		authData := data[pluginEndIndex+1:]
		return authData, plugin, nil

	default: // Error otherwise
		return nil, "", c.handleErrorPacket(data)
	}
}

func (c *Conn) readOK() (*Result, error) {
	data, err := c.ReadPacket()
	if err != nil {
		return nil, errors.Trace(err)
	}

	if data[0] == OK_HEADER {
		return c.handleOKPacket(data)
	} else if data[0] == ERR_HEADER {
		return nil, c.handleErrorPacket(data)
	} else {
		return nil, errors.New("invalid ok packet")
	}
}

func (c *Conn) readResult(binary bool) (*Result, error) {
	firstPkgBuf, err := c.ReadPacketReuseMem(utils.ByteSliceGet(16)[:0])
	defer utils.ByteSlicePut(firstPkgBuf)

	if err != nil {
		return nil, errors.Trace(err)
	}

	if firstPkgBuf[0] == OK_HEADER {
		return c.handleOKPacket(firstPkgBuf)
	} else if firstPkgBuf[0] == ERR_HEADER {
		return nil, c.handleErrorPacket(append([]byte{}, firstPkgBuf...))
	} else if firstPkgBuf[0] == LocalInFile_HEADER {
		return nil, ErrMalformPacket
	}

	return c.readResultset(firstPkgBuf, binary)
}

func (c *Conn) readResultset(data []byte, binary bool) (*Result, error) {
	// column count
	count, _, n := LengthEncodedInt(data)

	if n-len(data) != 0 {
		return nil, ErrMalformPacket
	}

	result := &Result{
		Resultset: NewResultset(int(count)),
	}

	if err := c.readResultColumns(result); err != nil {
		return nil, errors.Trace(err)
	}

	if err := c.readResultRows(result, binary); err != nil {
		return nil, errors.Trace(err)
	}

	return result, nil
}

func (c *Conn) readResultColumns(result *Result) (err error) {
	var i int = 0
	var data []byte

	for {
		rawPkgLen := len(result.RawPkg)
		result.RawPkg, err = c.ReadPacketReuseMem(result.RawPkg)
		if err != nil {
			return
		}
		data = result.RawPkg[rawPkgLen:]

		// EOF Packet
		if c.isEOFPacket(data) {
			if c.capability&CLIENT_PROTOCOL_41 > 0 {
				//result.Warnings = binary.LittleEndian.Uint16(data[1:])
				//todo add strict_mode, warning will be treat as error
				result.Status = binary.LittleEndian.Uint16(data[3:])
				c.status = result.Status
			}

			if i != len(result.Fields) {
				err = ErrMalformPacket
			}

			return
		}

		if result.Fields[i] == nil {
			result.Fields[i] = &Field{}
		}
		err = result.Fields[i].Parse(data)
		if err != nil {
			return
		}

		result.FieldNames[hack.String(result.Fields[i].Name)] = i

		i++
	}
}

func (c *Conn) readResultRows(result *Result, isBinary bool) (err error) {
	var data []byte

	for {
		rawPkgLen := len(result.RawPkg)
		result.RawPkg, err = c.ReadPacketReuseMem(result.RawPkg)
		if err != nil {
			return
		}
		data = result.RawPkg[rawPkgLen:]

		// EOF Packet
		if c.isEOFPacket(data) {
			if c.capability&CLIENT_PROTOCOL_41 > 0 {
				//result.Warnings = binary.LittleEndian.Uint16(data[1:])
				//todo add strict_mode, warning will be treat as error
				result.Status = binary.LittleEndian.Uint16(data[3:])
				c.status = result.Status
			}

			break
		}

		if data[0] == ERR_HEADER {
			return c.handleErrorPacket(data)
		}

		result.RowDatas = append(result.RowDatas, data)
	}

	if cap(result.Values) < len(result.RowDatas) {
		result.Values = make([][]FieldValue, len(result.RowDatas))
	} else {
		result.Values = result.Values[:len(result.RowDatas)]
	}

	for i := range result.Values {
		result.Values[i], err = result.RowDatas[i].Parse(result.Fields, isBinary, result.Values[i])

		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}
