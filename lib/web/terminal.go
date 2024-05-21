/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/diagnostics/latency"
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

	// ParticipantMode is the mode that determines what you can do when you join an active session.
	ParticipantMode types.SessionParticipantMode `json:"mode"`
}

// UserAuthClient is a subset of the Auth API that performs
// operations on behalf of the user so that the correct RBAC is applied.
type UserAuthClient interface {
	GetSessionEvents(namespace string, sid session.ID, after int) ([]events.EventFields, error)
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)
	IsMFARequired(ctx context.Context, req *authproto.IsMFARequiredRequest) (*authproto.IsMFARequiredResponse, error)
	CreateAuthenticateChallenge(ctx context.Context, req *authproto.CreateAuthenticateChallengeRequest) (*authproto.MFAAuthenticateChallenge, error)
	GenerateUserCerts(ctx context.Context, req authproto.UserCertsRequest) (*authproto.Certs, error)
	MaintainSessionPresence(ctx context.Context) (authproto.AuthService_MaintainSessionPresenceClient, error)
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
		sshBaseHandler: sshBaseHandler{
			log: logrus.WithFields(logrus.Fields{
				teleport.ComponentKey: teleport.ComponentWebsocket,
				"session_id":          cfg.SessionData.ID.String(),
			}),
			ctx:                cfg.SessionCtx,
			userAuthClient:     cfg.UserAuthClient,
			localAccessPoint:   cfg.LocalAccessPoint,
			sessionData:        cfg.SessionData,
			keepAliveInterval:  cfg.KeepAliveInterval,
			proxyHostPort:      cfg.ProxyHostPort,
			proxyPublicAddr:    cfg.ProxyPublicAddr,
			interactiveCommand: cfg.InteractiveCommand,
			router:             cfg.Router,
			tracer:             cfg.tracer,
			resolver:           cfg.HostNameResolver,
		},
		displayLogin:    cfg.DisplayLogin,
		term:            cfg.Term,
		proxySigner:     cfg.PROXYSigner,
		participantMode: cfg.ParticipantMode,
		tracker:         cfg.Tracker,
		presenceChecker: cfg.PresenceChecker,
		websocketConn:   cfg.WebsocketConn,
	}, nil
}

