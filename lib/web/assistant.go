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

	"github.com/gorilla/websocket"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sashabaranov/go-openai"

	"github.com/gravitational/teleport/api/defaults"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
)

func (h *Handler) createAssistantConversation(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := &proto.CreateAssistantConversationRequest{}

	resp, err := authClient.CreateAssistantConversation(r.Context(), req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (h *Handler) getAssistantConversationByID(_ http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conversationID := p.ByName("conversation_id")

	resp, err := authClient.GetAssistantMessages(r.Context(), conversationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type response struct {
		Messages []*proto.AssistantMessage
	}

	return &response{
		Messages: resp.Messages,
	}, err
}

func (h *Handler) getAssistantConversations(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext) (any, error) {
	authClient, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := authClient.GetAssistantConversations(r.Context(), &proto.GetAssistantConversationsRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, err
}

func (h *Handler) generateAssistantTitle(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext) (any, error) {
	return nil, trace.NotImplemented("handler is not implemented")
}

func (h *Handler) assistant(w http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext) (any, error) {
	// moved into a separate function for error management/debug purposes
	err := runAssistant(h, w, r, sctx)
	if err != nil {
		h.log.Warn(trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

type commandPayload struct {
	Command string     `json:"command,omitempty"`
	Nodes   []string   `json:"nodes,omitempty"`
	Labels  []ai.Label `json:"labels,omitempty"`
}

// runAssistant upgrades the HTTP connection to a websocket and starts a chat loop.
func runAssistant(h *Handler, w http.ResponseWriter, r *http.Request, sctx *SessionContext) error {
	authClient, err := sctx.GetClient()
	if err != nil {
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

	// Use the default interval as this handler doesn't have access to network config.
	// Note: This time should be longer than OpenAI response time.
	keepAliveInterval := defaults.KeepAliveInterval()
	err = ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
	if err != nil {
		h.log.WithError(err).Error("Error setting websocket readline")
		return nil
	}
	defer ws.Close()

	// Update the read deadline upon receiving a pong message.
	ws.SetPongHandler(func(_ string) error {
		ws.SetReadDeadline(deadlineForInterval(keepAliveInterval))
		return nil
	})

	go startPingLoop(r.Context(), ws, keepAliveInterval, h.log, nil)

	prefs, err := h.cfg.ProxyClient.GetAuthPreference(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	prefsV2, ok := prefs.(*types.AuthPreferenceV2)
	if !ok {
		return trace.Errorf("bad case, expected AuthPreferenceV2 found %T", prefs)
	}

	if prefsV2.Spec.Assist == nil {
		return trace.Errorf("assist spec is not set")
	}

	client := ai.NewClient(prefsV2.Spec.Assist.APIURL)
	chat := client.NewChat(sctx.cfg.User)

	q := r.URL.Query()
	conversationID := q.Get("conversation_id")
	if conversationID == "" {
		return trace.BadParameter("conversation ID is required")
	}

	// existing conversation, retrieve old messages
	messages, err := authClient.GetAssistantMessages(r.Context(), conversationID)
	if err != nil {
		return trace.Wrap(err)
	}

	// restore conversation context.
	for _, msg := range messages.GetMessages() {
		role := kindToRole(msg.Type)
		if role != "" {
			chat.Insert(role, msg.Payload)
		}
	}

	if len(messages.GetMessages()) == 0 {
		// new conversation, generate hello message
		if err := processComplete(h, r.Context(), chat, conversationID, ws, authClient); err != nil {
			return trace.Wrap(err)
		}
	}

	for {
		_, payload, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF || websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				break
			}
			return trace.Wrap(err)
		}

		var wsIncoming proto.AssistantMessage
		if err := json.Unmarshal(payload, &wsIncoming); err != nil {
			return trace.Wrap(err)
		}

		// write user message to both in-memory chain and persistent storage
		chat.Insert(openai.ChatMessageRoleUser, wsIncoming.Payload)

		if _, err := authClient.InsertAssistantMessage(r.Context(), &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           "CHAT_MESSAGE_USER",
			Payload:        wsIncoming.Payload,
			CreatedTime:    h.clock.Now().UTC(),
		}); err != nil {
			return trace.Wrap(err)
		}

		if err := processComplete(h, r.Context(), chat, conversationID, ws, authClient); err != nil {
			return trace.Wrap(err)
		}
	}

	h.log.Debugf("end assistant conversation loop")

	return nil
}

func processComplete(h *Handler, ctx context.Context, chat *ai.Chat, conversationID string,
	ws *websocket.Conn, authClient auth.ClientI,
) error {
	// query the assistant and fetch an answer
	message, err := chat.Complete(ctx, 500)
	if err != nil {
		return trace.Wrap(err)
	}

	switch message := message.(type) {
	case *ai.Message:
		// write assistant message to both in-memory chain and persistent storage
		chat.Insert(message.Role, message.Content)
		protoMsg := &proto.AssistantMessage{
			ConversationId: conversationID,
			Type:           "CHAT_MESSAGE_ASSISTANT",
			Payload:        message.Content,
			CreatedTime:    h.clock.Now().UTC(),
		}
		if _, err := authClient.InsertAssistantMessage(ctx, protoMsg); err != nil {
			return trace.Wrap(err)
		}

		if err := ws.WriteJSON(protoMsg); err != nil {
			return trace.Wrap(err)
		}
	case *ai.CompletionCommand:
		payload := commandPayload{
			Command: message.Command,
			Nodes:   message.Nodes,
			Labels:  message.Labels,
		}

		payloadJson, err := json.Marshal(payload)
		if err != nil {
			return trace.Wrap(err)
		}

		msg := &proto.AssistantMessage{
			Type:           "COMMAND",
			ConversationId: conversationID,
			Payload:        string(payloadJson),
			CreatedTime:    h.clock.Now().UTC(),
		}

		if _, err := authClient.InsertAssistantMessage(ctx, msg); err != nil {
			return trace.Wrap(err)
		}

		if err := ws.WriteJSON(msg); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.Errorf("unknown message type")
	}

	return nil
}

func kindToRole(kind string) string {
	switch kind {
	case "CHAT_MESSAGE_USER":
		return openai.ChatMessageRoleUser
	case "CHAT_MESSAGE_ASSISTANT":
		return openai.ChatMessageRoleAssistant
	case "CHAT_MESSAGE_SYSTEM":
		return openai.ChatMessageRoleSystem
	default:
		return ""
	}
}
