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
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// TerminalRequest describes a request to crate a web-based terminal
// to a remote SSH server
type TerminalRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port])
	Server string `json:"server_id"`
	// User is linux username to connect as
	Login string `json:"login"`
	// Term sets PTY params like width and height
	Term session.TerminalParams `json:"term"`
	// SessionID is a teleport session ID to join as
	SessionID session.ID `json:"sid"`
	// Namespace is node namespace
	Namespace string `json:"namespace"`
	// Proxy server address
	ProxyHostPort string `json:"-"`
	// Remote cluster name
	Cluster string `json:"-"`
	// InteractiveCommand is a command to execute
	InteractiveCommand []string `json:"-"`
}

// NodeProvider is a provider of nodes for namespace
type NodeProvider interface {
	GetNodes(namespace string) ([]services.Server, error)
}

// newTerminal creates a web-based terminal based on WebSockets and returns a new
// TerminalHandler
func NewTerminal(req TerminalRequest, provider NodeProvider, ctx *SessionContext) (*TerminalHandler, error) {
	// make sure whatever session is requested is a valid session
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

	servers, err := provider.GetNodes(req.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostName, hostPort, err := resolveServerHostPort(req.Server, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", req.Server, err)
	}

	return &TerminalHandler{
		params:   req,
		ctx:      ctx,
		hostName: hostName,
		hostPort: hostPort,
	}, nil
}

// TerminalHandler connects together an SSH session with a web-based
// terminal via a web socket.
type TerminalHandler struct {
	// params describe the terminal configuration
	params TerminalRequest
	// ctx is a web session context for the currently logged in user
	ctx *SessionContext
	// ws is the websocket which is connected to stdin/out/err of the terminal shell
	ws *websocket.Conn
	// hostName we're connected to
	hostName string
	// hostPort we're connected to
	hostPort int
	// sshClient is initialized after an SSH connection to a node is established
	sshSession *ssh.Session

	teleportClient *client.TeleportClient

	streamContext context.Context
	streamCancel  context.CancelFunc

	request *http.Request
}

// Run creates a new websocket connection to the SSH server and runs
// the "loop" piping the input/output of the SSH session into the
// js-based terminal.
func (t *TerminalHandler) Serve(w http.ResponseWriter, r *http.Request) {
	t.request = r

	// this is to make sure we close web socket connections once
	// sessionContext that owns them expires
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	ws := &websocket.Server{Handler: t.handler}
	ws.ServeHTTP(w, r)
}

func (t *TerminalHandler) Close() error {
	if t.ws != nil {
		t.ws.Close()
	}
	if t.sshSession != nil {
		t.sshSession.Close()
	}

	// If the terminal handler was closed (most likely due to the *SessionContext
	// closing) then the stream should be closed as well.
	t.streamCancel()

	return nil
}

func (t *TerminalHandler) handler(ws *websocket.Conn) {
	tc, err := t.makeClient(ws)
	if err != nil {
		errToTerm(err, ws)
		return
	}

	t.streamContext, t.streamCancel = context.WithCancel(context.Background())

	go t.streamTerminal(ws, tc)
	go t.streamEvents(ws, tc)

	// Block until streaming is complete (terminal session is over)
	<-t.streamContext.Done()
}

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

	//output := utils.NewWebSockWrapper(ws, utils.WebSocketTextMode)
	output := newWebsocketWrapper(ws)
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
		Stdout:           output,
		Stderr:           output,
		Stdin:            ws,
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
		t.windowChange(t.params.Term)
		return false, nil
	}

	return tc, nil
}

func (t *TerminalHandler) streamTerminal(ws *websocket.Conn, tc *client.TeleportClient) {
	defer t.streamCancel()

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	err := tc.SSH(t.streamContext, t.params.InteractiveCommand, false)
	if err != nil {
		log.Warningf("failed to SSH: %v", err)
		errToTerm(err, ws)
	}
}

type auditEvent struct {
	Type    string             `json:"type"`
	Payload events.EventFields `json:"payload"`
}

func (t *TerminalHandler) streamEvents(ws *websocket.Conn, tc *client.TeleportClient) {
	for {
		select {
		case event := <-tc.EventsChannel():
			e := auditEvent{
				Type:    "audit",
				Payload: event,
			}
			log.Debugf("Sending audit event %v to web client.", event.GetType())

			err := websocket.JSON.Send(ws, e)
			if err != nil {
				log.Errorf("Unable to %v event to web client: %v.", event.GetType(), err)
				continue
			}
		case <-t.streamContext.Done():
			fmt.Printf("--> stream closing.\n")
			return
		}
	}
}

// windowChange is called when the browser window is resized. It sends a
// "window-change" channel request to the server.
func (t *TerminalHandler) windowChange(params session.TerminalParams) error {
	if t.sshSession == nil {
		return nil
	}

	_, err := t.sshSession.SendRequest(
		// send SSH "window resized" SSH request:
		sshutils.WindowChangeRequest,
		// no response needed
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

func errToTerm(err error, w io.Writer) {
	fmt.Fprintf(w, "%s\n\r", err.Error())
	//log.Error(err)
}

// resolveServerHostPort parses server name  and attempts to resolve hostname and port
func resolveServerHostPort(servername string, existingServers []services.Server) (string, int, error) {
	// if port is 0, it means the client wants us to figure out which port to use
	var defaultPort = 0

	if servername == "" {
		return "", defaultPort, trace.BadParameter("empty server name")
	}

	// check if servername is UUID
	for i := range existingServers {
		node := existingServers[i]
		if node.GetName() == servername {
			return node.GetHostname(), defaultPort, nil
		}
	}

	if !strings.Contains(servername, ":") {
		return servername, defaultPort, nil
	}

	// check for explicitly specified port
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

type rawEvent struct {
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
}

type websocketWrapper struct {
	//io.ReadWriteCloser
	//sync.Mutex

	ws *websocket.Conn
	//mode WebSocketMode

	encoder *encoding.Encoder
	decoder *encoding.Decoder
}

func newWebsocketWrapper(ws *websocket.Conn) *websocketWrapper {
	if ws == nil {
		return nil
	}
	return &websocketWrapper{
		ws: ws,
		//mode:    m,
		encoder: unicode.UTF8.NewEncoder(),
		decoder: unicode.UTF8.NewDecoder(),
	}
}

func (w *websocketWrapper) Write(data []byte) (n int, err error) {
	encodedBytes, err := w.encoder.Bytes(data)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	e := rawEvent{
		Type:    "raw",
		Payload: encodedBytes,
	}

	err = websocket.JSON.Send(w.ws, e)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(data), nil
}

func (w *websocketWrapper) Read(out []byte) (n int, err error) {
	var utf8 string
	err = websocket.Message.Receive(w.ws, &utf8)
	if err != nil {
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, trace.Wrap(err)
	}

	var data []byte
	data, err = w.decoder.Bytes([]byte(utf8))
	if err != nil {
		return 0, trace.Wrap(err)
	}

	if len(out) < len(data) {
		log.Warningf("websocket failed to receive everything: %d vs %d", len(out), len(data))
	}

	return copy(out, data), nil
}

func (w *websocketWrapper) Close() error {
	return w.ws.Close()
}