// TerminalHandlerConfig contains the configuration options necessary to
// correctly set up the TerminalHandler
type TerminalHandlerConfig struct {
	// Term is the initial PTY size.
	Term session.TerminalParams
	// SessionCtx is the context for the users web session.
	SessionCtx *SessionContext
	// UserAuthClient is used to fetch nodes and sessions from the backend.
	UserAuthClient UserAuthClient
	// LocalAccessPoint is the subset of the Proxy cache required to
	// look up information from the local cluster. This should not
	// be used for anything that requires RBAC on behalf of the user.
	// Requests that should be made on behalf of the user should
	// use [UserAuthClient].
	LocalAccessPoint localAccessPoint
	// HostNameResolver allows the hostname to be determined from a server UUID
	// so that a friendly name can be displayed in the console tab.
	HostNameResolver func(serverID string) (hostname string, err error)
	// DisplayLogin is the login name to display in the UI.
	DisplayLogin string
	// SessionData is the data to send to the client on the initial session creation.
	SessionData session.Session
	// KeepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	KeepAliveInterval time.Duration
	// ProxyHostPort is the address of the server to connect to.
	ProxyHostPort string
	// ProxyPublicAddr is the public web proxy address.
	ProxyPublicAddr string
	// InteractiveCommand is a command to execute.
	InteractiveCommand []string
	// Router determines how connections to nodes are created
	Router *proxy.Router
	// TracerProvider is used to create the tracer
	TracerProvider oteltrace.TracerProvider
	// PROXYSigner is used to sign PROXY header and securely propagate client IP information
	PROXYSigner multiplexer.PROXYHeaderSigner
	// tracer is used to create spans
	tracer oteltrace.Tracer
	// ParticipantMode is the mode that determines what you can do when you join an active session.
	ParticipantMode types.SessionParticipantMode
	// Tracker is the session tracker of the session being joined. May be nil
	// if the user is not joining a session.
	Tracker types.SessionTracker
	// PresenceChecker used for presence checking.
	PresenceChecker PresenceChecker
	// Clock allows interaction with time.
	Clock clockwork.Clock
	// WebsocketConn is the active websocket connection
	WebsocketConn *websocket.Conn
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

	if t.UserAuthClient == nil {
		return trace.BadParameter("UserAuthClient must be provided")
	}

	if t.LocalAccessPoint == nil {
		return trace.BadParameter("localAccessPoint must be provided")
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

	if t.Clock == nil {
		t.Clock = clockwork.NewRealClock()
	}

	t.tracer = t.TracerProvider.Tracer("webterminal")

	return nil
}

// sshBaseHandler is a base handler for web SSH connections.
type sshBaseHandler struct {
	// log holds the structured logger.
	log *logrus.Entry
	// ctx is a web session context for the currently logged-in user.
	ctx *SessionContext
	// userAuthClient is used to fetch nodes and sessions from the backend via the users' identity.
	userAuthClient UserAuthClient
	// proxyHostPort is the address of the server to connect to.
	proxyHostPort string
	// proxyPublicAddr is the public web proxy address.
	proxyPublicAddr string
	// keepAliveInterval is the interval for sending ping frames to a web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	keepAliveInterval time.Duration
	// The server data for the active session.
	sessionData session.Session
	// router is used to dial the host
	router *proxy.Router
	// tracer creates spans
	tracer oteltrace.Tracer
	// localAccessPoint is the subset of the Proxy cache required to
	// look up information from the local cluster. This should not
	// be used for anything that requires RBAC on behalf of the user.
	// Requests that should be made on behalf of the user should
	// use [UserAuthClient].
	localAccessPoint localAccessPoint
	// interactiveCommand is a command to execute.
	interactiveCommand []string
	// resolver looks up the hostname for the server UUID.
	resolver func(serverID string) (hostname string, err error)
}

// localAccessPoint is a subset of the cache used to look up
// various cluster details.
type localAccessPoint interface {
	GetUser(ctx context.Context, username string, withSecrets bool) (types.User, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
}

// TerminalHandler connects together an SSH session with a web-based
// terminal via a web socket.
type TerminalHandler struct {
	sshBaseHandler

	// displayLogin is the login name to display in the UI.
	displayLogin string

	closeOnce sync.Once

	// term is the initial PTY size.
	term session.TerminalParams

	// proxySigner is used to sign PROXY header and securely propagate client IP information
	proxySigner multiplexer.PROXYHeaderSigner

	// participantMode is the mode that determines what you can do when you join an active session.
	participantMode types.SessionParticipantMode

	// stream manages sending and receiving [Envelope] to the UI
	// for the duration of the session
	stream *TerminalStream
	// tracker is the session tracker of the session being joined. May be nil
	// if the user is not joining a session.
	tracker types.SessionTracker

	// presenceChecker to use for presence checking
	presenceChecker PresenceChecker

	// closedByClient indicates if the websocket connection was closed by the
	// user (closing the browser tab, exiting the session, etc).
	closedByClient atomic.Bool

	// clock used to interact with time.
	clock clockwork.Clock

	// websocketConn is the active websocket connection
	websocketConn *websocket.Conn
}

// ServeHTTP builds a connection to the remote node and then pumps back two types of
// events: raw input/output events for what's happening on the terminal itself
// and audit log events relevant to this session.
func (t *TerminalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// This allows closing of the websocket if the user logs out before exiting
	// the session.
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	ws := t.websocketConn

	err := ws.SetReadDeadline(deadlineForInterval(t.keepAliveInterval))
	if err != nil {
		t.log.WithError(err).Error("Error setting websocket readline")
		return
	}

	t.handler(ws, r)
}

func (t *TerminalHandler) writeSessionData(ctx context.Context) error {
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
	}

	sessionDataTemp := t.sessionData

	// If the displayLogin is set then use it in the session metadata instead of the
	// login name used in the SSH connection. This is specifically for the use case
	// when joining a session to avoid displaying "-teleport-internal-join" as the username.
	if t.displayLogin != "" {
		sessionDataTemp.Login = t.displayLogin
		sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: sessionDataTemp})
		if err != nil {
			t.sendError("unable to marshal session response", err, t.stream.ws)
			return trace.Wrap(err)
		}
		envelope.Payload = string(sessionMetadataResponse)
	} else {
		// The Proxy cache is used to retrieve the server and resolve the hostname here instead
		// of the user auth client to avoid a round trip to the Auth server. This would normally
		// not be ok since this bypasses user RBAC, however, since at this point we have already
		// established a connection to the target host via the user identity, the user MUST have
		// access to the target host.

		hostname, err := t.resolver(sessionDataTemp.ServerID)
		if err != nil {
			return trace.Wrap(err)
		}
		sessionDataTemp.ServerHostname = hostname

		sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: sessionDataTemp})
		if err != nil {
			t.sendError("unable to marshal session response", err, t.stream.ws)
			return trace.Wrap(err)
		}
		envelope.Payload = string(sessionMetadataResponse)
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		t.sendError("unable to marshal session data event for web client", err, t.stream.ws)
		return trace.Wrap(err)
	}

	if err := t.stream.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
		t.sendError("unable to write message to socket", err, t.stream.ws)
		return trace.Wrap(err)
	}

	return nil
}

// Close the websocket stream.
func (t *TerminalHandler) Close() error {
	var err error
	t.closeOnce.Do(func() {
		if t.stream == nil {
			return
		}

		if t.stream.sshSession != nil {
			err = trace.NewAggregate(t.stream.sshSession.Close(), t.stream.Close())
		} else {
			err = trace.Wrap(t.stream.Close())
		}
	})
	return trace.Wrap(err)
}

