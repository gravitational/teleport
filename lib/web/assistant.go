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
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/lib/ai/model/tools"
	"github.com/gravitational/teleport/lib/ai/tokens"
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

const (
	// actionSSHGenerateCommand is a name of the action for generating SSH commands.
	actionSSHGenerateCommand = "ssh-cmdgen"
	// actionSSHExplainCommand is a name of the action for explaining terminal output in SSH session.
	actionSSHExplainCommand = "ssh-explain"
	// actionGenerateAuditQuery is the name of the action for generating audit queries.
	actionGenerateAuditQuery = "audit-query"
	// We can not know how many tokens we will consume in advance.
	// Try to consume a small amount of tokens first.
	lookaheadTokens = 100
)

// createAssistantConversationResponse is a response for POST /webapi/assistant/conversations.
type createdAssistantConversationResponse struct {
	// ID is a conversation ID.
	ID string `json:"id"`
}

// createAssistantConversation is a handler for POST /webapi/assistant/conversations.
func (h *Handler) createAssistantConversation(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	req := &assistpb.CreateAssistantConversationRequest{
		CreatedTime: timestamppb.New(h.clock.Now().UTC()),
		Username:    sctx.GetUser(),
	}

	resp, err := authClient.CreateAssistantConversation(r.Context(), req)
	if err != nil {
		return nil, err
	}

	return &createdAssistantConversationResponse{
		ID: resp.Id,
	}, nil
}

