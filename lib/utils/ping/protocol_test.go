package ping

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtocol(t *testing.T) {
	w, r := net.Pipe()

	connr := NewConn(r)
	connw := NewConn(w)

	for i := 1; i < 100000; i += 50 {
		want := strings.Repeat("x", i)
		go func() {
			_, err := connw.Write([]byte(want))
			require.NoError(t, err)
		}()
		buff := make([]byte, 100001)
		n, err := connr.Read(buff)
		require.Equal(t, i, n)
		require.Equal(t, want, string(buff[:n]))
		require.NoError(t, err)
	}
}

/*
func TestProtocol3(t *testing.T) {
	var bf bytes.Buffer
	connr := NewConn(&bf)
	connw := NewConn(&bf)
	data := []byte("fooo")

	err := connw.Ping()
	require.NoError(t, err)

	n, err := connw.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)
	err = connw.Ping()
	require.NoError(t, err)

	buff := make([]byte, 1024)
	n, err = connr.Read(buff)
	require.NoError(t, err)
	require.Equal(t, data, buff[:n])

	n, err = connw.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	n, err = connr.Read(buff)
	require.NoError(t, err)
	require.Equal(t, data, buff[:n])
}
*/