// handler is the main websocket loop. It creates a Teleport client and then
// pumps raw events and audit events back to the client until the SSH session
// is complete.
func (t *TerminalHandler) handler(ws *websocket.Conn, r *http.Request) {
	defer ws.Close()

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		return trace.Wrap(ws.SetReadDeadline(deadlineForInterval(t.keepAliveInterval)))
	})

	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	ctx, cancel := context.WithCancel(tctx)
	defer cancel()
	t.stream = NewTerminalStream(ctx, TerminalStreamConfig{WS: ws, Logger: t.log})

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(ctx, t.stream, ws.RemoteAddr().String())
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.stream.writeError(err.Error())
		return
	}

	t.log.Debug("Creating websocket stream")

	defaultCloseHandler := ws.CloseHandler()
	ws.SetCloseHandler(func(code int, text string) error {
		t.closedByClient.Store(true)
		t.log.Debug("web socket was closed by client - terminating session")

		// Call the default close handler if one was set.
		if defaultCloseHandler != nil {
			err := defaultCloseHandler(code, text)
			return trace.NewAggregate(err, t.Close())
		}

		return trace.Wrap(t.Close())
	})

	// Start sending ping frames through websocket to client.
	go startPingLoop(ctx, ws, t.keepAliveInterval, t.log, t.Close)

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamEvents(ctx, tc)

	// Block until the terminal session is complete.
	t.streamTerminal(ctx, tc)
	t.log.Debug("Closing websocket stream")
}

// SSHSessionLatencyStats contain latency measurements for both
// legs of an ssh connection established via the Web UI.
type SSHSessionLatencyStats struct {
	// WebSocket measures the round trip time for a ping/pong via the websocket
	// established between the client and the Proxy.
	WebSocket int64 `json:"ws"`
	// SSH measures the round trip time for a keepalive@openssh.com request via the
	// connection established between the Proxy and the target host.
	SSH int64 `json:"ssh"`
}

type stderrWriter struct {
	stream *TerminalStream
}

