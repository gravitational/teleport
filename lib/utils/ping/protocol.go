package ping

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

var timeFormat = "15:04:05.000"

func NewConn(conn net.Conn) *Conn {
	return &Conn{
		Conn: conn,
	}
}

type Conn struct {
	net.Conn
	nr   int
	size int32
	buff []byte
	mtx  sync.Mutex
}

func (c Conn) Read(b []byte) (int, error) {
	if c.nr == 0 {
		for c.size == 0 {
			if err := binary.Read(c.Conn, binary.LittleEndian, &c.size); err != nil {
				return 0, trace.Wrap(err)
			}
			if c.size == 0 {
				fmt.Printf("[%v] [PingConn]: Ping package received\n", time.Now().Format(timeFormat))
			}
		}
	}
	n, err := c.Conn.Read(b[:c.size])
	c.nr += n
	if int32(c.nr) >= c.size {
		c.nr = 0
	}
	return n, err
}

func (c Conn) Ping() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err := binary.Write(c.Conn, binary.LittleEndian, int32(0)); err != nil {
		return err
	}
	fmt.Printf("[%v] [PingConn]: Ping package send\n", time.Now().Format(timeFormat))
	return nil
}

func (c Conn) Write(b []byte) (int, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if len(b) == 0 {
		return 0, nil
	}
	size := int32(len(b))
	if err := binary.Write(c.Conn, binary.LittleEndian, size); err != nil {
		return 0, err
	}
	var total int
	for {
		n, err := c.Conn.Write(b)
		total += n
		if err != nil {
			return total, err
		}
		if total == len(b) {
			return total, nil
		}
	}
}
