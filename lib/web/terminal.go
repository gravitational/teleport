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
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
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

	// SessionTimeout is how long to wait for the session end event to arrive.
	SessionTimeout time.Duration
}

// AuthProvider is a subset of the full Auth API.
type AuthProvider interface {
	GetNodes(namespace string) ([]services.Server, error)
	GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]events.EventFields, error)
}

// newTerminal creates a web-based terminal based on WebSockets and returns a
// new TerminalHandler.
func NewTerminal(req TerminalRequest, authProvider AuthProvider, ctx *SessionContext) (*TerminalHandler, error) {
	if req.SessionTimeout == 0 {
		req.SessionTimeout = defaults.HTTPIdleTimeout
	}

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

	servers, err := authProvider.GetNodes(req.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostName, hostPort, err := resolveServerHostPort(req.Server, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", req.Server, err)
	}

	return &TerminalHandler{
		namespace:      req.Namespace,
		sessionID:      req.SessionID,
		params:         req,
		ctx:            ctx,
		hostName:       hostName,
		hostPort:       hostPort,
		authProvider:   authProvider,
		sessionTimeout: req.SessionTimeout,
	}, nil
}

// TerminalHandler connects together an SSH session with a web-based
// terminal via a web socket.
type TerminalHandler struct {
	// namespace is node namespace.
	namespace string

	// sessionID is a Teleport session ID to join as.
	sessionID session.ID

	// params is the initial PTY size.
	params TerminalRequest

	// ctx is a web session context for the currently logged in user.
	ctx *SessionContext

	// ws is the websocket which is connected to stdin/out/err of the terminal shell.
	ws *websocket.Conn

	// hostName is the hostname of the server.
	hostName string

	// hostPort is the port of the server.
	hostPort int

	// sshSession holds the "shell" SSH channel to the node.
	sshSession *ssh.Session

	// teleportClient is the client used to form the connection.
	teleportClient *client.TeleportClient

	// terminalContext is used to signal when the terminal sesson is closing.
	terminalContext context.Context

	// terminalCancel is used to signal when the terminal session is closing.
	terminalCancel context.CancelFunc

	// eventContext is used to signal when the event stream is closing.
	eventContext context.Context

	// eventCancel is used to signal when the event is closing.
	eventCancel context.CancelFunc

	// request is the HTTP request that initiated the websocket connection.
	request *http.Request

	// authProvider is used to fetch nodes and sessions from the backend.
	authProvider AuthProvider

	// sessionTimeout is how long to wait for the session end event to arrive.
	sessionTimeout time.Duration
}

// Serve builds a connect to the remote node and then pumps back two types of
// events: raw input/output events for what's happening on the terminal itself
// and audit log events relevant to this session.
func (t *TerminalHandler) Serve(w http.ResponseWriter, r *http.Request) {
	t.request = r

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
	// Close the websocket connection to the client web browser.
	if t.ws != nil {
		t.ws.Close()
	}

	// Close the SSH connection to the remote node.
	if t.sshSession != nil {
		t.sshSession.Close()
	}

	// If the terminal handler was closed (most likely due to the *SessionContext
	// closing) then the stream should be closed as well.
	t.terminalCancel()

	return nil
}

// handler is the main websocket loop. It creates a Teleport client and then
// pumps raw events and audit events back to the client until the SSH session
// is complete.
func (t *TerminalHandler) handler(ws *websocket.Conn) {
	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(ws)
	if err != nil {
		er := errToTerm(err, ws)
		if er != nil {
			log.Warnf("Unable to send error to terminal: %v: %v.", err, er)
		}
		return
	}

	// Create two contexts for signaling. The first
	t.terminalContext, t.terminalCancel = context.WithCancel(context.Background())
	t.eventContext, t.eventCancel = context.WithCancel(context.Background())

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamTerminal(ws, tc)
	go t.streamEvents(ws, tc)

	// Block until the terminal session is complete.
	<-t.terminalContext.Done()

	// Block until the session end event is sent or a timeout occurs.
	timeoutCh := time.After(t.sessionTimeout)
	for {
		select {
		case <-timeoutCh:
			t.eventCancel()
		case <-t.eventContext.Done():
		}

		log.Debugf("Closing websocket stream to web client.")
		return
	}
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *TerminalHandler) makeClient(ws *websocket.Conn) (*client.TeleportClient, error) {
	agent, cert, err := t.ctx.GetAgent()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	tlsConfig, err := t.ctx.ClientTLSConfig()
	if err != nil {
		return nil, trace.BadParameter("failed to get client TLS config: %v", err)
	}

	// Create a wrapped websocket to wrap/unwrap the envelope used to
	// communicate over the websocket.
	wrappedSock := newWrappedSocket(ws, t)

	clientConfig := &client.Config{
		SkipLocalAuth:    true,
		ForwardAgent:     true,
		Agent:            agent,
		TLS:              tlsConfig,
		AuthMethods:      []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		DefaultPrincipal: cert.ValidPrincipals[0],
		HostLogin:        t.params.Login,
		Username:         t.ctx.user,
		Namespace:        t.params.Namespace,
		Stdout:           wrappedSock,
		Stderr:           wrappedSock,
		Stdin:            wrappedSock,
		SiteName:         t.params.Cluster,
		ProxyHostPort:    t.params.ProxyHostPort,
		Host:             t.hostName,
		HostPort:         t.hostPort,
		Env:              map[string]string{sshutils.SessionEnvVar: string(t.params.SessionID)},
		HostKeyCallback:  func(string, net.Addr, ssh.PublicKey) error { return nil },
		ClientAddr:       t.request.RemoteAddr,
	}
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

	return tc, nil
}

// streamTerminal opens a SSH connection to the remote host and streams
// events back to the web client.
func (t *TerminalHandler) streamTerminal(ws *websocket.Conn, tc *client.TeleportClient) {
	defer t.terminalCancel()

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	err := tc.SSH(t.terminalContext, t.params.InteractiveCommand, false)
	if err != nil {
		log.Warnf("Unable to stream terminal: %v.", err)
		er := errToTerm(err, ws)
		if er != nil {
			log.Warnf("Unable to send error to terminal: %v: %v.", err, er)
		}
	}
}

// streamEvents receives events over the SSH connection (as well as periodic
// polling) to update the client with relevant audit events.
func (t *TerminalHandler) streamEvents(ws *websocket.Conn, tc *client.TeleportClient) {
	// A cursor are used to keep track of where we are in the event stream. This
	// is to find "session.end" events.
	var cursor int = -1

	tickerCh := time.NewTicker(defaults.SessionRefreshPeriod)
	defer tickerCh.Stop()

	for {
		select {
		// Send push events that come over the events channel to the web client.
		case event := <-tc.EventsChannel():
			e := eventEnvelope{
				Type:    defaults.AuditEnvelopeType,
				Payload: event,
			}
			log.Debugf("Sending audit event %v to web client.", event.GetType())

			err := websocket.JSON.Send(ws, e)
			if err != nil {
				log.Errorf("Unable to %v event to web client: %v.", event.GetType(), err)
				continue
			}
		// Poll for events to send to the web client. This is for events that can
		// not be sent over the events channel (like "session.end" which lingers for
		// a while after all party members have left).
		case <-tickerCh.C:
			// Fetch all session events from the backend.
			sessionEvents, cur, err := t.pollEvents(cursor)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Errorf("Unable to poll for events: %v.", err)
					continue
				}
				continue
			}

			// Update the cursor location.
			cursor = cur

			// Send all events to the web client.
			for _, sessionEvent := range sessionEvents {
				ee := eventEnvelope{
					Type:    defaults.AuditEnvelopeType,
					Payload: sessionEvent,
				}
				err = websocket.JSON.Send(ws, ee)
				if err != nil {
					log.Warnf("Unable to send %v events to web client: %v.", len(sessionEvents), err)
					continue
				}

				// The session end event was sent over the websocket, we can now close the
				// websocket.
				if sessionEvent.GetType() == events.SessionEndEvent {
					t.eventCancel()
					return
				}
			}
		case <-t.eventContext.Done():
			return
		}
	}
}

