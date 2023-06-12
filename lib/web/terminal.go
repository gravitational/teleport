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
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
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
	stream *TerminalStream
	// tracker is the session tracker of the session being joined. May be nil
	// if the user is not joining a session.
	tracker types.SessionTracker

	// clock to use for presence checking
	clock clockwork.Clock
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

	envelope := &Envelope{
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
		// If the terminal handler was closed (most likely due to the *SessionContext
		// closing) then the stream should be closed as well.
		if t.stream != nil {
			err = t.stream.Close()
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
	t.stream = NewTerminalStream(ctx, ws, t.log)

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(ctx, t.stream, ws.RemoteAddr().String())
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.stream.writeError(err.Error())
		return
	}

	t.log.Debug("Creating websocket stream")

	// Start sending ping frames through websocket to client.
	go startPingLoop(ctx, ws, t.keepAliveInterval, t.log, t.Close)

	// Pump raw terminal in/out and audit events into the websocket.
	go t.streamEvents(ctx, tc)

	// Block until the terminal session is complete.
	t.streamTerminal(ctx, tc)
	t.log.Debug("Closing websocket stream")
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
	clientConfig.Stderr = stream
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
func (t *sshBaseHandler) issueSessionMFACerts(ctx context.Context, tc *client.TeleportClient, wsStream *WSStream) ([]ssh.AuthMethod, error) {
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
		return nil, trace.Wrap(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
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
			return nil, trace.Wrap(client.MFARequiredUnknown(err))
		}

		if !mfaRequiredResp.Required {
			return nil, trace.Wrap(services.ErrSessionMFANotRequired)
		}
	case authproto.MFARequired_MFA_REQUIRED_YES:
	}

	span.AddEvent("prompting user with mfa challenge")
	assertion, err := promptMFAChallenge(wsStream, protobufMFACodec{})(ctx, tc.WebProxyAddr, challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	span.AddEvent("user completed mfa challenge")

	err = stream.Send(&authproto.UserSingleUseCertsRequest{Request: &authproto.UserSingleUseCertsRequest_MFAResponse{MFAResponse: assertion}})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err = stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
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

func promptMFAChallenge(
	stream *WSStream,
	codec mfaCodec,
) client.PromptMFAChallengeHandler {
	return func(ctx context.Context, proxyAddr string, c *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
		var challenge *client.MFAAuthenticateChallenge

		// Convert from proto to JSON types.
		switch {
		case c.GetWebauthnChallenge() != nil:
			challenge = &client.MFAAuthenticateChallenge{
				WebauthnChallenge: wanlib.CredentialAssertionFromProto(c.WebauthnChallenge),
			}
		default:
			return nil, trace.AccessDenied("only hardware keys are supported on the web terminal, please register a hardware device to connect to this server")
		}

		if err := stream.writeChallenge(challenge, codec); err != nil {
			return nil, trace.Wrap(err)
		}

		resp, err := stream.readChallengeResponse(codec)
		return resp, trace.Wrap(err)
	}
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
	case !trace.IsAccessDenied(directErr) && errors.Is(mfaErr, auth.ErrNoMFADevices):
		return nil, trace.Wrap(directErr)
	case !errors.Is(mfaErr, io.EOF) && // Ignore any errors from MFA due to locks being enforced, the direct error will be friendlier
		!errors.Is(mfaErr, client.MFARequiredUnknownErr{}) && // Ignore any failures that occurred before determining if MFA was required
		!errors.Is(mfaErr, services.ErrSessionMFANotRequired): // Ignore any errors caused by attempting the MFA ceremony when MFA will not grant access
		return nil, trace.Wrap(mfaErr)
	default:
		return nil, trace.Wrap(directErr)
	}
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

	var beforeStart func(io.Writer)
	if t.participantMode == types.SessionModeratorMode {
		beforeStart = func(out io.Writer) {
			nc.OnMFA = func() {
				prompt := func(ctx context.Context, proxyAddr string, c *authproto.MFAAuthenticateChallenge) (*authproto.MFAAuthenticateResponse, error) {
					resp, err := promptMFAChallenge(t.stream.WSStream, protobufMFACodec{})(ctx, proxyAddr, c)
					return resp, trace.Wrap(err)
				}
				if err := client.RunPresenceTask(ctx, out, t.authProvider, t.sessionData.ID.String(), prompt, client.WithPresenceClock(t.clock)); err != nil {
					t.log.WithError(err).Warn("Unable to stream terminal - failure performing presence checks")
					return
				}
			}
		}
	}

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunInteractiveShell(ctx, t.participantMode, t.tracker, beforeStart); err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure running interactive shell")
		t.stream.writeError(err.Error())
		return
	}

	if err := t.stream.Close(); err != nil {
		t.log.WithError(err).Error("Unable to send close event to web client.")
		return
	}

	t.log.Debug("Sent close event to web client.")
}

