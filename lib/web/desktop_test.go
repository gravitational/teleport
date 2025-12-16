package web

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	websocketBufferSize = 1024 * 16 // 16 KiB
)

func newWebsocketConn(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	var serverConn *websocket.Conn
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{
			ReadBufferSize:  websocketBufferSize,
			WriteBufferSize: websocketBufferSize,
		}
		serverConn, _ = u.Upgrade(w, r, nil)
	}))

	clientconn, _, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http", "ws", 1), nil)
	require.NoError(t, err)

	t.Cleanup(server.Close)
	return clientconn, serverConn
}

func TestProxyConnection(t *testing.T) {
	// Echos back any TDPB messages received
	tdpbEchoServer := func(conn net.Conn) error {
		rdr := bufio.NewReader(conn)
		for {
			msg, err := tdp.DecodeTDPB(rdr)
			if err != nil {
				return err
			}

			if err = msg.EncodeTo(conn); err != nil {
				return err
			}
		}
	}

	// Echos back any TDP messages received
	tdpEchoServer := func(conn net.Conn) error {
		tdpConn := tdp.NewConn(conn)
		for {
			msg, err := tdpConn.ReadMessage()
			if err != nil {
				return err
			}

			if err = tdpConn.WriteMessage(msg); err != nil {
				return err
			}
		}
	}

	tdpClient := func(w *websocket.Conn, expectLatency bool) error {
		conn := tdp.NewConn(&WebsocketIO{Conn: w})

		rdpMsg := tdp.RDPResponsePDU([]byte("hello"))
		mouseMsg := tdp.MouseWheel{Axis: 2, Delta: 4}

		if err := conn.WriteMessage(rdpMsg); err != nil {
			return err
		}
		if err := conn.WriteMessage(mouseMsg); err != nil {
			return err
		}

		// Read until we get these messages back
		gotRDP := false
		gotMouse := false
		gotLatency := !expectLatency
		for !(gotRDP && gotMouse && gotLatency) {
			msg, err := conn.ReadMessage()
			if err != nil {
				return err
			}

			switch m := msg.(type) {
			case tdp.RDPResponsePDU:
				assert.Equal(t, rdpMsg, m)
				gotRDP = true
			case tdp.MouseWheel:
				assert.Equal(t, mouseMsg, m)
				gotMouse = true
			case tdp.LatencyStats:
				if !expectLatency {
					t.Error("unexpected latency stats")
				}
				gotLatency = true
			default:
				return fmt.Errorf("received unexpected message type %T", m)
			}
		}
		return nil
	}

	tdpbClient := func(w *websocket.Conn, expectLatency bool) error {
		conn := tdp.NewConn(&WebsocketIO{Conn: w}, tdp.WithTDPBDecoder())

		rdpMsg := &tdpbv1.RDPResponsePDU{
			Response: []byte("hello"),
		}

		mouseMsg := &tdpbv1.MouseWheel{Axis: tdpbv1.MouseWheelAxis_MOUSE_WHEEL_AXIS_HORIZONTAL, Delta: 4}

		if err := conn.WriteMessage(tdp.NewTDPBMessage(rdpMsg)); err != nil {
			return err
		}
		if err := conn.WriteMessage(tdp.NewTDPBMessage(mouseMsg)); err != nil {
			return err
		}

		// Read until we get these messages back
		gotRDP := false
		gotMouse := false
		gotLatency := !expectLatency
		for !(gotRDP && gotMouse && gotLatency) {
			msg, err := conn.ReadMessage()
			if err != nil {
				return err
			}

			protoMsg, err := tdp.ToTDPBProto(msg)
			if err != nil {
				return err
			}

			switch m := protoMsg.(type) {
			case *tdpbv1.RDPResponsePDU:
				//assert.Truef(t, proto.Equal(m, rdpMsg), "got: %+v, expected: %+v", m, rdpMsg)
				assert.True(t, bytes.Equal(rdpMsg.Response, m.Response))
				gotRDP = true
			case *tdpbv1.MouseWheel:
				//assert.Truef(t, proto.Equal(m, mouseMsg), "got: %+v, expected: %+v", m, mouseMsg)
				assert.Equal(t, m.Axis, mouseMsg.Axis)
				assert.Equal(t, m.Delta, mouseMsg.Delta)
				gotMouse = true
			case *tdpbv1.LatencyStats:
				if !expectLatency {
					t.Error("unexpected latency stats")
				}
				gotLatency = true
			default:
				return fmt.Errorf("received unexpected message type %T", m)
			}
		}
		return nil
	}

	tests := []struct {
		name           string
		clientProtocol string
		serverProtocol string
		version        string
		echoFn         func(net.Conn) error
		clientFn       func(*websocket.Conn, bool) error
	}{
		{
			name:           "tdp-tdpb",
			clientProtocol: protocolTDP,
			serverProtocol: protocolTDPB,
			version:        "17.5.0",
			clientFn:       tdpClient,
			echoFn:         tdpbEchoServer,
		},
		{
			name:           "tdp-tdp",
			clientProtocol: protocolTDP,
			serverProtocol: protocolTDP,
			version:        "17.5.0",
			clientFn:       tdpClient,
			echoFn:         tdpEchoServer,
		},
		{
			name:           "tdpb-tdp",
			clientProtocol: protocolTDPB,
			serverProtocol: protocolTDP,
			version:        "17.5.0",
			clientFn:       tdpbClient,
			echoFn:         tdpEchoServer,
		},
		{
			name:           "tdpb-tdpb",
			clientProtocol: protocolTDPB,
			serverProtocol: protocolTDPB,
			version:        "17.5.0",
			clientFn:       tdpbClient,
			echoFn:         tdpbEchoServer,
		},
		{
			name:           "tdp-tdpb-no-latency-monitor",
			clientProtocol: protocolTDP,
			serverProtocol: protocolTDPB,
			/* server version does not support latency monitoring */
			version:  "17.0.0",
			clientFn: tdpClient,
			echoFn:   tdpbEchoServer,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We'll name the connection handles "inner" and "outter".
			// The proxy owns in "inner" handles while our test harness works with "outter" handles.
			// Outer Client Handle <---> Proxy(Inner Client Handle, Inner Server Handle) <---> Outer Server Handle
			serverIn, serverOut := net.Pipe()
			clientIn, clientOut := newWebsocketConn(t)

			wg := sync.WaitGroup{}
			var proxyErr, echoErr error
			wg.Go(func() {
				proxyErr = proxyWebsocketConn(t.Context(), clientIn, serverIn, test.version, test.clientProtocol, test.serverProtocol)
			})
			wg.Go(func() {
				echoErr = test.echoFn(serverOut)
			})

			latencySupported, _ := utils.MinVerWithoutPreRelease(test.version, "17.5.0")
			require.NoError(t, test.clientFn(clientOut, latencySupported))
			// Handle websocket shutdown
			require.NoError(t, clientOut.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Shutting down gracefully"), time.Now().Add(2*time.Second)))
			_, _, err := clientOut.ReadMessage()
			var wsErr *websocket.CloseError
			require.ErrorAs(t, err, &wsErr)
			require.Equal(t, wsErr.Code, websocket.CloseNormalClosure)

			wg.Wait()
			require.ErrorIs(t, echoErr, io.EOF)
			require.ErrorAs(t, proxyErr, &wsErr)
			require.Equal(t, wsErr.Code, websocket.CloseNormalClosure)
			_ = serverOut.Close()
		})
	}
}
