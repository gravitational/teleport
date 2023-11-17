package protocol

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzReadPacket(f *testing.F) {
	f.Fuzz(func(t *testing.T, body []byte) {
		require.NotPanics(t, func() {
			pw, pr := net.Pipe()
			defer pr.Close()
			conn := &Conn{
				oracleConn: oracleConn{
					Conn: pr,
				},
			}
			defer conn.Close()
			go func() {
				pw.Write(body)
				pw.Close()
			}()
			_, _ = conn.readPacket()
		})
	})
}

func FuzzReadInt64(f *testing.F) {
	f.Fuzz(func(t *testing.T, body []byte) {
		require.NotPanics(t, func() {
			_, _ = readInt64(bytes.NewReader(body))
		})
	})
}