// pollEvents polls the backend for events that don't get pushed over the
// SSH events channel. Eventually this function will be removed completely.
func (t *TerminalHandler) pollEvents(cursor int) ([]events.EventFields, int, error) {
	// Poll for events since the last call (cursor location).
	sessionEvents, err := t.authProvider.GetSessionEvents(t.namespace, t.sessionID, cursor+1, false)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, 0, trace.Wrap(err)
		}
		return nil, 0, trace.NotFound("no events from cursor: %v", cursor)
	}

	// Get the batch size to see if any events were returned.
	batchLen := len(sessionEvents)
	if batchLen == 0 {
		return nil, 0, trace.NotFound("no events from cursor: %v", cursor)
	}

	// Advance the cursor.
	newCursor := sessionEvents[batchLen-1].GetInt(events.EventCursor)

	// Filter out any resize events as we get them over push notifications.
	var filteredEvents []events.EventFields
	for _, event := range sessionEvents {
		if event.GetType() == events.ResizeEvent ||
			event.GetType() == events.SessionJoinEvent ||
			event.GetType() == events.SessionLeaveEvent ||
			event.GetType() == events.SessionPrintEvent {
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}

	return filteredEvents, newCursor, nil
}

// windowChange is called when the browser window is resized. It sends a
// "window-change" channel request to the server.
func (t *TerminalHandler) windowChange(params *session.TerminalParams) error {
	if t.sshSession == nil {
		return nil
	}

	_, err := t.sshSession.SendRequest(
		sshutils.WindowChangeRequest,
		false,
		ssh.Marshal(sshutils.WinChangeReqParams{
			W: uint32(params.W),
			H: uint32(params.H),
		}))
	if err != nil {
		log.Error(err)
	}

	return trace.Wrap(err)
}

