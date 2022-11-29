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
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
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

	// ProxyHostPort is the address of the server to connect to.
	ProxyHostPort string `json:"-"`

	// InteractiveCommand is a command to execute
	InteractiveCommand []string `json:"-"`

	// KeepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	KeepAliveInterval time.Duration
}

// AuthProvider is a subset of the full Auth API.
type AuthProvider interface {
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)
	GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]events.EventFields, error)
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)
	IsMFARequired(ctx context.Context, req *authproto.IsMFARequiredRequest) (*authproto.IsMFARequiredResponse, error)
	GenerateUserSingleUseCerts(ctx context.Context) (authproto.AuthService_GenerateUserSingleUseCertsClient, error)
}

// NewTerminal creates a web-based terminal based on WebSockets and returns a
// new TerminalHandler.
func NewTerminal(ctx context.Context, cfg TerminalHandlerConfig) (*TerminalHandler, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, span := cfg.tracer.Start(ctx, "NewTerminal")
	defer span.End()

	return &TerminalHandler{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentWebsocket,
			"session_id":    cfg.SessionData.ID.String(),
		}),
		ctx:                cfg.SessionCtx,
		authProvider:       cfg.AuthProvider,
		encoder:            unicode.UTF8.NewEncoder(),
		decoder:            unicode.UTF8.NewDecoder(),
		wsLock:             &sync.Mutex{},
		displayLogin:       cfg.DisplayLogin,
		sessionData:        cfg.SessionData,
		keepAliveInterval:  cfg.KeepAliveInterval,
		proxyHostPort:      cfg.ProxyHostPort,
		interactiveCommand: cfg.InteractiveCommand,
		term:               cfg.Term,
		router:             cfg.Router,
		tracer:             cfg.tracer,
	}, nil
}

// TerminalHandlerConfig contains the configuration options necessary to
// correctly setup the TerminalHandler
type TerminalHandlerConfig struct {
	// term is the initial PTY size.
	Term session.TerminalParams
	// sctx is the context for the users web session.
	SessionCtx *SessionContext
	// authProvider is used to fetch nodes and sessions from the backend.
	AuthProvider AuthProvider
	// displayLogin is the login name to display in the UI.
	DisplayLogin string
	// sessionData is the data to send to the client on the initial session creation.
	SessionData session.Session
	// keepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	KeepAliveInterval time.Duration
	// proxyHostPort is the address of the server to connect to.
	ProxyHostPort string
	// interactiveCommand is a command to execute.
	InteractiveCommand []string
	// Router determines how connections to nodes are created
	Router *proxy.Router
	// TracerProvider is used to create the tracer
	TracerProvider oteltrace.TracerProvider
	// tracer is used to create spans
	tracer oteltrace.Tracer
}

func (t *TerminalHandlerConfig) CheckAndSetDefaults() error {
	// Make sure whatever session is requested is a valid session id.
	_, err := session.ParseID(t.SessionData.ID.String())
	if err != nil {
		return trace.BadParameter("sid: invalid session id")
	}

	if t.SessionData.Login == "" {
		return trace.BadParameter("login: missing login")
	}

	if t.SessionData.ServerID == "" {
		return trace.BadParameter("server: missing server")
	}

	if t.Term.W <= 0 || t.Term.H <= 0 ||
		t.Term.W >= 4096 || t.Term.H >= 4096 {
		return trace.BadParameter("term: bad dimensions(%dx%d)", t.Term.W, t.Term.H)
	}

	if t.AuthProvider == nil {
		return trace.BadParameter("AuthProvider must be provided")
	}

	if t.SessionCtx == nil {
		return trace.BadParameter("SessionCtx must be provided")
	}

	if t.Router == nil {
		return trace.BadParameter("Router must be provided")
	}

	if t.TracerProvider == nil {
		t.TracerProvider = tracing.DefaultProvider()
	}

	t.tracer = t.TracerProvider.Tracer("webterminal")

	return nil
}

