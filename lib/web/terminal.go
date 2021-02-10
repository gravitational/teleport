/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	authresource "github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/gogo/protobuf/proto"
	"github.com/sirupsen/logrus"
)

// TerminalRequest describes a request to create a web-based terminal
// to a remote SSH server.
type TerminalRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port]).
	Server string `json:"server_id"`

	// Login is Linux username to connect as.
	Login string `json:"login"`

	// Term is the initial PTY size.
	Term session.TerminalParams `json:"term"`

	// SessionID is a Teleport session ID to join as.
	SessionID session.ID `json:"sid"`

	// Namespace is node namespace.
	Namespace string `json:"namespace"`

	// ProxyHostPort is the address of the server to connect to.
	ProxyHostPort string `json:"-"`

	// Cluster is the name of the remote cluster to connect to.
	Cluster string `json:"-"`

	// InteractiveCommand is a command to execut.e
	InteractiveCommand []string `json:"-"`

	// KeepAliveInterval is the interval for sending ping frames to web client.
	KeepAliveInterval time.Duration
}

// AuthProvider is a subset of the full Auth API.
type AuthProvider interface {
	GetNodes(namespace string, opts ...auth.MarshalOption) ([]services.Server, error)
	GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]events.EventFields, error)
}

// NewTerminal creates a web-based terminal based on WebSockets and returns a
// new TerminalHandler.
func NewTerminal(req TerminalRequest, authProvider AuthProvider, ctx *SessionContext) (*TerminalHandler, error) {
	// Make sure whatever session is requested is a valid session.
	_, err := session.ParseID(string(req.SessionID))
	if err != nil {
		return nil, trace.BadParameter("sid: invalid session id")
	}

	if req.Login == "" {
		return nil, trace.BadParameter("login: missing login")
	}
	if req.Term.W <= 0 || req.Term.H <= 0 {
		return nil, trace.BadParameter("term: bad term dimensions")
	}

	servers, err := authProvider.GetNodes(req.Namespace, authresource.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DELETE IN: 5.0
	//
	// All proxies will support lookup by uuid, so host/port lookup
	// and fallback can be dropped entirely.
	hostName, hostPort, err := resolveServerHostPort(req.Server, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", req.Server, err)
	}

	return &TerminalHandler{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentWebsocket,
		}),
		params:       req,
		ctx:          ctx,
		hostName:     hostName,
		hostPort:     hostPort,
		hostUUID:     req.Server,
		authProvider: authProvider,
		encoder:      unicode.UTF8.NewEncoder(),
		decoder:      unicode.UTF8.NewDecoder(),
	}, nil
}

// TerminalHandler connects together an SSH session with a web-based
// terminal via a web socket.
type TerminalHandler struct {
	// log holds the structured logger.
	log *logrus.Entry

	// params is the initial PTY size.
	params TerminalRequest

	// ctx is a web session context for the currently logged in user.
	ctx *SessionContext

	// hostName is the hostname of the server.
	hostName string

	// hostPort is the port of the server.
	hostPort int

	// hostUUID is the UUID of the server.
	hostUUID string

	// sshSession holds the "shell" SSH channel to the node.
	sshSession *ssh.Session

	// terminalContext is used to signal when the terminal sesson is closing.
	terminalContext context.Context

	// terminalCancel is used to signal when the terminal session is closing.
	terminalCancel context.CancelFunc

	// authProvider is used to fetch nodes and sessions from the backend.
	authProvider AuthProvider

	// encoder is used to encode strings into UTF-8.
	encoder *encoding.Encoder

	// decoder is used to decode UTF-8 strings.
	decoder *encoding.Decoder

	// buffer is a buffer used to store the remaining payload data if it did not
	// fit into the buffer provided by the callee to Read method
	buffer []byte

	closeOnce sync.Once
}