func (s stderrWriter) Write(b []byte) (int, error) {
	s.stream.writeError(string(b))
	return len(b), nil
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *TerminalHandler) makeClient(ctx context.Context, stream *TerminalStream, clientAddr string) (*client.TeleportClient, error) {
	ctx, span := tracing.DefaultProvider().Tracer("terminal").Start(ctx, "terminal/makeClient")
	defer span.End()

	clientConfig, err := makeTeleportClientConfig(ctx, t.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfig.HostLogin = t.sessionData.Login
	clientConfig.ForwardAgent = client.ForwardAgentLocal
	clientConfig.Namespace = apidefaults.Namespace
	clientConfig.Stdout = stream
	clientConfig.Stderr = stderrWriter{stream: stream}
	clientConfig.Stdin = stream
	clientConfig.SiteName = t.sessionData.ClusterName
	if err := clientConfig.ParseProxyHost(t.proxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	clientConfig.Host = t.sessionData.ServerHostname
	clientConfig.HostPort = t.sessionData.ServerHostPort
	clientConfig.SessionID = t.sessionData.ID.String()
	clientConfig.ClientAddr = clientAddr
	clientConfig.Tracer = t.tracer

	if len(t.interactiveCommand) > 0 {
		clientConfig.InteractiveCommand = true
	}

	tc, err := client.NewClient(clientConfig)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	// Save the *ssh.Session after the shell has been created. The session is
	// used to update all other parties window size to that of the web client and
	// to allow future window changes.
	tc.OnShellCreated = func(s *tracessh.Session, c *tracessh.Client, _ io.ReadWriteCloser) (bool, error) {
		t.stream.sessionCreated(s)

		// The web session was closed by the client while the ssh connection was being established.
		// Attempt to close the SSH session instead of proceeding with the window change request.
		if t.closedByClient.Load() {
			t.log.Debug("websocket was closed by client, terminating established ssh connection to host")
			return false, trace.Wrap(s.Close())
		}

		if err := s.WindowChange(ctx, t.term.H, t.term.W); err != nil {
			t.log.Error(err)
		}

		return false, nil
	}

	return tc, nil
}

// issueSessionMFACerts performs the mfa ceremony to retrieve new certs that can be
// used to access nodes which require per-session mfa. The ceremony is performed directly
// to make use of the userAuthClient already established for the session instead of leveraging
// the TeleportClient which would require dialing the auth server a second time.
func (t *sshBaseHandler) issueSessionMFACerts(ctx context.Context, tc *client.TeleportClient, wsStream *WSStream) ([]ssh.AuthMethod, error) {
	ctx, span := t.tracer.Start(ctx, "terminal/issueSessionMFACerts")
	defer span.End()

	log.Debug("Attempting to issue a single-use user certificate with an MFA check.")

	// Prepare MFA check request.
	mfaRequiredReq := &authproto.IsMFARequiredRequest{
		Target: &authproto.IsMFARequiredRequest_Node{
			Node: &authproto.NodeLogin{
				Node:  t.sessionData.ServerID,
				Login: tc.HostLogin,
			},
		},
	}

	// Prepare UserCertsRequest.
	pk, err := keys.ParsePrivateKey(t.ctx.cfg.Session.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key := &client.Key{
		PrivateKey: pk,
		Cert:       t.ctx.cfg.Session.GetPub(),
		TLSCert:    t.ctx.cfg.Session.GetTLSCert(),
	}
	tlsCert, err := key.TeleportTLSCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certsReq := &authproto.UserCertsRequest{
		PublicKey:      key.MarshalSSHPublicKey(),
		Username:       tlsCert.Subject.CommonName,
		Expires:        tlsCert.NotAfter,
		RouteToCluster: t.sessionData.ClusterName,
		NodeName:       t.sessionData.ServerID,
		Usage:          authproto.UserCertsRequest_SSH,
		Format:         tc.CertificateFormat,
		SSHLogin:       tc.HostLogin,
	}

	key, _, err = client.PerformMFACeremony(ctx, client.PerformMFACeremonyParams{
		CurrentAuthClient: t.userAuthClient,
		RootAuthClient:    t.ctx.cfg.RootClient,
		MFAPrompt: mfa.PromptFunc(func(ctx context.Context, chal *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
			span.AddEvent("prompting user with mfa challenge")
			assertion, err := promptMFAChallenge(wsStream, protobufMFACodec{}).Run(ctx, chal)
			span.AddEvent("user completed mfa challenge")
			return assertion, trace.Wrap(err)
		}),
		MFAAgainstRoot: t.ctx.cfg.RootClusterName == tc.SiteName,
		MFARequiredReq: mfaRequiredReq,
		ChallengeExtensions: mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
		CertsReq: certsReq,
		Key:      key,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.ClusterName = t.sessionData.ClusterName

	am, err := key.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []ssh.AuthMethod{am}, nil
}

func promptMFAChallenge(stream *WSStream, codec mfaCodec) mfa.Prompt {
	return mfa.PromptFunc(func(ctx context.Context, chal *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
		var challenge *client.MFAAuthenticateChallenge

		// Convert from proto to JSON types.
		switch {
		case chal.GetWebauthnChallenge() != nil:
			challenge = &client.MFAAuthenticateChallenge{
				WebauthnChallenge: wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge),
			}
		default:
			return nil, trace.AccessDenied("only hardware keys are supported on the web terminal, please register a hardware device to connect to this server")
		}

		if err := stream.writeChallenge(challenge, codec); err != nil {
			return nil, trace.Wrap(err)
		}

		resp, err := stream.readChallengeResponse(codec)
		return resp, trace.Wrap(err)
	})
}

type connectWithMFAFn = func(ctx context.Context, ws WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error)

// connectToHost establishes a connection to the target host. To reduce connection
// latency if per session mfa is required, connections are tried with the existing
// certs and with single use certs after completing the mfa ceremony. Only one of
// the operations will succeed, and if per session mfa will not gain access to the
// target it will abort before prompting a user to perform the ceremony.
func (t *sshBaseHandler) connectToHost(ctx context.Context, ws WSConn, tc *client.TeleportClient, connectToNodeWithMFA connectWithMFAFn) (*client.NodeClient, error) {
	ctx, span := t.tracer.Start(ctx, "terminal/connectToHost")
	defer span.End()

	accessChecker, err := t.ctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	getAgent := func() (teleagent.Agent, error) {
		return teleagent.NopCloser(tc.LocalAgent()), nil
	}
	cert, err := t.ctx.GetSSHCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer := agentless.SignerFromSSHCertificate(cert, t.localAccessPoint, tc.SiteName, tc.Username)

	type clientRes struct {
		clt *client.NodeClient
		err error
	}

	directResultC := make(chan clientRes, 1)
	mfaResultC := make(chan clientRes, 1)

	// use a child context so the goroutines can terminate the other if they succeed

	directCtx, directCancel := context.WithCancel(ctx)
	mfaCtx, mfaCancel := context.WithCancel(ctx)
	go func() {
		// try connecting to the node with the certs we already have
		clt, err := t.connectToNode(directCtx, ws, tc, accessChecker, getAgent, signer)
		directResultC <- clientRes{clt: clt, err: err}
	}()

	// use a child context so the goroutine ends if this
	// function returns early
	go func() {
		// try performing mfa and then connecting with the single use certs
		clt, err := connectToNodeWithMFA(mfaCtx, ws, tc, accessChecker, getAgent, signer)
		mfaResultC <- clientRes{clt: clt, err: err}
	}()

	var directErr, mfaErr error
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			mfaCancel()
			directCancel()
			return nil, ctx.Err()
		case res := <-directResultC:
			if res.clt != nil {
				mfaCancel()
				res.clt.AddCancel(directCancel)
				return res.clt, nil
			}

			directErr = res.err
		case res := <-mfaResultC:
			if res.clt != nil {
				directCancel()
				res.clt.AddCancel(mfaCancel)
				return res.clt, nil
			}

			mfaErr = res.err
		}
	}

	mfaCancel()
	directCancel()

	switch {
	// No MFA errors, return any errors from the direct connection
	case mfaErr == nil:
		return nil, trace.Wrap(directErr)
	// Any direct connection errors other than access denied, which should be returned
	// if MFA is required, take precedent over MFA errors due to users not having any
	// enrolled devices.
	case !trace.IsAccessDenied(directErr) && errors.Is(mfaErr, authclient.ErrNoMFADevices):
		return nil, trace.Wrap(directErr)
	case !errors.Is(mfaErr, io.EOF) && // Ignore any errors from MFA due to locks being enforced, the direct error will be friendlier
		!errors.Is(mfaErr, client.MFARequiredUnknownErr{}) && // Ignore any failures that occurred before determining if MFA was required
		!errors.Is(mfaErr, services.ErrSessionMFANotRequired): // Ignore any errors caused by attempting the MFA ceremony when MFA will not grant access
		return nil, trace.Wrap(mfaErr)
	default:
		return nil, trace.Wrap(directErr)
	}
}

func monitorSessionLatency(ctx context.Context, clock clockwork.Clock, stream *WSStream, sshClient *tracessh.Client) error {
	wsPinger, err := latency.NewWebsocketPinger(clock, stream.ws)
	if err != nil {
		return trace.Wrap(err, "creating websocket pinger")
	}

	sshPinger, err := latency.NewSSHPinger(sshClient)
	if err != nil {
		return trace.Wrap(err, "creating ssh pinger")
	}

	monitor, err := latency.NewMonitor(latency.MonitorConfig{
		ClientPinger: wsPinger,
		ServerPinger: sshPinger,
		Reporter: latency.ReporterFunc(func(ctx context.Context, statistics latency.Statistics) error {
			return trace.Wrap(stream.writeLatency(SSHSessionLatencyStats{
				WebSocket: statistics.Client,
				SSH:       statistics.Server,
			}))
		}),
		Clock: clock,
	})
	if err != nil {
		return trace.Wrap(err, "creating latency monitor")
	}

	monitor.Run(ctx)
	return nil
}

// streamTerminal opens a SSH connection to the remote host and streams
// events back to the web client.
func (t *TerminalHandler) streamTerminal(ctx context.Context, tc *client.TeleportClient) {
	ctx, span := t.tracer.Start(ctx, "terminal/streamTerminal")
	defer span.End()

	nc, err := t.connectToHost(ctx, t.stream.ws, tc, t.connectToNodeWithMFA)
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure connecting to host")
		t.stream.writeError(err.Error())
		return
	}
	defer nc.Close()

	// If the session was terminated by client while the connection to the host
	// was being established, then return early before creating the shell. Any terminations
	// by the client from here on out should either get caught in the OnShellCreated callback
	// set on the [tc] or in [TerminalHandler.Close].
	if t.closedByClient.Load() {
		t.log.Debug("websocket was closed by client, aborting establishing ssh connection to host")
		return
	}

	if err := t.writeSessionData(ctx); err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure sending session data")
	}

	var beforeStart func(io.Writer)
	if t.participantMode == types.SessionModeratorMode {
		beforeStart = func(out io.Writer) {
			nc.OnMFA = func() {
				if err := t.presenceChecker(ctx, out, t.userAuthClient, t.sessionData.ID.String(), promptMFAChallenge(t.stream.WSStream, protobufMFACodec{})); err != nil {
					t.log.WithError(err).Warn("Unable to stream terminal - failure performing presence checks")
					return
				}
			}
		}
	}

	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()
	go func() {
		if err := monitorSessionLatency(monitorCtx, t.clock, t.stream.WSStream, nc.Client); err != nil {
			t.log.WithError(err).Warn("failure monitoring session latency")
		}
	}()

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunInteractiveShell(ctx, t.participantMode, t.tracker, beforeStart); err != nil {
		if !t.closedByClient.Load() {
			t.stream.writeError(err.Error())
		}
		return
	}

	if t.closedByClient.Load() {
		return
	}

	// Send close envelope to web terminal upon exit without an error.
	if err := t.stream.SendCloseMessage(sessionEndEvent{NodeID: t.sessionData.ServerID}); err != nil {
		t.log.WithError(err).Error("Unable to send close event to web client.")
	}

	if err := t.stream.Close(); err != nil {
		t.log.WithError(err).Error("Unable to close client web socket.")
		return
	}

	t.log.Debug("Sent close event to web client.")
}

