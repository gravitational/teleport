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
	"github.com/gravitational/trace/trail"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
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

// AuthProvider is a subset of the full Auth API.
type AuthProvider interface {
	GetNodes(ctx context.Context, namespace string) ([]types.Server, error)
	GetSessionEvents(namespace string, sid session.ID, after int) ([]events.EventFields, error)
	GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error)
	IsMFARequired(ctx context.Context, req *authproto.IsMFARequiredRequest) (*authproto.IsMFARequiredResponse, error)
	GenerateUserSingleUseCerts(ctx context.Context) (authproto.AuthService_GenerateUserSingleUseCertsClient, error)
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
			log: logrus.WithFields(logrus.Fields{
				trace.Component: teleport.ComponentWebsocket,
				"session_id":    cfg.SessionData.ID.String(),
			}),
			ctx:                cfg.SessionCtx,
			authProvider:       cfg.AuthProvider,
			localAuthProvider:  cfg.LocalAuthProvider,
			sessionData:        cfg.SessionData,
			keepAliveInterval:  cfg.KeepAliveInterval,
			proxyHostPort:      cfg.ProxyHostPort,
			proxyPublicAddr:    cfg.ProxyPublicAddr,
			interactiveCommand: cfg.InteractiveCommand,
			router:             cfg.Router,
			tracer:             cfg.tracer,
		},
		displayLogin:    cfg.DisplayLogin,
		term:            cfg.Term,
		proxySigner:     cfg.PROXYSigner,
		participantMode: cfg.ParticipantMode,
		tracker:         cfg.Tracker,
		clock:           cfg.Clock,
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
	// AuthProvider is used to fetch nodes and sessions from the backend.
	AuthProvider AuthProvider
	// LocalAuthProvider is used to fetch user information from the
	// local cluster when connecting to agentless nodes.
	LocalAuthProvider agentless.AuthProvider
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
	// Clock used for presence checking.
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

	if t.AuthProvider == nil {
		return trace.BadParameter("AuthProvider must be provided")
	}

	if t.LocalAuthProvider == nil {
		return trace.BadParameter("LocalAuthProvider must be provided")
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
	// authProvider is used to fetch nodes and sessions from the backend.
	authProvider AuthProvider
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
	// localAuthProvider is used to fetch user information from the
	// local cluster when connecting to agentless nodes.
	localAuthProvider agentless.AuthProvider
	// interactiveCommand is a command to execute.
	interactiveCommand []string
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

	var sessionMetadataResponse []byte

	// If the displayLogin is set then use it in the session metadata instead of the
	// login name used in the SSH connection. This is specifically for the use case
	// when joining a session to avoid displaying "-teleport-internal-join" as the username.
	if t.displayLogin != "" {
		sessionDataTemp := t.sessionData
		sessionDataTemp.Login = t.displayLogin
		sessionMetadataResponse, err = json.Marshal(siteSessionGenerateResponse{Session: sessionDataTemp})
	} else {
		sessionMetadataResponse, err = json.Marshal(siteSessionGenerateResponse{Session: t.sessionData})
	}

	if err != nil {
		t.sendError("unable to marshal session response", err, ws)
		return
	}

	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		t.sendError("unable to marshal session data event for web client", err, ws)
		return
	}

	err = ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		t.sendError("unable to write message to socket", err, ws)
		return
	}

	t.handler(ws, r)
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
	t.stream = terminal.NewStream(ctx, terminal.StreamConfig{WS: ws, Logger: t.log})

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(ctx, t.stream, ws.RemoteAddr().String())
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.stream.WriteError(err.Error())
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
	go startWSPingLoop(ctx, ws, t.keepAliveInterval, t.log, t.Close)

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamEvents(ctx, tc)

	// Block until the terminal session is complete.
	t.streamTerminal(ctx, tc)
	t.log.Debug("Closing websocket stream")
}

type stderrWriter struct {
	stream *terminal.Stream
}

