/*

 Copyright 2023 Gravitational, Inc.

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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

type CommandRequest struct {
	// Command is the command to be executed on all nodes.
	Command string `json:"command"`
	// Login is a Linux username to connect as.
	Login string `json:"login"`
	//NodeIDs are the node IDs where the command should be executed.
	NodeIDs []string `json:"node_ids"`
	// Labels are the nodes labels where the command should be executed.
	Labels map[string]string `json:"labels"`
}

func (h *Handler) executeCommand(
	w http.ResponseWriter,
	r *http.Request,
	_ httprouter.Params,
	sessionCtx *SessionContext,
	site reversetunnel.RemoteSite,
) (any, error) {
	q := r.URL.Query()
	params := q.Get("params")
	if params == "" {
		return nil, trace.BadParameter("missing params")
	}
	var req *CommandRequest
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sessionCtx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := createIdentityContext(req.Login, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), identity, h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authAccessPoint, err := site.CachingAccessPoint()
	if err != nil {
		h.log.WithError(err).Debug("Unable to get auth access point.")
		return nil, trace.Wrap(err)
	}

	netConfig, err := authAccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		h.log.WithError(err).Debug("Unable to fetch cluster networking config.")
		return nil, trace.Wrap(err)
	}

	clusterName := site.GetName()

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		errMsg := "Error upgrading to websocket"
		h.log.WithError(err).Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return nil, nil
	}

	defer func() {
		ws.WriteMessage(websocket.CloseMessage, nil)
		ws.Close()
	}()

	keepAliveInterval := netConfig.GetKeepAliveInterval()
	err = ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil, nil
	}

	hosts, err := findByLabels(ctx, clt, req.Labels)
	if err != nil {
		log.WithError(err).Warn("failed to find nodes by labels")
	}

	for _, nodeID := range req.NodeIDs {
		host, err := findByHost(ctx, clt, nodeID)
		if err != nil {
			h.log.WithError(err).Warn("failed to find host by node ID")
			continue
		}

		hosts = append(hosts, *host)
	}

	if len(hosts) == 0 {
		const errMsg = "no server founds"
		h.log.Error(errMsg)
		return nil, trace.Errorf(errMsg)
	}

	hosts = removeDuplicates(hosts)

	h.log.Debugf("found %d hosts", len(hosts))

	for _, host := range hosts {
		err := func() error {
			sessionData, err := h.generateCommandSession(&host, req.Login, clusterName, sessionCtx.cfg.User)
			if err != nil {
				h.log.WithError(err).Debug("Unable to generate new ssh session.")
				return trace.Wrap(err)
			}

			h.log.Debugf("New command request for server=%s, labels=%v, login=%s, sid=%s, websid=%s.",
				req.NodeIDs, req.Labels, req.Login, sessionData.ID, sessionCtx.GetSessionID())

			commandHandlerConfig := CommandHandlerConfig{
				SessionCtx:         sessionCtx,
				AuthProvider:       clt,
				SessionData:        sessionData,
				KeepAliveInterval:  netConfig.GetKeepAliveInterval(),
				ProxyHostPort:      h.ProxyHostPort(),
				InteractiveCommand: strings.Split(req.Command, " "),
				Router:             h.cfg.Router,
				TracerProvider:     h.cfg.TracerProvider,
				proxySigner:        h.cfg.PROXYSigner,
			}

			handler, err := newCommandHandler(ctx, commandHandlerConfig)
			if err != nil {
				h.log.WithError(err).Error("Unable to create terminal.")
				return trace.Wrap(err)
			}
			handler.ws = &noopCloserWS{ws}

			h.userConns.Add(1)
			defer h.userConns.Add(-1)

			h.log.Infof("Executing command: %#v.", req)
			httplib.MakeTracingHandler(handler, teleport.ComponentProxy).ServeHTTP(w, r)

			return nil
		}()

		if err != nil {
			h.log.WithError(err).Warnf("Failed to start session: %v", host.hostName)
			continue
		}
	}

	return nil, nil
}

func newCommandHandler(ctx context.Context, cfg CommandHandlerConfig) (*commandHandler, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, span := cfg.tracer.Start(ctx, "NewCommand")
	defer span.End()

	return &commandHandler{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentWebsocket,
			"session_id":    cfg.SessionData.ID.String(),
		}),
		ctx:                cfg.SessionCtx,
		authProvider:       cfg.AuthProvider,
		sessionData:        cfg.SessionData,
		keepAliveInterval:  cfg.KeepAliveInterval,
		proxyHostPort:      cfg.ProxyHostPort,
		interactiveCommand: cfg.InteractiveCommand,
		router:             cfg.Router,
		proxySigner:        cfg.proxySigner,
		tracer:             cfg.tracer,
	}, nil
}

type CommandHandlerConfig struct {
	// sctx is the context for the users web session.
	SessionCtx *SessionContext
	// authProvider is used to fetch nodes and sessions from the backend.
	AuthProvider AuthProvider
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
	// ProxySigner is used to sign PROXY header and securely propagate client IP information
	proxySigner multiplexer.PROXYHeaderSigner
	// tracer is used to create spans
	tracer oteltrace.Tracer
}

func (t *CommandHandlerConfig) CheckAndSetDefaults() error {
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

	t.tracer = t.TracerProvider.Tracer("webcommand")

	return nil
}

type commandHandler struct {
	// log holds the structured logger.
	log *logrus.Entry
	// ctx is a web session context for the currently logged in user.
	ctx *SessionContext
	// authProvider is used to fetch nodes and sessions from the backend.
	authProvider AuthProvider
	// proxyHostPort is the address of the server to connect to.
	proxyHostPort string

	// keepAliveInterval is the interval for sending ping frames to web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	keepAliveInterval time.Duration

	// The server data for the active session.
	sessionData session.Session

	// router is used to dial the host
	router *proxy.Router

	stream *WsStream

	// tracer creates spans
	tracer oteltrace.Tracer

	// sshSession holds the "shell" SSH channel to the node.
	sshSession *tracessh.Session

	// ProxySigner is used to sign PROXY header and securely propagate client IP information
	proxySigner multiplexer.PROXYHeaderSigner

	// interactiveCommand is a command to execute.
	interactiveCommand []string

	// ws is the websocket connection to the client.
	ws WSConn
}

func (t *commandHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	// Allow closing websocket if the user logs out before exiting
	// the session.
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	sendError := func(errMsg string, err error, ws WSConn) {
		envelope := &Envelope{
			Version: defaults.WebsocketVersion,
			Type:    defaults.WebsocketError,
			Payload: fmt.Sprintf("%s: %s", errMsg, err.Error()),
		}

		envelopeBytes, err := proto.Marshal(envelope)
		if err != nil {
			t.log.WithError(err).Error("failed to marshal error message")
		}
		if err := ws.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
			t.log.WithError(err).Error("failed to send error message")
		}
	}

	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: t.sessionData})
	if err != nil {
		sendError("unable to marshal session response", err, t.ws)
		return
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		sendError("unable to marshal session data event for web client", err, t.ws)
		return
	}

	err = t.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		sendError("unable to write message to socket", err, t.ws)
		return
	}

	t.handler(r)
}

func (t *commandHandler) handler(r *http.Request) {
	t.stream = NewWStream(t.ws)

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(r.Context(), t.ws)
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.writeError(err)
		return
	}

	t.log.Debug("Creating websocket stream")

	// Update the read deadline upon receiving a pong message.
	t.ws.SetPongHandler(func(_ string) error {
		t.ws.SetReadDeadline(deadlineForInterval(t.keepAliveInterval))
		return nil
	})

	// Start sending ping frames through websocket to the client.
	go t.startPingLoop(r.Context(), t.ws)

	go t.streamEvents(r.Context(), tc)
	// Pump raw terminal in/out and audit events into the websocket.
	t.streamOutput(r.Context(), t.ws, tc)
}

// streamTerminal opens a SSH connection to the remote host and streams
// events back to the web client.
func (t *commandHandler) streamOutput(ctx context.Context, ws WSConn, tc *client.TeleportClient) {
	ctx, span := t.tracer.Start(ctx, "commandHandler/streamOutput")
	defer span.End()

	accessChecker, err := t.ctx.GetUserAccessChecker()
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failed to get access checker")
		t.writeError(err)
		return
	}

	getAgent := func() (teleagent.Agent, error) {
		return teleagent.NopCloser(tc.LocalAgent()), nil
	}
	signerCreator := func() (ssh.Signer, error) {
		cert, err := t.ctx.GetSSHCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		validBefore := time.Unix(int64(cert.ValidBefore), 0)
		ttl := time.Until(validBefore)
		return agentless.CreateAuthSigner(ctx, t.ctx.GetUser(), tc.SiteName, ttl, t.router)
	}
	conn, _, err := t.router.DialHost(ctx, ws.RemoteAddr(), ws.LocalAddr(), t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort), tc.SiteName, accessChecker, getAgent, signerCreator)
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

	nc, connectErr := client.NewNodeClient(ctx, sshConfig, conn, net.JoinHostPort(t.sessionData.ServerID, strconv.Itoa(t.sessionData.ServerHostPort)), tc, modules.GetModules().IsBoringBinary())
	switch {
	case connectErr != nil && !trace.IsAccessDenied(connectErr): // catastrophic error, return it
		t.log.WithError(connectErr).Warn("Unable to stream terminal - failed to create node client")
		t.writeError(connectErr)
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
			// write the original connection error
			t.writeError(connectErr)
			return
		}

		if !mfaRequiredResp.Required {
			t.log.WithError(connectErr).Warn("Unable to stream terminal - user does not have access to host")
			// write the original connection  error
			t.writeError(connectErr)
			return
		}

		//TODO(jakule): Implement MFA support
		t.log.Errorf("MFA support is not implemented")
		t.writeError(errors.New("MFA support is not implemented"))
		return
	}

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunCommand(ctx, t.interactiveCommand, nil); err != nil {
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

// startPingLoop starts a loop that will continuously send a ping frame through the websocket
// to prevent the connection between web client and teleport proxy from becoming idle.
// Interval is determined by the keep_alive_interval config set by user (or default).
// Loop will terminate when there is an error sending ping frame or when terminal session is closed.
func (t *commandHandler) startPingLoop(ctx context.Context, ws WSConn) {
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
		case <-ctx.Done():
			t.log.Debug("Terminating websocket ping loop.")
			return
		}
	}
}

func (t *commandHandler) Close() error {
	return nil
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *commandHandler) makeClient(ctx context.Context, ws WSConn) (*client.TeleportClient, error) {
	ctx, span := tracing.DefaultProvider().Tracer("terminal").Start(ctx, "commandHandler/makeClient")
	defer span.End()

	clientConfig, err := makeTeleportClientConfig(ctx, t.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfig.HostLogin = t.sessionData.Login
	clientConfig.ForwardAgent = client.ForwardAgentLocal
	clientConfig.Namespace = apidefaults.Namespace
	clientConfig.Stdout = newPayloadWriter(t.sessionData.ServerID, "stdout", t.stream)
	clientConfig.Stderr = newPayloadWriter(t.sessionData.ServerID, "stderr", t.stream)
	clientConfig.Stdin = &bytes.Buffer{} // set stdin to a dummy buffer
	clientConfig.SiteName = t.sessionData.ClusterName
	if err := clientConfig.ParseProxyHost(t.proxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	clientConfig.Host = t.sessionData.ServerHostname
	clientConfig.HostPort = t.sessionData.ServerHostPort
	clientConfig.SessionID = t.sessionData.ID.String()
	clientConfig.ClientAddr = ws.RemoteAddr().String()
	clientConfig.Tracer = t.tracer

	tc, err := client.NewClient(clientConfig)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	// Save the *ssh.Session after the shell has been created. The session is
	// used to update all other parties window size to that of the web client and
	// to allow future window changes.
	tc.OnShellCreated = func(s *tracessh.Session, c *tracessh.Client, _ io.ReadWriteCloser) (bool, error) {
		t.sshSession = s

		return false, nil
	}

	return tc, nil
}

func (t *commandHandler) streamEvents(ctx context.Context, tc *client.TeleportClient) {
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

// writeError displays an error in the terminal window.
func (t *commandHandler) writeError(err error) {
	out := &outEnvelope{
		NodeID:  t.sessionData.ServerID,
		Type:    "teleport-error",
		Payload: []byte(err.Error()),
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.log.WithError(err).Error("failed to marshal error message")
		return
	}

	if _, writeErr := t.stream.Write(data); writeErr != nil {
		t.log.WithError(writeErr).Warnf("Unable to send error to terminal: %v", err)
	}
}
