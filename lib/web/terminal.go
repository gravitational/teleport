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
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/diagnostics/latency"
	"github.com/gravitational/teleport/lib/web/terminal"
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
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)
	IsMFARequired(ctx context.Context, req *authproto.IsMFARequiredRequest) (*authproto.IsMFARequiredResponse, error)
	CreateAuthenticateChallenge(ctx context.Context, req *authproto.CreateAuthenticateChallengeRequest) (*authproto.MFAAuthenticateChallenge, error)
	GenerateUserCerts(ctx context.Context, req authproto.UserCertsRequest) (*authproto.Certs, error)
	MaintainSessionPresence(ctx context.Context) (authproto.AuthService_MaintainSessionPresenceClient, error)
	ListUnifiedResources(ctx context.Context, req *authproto.ListUnifiedResourcesRequest) (*authproto.ListUnifiedResourcesResponse, error)
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
			logger: cfg.Logger.With(
				teleport.ComponentKey, teleport.ComponentWebsocket,
				"session_id", cfg.SessionData.ID.String(),
			),
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
			sshDialTimeout:     cfg.SSHDialTimeout,
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
	// Logger specifies the logger.
	Logger *slog.Logger
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
	// SSHDialTimeout is the dial timeout that should be enforced on ssh connections.
	SSHDialTimeout time.Duration
}

func (t *TerminalHandlerConfig) CheckAndSetDefaults() error {
	if t.Logger == nil {
		t.Logger = slog.Default().With(teleport.ComponentKey, teleport.ComponentWebsocket)
	}

	// Make sure whatever session is requested is a valid session id.
	if !t.SessionData.ID.IsZero() {
		_, err := session.ParseID(t.SessionData.ID.String())
		if err != nil {
			return trace.BadParameter("sid: invalid session id")
		}
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
	// logger holds the structured logger.
	logger *slog.Logger
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
	// sshDialTimeout is the maximum time to wait for an SSH connection
	// to be established before aborting.
	sshDialTimeout time.Duration
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
	stream *terminal.Stream
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
		t.logger.ErrorContext(r.Context(), "Error setting websocket readline", "error", err)
		return
	}

	t.handler(ws, r)
}

func (t *TerminalHandler) writeSessionData(ctx context.Context) error {
	envelope := &terminal.Envelope{
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
			t.sendError(ctx, "unable to marshal session response", err, t.stream)
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
			t.sendError(ctx, "unable to marshal session response", err, t.stream)
			return trace.Wrap(err)
		}
		envelope.Payload = string(sessionMetadataResponse)
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		t.sendError(ctx, "unable to marshal session data event for web client", err, t.stream)
		return trace.Wrap(err)
	}

	if err := t.stream.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
		t.sendError(ctx, "unable to write message to socket", err, t.stream)
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

		err = trace.Wrap(t.stream.Close())
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
	t.stream = terminal.NewStream(ctx, terminal.StreamConfig{WS: ws, Logger: t.logger})

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(ctx, t.stream, ws.RemoteAddr().String())
	if err != nil {
		t.logger.InfoContext(ctx, "Failed creating a client for session", "error", err)
		t.stream.WriteError(ctx, err.Error())
		return
	}

	t.logger.DebugContext(ctx, "Creating websocket stream")

	defaultCloseHandler := ws.CloseHandler()
	ws.SetCloseHandler(func(code int, text string) error {
		t.closedByClient.Store(true)
		t.logger.DebugContext(ctx, "web socket was closed by client - terminating session")

		// Call the default close handler if one was set.
		if defaultCloseHandler != nil {
			err := defaultCloseHandler(code, text)
			return trace.NewAggregate(err, t.Close())
		}

		return trace.Wrap(t.Close())
	})

	// Start sending ping frames through websocket to client.
	go startWSPingLoop(ctx, ws, t.keepAliveInterval, t.logger, t.Close)

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamEvents(ctx, tc)

	// Block until the terminal session is complete.
	t.streamTerminal(ctx, tc)
	t.logger.DebugContext(ctx, "Closing websocket stream")
}

