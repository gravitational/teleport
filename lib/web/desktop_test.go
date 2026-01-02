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
	"github.com/gravitational/teleport/api/client/proto"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	tdp "github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
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
			msg, err := tdpb.Decode(rdr)
			if err != nil {
				return err
			}

			if err = tdp.EncodeTo(conn, msg); err != nil {
				return err
			}
		}
	}

	// Echos back any TDP messages received
	tdpEchoServer := func(conn net.Conn) error {
		tdpConn := tdp.NewConn(conn, legacy.Decode)
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
		conn := tdp.NewConn(&WebsocketIO{Conn: w}, legacy.Decode)

		rdpMsg := legacy.RDPResponsePDU([]byte("hello"))
		mouseMsg := legacy.MouseWheel{Axis: 2, Delta: 4}

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
			case legacy.RDPResponsePDU:
				assert.Equal(t, rdpMsg, m)
				gotRDP = true
			case legacy.MouseWheel:
				assert.Equal(t, mouseMsg, m)
				gotMouse = true
			case legacy.LatencyStats:
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
		conn := tdp.NewConn(&WebsocketIO{Conn: w}, tdp.DecoderAdapter(tdpb.Decode))

		rdpMsg := &tdpb.RDPResponsePDU{
			Response: []byte("hello"),
		}

		mouseMsg := &tdpb.MouseWheel{Axis: tdpbv1.MouseWheelAxis_MOUSE_WHEEL_AXIS_HORIZONTAL, Delta: 4}

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
			case *tdpb.RDPResponsePDU:
				//assert.Truef(t, proto.Equal(m, rdpMsg), "got: %+v, expected: %+v", m, rdpMsg)
				assert.True(t, bytes.Equal(rdpMsg.Response, m.Response))
				gotRDP = true
			case *tdpb.MouseWheel:
				//assert.Truef(t, proto.Equal(m, mouseMsg), "got: %+v, expected: %+v", m, mouseMsg)
				assert.Equal(t, m.Axis, mouseMsg.Axis)
				assert.Equal(t, m.Delta, mouseMsg.Delta)
				gotMouse = true
			case *tdpb.LatencyStats:
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

func TestHandshakeData(t *testing.T) {

	t.Run("empty-handshake", func(t *testing.T) {
		// Make sure empty handshake data doesn't cause a panic
		data := handshakeData{}
		require.Error(t, data.ForwardTDP(io.Discard, "user", false))
		require.Error(t, data.ForwardTDPB(io.Discard, "user"))
	})

	// Make sure that all combinations of TDP/TDPB input and output
	for _, test := range []struct {
		name string
		data handshakeData
	}{
		{"tdpb-input", handshakeData{
			hello: &tdpb.ClientHello{
				ScreenSpec: &tdpbv1.ClientScreenSpec{
					Height: 64,
					Width:  128,
				},
				KeyboardLayout: 1,
			},
		}},
		{"tdp-input", handshakeData{
			screenSpec: &legacy.ClientScreenSpec{
				Height: 64,
				Width:  128,
			},
			keyboardLayout: &legacy.ClientKeyboardLayout{
				KeyboardLayout: 1,
			},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			buf := bufCloser{Buffer: &bytes.Buffer{}}
			// ForwardTDPB should yield a single client_hello
			require.NoError(t, test.data.ForwardTDPB(buf, "user"))
			msg, err := tdpb.Decode(buf)
			require.NoError(t, err)
			require.IsType(t, &tdpb.ClientHello{}, msg)
			assert.Equal(t, "user", msg.(*tdpb.ClientHello).Username)
			_, err = tdpb.Decode(buf)
			require.ErrorIs(t, err, io.EOF)

			// ForwardTDP should yield 3 messages (if forwardKeyboardLayout == true)
			require.NoError(t, test.data.ForwardTDP(buf, "user", true))
			conn := tdp.NewConn(buf, legacy.Decode)

			tdpMessage, err := conn.ReadMessage()
			require.NoError(t, err)
			requireMessageIs[legacy.ClientUsername](t, tdpMessage)

			tdpMessage, err = conn.ReadMessage()
			require.NoError(t, err)
			requireMessageIs[legacy.ClientScreenSpec](t, tdpMessage)

			tdpMessage, err = conn.ReadMessage()
			require.NoError(t, err)
			requireMessageIs[legacy.ClientKeyboardLayout](t, tdpMessage)

			_, err = conn.ReadMessage()
			require.ErrorIs(t, err, io.EOF)
		})
	}

}

func requireMessageIs[T any](t *testing.T, msg tdp.Message) {
	_, ok := msg.(T)
	require.True(t, ok)
}

// Need a simple buffer that can act as a tdp.ReadWriterCloser
type bufCloser struct {
	*bytes.Buffer
}

func (_ bufCloser) Close() error { return nil }

func TestTDPBMFAFlow(t *testing.T) {
	client, server := net.Pipe()
	clientConn := tdp.NewConn(client, tdp.DecoderAdapter(tdpb.Decode))
	serverConn := tdp.NewConn(server, tdp.DecoderAdapter(tdpb.Decode))
	defer clientConn.Close()
	defer serverConn.Close()

	witheld := []tdp.Message{}
	promptFn := newTDPBMFAPrompt(serverConn, &witheld)("channel_id")
	requestMsg := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: &webauthnpb.CredentialAssertion{
			PublicKey: &webauthnpb.PublicKeyCredentialRequestOptions{
				Challenge: []byte("Some challenge"),
				TimeoutMs: 1000,
				RpId:      "teleport",
				AllowCredentials: []*webauthnpb.CredentialDescriptor{
					{Type: "some device", Id: []byte("1234")},
				},
			},
		},
	}

	type result struct {
		response *proto.MFAAuthenticateResponse
		err      error
	}

	done := make(chan result)
	go func() {
		response, err := promptFn(t.Context(), requestMsg)
		done <- result{response, err}
	}()

	// Simulate the client
	mfaMessage := expectTDPBMessage[*tdpb.MFA](t, clientConn)
	// Validate the received MFA challenge matches wahat was sent
	assert.Equal(t, requestMsg.WebauthnChallenge, mfaMessage.Challenge.WebauthnChallenge)
	assert.Equal(t, "channel_id", mfaMessage.ChannelId)

	// Send a random, non-MFA TDPB message
	require.NoError(t, clientConn.WriteMessage(&tdpb.Alert{Message: "random message!"}))

	response := &mfav1.AuthenticateResponse{
		Response: &mfav1.AuthenticateResponse_Webauthn{
			Webauthn: &webauthnpb.CredentialAssertionResponse{
				Type:  "sometype",
				RawId: []byte("rawid"),
				Response: &webauthnpb.AuthenticatorAssertionResponse{
					ClientDataJson: []byte(`{"data": "value"}`),
					Signature:      []byte("john hancock"),
				},
			},
		},
	}
	// Send response
	err := clientConn.WriteMessage(
		&tdpb.MFA{
			AuthenticationResponse: response,
		},
	)
	require.NoError(t, err)
	// Wait for MFA flow to complete and return the response
	res := <-done
	require.NoError(t, res.err)
	// Response should match what was sent
	assert.Equal(t, response.GetWebauthn(), res.response.GetWebauthn())
	// Should still have that alert message in our withheld message slice
	assert.Len(t, witheld, 1)
}

func expectTDPBMessage[T any](t *testing.T, c *tdp.Conn) T {
	var zero T
	msg, err := c.ReadMessage()
	require.NoError(t, err)

	require.IsType(t, msg, zero)
	return msg.(T)
}