// errToTerm displays an error in the terminal window.
func errToTerm(err error, w io.Writer) error {
	// Replace \n with \r\n so the message correctly aligned.
	r := strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n")
	errMessage := []byte(r.Replace(err.Error()))

	// Create an envelope that contains the error message.
	re := rawEnvelope{
		Type:    defaults.RawEnvelopeType,
		Payload: errMessage,
	}
	b, err := json.Marshal(re)
	if err != nil {
		return trace.Wrap(err)
	}

	// Write the error to the websocket.
	_, err = w.Write(b)
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

// wrappedSocket wraps and unwraps the envelope that is used to send events
// over the websocket.
type wrappedSocket struct {
	ws       *websocket.Conn
	terminal *TerminalHandler

	encoder *encoding.Encoder
	decoder *encoding.Decoder
}

func newWrappedSocket(ws *websocket.Conn, terminal *TerminalHandler) *wrappedSocket {
	if ws == nil {
		return nil
	}
	return &wrappedSocket{
		ws:       ws,
		terminal: terminal,
		encoder:  unicode.UTF8.NewEncoder(),
		decoder:  unicode.UTF8.NewDecoder(),
	}
}

// Write wraps the data bytes in a raw envelope and sends.
func (w *wrappedSocket) Write(data []byte) (n int, err error) {
	encodedBytes, err := w.encoder.Bytes(data)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	e := rawEnvelope{
		Type:    defaults.RawEnvelopeType,
		Payload: encodedBytes,
	}

	err = websocket.JSON.Send(w.ws, e)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(data), nil
}

// Read unwraps the envelope and either fills out the passed in bytes or
// performs an action on the connection (sending window-change request).
func (w *wrappedSocket) Read(out []byte) (n int, err error) {
	var ue unknownEnvelope
	err = websocket.JSON.Receive(w.ws, &ue)
	if err != nil {
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, trace.Wrap(err)
	}

	switch ue.Type {
	case defaults.RawEnvelopeType:
		var re rawEnvelope
		err := json.Unmarshal(ue.Raw, &re)
		if err != nil {
			return 0, trace.Wrap(err)
		}

		var data []byte
		data, err = w.decoder.Bytes(re.Payload)
		if err != nil {
			return 0, trace.Wrap(err)
		}

		if len(out) < len(data) {
			log.Warningf("websocket failed to receive everything: %d vs %d", len(out), len(data))
		}

		return copy(out, data), nil
	case defaults.ResizeRequestEnvelopeType:
		if w.terminal == nil {
			return 0, nil
		}

		var ee eventEnvelope
		err := json.Unmarshal(ue.Raw, &ee)
		if err != nil {
			return 0, trace.Wrap(err)
		}

		params, err := session.UnmarshalTerminalParams(ee.Payload.GetString("size"))
		if err != nil {
			return 0, trace.Wrap(err)
		}

		// Send the window change request in a goroutine so reads are not blocked
		// by network connectivity issues.
		go w.terminal.windowChange(params)

		return 0, nil
	default:
		return 0, trace.BadParameter("unknown envelope type")
	}
}

// SetReadDeadline sets the network read deadline on the underlying websocket.
func (w *wrappedSocket) SetReadDeadline(t time.Time) error {
	return w.ws.SetReadDeadline(t)
}

// Close the websocket.
func (w *wrappedSocket) Close() error {
	return w.ws.Close()
}

// eventEnvelope is used to send/receive audit events.
type eventEnvelope struct {
	Type    string             `json:"type"`
	Payload events.EventFields `json:"payload"`
}

// rawEnvelope is used to send/receive terminal bytes.
type rawEnvelope struct {
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
}

// unknownEnvelope is used to figure out the type of data being unmarshaled.
type unknownEnvelope struct {
	envelopeHeader
	Raw []byte
}

type envelopeHeader struct {
	Type string `json:"type"`
}

func (u *unknownEnvelope) UnmarshalJSON(raw []byte) error {
	var eh envelopeHeader
	if err := json.Unmarshal(raw, &eh); err != nil {
		return err
	}
	u.Type = eh.Type
	u.Raw = make([]byte, len(raw))
	copy(u.Raw, raw)
	return nil
}