// Serve builds a connect to the remote node and then pumps back two types of
// events: raw input/output events for what's happening on the terminal itself
// and audit log events relevant to this session.
func (t *TerminalHandler) Serve(w http.ResponseWriter, r *http.Request) {
	// This allows closing of the websocket if the user logs out before exiting
	// the session.
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	// We initial a server explicitly here instead of using websocket.HandlerFunc
	// to set an empty origin checker (this is to make our lives easier in tests).
	// The main use of the origin checker is to enforce the browsers same-origin
	// policy. That does not matter here because even if malicious Javascript
	// would try and open a websocket the request to this endpoint requires the
	// bearer token to be in the URL so it would not be sent along by default
	// like cookies are.
	ws := &websocket.Server{Handler: t.handler}
	ws.ServeHTTP(w, r)
}

// Close the websocket stream.
func (t *TerminalHandler) Close() error {
	t.closeOnce.Do(func() {
		// Close the SSH connection to the remote node.
		if t.sshSession != nil {
			t.sshSession.Close()
		}

		// If the terminal handler was closed (most likely due to the *SessionContext
		// closing) then the stream should be closed as well.
		t.terminalCancel()
	})
	return nil
}

// handler is the main websocket loop. It creates a Teleport client and then
// pumps raw events and audit events back to the client until the SSH session
// is complete.
func (t *TerminalHandler) handler(ws *websocket.Conn) {
	defer ws.Close()

	// Create a context for signaling when the terminal session is over.
	t.terminalContext, t.terminalCancel = context.WithCancel(context.Background())

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(ws)
	if err != nil {
		t.log.WithError(err).Infof("Failed creating a client for session %v.", t.params.SessionID)
		writeErr := t.writeError(err, ws)
		if writeErr != nil {
			t.log.WithError(writeErr).Warnf("Unable to send error to terminal.")
		}
		return
	}

	t.log.Debugf("Creating websocket stream for %v.", t.params.SessionID)

	// Start sending ping frames through websocket to client.
	go t.startPingLoop(ws)

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamTerminal(ws, tc)
	go t.streamEvents(ws, tc)

	// Block until the terminal session is complete.
	<-t.terminalContext.Done()
	t.log.Debugf("Closing websocket stream for %v.", t.params.SessionID)
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *TerminalHandler) makeClient(ws *websocket.Conn) (*client.TeleportClient, error) {
	clientConfig, err := makeTeleportClientConfig(t.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a terminal stream that wraps/unwraps the envelope used to
	// communicate over the websocket.
	stream := t.asTerminalStream(ws)

	clientConfig.ForwardAgent = true
	clientConfig.HostLogin = t.params.Login
	clientConfig.Namespace = t.params.Namespace
	clientConfig.Stdout = stream
	clientConfig.Stderr = stream
	clientConfig.Stdin = stream
	clientConfig.SiteName = t.params.Cluster
	if err := clientConfig.ParseProxyHost(t.params.ProxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	clientConfig.Host = t.hostName
	clientConfig.HostPort = t.hostPort
	clientConfig.Env = map[string]string{sshutils.SessionEnvVar: string(t.params.SessionID)}
	clientConfig.ClientAddr = ws.Request().RemoteAddr

	if len(t.params.InteractiveCommand) > 0 {
		clientConfig.Interactive = true
	}

	tc, err := client.NewClient(clientConfig)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	// Save the *ssh.Session after the shell has been created. The session is
	// used to update all other parties window size to that of the web client and
	// to allow future window changes.
	tc.OnShellCreated = func(s *ssh.Session, c *ssh.Client, _ io.ReadWriteCloser) (bool, error) {
		t.sshSession = s
		t.windowChange(&t.params.Term)

		return false, nil
	}

	if err := t.issueSessionMFACerts(tc, ws); err != nil {
		return nil, trace.Wrap(err)
	}

	return tc, nil
}

func (t *TerminalHandler) issueSessionMFACerts(tc *client.TeleportClient, ws *websocket.Conn) error {
	pc, err := tc.ConnectToProxy(t.terminalContext)
	if err != nil {
		return trace.Wrap(err)
	}
	defer pc.Close()

	priv, err := ssh.ParsePrivateKey(t.ctx.session.GetPriv())
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := pc.IssueUserCertsWithMFA(t.terminalContext, client.ReissueParams{
		RouteToCluster: t.params.Cluster,
		NodeName:       t.params.Server,
		ExistingCreds: &client.Key{
			Pub:     ssh.MarshalAuthorizedKey(priv.PublicKey()),
			Priv:    t.ctx.session.GetPriv(),
			Cert:    t.ctx.session.GetPub(),
			TLSCert: t.ctx.session.GetTLSCert(),
		},
	}, t.promptMFAChallenge(ws))
	if err != nil {
		return trace.Wrap(err)
	}

	am, err := key.AsAuthMethod()
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AuthMethods = []ssh.AuthMethod{am}
	return nil
}

func (t *TerminalHandler) promptMFAChallenge(ws *websocket.Conn) client.PromptMFAChallengeHandler {
	return func(ctx context.Context, proxyAddr string, c *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
		if len(c.U2F) == 0 {
			return nil, trace.AccessDenied("only U2F challenges are supported on the web terminal, please register a U2F device using 'tsh mfa add' to connect to this server")
		}

		// Convert from proto to JSON types.
		u2fChals := make([]u2f.AuthenticateChallenge, 0, len(c.U2F))
		for _, uc := range c.U2F {
			u2fChals = append(u2fChals, u2f.AuthenticateChallenge{
				Version:   uc.Version,
				Challenge: uc.Challenge,
				KeyHandle: uc.KeyHandle,
				AppID:     uc.AppID,
			})
		}
		chal := &server.MFAAuthenticateChallenge{
			AuthenticateChallenge: &u2f.AuthenticateChallenge{
				// Get the common challenge fields from the first item.
				// All of these fields should be identical for all u2fChals.
				Challenge: u2fChals[0].Challenge,
				AppID:     u2fChals[0].AppID,
				Version:   u2fChals[0].Version,
			},
			U2FChallenges: u2fChals,
		}

		// Send the challenge over the socket.
		chalEnc, err := json.Marshal(chal)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		envelope := &Envelope{
			Version: defaults.WebsocketVersion,
			Type:    defaults.WebsocketU2FChallenge,
			Payload: string(chalEnc),
		}
		envelopeBytes, err := proto.Marshal(envelope)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = websocket.Message.Send(ws, envelopeBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Read the challenge response.
		var bytes []byte
		if err = websocket.Message.Receive(ws, &bytes); err != nil {
			return nil, trace.Wrap(err)
		}
		// Reset envelope to zero value.
		envelope = &Envelope{}
		if err = proto.Unmarshal(bytes, envelope); err != nil {
			return nil, trace.Wrap(err)
		}
		var resp u2f.AuthenticateChallengeResponse
		if err := json.Unmarshal([]byte(envelope.Payload), &resp); err != nil {
			return nil, trace.Wrap(err)
		}
		// Convert from JSON to proto.
		return &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_U2F{
				U2F: &authproto.U2FResponse{
					KeyHandle:  resp.KeyHandle,
					ClientData: resp.ClientData,
					Signature:  resp.SignatureData,
				},
			},
		}, nil
	}
}

// startPingLoop starts a loop that will continuously send a ping frame through the websocket
// to prevent the connection between web client and teleport proxy from becoming idle.
// Interval is determined by the keep_alive_interval config set by user (or default).
// Loop will terminate when there is an error sending ping frame or when terminal session is closed.
func (t *TerminalHandler) startPingLoop(ws *websocket.Conn) {
	// Define our own marshal func to just return a ping payload type.
	codec := websocket.Codec{Marshal: func(v interface{}) (data []byte, payloadType byte, err error) {
		return nil, websocket.PingFrame, nil
	}}

	t.log.Debugf("Starting websocket ping loop with interval %v.", t.params.KeepAliveInterval)
	tickerCh := time.NewTicker(t.params.KeepAliveInterval)
	defer tickerCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			// Pongs are internally handled by the websocket library.
			// https://github.com/golang/net/blob/master/websocket/hybi.go#L291
			if err := codec.Send(ws, nil); err != nil {
				t.log.Errorf("Unable to send ping frame to web client: %v.", err)
				t.Close()
				return
			}
		case <-t.terminalContext.Done():
			t.log.Debug("Terminating websocket ping loop.")
			return
		}
	}
}