// connectToNode attempts to connect to the host with the already
// provisioned certs for the user.
func (t *sshBaseHandler) connectToNode(ctx context.Context, ws WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error) {
	conn, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, getAgent, signer)
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to dial host.")

		if errors.Is(err, trace.NotFound(teleport.NodeIsAmbiguous)) {
			const message = "error: ambiguous host could match multiple nodes\n\nHint: try addressing the node by unique id (ex: user@node-id)\n"
			return nil, trace.NotFound(message)
		}

		return nil, trace.Wrap(err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            tc.HostLogin,
		Auth:            tc.AuthMethods,
		HostKeyCallback: tc.HostKeyCallback,
	}

	clt, err := client.NewNodeClient(ctx, sshConfig, conn,
		net.JoinHostPort(t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort)),
		t.sessionData.ServerHostname,
		tc, modules.GetModules().IsBoringBinary())
	if err != nil {
		// The close error is ignored instead of using [trace.NewAggregate] because
		// aggregate errors do not allow error inspection with things like [trace.IsAccessDenied].
		_ = conn.Close()
		return nil, trace.Wrap(err)
	}

	clt.ProxyPublicAddr = t.proxyPublicAddr

	return clt, nil
}

// connectToNodeWithMFA attempts to perform the mfa ceremony and then dial the
// host with the retrieved single use certs.
func (t *TerminalHandler) connectToNodeWithMFA(ctx context.Context, ws WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error) {
	// perform mfa ceremony and retrieve new certs
	authMethods, err := t.issueSessionMFACerts(ctx, tc, t.stream.WSStream)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return t.connectToNodeWithMFABase(ctx, ws, tc, accessChecker, getAgent, signer, authMethods)
}