// TerminalHandler connects together an SSH session with a web-based
// terminal via a web socket.
type TerminalHandler struct {
	// log holds the structured logger.
	log *logrus.Entry

	// ctx is a web session context for the currently logged in user.
	ctx *SessionContext

	// displayLogin is the login name to display in the UI.
	displayLogin string

	// sshSession holds the "shell" SSH channel to the node.
	sshSession *tracessh.Session

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

	wsLock *sync.Mutex

	// keepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	keepAliveInterval time.Duration

	// proxyHostPort is the address of the server to connect to.
	proxyHostPort string

	// interactiveCommand is a command to execute.
	interactiveCommand []string

	// term is the initial PTY size.
	term session.TerminalParams

	// The server data for the active session.
	sessionData session.Session

	// router is used to dial the host
	router *proxy.Router

	// tracer creates spans
	tracer oteltrace.Tracer
}

// ServeHTTP builds a connection to the remote node and then pumps back two types of
// events: raw input/output events for what's happening on the terminal itself
// and audit log events relevant to this session.
func (t *TerminalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This allows closing of the websocket if the user logs out before exiting
	// the session.
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		errMsg := "Error upgrading to websocket"
		t.log.WithError(err).Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	err = ws.SetReadDeadline(deadlineForInterval(t.keepAliveInterval))
	if err != nil {
		t.log.WithError(err).Error("Error setting websocket readline")
		return
	}

	// If the displayLogin is set then use it instead of the login name used in
	// the SSH connection. This is specifically for the use case when joining
	// a session to avoid displaying "-teleport-internal-join" as the username.
	if t.displayLogin != "" {
		t.sessionData.Login = t.displayLogin
	}

	sendError := func(errMsg string, err error, ws *websocket.Conn) {
		envelope := &Envelope{
			Version: defaults.WebsocketVersion,
			Type:    defaults.WebsocketError,
			Payload: fmt.Sprintf("%s: %s", errMsg, err.Error()),
		}

		envelopeBytes, _ := proto.Marshal(envelope)
		ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	}

	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: t.sessionData})
	if err != nil {
		sendError("unable to marshal session response", err, ws)
		return
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		sendError("unable to marshal session data event for web client", err, ws)
		return
	}

	err = ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		sendError("unable to write message to socket", err, ws)
		return
	}

	t.handler(ws, r)
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

// startPingLoop starts a loop that will continuously send a ping frame through the websocket
// to prevent the connection between web client and teleport proxy from becoming idle.
// Interval is determined by the keep_alive_interval config set by user (or default).
// Loop will terminate when there is an error sending ping frame or when terminal session is closed.
func (t *TerminalHandler) startPingLoop(ws *websocket.Conn) {
	t.log.Debugf("Starting websocket ping loop with interval %v.", t.keepAliveInterval)
	tickerCh := time.NewTicker(t.keepAliveInterval)
	defer tickerCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			// A short deadline is used here to detect a broken connection quickly.
			// If this is just a temporary issue, we will retry shortly anyway.
			deadline := time.Now().Add(time.Second)
			if err := ws.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				t.log.WithError(err).Error("Unable to send ping frame to web client")
				t.Close()
				return
			}
		case <-t.terminalContext.Done():
			t.log.Debug("Terminating websocket ping loop.")
			return
		}
	}
}