// deleteAssistantConversation is a handler for DELETE /webapi/assistant/conversations/:conversation_id.
func (h *Handler) deleteAssistantConversation(_ http.ResponseWriter, r *http.Request,
	p httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	conversationID := p.ByName("conversation_id")

	if err := authClient.DeleteAssistantConversation(r.Context(), &assistpb.DeleteAssistantConversationRequest{
		ConversationId: conversationID,
		Username:       sctx.GetUser(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// assistantMessage is an assistant message that is sent to the client.
type assistantMessage struct {
	// Type is a type of the message.
	Type assist.MessageType `json:"type"`
	// CreatedTime is a time when the message was created in RFC3339 format.
	CreatedTime string `json:"created_time"`
	// Payload is a message payload in JSON format.
	Payload string `json:"payload"`
}

// getAssistantConversation is a handler for GET /webapi/assistant/conversations/:conversation_id.
func (h *Handler) getAssistantConversationByID(_ http.ResponseWriter, r *http.Request,
	p httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	conversationID := p.ByName("conversation_id")

	resp, err := authClient.GetAssistantMessages(r.Context(), &assistpb.GetAssistantMessagesRequest{
		ConversationId: conversationID,
		Username:       sctx.GetUser(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conversationResponse(resp), nil
}

// conversationResponse creates a response for GET conversation response.
func conversationResponse(resp *assistpb.GetAssistantMessagesResponse) any {
	type response struct {
		Messages []assistantMessage `json:"messages"`
	}

	jsonResp := &response{
		Messages: make([]assistantMessage, 0, len(resp.Messages)),
	}

	for _, message := range resp.Messages {
		jsonResp.Messages = append(jsonResp.Messages, assistantMessage{
			Type:        assist.MessageType(message.Type),
			CreatedTime: message.CreatedTime.AsTime().Format(time.RFC3339),
			Payload:     message.Payload,
		})
	}

	return jsonResp
}

// conversationInfo is a response for GET conversation response.
type conversationInfo struct {
	// ID is a conversation ID.
	ID string `json:"id"`
	// Title is a conversation title.
	Title string `json:"title,omitempty"`
	// CreatedTime is a time when the conversation was created in RFC3339 format.
	CreatedTime string `json:"created_time"`
}

// conversationsResponse is a response for GET conversation response.
type conversationsResponse struct {
	Conversations []conversationInfo `json:"conversations"`
}

// getAssistantConversations is a handler for GET /webapi/assistant/conversations.
func (h *Handler) getAssistantConversations(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := authClient.GetAssistantConversations(r.Context(), &assistpb.GetAssistantConversationsRequest{
		Username: sctx.GetUser(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return genConversationsResponse(resp), nil
}

func genConversationsResponse(resp *assistpb.GetAssistantConversationsResponse) *conversationsResponse {
	jsonResp := &conversationsResponse{
		Conversations: make([]conversationInfo, 0, len(resp.Conversations)),
	}

	for _, conversation := range resp.Conversations {
		jsonResp.Conversations = append(jsonResp.Conversations, conversationInfo{
			ID:          conversation.Id,
			Title:       conversation.Title,
			CreatedTime: conversation.CreatedTime.AsTime().Format(time.RFC3339),
		})
	}

	return jsonResp
}

// setAssistantTitle is a handler for POST /webapi/assistant/conversations/:conversation_id/title.
func (h *Handler) setAssistantTitle(_ http.ResponseWriter, r *http.Request,
	p httprouter.Params, sctx *SessionContext,
) (any, error) {
	req := struct {
		Title string `json:"title"`
	}{}

	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	conversationID := p.ByName("conversation_id")

	conversationInfo := &assistpb.UpdateAssistantConversationInfoRequest{
		ConversationId: conversationID,
		Username:       sctx.GetUser(),
		Title:          req.Title,
	}

	if err := authClient.UpdateAssistantConversationInfo(r.Context(), conversationInfo); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// generateAssistantTitleRequest is a request for POST /webapi/assistant/title/summary.
type generateAssistantTitleRequest struct {
	Message string `json:"message"`
}

// generateAssistantTitle is a handler for POST /webapi/assistant/title/summary.
func (h *Handler) generateAssistantTitle(_ http.ResponseWriter, r *http.Request,
	_ httprouter.Params, sctx *SessionContext,
) (any, error) {
	var req generateAssistantTitleRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := assist.NewClient(r.Context(), h.cfg.ProxyClient,
		h.cfg.ProxySettings, h.cfg.OpenAIConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	titleSummary, err := client.GenerateSummary(r.Context(), req.Message)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationInfo := &conversationInfo{
		Title: titleSummary,
	}

	// We only want to emmit
	if modules.GetModules().Features().Cloud {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			class, err := client.ClassifyMessage(ctx, req.Message, assist.MessageClasses)
			if err != nil {
				return
			}
			h.log.Debugf("message classified as '%s'", class)
			// TODO(shaka): emit event here to report the message class
			usageEventReq := &proto.SubmitUsageEventRequest{
				Event: &usageeventsv1.UsageEventOneOf{
					Event: &usageeventsv1.UsageEventOneOf_AssistNewConversation{
						AssistNewConversation: &usageeventsv1.AssistNewConversationEvent{
							Category: class,
						},
					},
				},
			}
			if err := authClient.SubmitUsageEvent(ctx, usageEventReq); err != nil {
				h.log.WithError(err).Warn("Failed to emit usage event")
			}
		}()

	}

	return conversationInfo, nil
}

// assistant is a handler for GET /webapi/sites/:site/assistant.
// This handler covers the main chat conversation as well as the
// SSH completition (SSH command generation and output explanation).
func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params,
	sctx *SessionContext, site reversetunnelclient.RemoteSite, ws *websocket.Conn,
) (any, error) {
	if err := runAssistant(h, w, r, sctx, site, ws); err != nil {
		h.log.Warn(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// reserveTokens preemptively reserves tokens in the ratelimiter.
func (h *Handler) reserveTokens(usedTokens *tokens.TokenCount) (int, int) {
	promptTokens, completionTokens := usedTokens.CountAll()

	// Once we know how many tokens were consumed for prompt+completion,
	// consume the remaining tokens from the rate limiter bucket.
	extraTokens := promptTokens + completionTokens - lookaheadTokens
	if extraTokens < 0 {
		extraTokens = 0
	}
	h.assistantLimiter.ReserveN(time.Now(), extraTokens)
	return promptTokens, completionTokens
}

// reportTokenUsage sends a token usage event for a conversation.
func (h *Handler) reportConversationTokenUsage(authClient authclient.ClientI, usedTokens *tokens.TokenCount, conversationID string) {
	// Create a new context to not be bounded by the request timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	promptTokens, completionTokens := h.reserveTokens(usedTokens)
	usageEventReq := &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AssistCompletion{
				AssistCompletion: &usageeventsv1.AssistCompletionEvent{
					ConversationId:   conversationID,
					TotalTokens:      int64(promptTokens + completionTokens),
					PromptTokens:     int64(promptTokens),
					CompletionTokens: int64(completionTokens),
				},
			},
		},
	}

	if err := authClient.SubmitUsageEvent(ctx, usageEventReq); err != nil {
		h.log.WithError(err).Warn("Failed to emit usage event")
	}
}

// reportTokenUsage sends a token usage event for an action.
func (h *Handler) reportActionTokenUsage(authClient authclient.ClientI, usedTokens *tokens.TokenCount, action string) {
	// Create a new context to not be bounded by the request timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	promptTokens, completionTokens := h.reserveTokens(usedTokens)
	usageEventReq := &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AssistAction{
				AssistAction: &usageeventsv1.AssistAction{
					Action:           action,
					TotalTokens:      int64(promptTokens + completionTokens),
					PromptTokens:     int64(promptTokens),
					CompletionTokens: int64(completionTokens),
				},
			},
		},
	}

	if err := authClient.SubmitUsageEvent(ctx, usageEventReq); err != nil {
		h.log.WithError(err).Warn("Failed to emit usage event")
	}
}

func checkAssistEnabled(a authclient.ClientI, ctx context.Context) error {
	enabled, err := a.IsAssistEnabled(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if !enabled.Enabled {
		return trace.AccessDenied("Assist is not enabled")
	}

	return nil
}

// runAssistant upgrades the HTTP connection to a websocket and starts a chat loop.
func runAssistant(h *Handler, w http.ResponseWriter, r *http.Request,
	sctx *SessionContext, site reversetunnelclient.RemoteSite, ws *websocket.Conn,
) (err error) {
	q := r.URL.Query()
	conversationID := q.Get("conversation_id")
	actionParam := r.URL.Query().Get("action")
	if conversationID == "" && actionParam == "" {
		return trace.BadParameter("conversation ID or action is required")
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), sctx, sctx.GetUser(), h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	authAccessPoint, err := site.CachingAccessPoint()
	if err != nil {
		h.log.WithError(err).Debug("Unable to get auth access point.")
		return trace.Wrap(err)
	}

	netConfig, err := authAccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		h.log.WithError(err).Debug("Unable to fetch cluster networking config.")
		return trace.Wrap(err)
	}

	// Note: This time should be longer than OpenAI response time.
	keepAliveInterval := netConfig.GetKeepAliveInterval()
	err = ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil
	}
	defer func() {
		closureReason := websocket.CloseNormalClosure
		closureMsg := ""
		if err != nil {
			h.log.WithError(err).Error("Error in the Assistant loop")
			_ = ws.WriteJSON(&assistantMessage{
				Type:        assist.MessageKindError,
				Payload:     "An error has occurred. Please try again later.",
				CreatedTime: h.clock.Now().UTC().Format(time.RFC3339),
			})
			// Set server error code and message: https://datatracker.ietf.org/doc/html/rfc6455#section-7.4.1
			closureReason = websocket.CloseInternalServerErr
			closureMsg = err.Error()
		}
		// Send the close message to the client and close the connection
		if err := ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(closureReason, closureMsg),
			time.Now().Add(time.Second),
		); err != nil {
			h.log.Warnf("Failed to write close message: %v", err)
		}
		if err := ws.Close(); err != nil {
			h.log.Warnf("Failed to close websocket: %v", err)
		}
	}()

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		return trace.Wrap(ws.SetReadDeadline(deadlineForInterval(keepAliveInterval)))
	})

	ws.SetCloseHandler(func(code int, text string) error {
		h.log.Debugf("closing assistant websocket: %v %v", code, text)
		return nil
	})

	go startWSPingLoop(ctx, ws, keepAliveInterval, h.log, nil)

	assistClient, err := assist.NewClient(ctx, h.cfg.ProxyClient,
		h.cfg.ProxySettings, h.cfg.OpenAIConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	switch r.URL.Query().Get("action") {
	case actionSSHGenerateCommand:
		err = h.assistGenSSHCommandLoop(ctx, assistClient, authClient, ws, sctx.GetUser())
	case actionSSHExplainCommand:
		err = h.assistSSHExplainOutputLoop(ctx, assistClient, authClient, ws)
	case actionGenerateAuditQuery:
		err = h.assistGenAuditQueryLoop(ctx, assistClient, authClient, ws, sctx.GetUser())
	default:
		err = h.assistChatLoop(ctx, assistClient, authClient, conversationID, sctx, ws)
	}

	return trace.Wrap(err)
}

// assistGenAuditQueryLoop reads the user's input and generates an audit query.
func (h *Handler) assistGenAuditQueryLoop(ctx context.Context, assistClient *assist.Assist, authClient authclient.ClientI, ws *websocket.Conn, username string) error {
	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if wsIsClosed(err) {
				break
			}
			return trace.Wrap(err)
		}

		onMessage := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
			return onMessageFn(ws, kind, payload, createdTime)
		}

		toolCtx := &tools.ToolContext{User: username}

		if err := h.preliminaryRateLimitGuard(onMessage); err != nil {
			return trace.Wrap(err)
		}

		tokenCount, err := assistClient.RunTool(ctx, onMessage, tools.AuditQueryGenerationToolName, string(payload), toolCtx)
		if err != nil {
			return trace.Wrap(err)
		}

		go h.reportActionTokenUsage(authClient, tokenCount, tools.AuditQueryGenerationToolName)
	}
	return nil
}

// assistSSHExplainOutputLoop reads the user's input and generates a command summary.
func (h *Handler) assistSSHExplainOutputLoop(ctx context.Context, assistClient *assist.Assist, authClient authclient.ClientI, ws *websocket.Conn) error {
	_, payload, err := ws.ReadMessage()
	if err != nil {
		if wsIsClosed(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	modelMessages := []*assistpb.AssistantMessage{
		{
			Type:    string(assist.MessageKindUserMessage),
			Payload: string(payload),
		},
	}

	onMessage := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
		return onMessageFn(ws, kind, payload, createdTime)
	}

	if err := h.preliminaryRateLimitGuard(onMessage); err != nil {
		return trace.Wrap(err)
	}

	summary, tokenCount, err := assistClient.GenerateCommandSummary(ctx, modelMessages, map[string][]byte{})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := onMessageFn(ws, assist.MessageKindAssistantMessage, []byte(summary), h.clock.Now().UTC()); err != nil {
		return trace.Wrap(err)
	}

	go h.reportActionTokenUsage(authClient, tokenCount, "SSH Explain")
	return nil
}

// assistSSHCommandLoop reads the user's input and generates a Linux command.
func (h *Handler) assistGenSSHCommandLoop(ctx context.Context, assistClient *assist.Assist, authClient authclient.ClientI, ws *websocket.Conn, username string) error {
	chat, err := assistClient.NewLightweightChat(username)
	if err != nil {
		return trace.Wrap(err)
	}

	onMessage := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
		return trace.Wrap(onMessageFn(ws, kind, payload, createdTime))
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if wsIsClosed(err) {
				break
			}
			return trace.Wrap(err)
		}

		if err := h.preliminaryRateLimitGuard(onMessage); err != nil {
			return trace.Wrap(err)
		}

		tokenCount, err := chat.ProcessComplete(ctx, func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
			return onMessageFn(ws, kind, payload, createdTime)
		}, string(payload))
		if err != nil {
			return trace.Wrap(err)
		}

		tool := tools.CommandExecutionTool{}
		go h.reportActionTokenUsage(authClient, tokenCount, tool.Name())
	}
	return nil
}

// assistChatLoop is the main chat loop for the assistant.
// It reads the user's input from provided WS and generates a response.
func (h *Handler) assistChatLoop(ctx context.Context, assistClient *assist.Assist, authClient authclient.ClientI,
	conversationID string, sctx *SessionContext, ws *websocket.Conn,
) error {
	ac, err := sctx.GetUserAccessChecker()
	if err != nil {
		return trace.Wrap(err)
	}

	toolContext := &tools.ToolContext{
		AssistEmbeddingServiceClient: authClient.EmbeddingClient(),
		AccessRequestClient:          authClient,
		AccessChecker:                ac,
		NodeWatcher:                  h.nodeWatcher,
		ClusterName:                  sctx.cfg.Parent.clusterName,
		User:                         sctx.GetUser(),
	}

	chat, err := assistClient.NewChat(ctx, authClient, toolContext, conversationID)
	if err != nil {
		return trace.Wrap(err)
	}

	onMessage := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
		return trace.Wrap(onMessageFn(ws, kind, payload, createdTime))
	}

	if chat.IsNewConversation() {
		// new conversation, generate a hello message
		if _, err := chat.ProcessComplete(ctx, onMessage, ""); err != nil {
			return trace.Wrap(err)
		}
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if wsIsClosed(err) {
				break
			}
			return trace.Wrap(err)
		}

		var wsIncoming assistantMessage
		if err := json.Unmarshal(payload, &wsIncoming); err != nil {
			return trace.Wrap(err)
		}

		if wsIncoming.Type == assist.MessageKindAccessRequestCreated {
			chat.RecordMesssage(ctx, wsIncoming.Type, wsIncoming.Payload)
		}

		if err := h.preliminaryRateLimitGuard(onMessage); err != nil {
			return trace.Wrap(err)
		}

		//TODO(jakule): Should we sanitize the payload?
		usedTokens, err := chat.ProcessComplete(ctx, onMessage, wsIncoming.Payload)
		if err != nil {
			return trace.Wrap(err)
		}

		// Token usage reporting is asynchronous as we might still be streaming
		// a message, and we don't want to block everything.
		go h.reportConversationTokenUsage(authClient, usedTokens, conversationID)
	}

	h.log.Debug("end assistant conversation loop")
	return nil
}