// connectToNodeWithMFABase attempts to dial the host with the provided auth
// methods.
func (t *sshBaseHandler) connectToNodeWithMFABase(ctx context.Context, ws WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator, authMethods []ssh.AuthMethod) (*client.NodeClient, error) {
	sshConfig := &ssh.ClientConfig{
		User:            tc.HostLogin,
		Auth:            authMethods,
		HostKeyCallback: tc.HostKeyCallback,
	}

	// connect to the node again with the new certs
	conn, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, getAgent, signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nc, err := client.NewNodeClient(ctx, sshConfig, conn,
		net.JoinHostPort(t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort)),
		t.sessionData.ServerHostname,
		tc, modules.GetModules().IsBoringBinary())
	if err != nil {
		return nil, trace.NewAggregate(err, conn.Close())
	}

	nc.ProxyPublicAddr = t.proxyPublicAddr

	return nc, nil
}

// streamEvents receives events over the SSH connection and forwards them to
// the web client.
func (t *TerminalHandler) streamEvents(ctx context.Context, tc *client.TeleportClient) {
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
				if errors.Is(err, websocket.ErrCloseSent) {
					logger.WithError(err).Debug("Websocket was closed, no longer streaming events")
					return
				}
				logger.WithError(err).Error("Unable to send audit event to web client")
				continue
			}

		// Once the terminal stream is over (and the close envelope has been sent),
		// close stop streaming envelopes.
		case <-ctx.Done():
			return
		}
	}
}

// the defaultPort of 0 indicates that the port is
// unknown or was not provided and should be guessed
const defaultPort = 0

// resolveServerHostPort parses server name and attempts to resolve hostname
// and port.
func resolveServerHostPort(servername string, existingServers []types.Server) (string, int, error) {
	if servername == "" {
		return "", defaultPort, trace.BadParameter("empty server name")
	}

	// Check if servername is UUID.
	for _, node := range existingServers {
		if node.GetName() == servername {
			return node.GetHostname(), defaultPort, nil
		}
	}

	host, port, err := serverHostPort(servername)
	return host, port, trace.Wrap(err)
}