// connectToNode attempts to connect to the host with the already
// provisioned certs for the user.
func (t *sshBaseHandler) connectToNode(ctx context.Context, ws WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error) {
	conn, _, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, getAgent, signer)
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
		return nil, trace.NewAggregate(err, conn.Close())
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
	conn, _, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, getAgent, signer)
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
		completedC: make(chan struct{}),
		rawC:       make(chan Envelope, 100),
		challengeC: make(chan Envelope, 1),
		handlers:   handlers,
	}

	go w.processMessages(ctx)

	return w
}

// NewTerminalStream creates a stream that manages reading and writing
// data over the provided [websocket.Conn]
func NewTerminalStream(ctx context.Context, ws *websocket.Conn, log logrus.FieldLogger) *TerminalStream {
	t := &TerminalStream{
		sessionReadyC: make(chan struct{}),
	}

	handlers := map[string]WSHandlerFunc{
		defaults.WebsocketResize:               t.handleWindowResize,
		defaults.WebsocketFileTransferRequest:  t.handleFileTransferRequest,
		defaults.WebsocketFileTransferDecision: t.handleFileTransferDecision,
	}

	t.WSStream = NewWStream(ctx, ws, log, handlers)

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
	completedC chan struct{}

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

func (t *WSStream) processMessages(ctx context.Context) {
	defer func() {
		close(t.completedC)
	}()
	t.ws.SetReadLimit(teleport.MaxHTTPRequestSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			ty, bytes, err := t.ws.ReadMessage()
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) ||
					websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
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

	var e map[string]string
	err := json.Unmarshal([]byte(envelope.Payload), &e)
	if err != nil {
		t.log.Warnf("Failed to parse resize payload: %v", err)
		return
	}

	params, err := session.UnmarshalTerminalParams(e["size"])
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
	select {
	case <-t.completedC:
		return nil, io.EOF
	case envelope := <-t.challengeC:
		resp, err := codec.decodeResponse([]byte(envelope.Payload), defaults.WebsocketWebauthnChallenge)
		return resp, trace.Wrap(err)
	}
}

// readChallenge reads and decodes the challenge from the
// websocket in the correct format.
func (t *WSStream) readChallenge(codec mfaCodec) (*authproto.MFAAuthenticateChallenge, error) {
	select {
	case <-t.completedC:
		return nil, io.EOF
	case envelope := <-t.challengeC:
		challenge, err := codec.decodeChallenge([]byte(envelope.Payload), defaults.WebsocketWebauthnChallenge)
		return challenge, trace.Wrap(err)
	}
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
// the previous envelope was not consumed in the last read any remaining data
// is returned prior to processing the next envelope.
func (t *WSStream) Read(out []byte) (n int, err error) {
	if len(t.buffer) > 0 {
		n := copy(out, t.buffer)
		if n == len(t.buffer) {
			t.buffer = []byte{}
		} else {
			t.buffer = t.buffer[n:]
		}
		return n, nil
	}

	select {
	case <-t.completedC:
		return 0, io.EOF
	case envelope := <-t.rawC:
		data, err := t.decoder.Bytes([]byte(envelope.Payload))
		if err != nil {
			return 0, trace.Wrap(err)
		}

		n := copy(out, data)
		// if payload size is greater than [out], store the remaining
		// part in the buffer to be processed on the next Read call
		if len(data) > n {
			t.buffer = data[n:]
		}
		return n, nil
	}
}

// Close sends a close message on the web socket and closes the web socket.
func (t *WSStream) Close() error {
	var closeErr error
	t.once.Do(func() {
		defer func() {
			<-t.completedC

			close(t.rawC)
			close(t.challengeC)
		}()

		// Send close envelope to web terminal upon exit without an error.
		envelope := &Envelope{
			Version: defaults.WebsocketVersion,
			Type:    defaults.WebsocketClose,
		}
		envelopeBytes, err := proto.Marshal(envelope)
		if err != nil {
			closeErr = trace.NewAggregate(err, t.ws.Close())
			return
		}

		t.mu.Lock()
		defer t.mu.Unlock()
		closeErr = trace.NewAggregate(t.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes), t.ws.Close())
	})

	return trace.Wrap(closeErr)
}

// deadlineForInterval returns a suitable network read deadline for a given ping interval.
// We chose to take the current time plus twice the interval to allow the timeframe of one interval
// to wait for a returned pong message.
func deadlineForInterval(interval time.Duration) time.Time {
	return time.Now().Add(interval * 2)
}