// handler is the main websocket loop. It creates a Teleport client and then
// pumps raw events and audit events back to the client until the SSH session
// is complete.
func (t *TerminalHandler) handler(ws *websocket.Conn, r *http.Request) {
	defer ws.Close()

	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	t.terminalContext, t.terminalCancel = context.WithCancel(tctx)

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(ws, r)
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.writeError(err, ws)
		return
	}

	t.log.Debug("Creating websocket stream")

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		ws.SetReadDeadline(deadlineForInterval(t.keepAliveInterval))
		return nil
	})

	// Start sending ping frames through websocket to client.
	go t.startPingLoop(ws)

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamTerminal(ws, tc)
	go t.streamEvents(ws, tc)

	// Block until the terminal session is complete.
	<-t.terminalContext.Done()
	t.log.Debug("Closing websocket stream")
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *TerminalHandler) makeClient(ws *websocket.Conn, r *http.Request) (*client.TeleportClient, error) {
	ctx, span := tracing.DefaultProvider().Tracer("terminal").Start(r.Context(), "terminal/makeClient")
	defer span.End()

	clientConfig, err := makeTeleportClientConfig(ctx, t.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a terminal stream that wraps/unwraps the envelope used to
	// communicate over the websocket.
	stream := t.asTerminalStream(ws)

	clientConfig.HostLogin = t.sessionData.Login
	clientConfig.ForwardAgent = client.ForwardAgentLocal
	clientConfig.Namespace = apidefaults.Namespace
	clientConfig.Stdout = stream
	clientConfig.Stderr = stream
	clientConfig.Stdin = stream
	clientConfig.SiteName = t.sessionData.ClusterName
	if err := clientConfig.ParseProxyHost(t.proxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	clientConfig.Host = t.sessionData.ServerHostname
	clientConfig.HostPort = t.sessionData.ServerHostPort
	clientConfig.Env = map[string]string{sshutils.SessionEnvVar: t.sessionData.ID.String()}
	clientConfig.ClientAddr = r.RemoteAddr
	clientConfig.Tracer = t.tracer

	if len(t.interactiveCommand) > 0 {
		clientConfig.Interactive = true
	}

	tc, err := client.NewClient(clientConfig)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	// Save the *ssh.Session after the shell has been created. The session is
	// used to update all other parties window size to that of the web client and
	// to allow future window changes.
	tc.OnShellCreated = func(s *tracessh.Session, c *tracessh.Client, _ io.ReadWriteCloser) (bool, error) {
		t.sshSession = s
		t.windowChange(r.Context(), &t.term)

		return false, nil
	}

	return tc, nil
}

// issueSessionMFACerts performs the mfa ceremony to retrieve new certs that can be
// used to access nodes which require per-session mfa. The ceremony is performed directly
// to make use of the authProvider already established for the session instead of leveraging
// the TeleportClient which would require dialing the auth server a second time.
func (t *TerminalHandler) issueSessionMFACerts(ctx context.Context, tc *client.TeleportClient, ws *websocket.Conn) error {
	ctx, span := t.tracer.Start(ctx, "terminal/issueSessionMFACerts")
	defer span.End()

	log.Debug("Attempting to issue a single-use user certificate with an MFA check.")
	stream, err := t.authProvider.GenerateUserSingleUseCerts(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		stream.CloseSend()
		stream.Recv()
	}()

	pk, err := keys.ParsePrivateKey(t.ctx.session.GetPriv())
	if err != nil {
		return trace.Wrap(err)
	}

	key := &client.Key{
		PrivateKey: pk,
		Cert:       t.ctx.session.GetPub(),
		TLSCert:    t.ctx.session.GetTLSCert(),
	}

	tlsCert, err := key.TeleportTLSCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := stream.Send(
		&authproto.UserSingleUseCertsRequest{
			Request: &authproto.UserSingleUseCertsRequest_Init{
				Init: &authproto.UserCertsRequest{
					PublicKey:      key.MarshalSSHPublicKey(),
					Username:       tlsCert.Subject.CommonName,
					Expires:        tlsCert.NotAfter,
					RouteToCluster: t.sessionData.ClusterName,
					NodeName:       t.sessionData.ServerID,
					Usage:          authproto.UserCertsRequest_SSH,
					Format:         tc.CertificateFormat,
				},
			},
		}); err != nil {
		return trace.Wrap(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	challenge := resp.GetMFAChallenge()
	if challenge == nil {
		return trace.BadParameter("server sent a %T on GenerateUserSingleUseCerts, expected MFAChallenge", resp.Response)
	}

	span.AddEvent("prompting user with mfa challenge")
	assertion, err := promptMFAChallenge(ws, t.wsLock, protobufMFACodec{})(ctx, tc.WebProxyAddr, challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	span.AddEvent("user completed mfa challenge")

	err = stream.Send(&authproto.UserSingleUseCertsRequest{Request: &authproto.UserSingleUseCertsRequest_MFAResponse{MFAResponse: assertion}})
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err = stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	certResp := resp.GetCert()
	if certResp == nil {
		return trace.BadParameter("server sent a %T on GenerateUserSingleUseCerts, expected SingleUseUserCert", resp.Response)
	}

	switch crt := certResp.Cert.(type) {
	case *authproto.SingleUseUserCert_SSH:
		key.Cert = crt.SSH
	default:
		return trace.BadParameter("server sent a %T SingleUseUserCert in response", certResp.Cert)
	}

	key.ClusterName = t.sessionData.ClusterName

	am, err := key.AsAuthMethod()
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AuthMethods = []ssh.AuthMethod{am}

	return nil
}

func promptMFAChallenge(
	ws *websocket.Conn,
	wsLock *sync.Mutex,
	codec mfaCodec,
) client.PromptMFAChallengeHandler {
	return func(ctx context.Context, proxyAddr string, c *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
		var chal *client.MFAAuthenticateChallenge
		var envelopeType string

		// Convert from proto to JSON types.
		switch {
		case c.GetWebauthnChallenge() != nil:
			envelopeType = defaults.WebsocketWebauthnChallenge
			chal = &client.MFAAuthenticateChallenge{
				WebauthnChallenge: wanlib.CredentialAssertionFromProto(c.WebauthnChallenge),
			}
		default:
			return nil, trace.AccessDenied("only hardware keys are supported on the web terminal, please register a hardware device to connect to this server")
		}

		// Send the challenge over the socket.
		msg, err := codec.encode(chal, envelopeType)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		wsLock.Lock()
		err = ws.WriteMessage(websocket.BinaryMessage, msg)
		wsLock.Unlock()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Read the challenge response.
		var bytes []byte
		ty, bytes, err := ws.ReadMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if ty != websocket.BinaryMessage {
			return nil, trace.BadParameter("expected websocket.BinaryMessage, got %v", ty)
		}

		return codec.decode(bytes, envelopeType)
	}
}

// streamTerminal opens a SSH connection to the remote host and streams
// events back to the web client.
func (t *TerminalHandler) streamTerminal(ws *websocket.Conn, tc *client.TeleportClient) {
	ctx, span := t.tracer.Start(t.terminalContext, "terminal/streamTerminal")
	defer span.End()

	defer t.terminalCancel()

	accessChecker, err := t.ctx.GetUserAccessChecker()
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to get access checker")
		t.writeError(err, ws)
		return
	}

	conn, err := t.router.DialHost(ctx, ws.RemoteAddr(), t.sessionData.ServerHostname, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, nil)
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to dial host.")
		t.writeError(err, ws)
		return
	}

	defer func() {
		if conn == nil {
			return
		}

		if err := conn.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
			t.log.WithError(err).Warn("Failed to close connection to host")
		}
	}()

	sshConfig := &ssh.ClientConfig{
		User:            tc.HostLogin,
		Auth:            tc.AuthMethods,
		HostKeyCallback: tc.HostKeyCallback,
	}

	nc, connectErr := client.NewNodeClient(ctx, sshConfig, conn, net.JoinHostPort(t.sessionData.ServerHostname, strconv.Itoa(t.sessionData.ServerHostPort)), tc, modules.GetModules().IsBoringBinary())
	switch {
	case connectErr != nil && !trace.IsAccessDenied(connectErr): // catastrophic error, return it
		t.log.WithError(connectErr).Warn("Unable to stream terminal - failed to create node client")
		t.writeError(connectErr, ws)
		return
	case connectErr != nil && trace.IsAccessDenied(connectErr): // see if per session mfa would allow access
		mfaRequiredResp, err := t.authProvider.IsMFARequired(ctx, &authproto.IsMFARequiredRequest{
			Target: &authproto.IsMFARequiredRequest_Node{
				Node: &authproto.NodeLogin{
					Node:  t.sessionData.ServerID,
					Login: tc.HostLogin,
				},
			},
		})
		if err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to determine if per session mfa is required")
			// write the original connect error
			t.writeError(connectErr, ws)
			return
		}

		if !mfaRequiredResp.Required {
			t.log.WithError(connectErr).Warn("Unable to stream terminal - user does not have access to host")
			// write the original connect error
			t.writeError(connectErr, ws)
			return
		}

		// perform mfa ceremony and retrieve new certs
		if err := t.issueSessionMFACerts(ctx, tc, ws); err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to perform mfa ceremony")
			t.writeError(err, ws)
			return
		}

		// update auth methods
		sshConfig.Auth = tc.AuthMethods

		// connect to the node again with the new certs
		conn, err = t.router.DialHost(ctx, ws.RemoteAddr(), t.sessionData.ServerHostname, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, nil)
		if err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to dial host")
			t.writeError(err, ws)
			return
		}

		nc, err = client.NewNodeClient(ctx, sshConfig, conn, net.JoinHostPort(t.sessionData.ServerHostname, strconv.Itoa(t.sessionData.ServerHostPort)), tc, modules.GetModules().IsBoringBinary())
		if err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to create node client")
			t.writeError(err, ws)
			return
		}
	}

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunInteractiveShell(ctx, types.SessionPeerMode, nil); err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure running interactive shell")
		t.writeError(err, ws)
		return
	}

	// Send close envelope to web terminal upon exit without an error.
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketClose,
		Payload: "",
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		t.log.WithError(err).Error("Unable to marshal close event for web client.")
		return
	}

	t.wsLock.Lock()
	err = ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	t.wsLock.Unlock()
	if err != nil {
		t.log.WithError(err).Error("Unable to send close event to web client.")
		return
	}

	t.log.Debug("Sent close event to web client.")
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
				logger.WithError(err).Errorf("Unable to marshal audit event")
				continue
			}

			logger.Debug("Sending audit event to web client.")

			// UTF-8 encode the error message and then wrap it in a raw envelope.
			encodedPayload, err := t.encoder.String(string(data))
			if err != nil {
				logger.WithError(err).Debug("Unable to send audit event to web client")
				continue
			}
			envelope := &Envelope{
				Version: defaults.WebsocketVersion,
				Type:    defaults.WebsocketAudit,
				Payload: encodedPayload,
			}
			envelopeBytes, err := proto.Marshal(envelope)
			if err != nil {
				logger.WithError(err).Debug("Unable to send audit event to web client")
				continue
			}

			// Send bytes over the websocket to the web client.
			t.wsLock.Lock()
			err = ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
			t.wsLock.Unlock()
			if err != nil {
				logger.WithError(err).Error("Unable to send audit event to web client")
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
func (t *TerminalHandler) windowChange(ctx context.Context, params *session.TerminalParams) {
	if t.sshSession == nil {
		return
	}

	if err := t.sshSession.WindowChange(ctx, params.H, params.W); err != nil {
		t.log.Error(err)
	}
}

// writeError displays an error in the terminal window.
func (t *TerminalHandler) writeError(err error, ws *websocket.Conn) {
	// Replace \n with \r\n so the message correctly aligned.
	r := strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n")
	errMessage := r.Replace(err.Error())

	if _, writeErr := t.write([]byte(errMessage), ws); writeErr != nil {
		t.log.WithError(writeErr).Warnf("Unable to send error to terminal: %v", err)
	}
}

// resolveServerHostPort parses server name and attempts to resolve hostname
// and port.
func resolveServerHostPort(servername string, existingServers []types.Server) (string, int, error) {
	// If port is 0, client wants us to figure out which port to use.
	defaultPort := 0

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
	t.wsLock.Lock()
	err = ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	t.wsLock.Unlock()
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

	ty, bytes, err := ws.ReadMessage()
	if err != nil {
		if err == io.EOF || websocket.IsCloseError(err, 1006) {
			return 0, io.EOF
		}

		return 0, trace.Wrap(err)
	}

	if ty != websocket.BinaryMessage {
		return 0, trace.BadParameter("expected binary message, got %v", ty)
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
		go t.windowChange(t.terminalContext, params)

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

// Close the websocket.
func (w *terminalStream) Close() error {
	return w.ws.Close()
}

// deadlineForInterval returns a suitable network read deadline for a given ping interval.
// We chose to take the current time plus twice the interval to allow the timeframe of one interval
// to wait for a returned pong message.
func deadlineForInterval(interval time.Duration) time.Time {
	return time.Now().Add(interval * 2)
}