// serverHostPort returns the host and port for [servername]
func serverHostPort(servername string) (string, int, error) {
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

func NewWStream(ctx context.Context, ws WSConn, log logrus.FieldLogger, handlers map[string]WSHandlerFunc) *WSStream {
	w := &WSStream{
		log:        log,
		ws:         ws,
		encoder:    unicode.UTF8.NewEncoder(),
		decoder:    unicode.UTF8.NewDecoder(),
		rawC:       make(chan Envelope, 100),
		challengeC: make(chan Envelope, 1),
		handlers:   handlers,
	}

	go w.processMessages(ctx)

	return w
}

// TerminalStreamConfig contains dependencies of a TerminalStream.
type TerminalStreamConfig struct {
	// The websocket to operate over. Required.
	WS WSConn
	// A logger to emit log messages. Optional.
	Logger logrus.FieldLogger
	// A custom set of handlers to process messages received
	// over the websocket. Optional.
	Handlers map[string]WSHandlerFunc
}

// NewTerminalStream creates a stream that manages reading and writing
// data over the provided [websocket.Conn]
func NewTerminalStream(ctx context.Context, cfg TerminalStreamConfig) *TerminalStream {
	t := &TerminalStream{
		sessionReadyC: make(chan struct{}),
	}

	if cfg.Handlers == nil {
		cfg.Handlers = map[string]WSHandlerFunc{}
	}

	if _, ok := cfg.Handlers[defaults.WebsocketResize]; !ok {
		cfg.Handlers[defaults.WebsocketResize] = t.handleWindowResize
	}

	if _, ok := cfg.Handlers[defaults.WebsocketFileTransferRequest]; !ok {
		cfg.Handlers[defaults.WebsocketFileTransferRequest] = t.handleFileTransferRequest
	}

	if _, ok := cfg.Handlers[defaults.WebsocketFileTransferDecision]; !ok {
		cfg.Handlers[defaults.WebsocketFileTransferDecision] = t.handleFileTransferDecision
	}

	if cfg.Logger == nil {
		cfg.Logger = utils.NewLogger()
	}

	t.WSStream = NewWStream(ctx, cfg.WS, cfg.Logger, cfg.Handlers)

	return t
}

// WSHandlerFunc specifies a handler that processes received a specific
// [Envelope] received via a web socket.
type WSHandlerFunc func(context.Context, Envelope)

// WSStream handles web socket communication with
// the frontend.
type WSStream struct {
	// encoder is used to encode UTF-8 strings.
	encoder *encoding.Encoder
	// decoder is used to decode UTF-8 strings.
	decoder *encoding.Decoder

	handlers map[string]WSHandlerFunc
	// once ensures that all channels are closed at most one time.
	once       sync.Once
	challengeC chan Envelope
	rawC       chan Envelope

	// buffer is a buffer used to store the remaining payload data if it did not
	// fit into the buffer provided by the callee to Read method
	buffer []byte

	// mu protects writes to ws
	mu sync.Mutex
	// ws the connection to the UI
	ws WSConn

	// log holds the structured logger.
	log logrus.FieldLogger
}

// TerminalStream manages the [websocket.Conn] to the web UI
// for a terminal session.
type TerminalStream struct {
	*WSStream

	// sshSession holds the "shell" SSH channel to the node.
	sshSession    *tracessh.Session
	sessionReadyC chan struct{}
}

// Replace \n with \r\n so the message is correctly aligned.
var replacer = strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n")

// writeError displays an error in the terminal window.
func (t *WSStream) writeError(msg string) {
	if _, writeErr := replacer.WriteString(t, msg); writeErr != nil {
		t.log.WithError(writeErr).Warnf("Unable to send error to terminal: %v", msg)
	}
}

func isOKWebsocketCloseError(err error) bool {
	return websocket.IsCloseError(err,
		websocket.CloseAbnormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNormalClosure,
	)
}

func (t *WSStream) SetReadDeadline(deadline time.Time) error {
	return t.ws.SetReadDeadline(deadline)
}

func (t *WSStream) processMessages(ctx context.Context) {
	defer func() {
		t.close()
	}()
	t.ws.SetReadLimit(teleport.MaxHTTPRequestSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			ty, bytes, err := t.ws.ReadMessage()
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || isOKWebsocketCloseError(err) {
					return
				}

				msg := err.Error()
				if len(bytes) > 0 {
					msg = string(bytes)
				}
				select {
				case <-ctx.Done():
				default:
					t.writeError(msg)
					return
				}
			}

			if ty != websocket.BinaryMessage {
				t.writeError(fmt.Sprintf("Expected binary message, got %v", ty))
				return
			}

			var envelope Envelope
			if err := proto.Unmarshal(bytes, &envelope); err != nil {
				t.writeError(fmt.Sprintf("Unable to parse message payload %v", err))
				return
			}

			switch envelope.Type {
			case defaults.WebsocketClose:
				return
			case defaults.WebsocketWebauthnChallenge:
				select {
				case <-ctx.Done():
					return
				case t.challengeC <- envelope:
				default:
				}
			case defaults.WebsocketRaw:
				select {
				case <-ctx.Done():
					return
				case t.rawC <- envelope:
				default:
				}
			default:
				if t.handlers == nil {
					continue
				}

				handler, ok := t.handlers[envelope.Type]
				if !ok {
					t.log.Warnf("Received web socket envelope with unknown type %v", envelope.Type)
					continue
				}

				go handler(ctx, envelope)
			}
		}
	}
}

// handleWindowResize receives window resize events and forwards
// them to the SSH session.
func (t *TerminalStream) handleWindowResize(ctx context.Context, envelope Envelope) {
	select {
	case <-ctx.Done():
		return
	case <-t.sessionReadyC:
	}

	if t.sshSession == nil {
		return
	}

	var e map[string]interface{}
	err := json.Unmarshal([]byte(envelope.Payload), &e)
	if err != nil {
		t.log.Warnf("Failed to parse resize payload: %v", err)
		return
	}

	size, ok := e["size"].(string)
	if !ok {
		t.log.Errorf("expected size to be of type string, got type %T instead", size)
		return
	}

	params, err := session.UnmarshalTerminalParams(size)
	if err != nil {
		t.log.Warnf("Failed to retrieve terminal size: %v", err)
		return
	}

	// nil params indicates the channel was closed
	if params == nil {
		return
	}

	if err := t.sshSession.WindowChange(ctx, params.H, params.W); err != nil {
		t.log.Error(err)
	}
}

func (t *TerminalStream) handleFileTransferDecision(ctx context.Context, envelope Envelope) {
	select {
	case <-ctx.Done():
		return
	case <-t.sessionReadyC:
	}

	if t.sshSession == nil {
		return
	}

	var e utils.Fields
	err := json.Unmarshal([]byte(envelope.Payload), &e)
	if err != nil {
		return
	}
	approved, ok := e["approved"].(bool)
	if !ok {
		t.log.Error("Unable to find approved status on response")
		return
	}

	if approved {
		err = t.sshSession.ApproveFileTransferRequest(ctx, e.GetString("requestId"))
	} else {
		err = t.sshSession.DenyFileTransferRequest(ctx, e.GetString("requestId"))
	}
	if err != nil {
		t.log.WithError(err).Error("Unable to respond to file transfer request")
	}
}

