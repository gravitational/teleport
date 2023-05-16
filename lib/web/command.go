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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/observability/tracing"
	assistlib "github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/teleagent"
)

// CommandRequest is a request to execute a command on all nodes that match the query.
type CommandRequest struct {
	// Command is the command to be executed on all nodes.
	Command string `json:"command"`
	// Login is a Linux username to connect as.
	Login string `json:"login"`
	// Query is the predicate query to filter nodes where the command will be executed.
	Query string `json:"query"`
	// ConversationID is the conversation context that was used to execute the command.
	ConversationID string `json:"conversation_id"`
	// ExecutionID is a unique ID used to identify the command execution.
	ExecutionID string `json:"execution_id"`
}

// Check checks if the request is valid.
func (c *CommandRequest) Check() error {
	if c.Command == "" {
		return trace.BadParameter("missing command")
	}

	if c.Query == "" {
		return trace.BadParameter("missing query")
	}

	if c.Login == "" {
		return trace.BadParameter("missing login")
	}

	if c.ConversationID == "" {
		return trace.BadParameter("missing conversation ID")
	}

	if c.ExecutionID == "" {
		return trace.BadParameter("missing execution ID")
	}

	return nil
}

// executeCommand executes a command on all nodes that match the query.
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
		return nil, trace.BadParameter("failed to read JSON message: %v", err)
	}

	if err := req.Check(); err != nil {
		return nil, trace.BadParameter("invalid payload: %v", err)
	}

	clt, err := sessionCtx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAssistEnabled(clt, r.Context()); err != nil {
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
		return nil, trace.Wrap(err)
	}

	hosts, err := findByQuery(ctx, clt, req.Query)
	if err != nil {
		log.WithError(err).Warn("Failed to find nodes by labels")
		return nil, trace.Wrap(err)
	}

	if len(hosts) == 0 {
		const errMsg = "no servers found"
		h.log.Error(errMsg)
		return nil, trace.Errorf(errMsg)
	}

	h.log.Debugf("Found %d hosts to run Assist command %q on.", len(hosts), req.Command)

	for _, host := range hosts {
		err := func() error {
			sessionData, err := h.generateCommandSession(&host, req.Login, clusterName, sessionCtx.cfg.User)
			if err != nil {
				h.log.WithError(err).Debug("Unable to generate new ssh session.")
				return trace.Wrap(err)
			}

			h.log.Debugf("New command request for server=%s, id=%v, login=%s, sid=%s, websid=%s.",
				host.hostName, host.id, req.Login, sessionData.ID, sessionCtx.GetSessionID())

			commandHandlerConfig := CommandHandlerConfig{
				SessionCtx:         sessionCtx,
				AuthProvider:       clt,
				SessionData:        sessionData,
				KeepAliveInterval:  netConfig.GetKeepAliveInterval(),
				ProxyHostPort:      h.ProxyHostPort(),
				InteractiveCommand: strings.Split(req.Command, " "),
				Router:             h.cfg.Router,
				TracerProvider:     h.cfg.TracerProvider,
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

			msgPayload, err := json.Marshal(struct {
				NodeID      string `json:"node_id"`
				ExecutionID string `json:"execution_id"`
				SessionID   string `json:"session_id"`
			}{
				NodeID:      host.id,
				ExecutionID: req.ExecutionID,
				SessionID:   string(sessionData.ID),
			})

			if err != nil {
				return trace.Wrap(err)
			}

			err = clt.CreateAssistantMessage(ctx, &assist.CreateAssistantMessageRequest{
				ConversationId: req.ConversationID,
				Username:       identity.TeleportUser,
				Message: &assist.AssistantMessage{
					Type:        string(assistlib.MessageKindCommandResult),
					CreatedTime: timestamppb.New(time.Now().UTC()),
					Payload:     string(msgPayload),
				},
			})

			return trace.Wrap(err)
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
		sshBaseHandler: sshBaseHandler{
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
			tracer:             cfg.tracer,
		},
	}, nil
}

// CommandHandlerConfig is the configuration for the command handler.
type CommandHandlerConfig struct {
	// SessionCtx is the context for the user's web session.
	SessionCtx *SessionContext
	// AuthProvider is used to fetch nodes and sessions from the backend.
	AuthProvider AuthProvider
	// SessionData is the data to send to the client on the initial session creation.
	SessionData session.Session
	// KeepAliveInterval is the interval for sending ping frames to a web client.
	// This value is pulled from the cluster network config and
	// guaranteed to be set to a nonzero value as it's enforced by the configuration.
	KeepAliveInterval time.Duration
	// ProxyHostPort is the address of the server to connect to.
	ProxyHostPort string
	// InteractiveCommand is a command to execute.
	InteractiveCommand []string
	// Router determines how connections to nodes are created
	Router *proxy.Router
	// TracerProvider is used to create the tracer
	TracerProvider oteltrace.TracerProvider
	// tracer is used to create spans
	tracer oteltrace.Tracer
}

// CheckAndSetDefaults checks and sets default values.
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

// commandHandler is a handler for executing commands on a remote node.
type commandHandler struct {
	sshBaseHandler

	// stream is the websocket stream to the client.
	stream *WSStream

	// ws a raw websocket connection to the client.
	ws WSConn
}

// sendError sends an error message to the client using the provided websocket.
func (t *sshBaseHandler) sendError(errMsg string, err error, ws WSConn) {
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

func (t *commandHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	// Allow closing websocket if the user logs out before exiting
	// the session.
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: t.sessionData})
	if err != nil {
		t.sendError("unable to marshal session response", err, t.ws)
		return
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		t.sendError("unable to marshal session data event for web client", err, t.ws)
		return
	}

	err = t.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		t.sendError("unable to write message to socket", err, t.ws)
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
	go startPingLoop(r.Context(), t.ws, t.keepAliveInterval, t.log, t.Close)

	// Pump raw terminal in/out and audit events into the websocket.
	t.streamOutput(r.Context(), tc)
}