type stderrWriter struct {
	stream *terminal.Stream
}

func (s stderrWriter) Write(b []byte) (int, error) {
	s.stream.WriteError(context.Background(), string(b))
	return len(b), nil
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *TerminalHandler) makeClient(ctx context.Context, stream *terminal.Stream, clientAddr string) (*client.TeleportClient, error) {
	ctx, span := tracing.DefaultProvider().Tracer("terminal").Start(ctx, "terminal/makeClient")
	defer span.End()

	clientConfig, err := makeTeleportClientConfig(ctx, t.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfig.HostLogin = t.sessionData.Login
	clientConfig.ForwardAgent = client.ForwardAgentLocal
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
	clientConfig.SSHDialTimeout = t.sshDialTimeout

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
		if err := t.stream.SessionCreated(s); err != nil {
			t.logger.DebugContext(ctx, "terminating established ssh connection to host",
				"error", err,
			)
			return false, trace.Wrap(s.Close())
		}

		// The web session was closed by the client while the ssh connection was being established.
		// Attempt to close the SSH session instead of proceeding with the window change request.
		if t.closedByClient.Load() {
			t.logger.DebugContext(ctx, "websocket was closed by client, terminating established ssh connection to host")
			return false, trace.Wrap(s.Close())
		}

		if err := s.WindowChange(ctx, t.term.H, t.term.W); err != nil {
			t.logger.ErrorContext(ctx, "failed to send window change request", "error", err)
		}

		return false, nil
	}

	return tc, nil
}