// streamTerminal opens a SSH connection to the remote host and streams
// events back to the web client.
func (t *TerminalHandler) streamTerminal(ws *websocket.Conn, tc *client.TeleportClient) {
	defer t.terminalCancel()

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	err := tc.SSH(t.terminalContext, t.params.InteractiveCommand, false)

	// TODO IN: 5.0
	//
	// Make connecting by UUID the default instead of the fallback.
	//
	if err != nil && strings.Contains(err.Error(), teleport.NodeIsAmbiguous) {
		t.log.Debugf("Ambiguous hostname %q, attempting to connect by UUID (%q).", t.hostName, t.hostUUID)
		tc.Host = t.hostUUID
		// We don't technically need to zero the HostPort, but future version won't look up
		// HostPort when connecting by UUID, so its best to keep behavior consistent.
		tc.HostPort = 0
		err = tc.SSH(t.terminalContext, t.params.InteractiveCommand, false)
	}

	if err != nil {
		t.log.Warnf("Unable to stream terminal: %v.", err)
		er := t.writeError(err, ws)
		if er != nil {
			t.log.Warnf("Unable to send error to terminal: %v: %v.", err, er)
		}
		return
	}

	// Check if remote process exited with error code, eg: RemoteCommandFailure (255).
	if t.sshSession != nil {
		if err := t.sshSession.Wait(); err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				t.log.Warnf("Remote shell exited with error code: %v", exitErr.ExitStatus())
				return
			}
		}
	}

	// Send close envelope to web terminal upon exit without an error.
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketClose,
		Payload: "",
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		t.log.Errorf("Unable to marshal close event for web client.")
		return
	}
	err = websocket.Message.Send(ws, envelopeBytes)
	if err != nil {
		t.log.Errorf("Unable to send close event to web client.")
		return
	}
	t.log.Debugf("Sent close event to web client.")
}