// streamOutput opens an SSH connection to the remote host and streams
// events back to the web client.
func (t *commandHandler) streamOutput(ctx context.Context, tc *client.TeleportClient) {
	ctx, span := t.tracer.Start(ctx, "commandHandler/streamOutput")
	defer span.End()

	mfaAuth := func(ctx context.Context, ws WSConn, tc *client.TeleportClient,
		accessChecker services.AccessChecker, getAgent teleagent.Getter,
	) (*client.NodeClient, error) {
		return nil, trace.NotImplemented("MFA is not supported for command execution")
	}

	//TODO(jakule): Implement MFA support
	nc, err := t.connectToHost(ctx, t.ws, tc, mfaAuth)
	if err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure connecting to host")
		t.writeError(err)
		return
	}

	defer nc.Close()

	// Enable session recording
	nc.AddEnv(teleport.EnableNonInteractiveSessionRecording, "true")

	// Establish SSH connection to the server. This function will block until
	// either an error occurs or it completes successfully.
	if err = nc.RunCommand(ctx, t.interactiveCommand); err != nil {
		t.log.WithError(err).Warn("Unable to stream terminal - failure running shell")
		t.writeError(err)
		return
	}

	if err := t.stream.Close(); err != nil {
		t.log.WithError(err).Error("Unable to send close event to web client.")
		return
	}

	t.log.Debug("Sent close event to web client.")
}

// Close is no-op as we never want to close the connection to the client.
// Connection should be closed in the handler when it was created.
func (t *commandHandler) Close() error {
	return nil
}

// makeClient builds a *client.TeleportClient for the connection.
func (t *commandHandler) makeClient(ctx context.Context, ws WSConn) (*client.TeleportClient, error) {
	ctx, span := tracing.DefaultProvider().Tracer("command").Start(ctx, "commandHandler/makeClient")
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

	return tc, nil
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