// issueSessionMFACerts performs the mfa ceremony to retrieve new certs that can be
// used to access nodes which require per-session mfa. The ceremony is performed directly
// to make use of the userAuthClient already established for the session instead of leveraging
// the TeleportClient which would require dialing the auth server a second time.
func (t *sshBaseHandler) issueSessionMFACerts(ctx context.Context, tc *client.TeleportClient, wsStream *terminal.WSStream) ([]ssh.AuthMethod, error) {
	ctx, span := t.tracer.Start(ctx, "terminal/issueSessionMFACerts")
	defer span.End()

	t.logger.DebugContext(ctx, "Attempting to issue a single-use user certificate with an MFA check")

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
	pk, err := keys.ParsePrivateKey(t.ctx.cfg.Session.GetSSHPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshCert, err := sshutils.ParseCertificate(t.ctx.cfg.Session.GetPub())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	expires := time.Unix(int64(sshCert.ValidBefore), 0)
	certsReq := &authproto.UserCertsRequest{
		SSHPublicKey:   pk.MarshalSSHPublicKey(),
		Username:       sshCert.KeyId, // SSH cert KeyId is set to teleport username.
		Expires:        expires,
		RouteToCluster: t.sessionData.ClusterName,
		NodeName:       t.sessionData.ServerID,
		Usage:          authproto.UserCertsRequest_SSH,
		Format:         tc.CertificateFormat,
		SSHLogin:       tc.HostLogin,
	}

	result, err := client.PerformSessionMFACeremony(ctx, client.PerformSessionMFACeremonyParams{
		CurrentAuthClient: t.userAuthClient,
		RootAuthClient:    t.ctx.cfg.RootClient,
		MFACeremony:       newMFACeremony(wsStream, t.ctx.cfg.RootClient.CreateAuthenticateChallenge, t.proxyPublicAddr),
		MFAAgainstRoot:    t.ctx.cfg.RootClusterName == tc.SiteName,
		MFARequiredReq:    mfaRequiredReq,
		CertsReq:          certsReq,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshCert, err = sshutils.ParseCertificate(result.NewCerts.SSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	am, err := sshutils.AsAuthMethod(sshCert, pk)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []ssh.AuthMethod{am}, nil
}

func newMFACeremony(stream *terminal.WSStream, createAuthenticateChallenge mfa.CreateAuthenticateChallengeFunc, proxyAddr string) *mfa.Ceremony {
	// channelID is used by the front end to differentiate between separate ongoing SSO challenges.
	var channelID string

	return &mfa.Ceremony{
		CreateAuthenticateChallenge: createAuthenticateChallenge,
		SSOMFACeremonyConstructor: func(ctx context.Context) (mfa.SSOMFACeremony, error) {
			id, err := uuid.NewRandom()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			channelID = id.String()

			u, err := url.Parse(sso.WebMFARedirect)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			u.RawQuery = url.Values{"channel_id": {channelID}}.Encode()
			return &sso.MFACeremony{
				ClientCallbackURL: u.String(),
				ProxyAddress:      proxyAddr,
			}, nil
		},
		PromptConstructor: func(...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(ctx context.Context, chal *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
				// Convert from proto to JSON types.
				var challenge client.MFAAuthenticateChallenge
				if chal.WebauthnChallenge != nil {
					challenge.WebauthnChallenge = wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge)
				}

				if chal.SSOChallenge != nil {
					challenge.SSOChallenge = client.SSOChallengeFromProto(chal.SSOChallenge)
					challenge.SSOChallenge.ChannelID = channelID
				}

				if chal.WebauthnChallenge == nil && chal.SSOChallenge == nil {
					return nil, trace.AccessDenied("only WebAuthn and SSO MFA methods are supported on the web terminal, please register a supported mfa method to connect to this server")
				}

				var codec protobufMFACodec
				if err := stream.WriteChallenge(&challenge, codec); err != nil {
					return nil, trace.Wrap(err)
				}

				resp, err := stream.ReadChallengeResponse(codec)
				return resp, trace.Wrap(err)
			})
		},
	}
}

type connectWithMFAFn = func(ctx context.Context, ws terminal.WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error)

// connectToHost establishes a connection to the target host. To reduce connection
// latency if per session mfa is required, connections are tried with the existing
// certs and with single use certs after completing the mfa ceremony. Only one of
// the operations will succeed, and if per session mfa will not gain access to the
// target it will abort before prompting a user to perform the ceremony.
func (t *sshBaseHandler) connectToHost(ctx context.Context, ws terminal.WSConn, tc *client.TeleportClient, connectToNodeWithMFA connectWithMFAFn) (*client.NodeClient, error) {
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

	ident, err := sshca.DecodeIdentity(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer := agentless.SignerFromSSHIdentity(ident, t.localAccessPoint, tc.SiteName, tc.Username)

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

// streamTerminal opens an SSH connection to the remote host and streams
// events back to the web client.
func (t *TerminalHandler) streamTerminal(ctx context.Context, tc *client.TeleportClient) {
	ctx, span := t.tracer.Start(ctx, "terminal/streamTerminal")
	defer span.End()

	nc, err := t.connectToHost(ctx, t.stream, tc, t.connectToNodeWithMFA)
	if err != nil {
		t.logger.WarnContext(ctx, "Unable to stream terminal - failure connecting to host", "error", err)
		t.stream.WriteError(ctx, err.Error())
		return
	}
	defer nc.Close()

	// If the session was terminated by client while the connection to the host
	// was being established, then return early before creating the shell. Any terminations
	// by the client from here on out should either get caught in the OnShellCreated callback
	// set on the [tc] or in [TerminalHandler.Close].
	if t.closedByClient.Load() {
		t.logger.DebugContext(ctx, "websocket was closed by client, aborting establishing ssh connection to host")
		return
	}

	var beforeStart func(io.Writer)
	if t.participantMode == types.SessionModeratorMode {
		beforeStart = func(out io.Writer) {
			nc.OnMFA = func() {
				baseCeremony := newMFACeremony(t.stream.WSStream, nil, t.proxyPublicAddr)
				if err := t.presenceChecker(ctx, out, t.userAuthClient, t.sessionData.ID.String(), baseCeremony); err != nil {
					t.logger.WarnContext(ctx, "Unable to stream terminal - failure performing presence checks", "error", err)
					return
				}
			}
		}
	}

	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()

	sshPinger, err := latency.NewSSHPinger(nc.Client)
	if err != nil {
		t.logger.WarnContext(monitorCtx, "failure monitoring session latency", "error", err)
	} else {
		go monitorLatency(monitorCtx, t.clock, t.stream.WSStream, sshPinger,
			latency.ReporterFunc(
				func(ctx context.Context, statistics latency.Statistics) error {
					return trace.Wrap(
						t.stream.WSStream.WriteLatency(terminal.SSHSessionLatencyStats{
							WebSocket: statistics.Client,
							SSH:       statistics.Server,
						}),
					)
				},
			),
		)
	}

	sessionDataSent := make(chan struct{})
	// If we are joining a session, send the session data right away, we
	// know the session ID
	if t.tracker != nil {
		if err := t.writeSessionData(ctx); err != nil {
			t.logger.WarnContext(ctx, "Failure sending session data", "error", err)
		}
		close(sessionDataSent)
	} else {
		// We are creating a new session and the server will generate a
		// new session ID, send the session data once the session is
		// created and the server sends us the session ID it is using
		writeSessionCtx, writeSessionCancel := context.WithCancel(ctx)
		defer writeSessionCancel()
		waitForSessionID := prepareToReceiveSessionID(writeSessionCtx, t.logger, nc)

		// wait in a new goroutine because the server won't set a
		// session ID until we open a shell
		go func() {
			defer close(sessionDataSent)

			sid, status := waitForSessionID()
			switch status {
			case sessionIDReceived:
				t.sessionData.ID = sid
				fallthrough
			case sessionIDNotModified:
				if err := t.writeSessionData(ctx); err != nil {
					t.logger.WarnContext(ctx, "Failure sending session data", "error", err)
				}
			case sessionIDNotSent:
				t.logger.WarnContext(ctx, "Failed to receive session data")
			default:
				t.logger.WarnContext(ctx, "Invalid session ID status", "status", status)
			}
		}()
	}

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunInteractiveShell(ctx, t.participantMode, t.tracker, nil, beforeStart); err != nil {
		if !t.closedByClient.Load() {
			t.stream.WriteError(ctx, err.Error())
		}
		return
	}

	if t.closedByClient.Load() {
		return
	}

	// Wait for the session data to be sent before closing the session
	<-sessionDataSent

	// Send close envelope to web terminal upon exit without an error.
	if err := t.stream.SendCloseMessage(t.sessionData.ServerID); err != nil {
		t.logger.ErrorContext(ctx, "Unable to send close event to web client", "error", err)
	}

	if err := t.stream.Close(); err != nil && !errors.Is(err, io.EOF) {
		t.logger.ErrorContext(ctx, "Unable to close client web socket", "error", err)
		return
	}

	t.logger.DebugContext(ctx, "Sent close event to web client")
}

// connectToNode attempts to connect to the host with the already
// provisioned certs for the user.
func (t *sshBaseHandler) connectToNode(ctx context.Context, ws terminal.WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error) {
	conn, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker.CheckAccessToRemoteCluster, getAgent, signer)
	if err != nil {
		t.logger.WarnContext(ctx, "Unable to stream terminal - failed to dial host", "error", err)

		if errors.Is(err, teleport.ErrNodeIsAmbiguous) {
			const message = "error: ambiguous host could match multiple nodes\n\nHint: try addressing the node by unique id (ex: user@node-id)\n"
			return nil, trace.NotFound("%s", message)
		}

		return nil, trace.Wrap(err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            tc.HostLogin,
		Auth:            tc.AuthMethods,
		HostKeyCallback: tc.HostKeyCallback,
		Timeout:         t.sshDialTimeout,
	}

	clt, err := client.NewNodeClient(ctx, sshConfig, conn,
		net.JoinHostPort(t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort)),
		t.sessionData.ServerHostname,
		tc, modules.GetModules().IsBoringBinary())
	if err != nil {
		// The close error is ignored instead of using [trace.NewAggregate] because
		// aggregate errors do not allow error inspection with things like [trace.IsAccessDenied].
		_ = conn.Close()

		// Since connection attempts are made via UUID and not hostname, any access denied errors
		// will not contain the resolved host address. To provide an easier troubleshooting experience
		// for users, attempt to resolve the hostname of the server and augment the error message with it.
		if trace.IsAccessDenied(err) {
			if resp, err := t.userAuthClient.ListUnifiedResources(ctx, &authproto.ListUnifiedResourcesRequest{
				SortBy:              types.SortBy{Field: types.ResourceKind},
				Kinds:               []string{types.KindNode},
				Limit:               1,
				PredicateExpression: fmt.Sprintf(`resource.metadata.name == "%s"`, t.sessionData.ServerID),
			}); err == nil && len(resp.Resources) > 0 {
				return nil, trace.AccessDenied("access denied to %q connecting to %v", sshConfig.User, resp.Resources[0].GetNode().GetHostname())
			}
		}

		return nil, trace.Wrap(err)
	}

	clt.ProxyPublicAddr = t.proxyPublicAddr

	return clt, nil
}

// connectToNodeWithMFA attempts to perform the mfa ceremony and then dial the
// host with the retrieved single use certs.
func (t *TerminalHandler) connectToNodeWithMFA(ctx context.Context, ws terminal.WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error) {
	// perform mfa ceremony and retrieve new certs
	authMethods, err := t.issueSessionMFACerts(ctx, tc, t.stream.WSStream)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return t.connectToNodeWithMFABase(ctx, ws, tc, accessChecker, getAgent, signer, authMethods)
}

// connectToNodeWithMFABase attempts to dial the host with the provided auth
// methods.
func (t *sshBaseHandler) connectToNodeWithMFABase(ctx context.Context, ws terminal.WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator, authMethods []ssh.AuthMethod) (*client.NodeClient, error) {
	sshConfig := &ssh.ClientConfig{
		User:            tc.HostLogin,
		Auth:            authMethods,
		HostKeyCallback: tc.HostKeyCallback,
		Timeout:         t.sshDialTimeout,
	}

	// connect to the node again with the new certs
	conn, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker.CheckAccessToRemoteCluster, getAgent, signer)
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

// sendError sends an error message to the client using the provided websocket.
func (t *sshBaseHandler) sendError(ctx context.Context, errMsg string, err error, ws terminal.WSConn) {
	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketError,
		Payload: fmt.Sprintf("%s: %s", errMsg, err.Error()),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		t.logger.ErrorContext(ctx, "failed to marshal error message", "error", err)
	}
	if err := ws.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
		t.logger.ErrorContext(ctx, "failed to send error message", "error", err)
	}
}

// streamEvents receives events over the SSH connection and forwards them to
// the web client.
func (t *TerminalHandler) streamEvents(ctx context.Context, tc *client.TeleportClient) {
	for {
		select {
		// Send push events that come over the events channel to the web client.
		case event := <-tc.EventsChannel():
			logger := t.logger.With("event", event.GetType())

			data, err := json.Marshal(event)
			if err != nil {
				logger.ErrorContext(ctx, "Unable to marshal audit event", "error", err)
				continue
			}

			logger.DebugContext(ctx, "Sending audit event to web client")

			if err := t.stream.WriteAuditEvent(data); err != nil {
				if errors.Is(err, websocket.ErrCloseSent) {
					logger.DebugContext(ctx, "Websocket was closed, no longer streaming events", "error", err)
					return
				}
				logger.ErrorContext(ctx, "Unable to send audit event to web client", "error", err)
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

// deadlineForInterval returns a suitable network read deadline for a given ping interval.
// We chose to take the current time plus twice the interval to allow the timeframe of one interval
// to wait for a returned pong message.
func deadlineForInterval(interval time.Duration) time.Time {
	return time.Now().Add(interval * 2)
}