// streamEvents receives events over the SSH connection and forwards them to
// the web client.
func (t *TerminalHandler) streamEvents(ws *websocket.Conn, tc *client.TeleportClient) {
	for {
		select {
		// Send push events that come over the events channel to the web client.
		case event := <-tc.EventsChannel():
			data, err := json.Marshal(event)
			logger := t.log.WithField("event", event.GetType())
			if err != nil {
				logger.Errorf("Unable to marshal audit event: %v.", err)
				continue
			}

			t.log.Debugf("Sending audit event %v to web client.", event.GetType())

			// UTF-8 encode the error message and then wrap it in a raw envelope.
			encodedPayload, err := t.encoder.String(string(data))
			if err != nil {
				logger.Debugf("Unable to send audit event to web client: %v.", err)
				continue
			}
			envelope := &Envelope{
				Version: defaults.WebsocketVersion,
				Type:    defaults.WebsocketAudit,
				Payload: encodedPayload,
			}
			envelopeBytes, err := proto.Marshal(envelope)
			if err != nil {
				logger.Debugf("Unable to send audit event to web client: %v.", err)
				continue
			}

			// Send bytes over the websocket to the web client.
			err = websocket.Message.Send(ws, envelopeBytes)
			if err != nil {
				logger.Errorf("Unable to send audit event to web client: %v.", err)
				continue
			}
		// Once the terminal stream is over (and the close envelope has been sent),
		// close stop streaming envelopes.
		case <-t.terminalContext.Done():
			return
		}
	}
}

// windowChange is called when the browser window is resized. It sends a
// "window-change" channel request to the server.
func (t *TerminalHandler) windowChange(params *session.TerminalParams) {
	if t.sshSession == nil {
		return
	}

	_, err := t.sshSession.SendRequest(
		sshutils.WindowChangeRequest,
		false,
		ssh.Marshal(sshutils.WinChangeReqParams{
			W: uint32(params.W),
			H: uint32(params.H),
		}))
	if err != nil {
		t.log.Error(err)
	}
}

