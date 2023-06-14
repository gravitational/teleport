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
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
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

// generateAssistantTitleRequest is a request for POST /webapi/assistant/conversations/:conversation_id/generate_title.
type generateAssistantTitleRequest struct {
	Message string `json:"message"`
}

// generateAssistantTitle is a handler for POST /webapi/assistant/conversations/:conversation_id/generate_title.
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

	client, err := assist.NewAssist(r.Context(), h.cfg.ProxyClient,
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

	return conversationInfo, nil
}

func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params,
	sctx *SessionContext, site reversetunnel.RemoteSite,
) (any, error) {
	if err := runAssistant(h, w, r, sctx, site); err != nil {
		h.log.Warn(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func checkAssistEnabled(a auth.ClientI, ctx context.Context) error {
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
	sctx *SessionContext, site reversetunnel.RemoteSite,
) (err error) {
	q := r.URL.Query()
	conversationID := q.Get("conversation_id")
	if conversationID == "" {
		return trace.BadParameter("conversation ID is required")
	}

	authClient, err := sctx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := checkAssistEnabled(authClient, r.Context()); err != nil {
		return trace.Wrap(err)
	}

	identity, err := createIdentityContext(sctx.GetUser(), sctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, err := h.cfg.SessionControl.AcquireSessionContext(r.Context(), identity, h.cfg.ProxyWebAddr.Addr, r.RemoteAddr)
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
		return nil
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
		ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
		return nil
	})

	ws.SetCloseHandler(func(code int, text string) error {
		h.log.Debugf("closing assistant websocket: %v %v", code, text)
		return nil
	})

	go startPingLoop(ctx, ws, keepAliveInterval, h.log, nil)

	assistClient, err := assist.NewAssist(ctx, h.cfg.ProxyClient,
		h.cfg.ProxySettings, h.cfg.OpenAIConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	chat, err := assistClient.NewChat(ctx, authClient, conversationID, sctx.GetUser())
	if err != nil {
		return trace.Wrap(err)
	}

	// onMessageFn is called when a message is received from the OpenAI API.
	onMessageFn := func(kind assist.MessageType, payload []byte, createdTime time.Time) error {
		msg := &assistantMessage{
			Type:        kind,
			Payload:     string(payload),
			CreatedTime: createdTime.Format(time.RFC3339),
		}

		return trace.Wrap(ws.WriteJSON(msg))
	}

	if chat.IsNewConversation() {
		// new conversation, generate a hello message
		if _, err := chat.ProcessComplete(ctx, onMessageFn, ""); err != nil {
			return trace.Wrap(err)
		}
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF || websocket.IsCloseError(err, websocket.CloseAbnormalClosure,
				websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				break
			}
			return trace.Wrap(err)
		}

		var wsIncoming assistantMessage
		if err := json.Unmarshal(payload, &wsIncoming); err != nil {
			return trace.Wrap(err)
		}

		// We can not know how many tokens we will consume in advance.
		// Try to consume a small amount of tokens first.
		const lookaheadTokens = 100
		if !h.assistantLimiter.AllowN(time.Now(), lookaheadTokens) {
			err := onMessageFn(assist.MessageKindError, []byte("You have reached the rate limit. Please try again later."), h.clock.Now().UTC())
			if err != nil {
				return trace.Wrap(err)
			}
			continue
		}

		//TODO(jakule): Should we sanitize the payload?
		usedTokens, err := chat.ProcessComplete(ctx, onMessageFn, wsIncoming.Payload)
		if err != nil {
			return trace.Wrap(err)
		}

		// Once we know how many tokens were consumed for prompt+completion,
		// consume the remaining tokens from the rate limiter bucket.
		extraTokens := usedTokens.Prompt + usedTokens.Completion - lookaheadTokens
		if extraTokens < 0 {
			extraTokens = 0
		}
		h.assistantLimiter.ReserveN(time.Now(), extraTokens)

		usageEventReq := &proto.SubmitUsageEventRequest{
			Event: &usageeventsv1.UsageEventOneOf{
				Event: &usageeventsv1.UsageEventOneOf_AssistCompletion{
					AssistCompletion: &usageeventsv1.AssistCompletionEvent{
						ConversationId:   conversationID,
						TotalTokens:      int64(usedTokens.Prompt + usedTokens.Completion),
						PromptTokens:     int64(usedTokens.Prompt),
						CompletionTokens: int64(usedTokens.Completion),
					},
				},
			},
		}
		if err := authClient.SubmitUsageEvent(r.Context(), usageEventReq); err != nil {
			h.log.WithError(err).Warn("Failed to emit usage event")
		}
	}

	h.log.Debugf("end assistant conversation loop")

	return nil
}
