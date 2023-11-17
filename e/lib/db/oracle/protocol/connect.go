package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"regexp"

	"github.com/gravitational/trace"
)

// ConnectPacket defines TNS client connect oracle packet
// sent by a client to the oracle server.
type ConnectPacket struct {
	*packet
	// ConnectionString is a client connection string.
	ConnectionString string
	// ServerName is Oracle ServerName to connect.
	// The value is extracted from ConnectionString.
	ServerName string
}

var (
	// serviceNameRegexp allows to extract SERVICE_NAME from Oracle ConnectionString.
	// Ref: https://github.com/sijms/go-ora/blob/master/network/connect_option.go#L131
	serviceNameRegexp = regexp.MustCompile(`(?i)\(\s*SERVICE_NAME\s*=\s*([\w,\.,\-]+)\s*\)`)
)

func (p *ConnectPacket) parseConnectionString() error {
	match := serviceNameRegexp.FindStringSubmatch(p.ConnectionString)
	if len(match) != 2 {
		return trace.BadParameter("Failed to parse Connect Packet connection string: %q", p.ConnectionString)
	}
	p.ServerName = match[1]
	return nil
}

func parseConnectPacket(bp *packet, c *oracleConn) (Packet, error) {
	r := bytes.NewReader(bp.buff)
	if _, err := r.Seek(PacketHeaderSize, io.SeekCurrent); err != nil {
		return nil, trace.Wrap(err)
	}
	// Seek to the Connection String offset.
	if _, err := r.Seek(24, io.SeekStart); err != nil {
		return nil, trace.Wrap(err)
	}
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	var offset uint16
	if err := binary.Read(r, binary.BigEndian, &offset); err != nil {
		return nil, trace.Wrap(err)
	}

	for int(length) > len(bp.buff)-int(offset) {
		// The connection string overflow the basic Connect  Packet max size.
		// The Data packet will be sent with a full context of connecting string.
		moreData, err := c.readMoreData()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(moreData) < 2 {
			return nil, trace.BadParameter("connection data packet invalid length")
		}
		bp.buff = append(bp.buff, moreData[2:]...)
	}
	r = bytes.NewReader(bp.buff)

	if _, err := r.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, trace.Wrap(err)
	}
	buff := bytes.NewBuffer(make([]byte, 0, length))
	if _, err := io.CopyN(buff, r, int64(length)); err != nil {
		return nil, trace.Wrap(err)
	}

	cp := &ConnectPacket{
		packet:           bp,
		ConnectionString: buff.String(),
	}

	if err := cp.parseConnectionString(); err != nil {
		return nil, trace.Wrap(err)
	}
	return cp, nil
}