// writeError displays an error in the terminal window.
func (t *TerminalHandler) writeError(err error, ws *websocket.Conn) error {
	// Replace \n with \r\n so the message correctly aligned.
	r := strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n")
	errMessage := r.Replace(err.Error())
	_, err = t.write([]byte(errMessage), ws)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// resolveServerHostPort parses server name and attempts to resolve hostname
// and port.
func resolveServerHostPort(servername string, existingServers []services.Server) (string, int, error) {
	// If port is 0, client wants us to figure out which port to use.
	var defaultPort = 0

	if servername == "" {
		return "", defaultPort, trace.BadParameter("empty server name")
	}

	// Check if servername is UUID.
	for i := range existingServers {
		node := existingServers[i]
		if node.GetName() == servername {
			return node.GetHostname(), defaultPort, nil
		}
	}

	if !strings.Contains(servername, ":") {
		return servername, defaultPort, nil
	}

	// Check for explicitly specified port.
	host, portString, err := utils.SplitHostPort(servername)
	if err != nil {
		return "", defaultPort, trace.Wrap(err)
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return "", defaultPort, trace.BadParameter("invalid port: %v", err)
	}

	return host, port, nil
}

func (t *TerminalHandler) write(data []byte, ws *websocket.Conn) (n int, err error) {
	// UTF-8 encode data and wrap it in a raw envelope.
	encodedPayload, err := t.encoder.String(string(data))
	if err != nil {
		return 0, trace.Wrap(err)
	}
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketRaw,
		Payload: encodedPayload,
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Send bytes over the websocket to the web client.
	err = websocket.Message.Send(ws, envelopeBytes)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(data), nil
}

// Read unwraps the envelope and either fills out the passed in bytes or
// performs an action on the connection (sending window-change request).
func (t *TerminalHandler) read(out []byte, ws *websocket.Conn) (n int, err error) {
	if len(t.buffer) > 0 {
		n := copy(out, t.buffer)
		if n == len(t.buffer) {
			t.buffer = []byte{}
		} else {
			t.buffer = t.buffer[n:]
		}
		return n, nil
	}

	var bytes []byte
	err = websocket.Message.Receive(ws, &bytes)
	if err != nil {
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, trace.Wrap(err)
	}

	var envelope Envelope
	err = proto.Unmarshal(bytes, &envelope)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	var data []byte
	data, err = t.decoder.Bytes([]byte(envelope.GetPayload()))
	if err != nil {
		return 0, trace.Wrap(err)
	}

	switch envelope.GetType() {
	case defaults.WebsocketRaw:
		n := copy(out, data)
		// if payload size is greater than [out], store the remaining
		// part in the buffer to be processed on the next Read call
		if len(data) > n {
			t.buffer = data[n:]
		}
		return n, nil
	case defaults.WebsocketResize:
		var e events.EventFields
		err := json.Unmarshal(data, &e)
		if err != nil {
			return 0, trace.Wrap(err)
		}

		params, err := session.UnmarshalTerminalParams(e.GetString("size"))
		if err != nil {
			return 0, trace.Wrap(err)
		}

		// Send the window change request in a goroutine so reads are not blocked
		// by network connectivity issues.
		go t.windowChange(params)

		return 0, nil
	default:
		return 0, trace.BadParameter("unknown prefix type: %v", envelope.GetType())
	}
}

func (t *TerminalHandler) asTerminalStream(ws *websocket.Conn) *terminalStream {
	return &terminalStream{
		ws:       ws,
		terminal: t,
	}
}

type terminalStream struct {
	ws       *websocket.Conn
	terminal *TerminalHandler
}

// Write wraps the data bytes in a raw envelope and sends.
func (w *terminalStream) Write(data []byte) (n int, err error) {
	return w.terminal.write(data, w.ws)
}

// Read unwraps the envelope and either fills out the passed in bytes or
// performs an action on the connection (sending window-change request).
func (w *terminalStream) Read(out []byte) (n int, err error) {
	return w.terminal.read(out, w.ws)
}

// SetReadDeadline sets the network read deadline on the underlying websocket.
func (w *terminalStream) SetReadDeadline(t time.Time) error {
	return w.ws.SetReadDeadline(t)
}

// Close the websocket.
func (w *terminalStream) Close() error {
	return w.ws.Close()
}
