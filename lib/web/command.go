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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/ai/tokens"
	assistlib "github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/teleagent"
)

// summaryBufferCapacity is the summary buffer size in bytes. The summary buffer
// is shared across all nodes the command is running on and stores the command
// output. If the command output exceeds the buffer capacity, the summary won't
// be computed.
const summaryBufferCapacity = 2000

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

// commandExecResult is a result of a command execution.
type commandExecResult struct {
	// NodeID is the ID of the node where the command was executed.
	NodeID string `json:"node_id"`
	// NodeName is the name of the node where the command was executed.
	NodeName string `json:"node_name"`
	// ExecutionID is a unique ID used to identify the command execution.
	ExecutionID string `json:"execution_id"`
	// SessionID is the ID of the session where the command was executed.
	SessionID string `json:"session_id"`
}

// sessionEndEvent is an event that is sent when a session ends.
type sessionEndEvent struct {
	// NodeID is the ID of the server where the session was created.
	NodeID string `json:"node_id"`
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
	site reversetunnelclient.RemoteSite,
	rawWS *websocket.Conn,
) (any, error) {
	q := r.URL.Query()
	params := q.Get("params")
	if params == "" {
		return nil, trace.BadParameter("missing params")
	}
	var req CommandRequest
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

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), sessionCtx, req.Login, h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
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

	defer func() {
		rawWS.WriteMessage(websocket.CloseMessage, nil)
		rawWS.Close()
	}()

	keepAliveInterval := netConfig.GetKeepAliveInterval()
	err = rawWS.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil, trace.Wrap(err)
	}
	// Update the read deadline upon receiving a pong message.
	rawWS.SetPongHandler(func(_ string) error {
		// This is intentonally called without a lock as this callback is
		// called from the same goroutine as the read loop which is already locked.
		return trace.Wrap(rawWS.SetReadDeadline(deadlineForInterval(keepAliveInterval)))
	})

	// Wrap the raw websocket connection in a syncRWWSConn so that we can
	// safely read and write to the the single websocket connection from
	// multiple goroutines/execution nodes.
	ws := &syncRWWSConn{WSConn: rawWS}

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

	mfaCacheFn := getMFACacheFn()
	interactiveCommand := strings.Split(req.Command, " ")

	buffer := newSummaryBuffer(summaryBufferCapacity)

	runCmd := func(host *hostInfo) error {
		sessionData, err := h.generateCommandSession(host, req.Login, clusterName, sessionCtx.cfg.User)
		if err != nil {
			h.log.WithError(err).Debug("Unable to generate new ssh session.")
			return trace.Wrap(err)
		}

		h.log.Debugf("New command request for server=%s, id=%v, login=%s, sid=%s, websid=%s.",
			host.hostName, host.id, req.Login, sessionData.ID, sessionCtx.GetSessionID())

		commandHandlerConfig := CommandHandlerConfig{
			SessionCtx:         sessionCtx,
			UserAuthClient:     clt,
			SessionData:        sessionData,
			KeepAliveInterval:  keepAliveInterval,
			ProxyHostPort:      h.ProxyHostPort(),
			InteractiveCommand: interactiveCommand,
			Router:             h.cfg.Router,
			TracerProvider:     h.cfg.TracerProvider,
			LocalAccessPoint:   h.auth.accessPoint,
			mfaFuncCache:       mfaCacheFn,
			buffer:             buffer,
			HostNameResolver: func(serverID string) (string, error) {
				return serverID, nil
			},
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

		msgPayload, err := json.Marshal(&commandExecResult{
			NodeID:      host.id,
			NodeName:    host.hostName,
			ExecutionID: req.ExecutionID,
			SessionID:   string(sessionData.ID),
		})

		if err != nil {
			return trace.Wrap(err)
		}

		err = clt.CreateAssistantMessage(ctx, &assist.CreateAssistantMessageRequest{
			ConversationId: req.ConversationID,
			Username:       sessionCtx.GetUser(),
			Message: &assist.AssistantMessage{
				Type:        string(assistlib.MessageKindCommandResult),
				CreatedTime: timestamppb.New(time.Now().UTC()),
				Payload:     string(msgPayload),
			},
		})

		return trace.Wrap(err)
	}

	runCommands(hosts, runCmd, int(netConfig.GetAssistCommandExecutionWorkers()), h.log)

	var tokenCount *tokens.TokenCount
	// Optionally, try to compute the command summary.
	if output, valid := buffer.Export(); valid {
		summaryReq := summaryRequest{
			hosts:          hosts,
			output:         output,
			authClient:     clt,
			username:       sessionCtx.GetUser(),
			executionID:    req.ExecutionID,
			conversationID: req.ConversationID,
			command:        req.Command,
		}
		tokenCount, err = h.computeAndSendSummary(ctx, &summaryReq, ws)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	prompt, completion := 0, 0
	if tokenCount != nil {
		prompt = tokenCount.Prompt.CountAll()
		completion = tokenCount.Completion.CountAll()
	}

	usageEventReq := &clientproto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AssistExecution{
				AssistExecution: &usageeventsv1.AssistExecutionEvent{
					ConversationId:   req.ConversationID,
					NodeCount:        int64(len(hosts)),
					TotalTokens:      int64(completion + prompt),
					PromptTokens:     int64(prompt),
					CompletionTokens: int64(completion),
				},
			},
		},
	}
	if err := clt.SubmitUsageEvent(ctx, usageEventReq); err != nil {
		h.log.WithError(err).Warn("Failed to emit usage event")
	}

	return nil, nil
}