func (s stderrWriter) Write(b []byte) (int, error) {
	s.stream.WriteError(string(b))
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
		t.stream.SessionCreated(s)

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
// to make use of the authProvider already established for the session instead of leveraging
// the TeleportClient which would require dialing the auth server a second time.
func (t *sshBaseHandler) issueSessionMFACerts(ctx context.Context, tc *client.TeleportClient, wsStream *terminal.WSStream) ([]ssh.AuthMethod, error) {
	ctx, span := t.tracer.Start(ctx, "terminal/issueSessionMFACerts")
	defer span.End()

	// Always acquire single-use certificates from the root cluster, that's where
	// both the user and their devices are registered.
	log.Debug("Attempting to issue a single-use user certificate with an MFA check.")
	stream, err := t.ctx.cfg.RootClient.GenerateUserSingleUseCerts(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		stream.CloseSend()
		stream.Recv()
	}()

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
					SSHLogin:       tc.HostLogin,
				},
			},
		}); err != nil {
		return nil, trail.FromGRPC(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		err = trail.FromGRPC(err)
		// If connecting to a host in a leaf cluster and MFA failed check to see
		// if the leaf cluster requires MFA. If it doesn't return an error indicating
		// that MFA was not required instead of the error received from the root cluster.
		if t.sessionData.ClusterName != tc.SiteName {
			check, err := t.authProvider.IsMFARequired(ctx, &authproto.IsMFARequiredRequest{
				Target: &authproto.IsMFARequiredRequest_Node{
					Node: &authproto.NodeLogin{
						Node:  t.sessionData.ServerID,
						Login: tc.HostLogin,
					},
				},
			})
			if err != nil {
				return nil, trace.Wrap(client.MFARequiredUnknown(err))
			}
			if !check.Required {
				return nil, trace.Wrap(services.ErrSessionMFANotRequired)
			}
		}

		return nil, trail.FromGRPC(err)
	}

	challenge := resp.GetMFAChallenge()
	if challenge == nil {
		return nil, trace.BadParameter("server sent a %T on GenerateUserSingleUseCerts, expected MFAChallenge", resp.Response)
	}

	switch challenge.MFARequired {
	case authproto.MFARequired_MFA_REQUIRED_NO:
		return nil, trace.Wrap(services.ErrSessionMFANotRequired)
	case authproto.MFARequired_MFA_REQUIRED_UNSPECIFIED:
		mfaRequiredResp, err := t.authProvider.IsMFARequired(ctx, &authproto.IsMFARequiredRequest{
			Target: &authproto.IsMFARequiredRequest_Node{
				Node: &authproto.NodeLogin{
					Node:  t.sessionData.ServerID,
					Login: tc.HostLogin,
				},
			},
		})
		if err != nil {
			return nil, trace.Wrap(client.MFARequiredUnknown(trail.FromGRPC(err)))
		}

		if !mfaRequiredResp.Required {
			return nil, trace.Wrap(services.ErrSessionMFANotRequired)
		}
	case authproto.MFARequired_MFA_REQUIRED_YES:
	}

	span.AddEvent("prompting user with mfa challenge")
	assertion, err := promptMFAChallenge(wsStream, protobufMFACodec{})(ctx, challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	span.AddEvent("user completed mfa challenge")

	err = stream.Send(&authproto.UserSingleUseCertsRequest{Request: &authproto.UserSingleUseCertsRequest_MFAResponse{MFAResponse: assertion}})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	resp, err = stream.Recv()
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	certResp := resp.GetCert()
	if certResp == nil {
		return nil, trace.BadParameter("server sent a %T on GenerateUserSingleUseCerts, expected SingleUseUserCert", resp.Response)
	}

	switch crt := certResp.Cert.(type) {
	case *authproto.SingleUseUserCert_SSH:
		key.Cert = crt.SSH
	default:
		return nil, trace.BadParameter("server sent a %T SingleUseUserCert in response", certResp.Cert)
	}

	key.ClusterName = t.sessionData.ClusterName

	am, err := key.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []ssh.AuthMethod{am}, nil
}

func promptMFAChallenge(stream *terminal.WSStream, codec terminal.MFACodec) client.PromptMFAFunc {
	return func(ctx context.Context, chal *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
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

		if err := stream.WriteChallenge(challenge, codec); err != nil {
			return nil, trace.Wrap(err)
		}

		resp, err := stream.ReadChallengeResponse(codec)
		return resp, trace.Wrap(err)
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
	signer := agentless.SignerFromSSHCertificate(cert, t.localAuthProvider, tc.SiteName, tc.Username)

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

func monitorSessionLatency(ctx context.Context, clock clockwork.Clock, stream *terminal.WSStream, sshClient *tracessh.Client) error {
	wsPinger, err := latency.NewWebsocketPinger(clock, stream)
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
			return trace.Wrap(stream.WriteLatency(terminal.SSHSessionLatencyStats{
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

// streamTerminal opens an SSH connection to the remote host and streams
// events back to the web client.
func (t *TerminalHandler) streamTerminal(ctx context.Context, tc *client.TeleportClient) {
	ctx, span := t.tracer.Start(ctx, "terminal/streamTerminal")
	defer span.End()

	nc, err := t.connectToHost(ctx, t.stream, tc, t.connectToNodeWithMFA)
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure connecting to host")
		t.stream.WriteError(err.Error())
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

	var beforeStart func(io.Writer)
	if t.participantMode == types.SessionModeratorMode {
		beforeStart = func(out io.Writer) {
			nc.OnMFA = func() {
				if err := client.RunPresenceTask(ctx, out, t.authProvider, t.sessionData.ID.String(), promptMFAChallenge(t.stream.WSStream, protobufMFACodec{}), client.WithPresenceClock(t.clock)); err != nil {
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
			t.stream.WriteError(err.Error())
		}
		return
	}

	if t.closedByClient.Load() {
		return
	}

	// Send close envelope to web terminal upon exit without an error.
	if err := t.stream.SendCloseMessage(t.sessionData.ServerID); err != nil {
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
func (t *sshBaseHandler) connectToNode(ctx context.Context, ws terminal.WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error) {
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

		// Since connection attempts are made via UUID and not hostname, any access denied errors
		// will not contain the resolved host address. To provide an easier troubleshooting experience
		// for users, attempt to resolve the hostname of the server and augment the error message with it.
		if trace.IsAccessDenied(err) {
			if resp, err := t.authProvider.ListUnifiedResources(ctx, &authproto.ListUnifiedResourcesRequest{
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

			if err := t.stream.WriteAuditEvent(data); err != nil {
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

// deadlineForInterval returns a suitable network read deadline for a given ping interval.
// We chose to take the current time plus twice the interval to allow the timeframe of one interval
// to wait for a returned pong message.
func deadlineForInterval(interval time.Duration) time.Time {
	return time.Now().Add(interval * 2)
}
