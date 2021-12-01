package packet

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"sync"

	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"

	. "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/utils"
	"github.com/pingcap/errors"
)

type BufPool struct {
	pool *sync.Pool
}

func NewBufPool() *BufPool {
	return &BufPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

func (b *BufPool) Get() *bytes.Buffer {
	return b.pool.Get().(*bytes.Buffer)
}

func (b *BufPool) Return(buf *bytes.Buffer) {
	buf.Reset()
	b.pool.Put(buf)
}

/*
	Conn is the base class to handle MySQL protocol.
*/
type Conn struct {
	net.Conn

	// we removed the buffer reader because it will cause the SSLRequest to block (tls connection handshake won't be
	// able to read the "Client Hello" data since it has been buffered into the buffer reader)

	bufPool *BufPool
	br      *bufio.Reader
	reader  io.Reader

	copyNBuf []byte

	header [4]byte

	Sequence uint8
}

func NewConn(conn net.Conn) *Conn {
	c := new(Conn)
	c.Conn = conn

	c.bufPool = NewBufPool()
	c.br = bufio.NewReaderSize(c, 65536) // 64kb
	c.reader = c.br

	c.copyNBuf = make([]byte, 16*1024)

	return c
}

func NewTLSConn(conn net.Conn) *Conn {
	c := new(Conn)
	c.Conn = conn

	c.bufPool = NewBufPool()
	c.reader = c

	c.copyNBuf = make([]byte, 16*1024)

	return c
}

func (c *Conn) ReadPacket() ([]byte, error) {
	return c.ReadPacketReuseMem(nil)
}

func (c *Conn) ReadPacketReuseMem(dst []byte) ([]byte, error) {
	// Here we use `sync.Pool` to avoid allocate/destroy buffers frequently.
	buf := utils.BytesBufferGet()
	defer utils.BytesBufferPut(buf)

	if err := c.ReadPacketTo(buf); err != nil {
		return nil, errors.Trace(err)
	} else {
		result := append(dst, buf.Bytes()...)
		return result, nil
	}
}

func (c *Conn) copyN(dst io.Writer, src io.Reader, n int64) (written int64, err error) {
	for n > 0 {
		bcap := cap(c.copyNBuf)
		if int64(bcap) > n {
			bcap = int(n)
		}
		buf := c.copyNBuf[:bcap]

		rd, err := io.ReadAtLeast(src, buf, bcap)
		n -= int64(rd)

		if err != nil {
			return written, errors.Trace(err)
		}

		wr, err := dst.Write(buf)
		written += int64(wr)
		if err != nil {
			return written, errors.Trace(err)
		}
	}

	return written, nil
}

func (c *Conn) ReadPacketTo(w io.Writer) error {
	if _, err := io.ReadFull(c.reader, c.header[:4]); err != nil {
		return errors.Wrapf(ErrBadConn, "io.ReadFull(header) failed. err %v", err)
	}

	length := int(uint32(c.header[0]) | uint32(c.header[1])<<8 | uint32(c.header[2])<<16)
	sequence := c.header[3]

	if sequence != c.Sequence {
		return errors.Errorf("invalid sequence %d != %d", sequence, c.Sequence)
	}

	c.Sequence++

	if buf, ok := w.(*bytes.Buffer); ok {
		// Allocate the buffer with expected length directly instead of call `grow` and migrate data many times.
		buf.Grow(length)
	}

	if n, err := c.copyN(w, c.reader, int64(length)); err != nil {
		return errors.Wrapf(ErrBadConn, "io.CopyN failed. err %v, copied %v, expected %v", err, n, length)
	} else if n != int64(length) {
		return errors.Wrapf(ErrBadConn, "io.CopyN failed(n != int64(length)). %v bytes copied, while %v expected", n, length)
	} else {
		if length < MaxPayloadLen {
			return nil
		}

		if err := c.ReadPacketTo(w); err != nil {
			return errors.Wrap(err, "ReadPacketTo failed")
		}
	}

	return nil
}

// WritePacket: data already has 4 bytes header
// will modify data inplace
func (c *Conn) WritePacket(data []byte) error {
	length := len(data) - 4

	for length >= MaxPayloadLen {
		data[0] = 0xff
		data[1] = 0xff
		data[2] = 0xff

		data[3] = c.Sequence

		if n, err := c.Write(data[:4+MaxPayloadLen]); err != nil {
			return errors.Wrapf(ErrBadConn, "Write(payload portion) failed. err %v", err)
		} else if n != (4 + MaxPayloadLen) {
			return errors.Wrapf(ErrBadConn, "Write(payload portion) failed. only %v bytes written, while %v expected", n, 4+MaxPayloadLen)
		} else {
			c.Sequence++
			length -= MaxPayloadLen
			data = data[MaxPayloadLen:]
		}
	}

	data[0] = byte(length)
	data[1] = byte(length >> 8)
	data[2] = byte(length >> 16)
	data[3] = c.Sequence

	if n, err := c.Write(data); err != nil {
		return errors.Wrapf(ErrBadConn, "Write failed. err %v", err)
	} else if n != len(data) {
		return errors.Wrapf(ErrBadConn, "Write failed. only %v bytes written, while %v expected", n, len(data))
	} else {
		c.Sequence++
		return nil
	}
}

// WriteClearAuthPacket: Client clear text authentication packet
// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::AuthSwitchResponse
func (c *Conn) WriteClearAuthPacket(password string) error {
	// Calculate the packet length and add a tailing 0
	pktLen := len(password) + 1
	data := make([]byte, 4+pktLen)

	// Add the clear password [null terminated string]
	copy(data[4:], password)
	data[4+pktLen-1] = 0x00

	return errors.Wrap(c.WritePacket(data), "WritePacket failed")
}

// WritePublicKeyAuthPacket: Caching sha2 authentication. Public key request and send encrypted password
// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::AuthSwitchResponse
func (c *Conn) WritePublicKeyAuthPacket(password string, cipher []byte) error {
	// request public key
	data := make([]byte, 4+1)
	data[4] = 2 // cachingSha2PasswordRequestPublicKey
	if err := c.WritePacket(data); err != nil {
		return errors.Wrap(err, "WritePacket(single byte) failed")
	}

	data, err := c.ReadPacket()
	if err != nil {
		return errors.Wrap(err, "ReadPacket failed")
	}

	block, _ := pem.Decode(data[1:])
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "x509.ParsePKIXPublicKey failed")
	}

	plain := make([]byte, len(password)+1)
	copy(plain, password)
	for i := range plain {
		j := i % len(cipher)
		plain[i] ^= cipher[j]
	}
	sha1v := sha1.New()
	enc, _ := rsa.EncryptOAEP(sha1v, rand.Reader, pub.(*rsa.PublicKey), plain, nil)
	data = make([]byte, 4+len(enc))
	copy(data[4:], enc)
	return errors.Wrap(c.WritePacket(data), "WritePacket failed")
}

func (c *Conn) WriteEncryptedPassword(password string, seed []byte, pub *rsa.PublicKey) error {
	enc, err := EncryptPassword(password, seed, pub)
	if err != nil {
		return errors.Wrap(err, "EncryptPassword failed")
	}
	return errors.Wrap(c.WriteAuthSwitchPacket(enc, false), "WriteAuthSwitchPacket failed")
}

// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::AuthSwitchResponse
func (c *Conn) WriteAuthSwitchPacket(authData []byte, addNUL bool) error {
	pktLen := 4 + len(authData)
	if addNUL {
		pktLen++
	}
	data := make([]byte, pktLen)

	// Add the auth data [EOF]
	copy(data[4:], authData)
	if addNUL {
		data[pktLen-1] = 0x00
	}

	return errors.Wrap(c.WritePacket(data), "WritePacket failed")
}

func (c *Conn) ResetSequence() {
	c.Sequence = 0
}

func (c *Conn) Close() error {
	c.Sequence = 0
	if c.Conn != nil {
		return errors.Wrap(c.Conn.Close(), "Conn.Close failed")
	}
	return nil
}