func (t *TerminalStream) handleFileTransferRequest(ctx context.Context, envelope Envelope) {
	select {
	case <-ctx.Done():
		return
	case <-t.sessionReadyC:
	}

	if t.sshSession == nil {
		return
	}

	var e utils.Fields
	err := json.Unmarshal([]byte(envelope.Payload), &e)
	if err != nil {
		return
	}
	download, ok := e["download"].(bool)
	if !ok {
		t.log.Error("Unable to find download param in response")
		return
	}

	if err := t.sshSession.RequestFileTransfer(ctx, tracessh.FileTransferReq{
		Download: download,
		Location: e.GetString("location"),
		Filename: e.GetString("filename"),
	}); err != nil {
		t.log.WithError(err).Error("Unable to request file transfer")
	}
}

func (t *TerminalStream) sessionCreated(s *tracessh.Session) {
	t.sshSession = s
	close(t.sessionReadyC)
}

// writeChallenge encodes and writes the challenge to the
// websocket in the correct format.
func (t *WSStream) writeChallenge(challenge *client.MFAAuthenticateChallenge, codec mfaCodec) error {
	// Send the challenge over the socket.
	msg, err := codec.encode(challenge, defaults.WebsocketWebauthnChallenge)
	if err != nil {
		return trace.Wrap(err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return trace.Wrap(t.ws.WriteMessage(websocket.BinaryMessage, msg))
}

// readChallengeResponse reads and decodes the challenge response from the
// websocket in the correct format.
func (t *WSStream) readChallengeResponse(codec mfaCodec) (*authproto.MFAAuthenticateResponse, error) {
	envelope, ok := <-t.challengeC
	if !ok {
		return nil, io.EOF
	}
	resp, err := codec.decodeResponse([]byte(envelope.Payload), defaults.WebsocketWebauthnChallenge)
	return resp, trace.Wrap(err)
}

// readChallenge reads and decodes the challenge from the
// websocket in the correct format.
func (t *WSStream) readChallenge(codec mfaCodec) (*authproto.MFAAuthenticateChallenge, error) {
	envelope, ok := <-t.challengeC
	if !ok {
		return nil, io.EOF
	}
	challenge, err := codec.decodeChallenge([]byte(envelope.Payload), defaults.WebsocketWebauthnChallenge)
	return challenge, trace.Wrap(err)
}

// writeAuditEvent encodes and writes the audit event to the
// websocket in the correct format.
func (t *WSStream) writeAuditEvent(event []byte) error {
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

func (t *WSStream) writeLatency(latency SSHSessionLatencyStats) error {
	data, err := json.Marshal(latency)
	if err != nil {
		return trace.Wrap(err)
	}

	encodedPayload, err := t.encoder.String(string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketLatency,
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
func (t *WSStream) Write(data []byte) (n int, err error) {
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

// Read provides data received from [defaults.WebsocketRaw] envelopes. If
// the previous envelope was not consumed in the last read, any remaining data
// is returned prior to processing the next envelope.
func (t *WSStream) Read(out []byte) (int, error) {
	if len(t.buffer) > 0 {
		n := copy(out, t.buffer)
		if n == len(t.buffer) {
			t.buffer = []byte{}
		} else {
			t.buffer = t.buffer[n:]
		}
		return n, nil
	}

	envelope, ok := <-t.rawC
	if !ok {
		return 0, io.EOF
	}

	data, err := t.decoder.Bytes([]byte(envelope.Payload))
	if err != nil {
		return 0, trace.Wrap(err)
	}

	n := copy(out, data)
	// if the payload size is greater than [out], store the remaining
	// part in the buffer to be processed on the next Read call
	if len(data) > n {
		t.buffer = data[n:]
	}
	return n, nil
}

// SendCloseMessage sends a close message on the web socket.
func (t *WSStream) SendCloseMessage(event sessionEndEvent) error {
	sessionMetadataPayload, err := json.Marshal(&event)
	if err != nil {
		return trace.Wrap(err)
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketClose,
		Payload: string(sessionMetadataPayload),
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return trace.Wrap(err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return trace.Wrap(t.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes))
}

func (t *WSStream) close() {
	t.once.Do(func() {
		close(t.rawC)
		close(t.challengeC)
	})
}

// Close sends a close message on the web socket and closes the web socket.
func (t *WSStream) Close() error {
	return trace.Wrap(t.ws.Close())
}

// deadlineForInterval returns a suitable network read deadline for a given ping interval.
// We chose to take the current time plus twice the interval to allow the timeframe of one interval
// to wait for a returned pong message.
func deadlineForInterval(interval time.Duration) time.Time {
	return time.Now().Add(interval * 2)
}