// preliminaryRateLimitGuard checks that some small amount of tokens are still available and the ratelimit is not exceeded.
// This is done because the changed quantity within the limiter is not known until after a request is processed.
func (h *Handler) preliminaryRateLimitGuard(onMessageFn func(kind assist.MessageType, payload []byte, createdTime time.Time) error) error {
	const errorMsg = "You have reached the rate limit. Please try again later."

	if !h.assistantLimiter.AllowN(time.Now(), lookaheadTokens) {
		err := onMessageFn(assist.MessageKindError, []byte(errorMsg), h.clock.Now().UTC())
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.LimitExceeded(errorMsg)
	}

	return nil
}

// wsIsClosed returns true if the error is caused by a closed websocket.
func wsIsClosed(err error) bool {
	return err == io.EOF || websocket.IsCloseError(err, websocket.CloseAbnormalClosure,
		websocket.CloseGoingAway, websocket.CloseNormalClosure)
}

// onMessageFn is a helper function used to send an assist message to the frontend.
// It deals with serializing the kind and payload into a wire and sending it over with the correct
// websocket frame type.
func onMessageFn(ws *websocket.Conn, kind assist.MessageType, payload []byte, createdTime time.Time) error {
	msg := &assistantMessage{
		Type:        kind,
		Payload:     string(payload),
		CreatedTime: createdTime.Format(time.RFC3339),
	}

	return trace.Wrap(ws.WriteJSON(msg))
}
