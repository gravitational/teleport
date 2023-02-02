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
	"errors"
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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/teleagent"
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

	// Namespace is node namespace.
	Namespace string `json:"namespace"`

	// ProxyHostPort is the address of the server to connect to.
	ProxyHostPort string `json:"-"`

	// Cluster is the name of the remote cluster to connect to.
	Cluster string `json:"-"`

	// InteractiveCommand is a command to execut.e
	InteractiveCommand []string `json:"-"`

	// KeepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	KeepAliveInterval time.Duration
}

// AuthProvider is a subset of the full Auth API.
type AuthProvider interface {
	GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)
	GetSessionEvents(namespace string, sid session.ID, after int, includePrintEvents bool) ([]events.EventFields, error)
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)
	IsMFARequired(ctx context.Context, req *authproto.IsMFARequiredRequest) (*authproto.IsMFARequiredResponse, error)
	GenerateUserSingleUseCerts(ctx context.Context) (authproto.AuthService_GenerateUserSingleUseCertsClient, error)
}

// TerminalHandlerConfig contains the configuration options necessary to
// correctly setup the TerminalHandler
type TerminalHandlerConfig struct {
	// Req are the terminal parameters from the UI
	Req TerminalRequest
	// AuthProvider is used to communicate with the auth server
	AuthProvider AuthProvider
	// SessionCtx is the user specific session context
	SessionCtx *SessionContext
	// Router determines how connections to nodes are created
	Router *proxy.Router
	// TracerProvider is used to create the tracer
	TracerProvider oteltrace.TracerProvider
	// tracer is used to create spans
	tracer oteltrace.Tracer
}

// CheckAndSetDefaults validates the provided dependencies
// are valid and sets defaults for any optional items.
func (c *TerminalHandlerConfig) CheckAndSetDefaults() error {
	if c.AuthProvider == nil {
		return trace.BadParameter("AuthProvider must be provided")
	}

	if c.SessionCtx == nil {
		return trace.BadParameter("SessionCtx must be provided")
	}

	if c.Router == nil {
		return trace.BadParameter("Router must be provided")
	}

	// Make sure whatever session is requested is a valid session.
	_, err := session.ParseID(string(c.Req.SessionID))
	if err != nil {
		return trace.BadParameter("invalid session id provided")
	}

	if c.Req.Login == "" {
		return trace.BadParameter("invalid login provided")
	}

	if c.Req.Term.W <= 0 || c.Req.Term.H <= 0 ||
		c.Req.Term.W >= 4096 || c.Req.Term.H >= 4096 {
		return trace.BadParameter("invalid dimensions(%dx%d)", c.Req.Term.W, c.Req.Term.H)
	}

	if c.TracerProvider == nil {
		c.TracerProvider = tracing.DefaultProvider()
	}

	c.tracer = c.TracerProvider.Tracer("webterminal")

	return nil
}

// NewTerminal creates a web-based terminal based on WebSockets and returns a
// new TerminalHandler.
func NewTerminal(ctx context.Context, cfg TerminalHandlerConfig) (*TerminalHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, span := cfg.tracer.Start(ctx, "NewTerminal")
	defer span.End()

	servers, err := cfg.AuthProvider.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DELETE IN: 5.0
	//
	// All proxies will support lookup by uuid, so host/port lookup
	// and fallback can be dropped entirely.
	hostName, hostPort, err := resolveServerHostPort(cfg.Req.Server, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", cfg.Req.Server, err)
	}

	return &TerminalHandler{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentWebsocket,
			"session_id":    cfg.Req.SessionID.String(),
		}),
		params:       cfg.Req,
		ctx:          cfg.SessionCtx,
		hostName:     hostName,
		hostPort:     hostPort,
		hostUUID:     cfg.Req.Server,
		authProvider: cfg.AuthProvider,
		router:       cfg.Router,
		tracer:       cfg.tracer,
	}, nil
}