type summaryRequest struct {
	hosts          []hostInfo
	output         map[string][]byte
	authClient     authclient.ClientI
	username       string
	executionID    string
	conversationID string
	command        string
}

func (h *Handler) computeAndSendSummary(
	ctx context.Context,
	req *summaryRequest,
	ws WSConn,
) (*tokens.TokenCount, error) {
	// Convert the map nodeId->output into a map nodeName->output
	namedOutput := outputByName(req.hosts, req.output)

	history, err := req.authClient.GetAssistantMessages(ctx, &assist.GetAssistantMessagesRequest{
		ConversationId: req.conversationID,
		Username:       req.username,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assistClient, err := assistlib.NewClient(ctx, req.authClient, h.cfg.ProxySettings, h.cfg.OpenAIConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	summary, tokenCount, err := assistClient.GenerateCommandSummary(ctx, history.GetMessages(), namedOutput)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Add the summary message to the backend, so it is persisted on chat
	// reload.
	messagePayload, err := json.Marshal(&assistlib.CommandExecSummary{
		ExecutionID: req.executionID,
		Command:     req.command,
		Summary:     summary,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	summaryMessage := &assist.CreateAssistantMessageRequest{
		ConversationId: req.conversationID,
		Username:       req.username,
		Message: &assist.AssistantMessage{
			Type:        string(assistlib.MessageKindCommandResultSummary),
			CreatedTime: timestamppb.New(time.Now().UTC()),
			Payload:     string(messagePayload),
		},
	}

	err = req.authClient.CreateAssistantMessage(ctx, summaryMessage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Send the summary over the execution websocket to provide instant
	// feedback to the user.
	out := &outEnvelope{
		Type:    envelopeTypeSummary,
		Payload: []byte(summary),
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stream := NewWStream(ctx, ws, log, nil)
	_, err = stream.Write(data)
	return tokenCount, trace.Wrap(err)
}

func outputByName(hosts []hostInfo, output map[string][]byte) map[string][]byte {
	hostIDToName := make(map[string]string, len(hosts))
	for _, host := range hosts {
		hostIDToName[host.id] = host.hostName
	}
	namedOutput := make(map[string][]byte, len(output))
	for id, data := range output {
		namedOutput[hostIDToName[id]] = data
	}
	return namedOutput
}

// runCommands runs the given command on the given hosts.
func runCommands(hosts []hostInfo, runCmd func(host *hostInfo) error, numParallel int, log logrus.FieldLogger) {
	var group errgroup.Group
	group.SetLimit(numParallel)

	for _, host := range hosts {
		host := host
		group.Go(func() error {
			return trace.Wrap(runCmd(&host), "failed to start session on %v", host.hostName)
		})
	}

	// Wait for all commands to finish.
	if err := group.Wait(); err != nil {
		log.WithError(err).Debug("Assist command execution failed")
	}
}

// getMFACacheFn returns a function that caches the result of the given
// get function. The cache is protected by a mutex, so it is safe to call
// the returned function from multiple goroutines.
func getMFACacheFn() mfaFuncCache {
	var mutex sync.Mutex
	var authMethods []ssh.AuthMethod

	return func(issueMfaAuthFn func() ([]ssh.AuthMethod, error)) ([]ssh.AuthMethod, error) {
		mutex.Lock()
		defer mutex.Unlock()

		if authMethods != nil {
			return authMethods, nil
		}

		authMethods, err := issueMfaAuthFn()
		return authMethods, trace.Wrap(err)
	}
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
				teleport.ComponentKey: teleport.ComponentWebsocket,
				"session_id":          cfg.SessionData.ID.String(),
			}),
			ctx:                cfg.SessionCtx,
			userAuthClient:     cfg.UserAuthClient,
			sessionData:        cfg.SessionData,
			keepAliveInterval:  cfg.KeepAliveInterval,
			proxyHostPort:      cfg.ProxyHostPort,
			interactiveCommand: cfg.InteractiveCommand,
			router:             cfg.Router,
			localAccessPoint:   cfg.LocalAccessPoint,
			tracer:             cfg.tracer,
			resolver:           cfg.HostNameResolver,
		},
		mfaAuthCache: cfg.mfaFuncCache,
		buffer:       cfg.buffer,
	}, nil
}

// CommandHandlerConfig is the configuration for the command handler.
type CommandHandlerConfig struct {
	// SessionCtx is the context for the user's web session.
	SessionCtx *SessionContext
	// UserAuthClient is used to fetch nodes and sessions from the backend via the users' identity.
	UserAuthClient UserAuthClient
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
	// LocalAccessPoint is the subset of the Proxy cache required to
	// look up information from the local cluster. This should not
	// be used for anything that requires RBAC on behalf of the user.
	// Anything requests that should be made on behalf of the user should
	// use [UserAuthClient].
	LocalAccessPoint localAccessPoint
	// HostNameResolver allows the hostname to be determined from a server UUID
	// so that a friendly name can be displayed in the console tab.
	HostNameResolver func(serverID string) (hostname string, err error)
	// tracer is used to create spans
	tracer oteltrace.Tracer
	// mfaFuncCache is used to cache the MFA auth method
	mfaFuncCache mfaFuncCache
	// buffer shared across multiple commandHandlers that saves the command
	// output in order to generate a summary of the executed commands.
	buffer *summaryBuffer
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

	if t.UserAuthClient == nil {
		return trace.BadParameter("UserAuthClient must be provided")
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

	if t.LocalAccessPoint == nil {
		return trace.BadParameter("localAccessPoint must be provided")
	}

	if t.mfaFuncCache == nil {
		return trace.BadParameter("mfaFuncCache must be provided")
	}

	t.tracer = t.TracerProvider.Tracer("webcommand")

	return nil
}

// mfaFuncCache is a function type that caches the result of a function that
// returns a list of ssh.AuthMethods.
type mfaFuncCache func(func() ([]ssh.AuthMethod, error)) ([]ssh.AuthMethod, error)

// commandHandler is a handler for executing commands on a remote node.
type commandHandler struct {
	sshBaseHandler

	// stream is the websocket stream to the client.
	stream *WSStream

	// ws a raw websocket connection to the client.
	ws WSConn

	// mfaAuthCache is a function that caches the result of a function that
	// returns a list of ssh.AuthMethods. It is used to cache the result of
	// the MFA challenge.
	mfaAuthCache mfaFuncCache

	// buffer shared across multiple commandHandlers that saves the command
	// output in order to generate a summary of the executed commands.
	buffer *summaryBuffer
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
	t.stream = NewWStream(r.Context(), t.ws, t.log, nil)

	// Create a Teleport client, if not able to, show the reason to the user in
	// the terminal.
	tc, err := t.makeClient(r.Context(), t.ws)
	if err != nil {
		t.log.WithError(err).Info("Failed creating a client for session")
		t.writeError(err)
		return
	}

	t.log.Debug("Creating websocket stream")

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

	nc, err := t.connectToHost(ctx, t.ws, tc, t.connectToNodeWithMFA)
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

	if err := t.stream.SendCloseMessage(sessionEndEvent{NodeID: t.sessionData.ServerID}); err != nil {
		t.log.WithError(err).Error("Unable to send close event to web client.")
		return
	}

	t.log.Debug("Sent close event to web client.")
}

// connectToNodeWithMFA attempts to perform the mfa ceremony and then dial the
// host with the retrieved single use certs.
// If called multiple times, the mfa ceremony will only be performed once.
func (t *commandHandler) connectToNodeWithMFA(ctx context.Context, ws WSConn, tc *client.TeleportClient, accessChecker services.AccessChecker, getAgent teleagent.Getter, signer agentless.SignerCreator) (*client.NodeClient, error) {
	authMethods, err := t.mfaAuthCache(func() ([]ssh.AuthMethod, error) {
		// perform mfa ceremony and retrieve new certs
		authMethods, err := t.issueSessionMFACerts(ctx, tc, t.stream)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return authMethods, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return t.connectToNodeWithMFABase(ctx, ws, tc, accessChecker, getAgent, signer, authMethods)
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
	clientConfig.Stdout = newBufferedPayloadWriter(newPayloadWriter(t.sessionData.ServerID, EnvelopeTypeStdout, t.stream), t.buffer)
	clientConfig.Stderr = newBufferedPayloadWriter(newPayloadWriter(t.sessionData.ServerID, envelopeTypeStderr, t.stream), t.buffer)
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
		Type:    envelopeTypeError,
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