// TerminalHandler connects together an SSH session with a web-based
// terminal via a web socket.
type TerminalHandler struct {
	// log holds the structured logger.
	log *logrus.Entry

	// params describes the request for a PTY
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
	sshSession *tracessh.Session

	// terminalContext is used to signal when the terminal sesson is closing.
	terminalContext context.Context

	// terminalCancel is used to signal when the terminal session is closing.
	terminalCancel context.CancelFunc

	// authProvider is used to fetch nodes and sessions from the backend.
	authProvider AuthProvider

	closeOnce sync.Once

	// router is used to dial the host
	router *proxy.Router

	// tracer creates spans
	tracer oteltrace.Tracer

	// stream manages sending and receiving [Envelope] to the UI
	// for the duration of the session
	stream *TerminalStream
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

	ws.SetReadDeadline(deadlineForInterval(t.params.KeepAliveInterval))
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
	t.log.Debugf("Starting websocket ping loop with interval %v.", t.params.KeepAliveInterval)
	tickerCh := time.NewTicker(t.params.KeepAliveInterval)
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

	// Create a terminal stream that wraps/unwraps the envelope used to
	// communicate over the websocket.
	resizeC := make(chan *session.TerminalParams, 1)
	stream, err := NewTerminalStream(ws, WithTerminalStreamResizeHandler(resizeC))
	if err != nil {
		t.log.WithError(err).Info("Failed creating a terminal stream for session")
		t.writeError(err)
		return
	}
	t.stream = stream

	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	t.terminalContext, t.terminalCancel = context.WithCancel(tctx)

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(r.Context(), ws)
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.writeError(err)
		return
	}

	t.log.Debug("Creating websocket stream")

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		ws.SetReadDeadline(deadlineForInterval(t.params.KeepAliveInterval))
		return nil
	})

	// Start sending ping frames through websocket to client.
	go t.startPingLoop(ws)

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamTerminal(ws, tc)
	go t.streamEvents(tc)

	// process window resizing
	go t.handleWindowResize(resizeC)

	// Block until the terminal session is complete.
	<-t.terminalContext.Done()
	t.log.Debug("Closing websocket stream")
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *TerminalHandler) makeClient(ctx context.Context, ws *websocket.Conn) (*client.TeleportClient, error) {
	ctx, span := tracing.DefaultProvider().Tracer("terminal").Start(ctx, "terminal/makeClient")
	defer span.End()

	clientConfig, err := makeTeleportClientConfig(ctx, t.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfig.ForwardAgent = client.ForwardAgentLocal
	clientConfig.HostLogin = t.params.Login
	clientConfig.Namespace = t.params.Namespace
	clientConfig.Stdout = t.stream
	clientConfig.Stderr = t.stream
	clientConfig.Stdin = t.stream
	clientConfig.SiteName = t.params.Cluster
	if err := clientConfig.ParseProxyHost(t.params.ProxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	clientConfig.Host = t.hostName
	clientConfig.HostPort = t.hostPort
	clientConfig.Env = map[string]string{sshutils.SessionEnvVar: string(t.params.SessionID)}
	clientConfig.ClientAddr = ws.RemoteAddr().String()
	clientConfig.Tracer = t.tracer

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
	tc.OnShellCreated = func(s *tracessh.Session, c *tracessh.Client, _ io.ReadWriteCloser) (bool, error) {
		t.sshSession = s
		t.windowChange(ctx, &t.params.Term)

		return false, nil
	}

	return tc, nil
}

// issueSessionMFACerts performs the mfa ceremony to retrieve new certs that can be
// used to access nodes which require per-session mfa. The ceremony is performed directly
// to make use of the authProvider already established for the session instead of leveraging
// the TeleportClient which would require dialing the auth server a second time.
func (t *TerminalHandler) issueSessionMFACerts(ctx context.Context, tc *client.TeleportClient) error {
	ctx, span := t.tracer.Start(ctx, "terminal/issueSessionMFACerts")
	defer span.End()

	// Always acquire single-use certificates from the root cluster, that's where
	// both the user and their devices are registered.
	log.Debug("Attempting to issue a single-use user certificate with an MFA check.")
	stream, err := t.ctx.cfg.RootClient.GenerateUserSingleUseCerts(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		stream.CloseSend()
		stream.Recv()
	}()

	priv, err := ssh.ParsePrivateKey(t.ctx.cfg.Session.GetPriv())
	if err != nil {
		return trace.Wrap(err)
	}

	key := &client.Key{
		Pub:     ssh.MarshalAuthorizedKey(priv.PublicKey()),
		Priv:    t.ctx.cfg.Session.GetPriv(),
		Cert:    t.ctx.cfg.Session.GetPub(),
		TLSCert: t.ctx.cfg.Session.GetTLSCert(),
	}

	tlsCert, err := key.TeleportTLSCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := stream.Send(
		&authproto.UserSingleUseCertsRequest{
			Request: &authproto.UserSingleUseCertsRequest_Init{
				Init: &authproto.UserCertsRequest{
					PublicKey:      key.Pub,
					Username:       tlsCert.Subject.CommonName,
					Expires:        tlsCert.NotAfter,
					RouteToCluster: t.params.Cluster,
					NodeName:       t.params.Server,
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
	assertion, err := promptMFAChallenge(t.stream, protobufMFACodec{})(ctx, tc.WebProxyAddr, challenge)
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

	key.ClusterName = t.params.Cluster

	am, err := key.AsAuthMethod()
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AuthMethods = []ssh.AuthMethod{am}

	return nil
}

func promptMFAChallenge(
	stream *TerminalStream,
	codec mfaCodec,
) client.PromptMFAChallengeHandler {
	return func(ctx context.Context, proxyAddr string, c *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
		var challenge *auth.MFAAuthenticateChallenge
		var envelopeType string

		// Convert from proto to JSON types.
		switch {
		// Webauthn takes precedence.
		case c.GetWebauthnChallenge() != nil:
			envelopeType = defaults.WebsocketWebauthnChallenge
			challenge = &auth.MFAAuthenticateChallenge{
				WebauthnChallenge: wanlib.CredentialAssertionFromProto(c.WebauthnChallenge),
			}
		case len(c.U2F) > 0:
			u2fChals := make([]u2f.AuthenticateChallenge, 0, len(c.U2F))
			envelopeType = defaults.WebsocketU2FChallenge
			for _, uc := range c.U2F {
				u2fChals = append(u2fChals, u2f.AuthenticateChallenge{
					Version:   uc.Version,
					Challenge: uc.Challenge,
					KeyHandle: uc.KeyHandle,
					AppID:     uc.AppID,
				})
			}
			challenge = &auth.MFAAuthenticateChallenge{
				AuthenticateChallenge: &u2f.AuthenticateChallenge{
					// Get the common challenge fields from the first item.
					// All of these fields should be identical for all u2fChals.
					Challenge: u2fChals[0].Challenge,
					AppID:     u2fChals[0].AppID,
					Version:   u2fChals[0].Version,
				},
				U2FChallenges: u2fChals,
			}
		default:
			return nil, trace.AccessDenied("only hardware keys are supported on the web terminal, please register a hardware device to connect to this server")
		}

		if err := stream.writeChallenge(challenge, codec, envelopeType); err != nil {
			return nil, trace.Wrap(err)
		}

		resp, err := stream.readChallenge(codec, envelopeType)
		return resp, trace.Wrap(err)
	}
}

// streamTerminal opens a SSH connection to the remote host and streams
// events back to the web client.
func (t *TerminalHandler) streamTerminal(ws *websocket.Conn, tc *client.TeleportClient) {
	ctx, span := t.tracer.Start(t.terminalContext, "terminal/streamTerminal")
	defer span.End()

	defer t.terminalCancel()

	roleset, err := t.ctx.GetUserRoles()
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to get access checker")
		t.writeError(err)
		return
	}

	agentGetter := func() (teleagent.Agent, error) {
		return teleagent.NopCloser(tc.LocalAgent()), nil
	}

	conn, err := t.router.DialHost(ctx, ws.RemoteAddr(), t.hostName, strconv.Itoa(t.hostPort), tc.SiteName, roleset, agentGetter)
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to dial host.")

		if errors.Is(err, trace.NotFound(teleport.NodeIsAmbiguous)) {
			const message = "error: ambiguous host could match multiple nodes\n\nHint: try addressing the node by unique id (ex: user@node-id)\n"
			t.writeError(trace.NotFound(message))
			return
		}

		t.writeError(err)
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

	nc, connectErr := client.NewNodeClient(ctx, sshConfig, conn, net.JoinHostPort(t.hostName, strconv.Itoa(t.hostPort)), tc, modules.GetModules().IsBoringBinary())
	switch {
	case connectErr != nil && !trace.IsAccessDenied(connectErr): // catastrophic error, return it
		t.log.WithError(connectErr).Warn("Unable to stream terminal - failed to create node client")
		t.writeError(connectErr)
		return
	case connectErr != nil && trace.IsAccessDenied(connectErr): // see if per session mfa would allow access
		mfaRequiredResp, err := t.authProvider.IsMFARequired(ctx, &authproto.IsMFARequiredRequest{
			Target: &authproto.IsMFARequiredRequest_Node{
				Node: &authproto.NodeLogin{
					Node:  t.params.Server,
					Login: tc.HostLogin,
				},
			},
		})
		if err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to determine if per session mfa is required")
			// write the original connect error
			t.writeError(connectErr)
			return
		}

		if !mfaRequiredResp.Required {
			t.log.WithError(connectErr).Warn("Unable to stream terminal - user does not have access to host")
			// write the original connect error
			t.writeError(connectErr)
			return
		}

		// perform mfa ceremony and retrieve new certs
		if err := t.issueSessionMFACerts(ctx, tc); err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to perform mfa ceremony")
			t.writeError(err)
			return
		}

		// update auth methods
		sshConfig.Auth = tc.AuthMethods

		// connect to the node again with the new certs
		conn, err = t.router.DialHost(ctx, ws.RemoteAddr(), t.hostName, strconv.Itoa(t.hostPort), tc.SiteName, roleset, agentGetter)
		if err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to dial host")
			t.writeError(err)
			return
		}

		nc, err = client.NewNodeClient(ctx, sshConfig, conn, net.JoinHostPort(t.hostName, strconv.Itoa(t.hostPort)), tc, modules.GetModules().IsBoringBinary())
		if err != nil {
			t.log.WithError(err).Warn("Unable to stream terminal - failed to create node client")
			t.writeError(err)
			return
		}
	}

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunInteractiveShell(ctx, types.SessionPeerMode, nil); err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure running interactive shell")
		t.writeError(err)
		return
	}

	if err := t.stream.Close(); err != nil {
		t.log.WithError(err).Error("Unable to send close event to web client.")
		return
	}

	t.log.Debug("Sent close event to web client.")
}

// streamEvents receives events over the SSH connection and forwards them to
// the web client.
func (t *TerminalHandler) streamEvents(tc *client.TeleportClient) {
	for {
		select {
		// Send push events that come over the events channel to the web client.
		case event := <-tc.EventsChannel():
			logger := t.log.WithField("event", event.GetType())

			data, err := json.Marshal(event)
			if err != nil {
				logger.WithError(err).Error("Unable to marshal audit event")
				continue
			}

			logger.Debug("Sending audit event to web client.")

			if err := t.stream.writeAuditEvent(data); err != nil {
				if err != nil {
					if errors.Is(err, websocket.ErrCloseSent) {
						logger.WithError(err).Debug("Websocket was closed, no longer streaming events")
						return
					}
					logger.WithError(err).Error("Unable to send audit event to web client")
					continue
				}
			}

		// Once the terminal stream is over (and the close envelope has been sent),
		// close stop streaming envelopes.
		case <-t.terminalContext.Done():
			return
		}
	}
}

// handleWindowResize receives window resize events and forwards
// them to the SSH session.
func (t *TerminalHandler) handleWindowResize(resizeC <-chan *session.TerminalParams) {
	for {
		select {
		case <-t.terminalContext.Done():
			return
		case params := <-resizeC:
			// nil params indicates the channel was closed
			if params == nil {
				return
			}
			// process window change
			t.windowChange(t.terminalContext, params)
		}
	}
}

// writeError displays an error in the terminal window.
func (t *TerminalHandler) windowChange(ctx context.Context, params *session.TerminalParams) {
	if t.sshSession == nil {
		return
	}

	if err := t.sshSession.WindowChange(ctx, params.H, params.W); err != nil {
		t.log.Error(err)
	}
}

// writeError displays an error in the terminal window.
func (t *TerminalHandler) writeError(err error) {
	if writeErr := t.stream.writeError(err); writeErr != nil {
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

// WithTerminalStreamEncoder overrides the default stream encoder
func WithTerminalStreamEncoder(enc *encoding.Encoder) func(stream *TerminalStream) {
	return func(stream *TerminalStream) {
		stream.encoder = enc
	}
}

// WithTerminalStreamDecoder overrides the default stream decoder
func WithTerminalStreamDecoder(dec *encoding.Decoder) func(stream *TerminalStream) {
	return func(stream *TerminalStream) {
		stream.decoder = dec
	}
}

// WithTerminalStreamResizeHandler provides a channel to subscribe to
// terminal resize events
func WithTerminalStreamResizeHandler(resizeC chan<- *session.TerminalParams) func(stream *TerminalStream) {
	return func(stream *TerminalStream) {
		stream.resizeC = resizeC
	}
}

// NewTerminalStream creates a stream that manages reading and writing
// data over the provided [websocket.Conn]
func NewTerminalStream(ws *websocket.Conn, opts ...func(*TerminalStream)) (*TerminalStream, error) {
	switch {
	case ws == nil:
		return nil, trace.BadParameter("required parameter ws not provided")
	}

	t := &TerminalStream{
		ws:      ws,
		encoder: unicode.UTF8.NewEncoder(),
		decoder: unicode.UTF8.NewDecoder(),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

// TerminalStream manages the [websocket.Conn] to the web UI
// for a terminal session.
type TerminalStream struct {
	// encoder is used to encode UTF-8 strings.
	encoder *encoding.Encoder
	// decoder is used to decode UTF-8 strings.
	decoder *encoding.Decoder

	// buffer is a buffer used to store the remaining payload data if it did not
	// fit into the buffer provided by the callee to Read method
	buffer []byte

	// once ensures that resizeC is closed at most one time
	once sync.Once
	// resizeC a channel to forward resize events so that
	// they happen out of band and don't block reads
	resizeC chan<- *session.TerminalParams

	// mu protects writes to ws
	mu sync.Mutex
	// ws the connection to the UI
	ws *websocket.Conn
}

// Replace \n with \r\n so the message is correctly aligned.
var replacer = strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n")

// writeError displays an error in the terminal window.
func (t *TerminalStream) writeError(err error) error {
	_, writeErr := replacer.WriteString(t, err.Error())
	return trace.Wrap(writeErr)
}

// writeChallenge encodes and writes the challenge to the
// websocket in the correct format.
func (t *TerminalStream) writeChallenge(challenge *auth.MFAAuthenticateChallenge, codec mfaCodec, envelopeType string) error {
	// Send the challenge over the socket.
	msg, err := codec.encode(challenge, envelopeType)
	if err != nil {
		return trace.Wrap(err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return trace.Wrap(t.ws.WriteMessage(websocket.BinaryMessage, msg))
}

// readChallenge reads and decodes the challenge response from the
// websocket in the correct format.
func (t *TerminalStream) readChallenge(codec mfaCodec, envelopeType string) (*authproto.MFAAuthenticateResponse, error) {
	// Read the challenge response.
	ty, bytes, err := t.ws.ReadMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ty != websocket.BinaryMessage {
		return nil, trace.BadParameter("expected websocket.BinaryMessage, got %v", ty)
	}

	resp, err := codec.decode(bytes, envelopeType)
	return resp, trace.Wrap(err)
}

// writeAuditEvent encodes and writes the audit event to the
// websocket in the correct format.
func (t *TerminalStream) writeAuditEvent(event []byte) error {
	// UTF-8 encode the error message and then wrap it in a raw envelope.
	encodedPayload, err := t.encoder.String(string(event))
	if err != nil {
		return trace.Wrap(err)
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketAudit,
		Payload: encodedPayload,
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return trace.Wrap(err)
	}

	// Send bytes over the websocket to the web client.
	t.mu.Lock()
	defer t.mu.Unlock()
	return trace.Wrap(t.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes))
}

// Write wraps the data bytes in a raw envelope and sends.
func (t *TerminalStream) Write(data []byte) (n int, err error) {
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
	t.mu.Lock()
	err = t.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	t.mu.Unlock()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(data), nil
}

// Read unwraps the envelope and either fills out the passed in bytes or
// performs an action on the connection (sending window-change request).
func (t *TerminalStream) Read(out []byte) (n int, err error) {
	if len(t.buffer) > 0 {
		n := copy(out, t.buffer)
		if n == len(t.buffer) {
			t.buffer = []byte{}
		} else {
			t.buffer = t.buffer[n:]
		}
		return n, nil
	}

	ty, bytes, err := t.ws.ReadMessage()
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
	// the session was closed
	case defaults.WebsocketClose:
		return 0, io.EOF
	case defaults.WebsocketRaw:
		n := copy(out, data)
		// if payload size is greater than [out], store the remaining
		// part in the buffer to be processed on the next Read call
		if len(data) > n {
			t.buffer = data[n:]
		}
		return n, nil
	case defaults.WebsocketResize:
		if t.resizeC == nil {
			return n, nil
		}

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
		select {
		case t.resizeC <- params:
		default:
		}

		return 0, nil
	default:
		return 0, trace.BadParameter("unknown prefix type: %v", envelope.GetType())
	}
}

// Close send a close message on the web socket
// prior to closing the web socket altogether.
func (t *TerminalStream) Close() error {
	if t.resizeC != nil {
		t.once.Do(func() {
			close(t.resizeC)
		})
	}

	// Send close envelope to web terminal upon exit without an error.
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketClose,
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return trace.NewAggregate(err, t.ws.Close())
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return trace.NewAggregate(t.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes), t.ws.Close())
}

// deadlineForInterval returns a suitable network read deadline for a given ping interval.
// We chose to take the current time plus twice the interval to allow the timeframe of one interval
// to wait for a returned pong message.
func deadlineForInterval(interval time.Duration) time.Time {
	return time.Now().Add(interval * 2)
}
